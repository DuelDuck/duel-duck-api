package service

import (
	"context"
	"errors"
	"math"
	"slices"
	"sort"
	"time"

	"github.com/gagliardetto/solana-go"
	associatedtokenaccount "github.com/gagliardetto/solana-go/programs/associated-token-account"
	computebudget "github.com/gagliardetto/solana-go/programs/compute-budget"
	"github.com/gagliardetto/solana-go/programs/system"
	"github.com/gagliardetto/solana-go/programs/token"
	"github.com/gagliardetto/solana-go/rpc"
	"github.com/google/uuid"
	"gitlab.com/duel-duck/duel-duck-api/internal/client/jupiter"
	"gitlab.com/duel-duck/duel-duck-api/internal/model"
	"gitlab.com/duel-duck/duel-duck-api/pkg/apperrors"
	"go.uber.org/zap"
)

const (
	// RequiredSolInUSD represents the required SOL amount in USD for autoswap (0.5 USD)
	// Stored as raw amount (0.5 * USDCRawAmountMultiplierFloat = 500,000) for consistency with USDC amounts
	RequiredSolInUSD = 0.5 * USDCRawAmountMultiplierFloat
)

func (s *WalletService) Autoswap(
	ctx context.Context,
	params *AutoswapParams,
) (*model.ComprehensiveAutoswapResult, error) {
	result := &model.ComprehensiveAutoswapResult{}

	publicKey := params.PrivateKey.PublicKey()

	// Get all available tokens for autoswap first
	swappable, err := s.findATAsAvailableForSwap(ctx, params.UserID, publicKey)
	if err != nil {
		return nil, err
	}

	// Early exit if no tokens available
	if len(swappable) == 0 {
		return nil, apperrors.BadRequest("no tokens available for autoswap")
	}

	// Prepare token addresses for price lookup (including SOL)
	tokenAddresses := make([]string, 0, len(swappable)+1)
	for _, t := range swappable {
		tokenAddresses = append(tokenAddresses, t.TokenAddress)
	}
	// Always add SOL address for price lookup - we need SOL price for balance checks
	// regardless of whether user has allowed SOL for swapping
	tokenAddresses = append(tokenAddresses, solana.SolMint.String())

	// Get prices for all tokens (including SOL) in one request
	prices, err := s.Jupiter.Price(ctx, tokenAddresses...)
	if err != nil {
		return nil, err
	}

	// Get SOL price from provided prices
	// SOL should always be included in prices as we add it to tokenAddresses in Autoswap
	solPrice, ok := prices[solana.SolMint.String()]
	if !ok || solPrice.UsdPrice <= 0 {
		return nil, apperrors.ServiceUnavailable("failed to get valid SOL price from jupiter; SOL should always be included", err)
	}

	var usdcNeeded = false
	var missingUSDC uint64 = 0
	if params.USDCBalance < params.RequiredUSDC {
		missingUSDC = uint64(float64(params.RequiredUSDC-params.USDCBalance) * 1.1)
		usdcNeeded = true
	}

	requiredSolRawAmount := uint64(RequiredSolInUSD * TokenMultiplierByDecimals(SolMintDecimals) / USDCRawAmountMultiplierFloat / solPrice.UsdPrice)
	solNeeded := isSolAutoswapNeeded(params.SolBalance, requiredSolRawAmount)

	// Early exit if no autoswap needed
	if !solNeeded && !usdcNeeded {
		return result, nil
	}

	// Get tokens with values using the prices we already have
	tokensWithValue := s.getSwappableATAsOrderByUSDValue(swappable, prices, params, usdcNeeded)
	if len(tokensWithValue) == 0 {
		return nil, apperrors.BadRequest("no tokens available for autoswap")
	}

	// Pre-validation: check if user has enough tokens for both autoswaps
	if solNeeded && usdcNeeded {
		zap.L().Info("[0] validation both swaps")
		ok = canAffordBothAutoswaps(tokensWithValue, uint64(RequiredSolInUSD), missingUSDC)
		if !ok {
			return nil, apperrors.BadRequest("insufficient tokens for both SOL and USDC autoswap")
		}
		zap.L().Info("[1] validation finished")
	}

	if solNeeded {
		zap.L().Info("[2] sol autoswap started")
		theMostValuableATA := &tokensWithValue[0]
		tokenForSol, err := prepareTokensForSolSwap(theMostValuableATA, RequiredSolInUSD, usdcNeeded)
		if err != nil {
			return nil, err
		}

		if tokenForSol == nil {
			return nil, apperrors.BadRequest("no suitable token found for SOL autoswap")
		}

		solResult, err := s.performSolAutoswap(ctx, params.PrivateKey, s.adminPrivateKey, tokenForSol, solPrice.UsdPrice)
		if err != nil {
			return nil, err
		}

		result.SolAutoswapResult = solResult
		result.SolAutoswapNeeded = true

		zap.L().Info("[3] sol autoswap waiting for balance update")

		ok, err = s.waitForSolBalanceUpdate(ctx, publicKey, requiredSolRawAmount, 60*time.Second, 3*time.Second)
		if err != nil {
			return nil, err
		}

		if !ok {
			return nil, apperrors.BadRequest("not enough sol (timeout)")
		}

		zap.L().Info("[4] sol autoswap finished")
	}

	if usdcNeeded {
		zap.L().Info("[5] usdc autoswap started")
		tokensToSwap, err := s.prepareTokensForSwap(tokensWithValue, missingUSDC)
		if err != nil {
			return nil, err
		}

		usdcResults, err := s.performUSDCAutoswap(ctx, params.UserID, params.PrivateKey, tokensToSwap)
		if err != nil {
			return nil, err
		}

		usdcATA, _, err := solana.FindAssociatedTokenAddress(publicKey, USDCMintAddress)
		if err != nil {
			return nil, apperrors.BadRequest("failed to find associated token address", err)
		}

		zap.L().Info("[6] usdc autoswap waiting for balance update")
		ok, err := s.waitForEnoughTokenBalance(ctx, usdcATA, params.RequiredUSDC, 60*time.Second, 3*time.Second)
		if err != nil {
			return nil, err
		}

		if !ok {
			return nil, apperrors.BadRequest("not enough token balance (timeout)")
		}

		result.USDCAutoswapResults = usdcResults
		result.USDCAutoswapNeeded = true
		zap.L().Info("[7] usdc autoswap finished")
	}

	return result, nil
}

// intersectTokenAccountsWithMints returns the subset of TokenAccounts whose TokenAddress is in the swappable list.
// It preserves the Amount and other fields from TokenAccount.
func intersectTokenAccountsWithMints(accounts []model.TokenAccount, swappable []string) []model.TokenAccount {
	if len(accounts) == 0 || len(swappable) == 0 {
		return nil
	}
	set := make(map[string]struct{}, len(swappable))
	for _, mint := range swappable {
		set[mint] = struct{}{}
	}

	var result []model.TokenAccount
	for _, acc := range accounts {
		if _, ok := set[acc.TokenAddress]; ok {
			result = append(result, acc)
		}
	}

	return result
}

// getAvailableSolSwapAmount handles special logic for SOL token:
// - If SOL value is less than $1, returns nil (not swappable)
// - If SOL value is >= $1, adjusts amount to leave at least $1 worth of SOL
func (s *WalletService) getAvailableSolSwapAmount(tokenAcc model.TokenAccount, price float64) (int64, bool) {
	if tokenAcc.TokenAddress != solana.SolMint.String() {
		return 0, false
	}

	tokenRawAmount := float64(tokenAcc.Amount)
	tokenDecimals := tokenAcc.TokenDecimals

	// Calculate total SOL USD value
	oneUSDCRaw := TokenMultiplierByDecimals(USDCMintDecimals)
	usdRawPrice := price * oneUSDCRaw
	totalSolUsdRaw := usdRawPrice * (tokenRawAmount / TokenMultiplierByDecimals(tokenDecimals))

	if totalSolUsdRaw < oneUSDCRaw {
		// SOL value less than $1, not swappable
		return 0, false
	}

	// Calculate $1 worth of SOL in raw units
	oneUsdInSol := 1.0 * TokenMultiplierByDecimals(tokenDecimals) / price

	// Adjust amount to leave at least $1 worth of SOL
	swappableAmount := tokenRawAmount - oneUsdInSol
	if swappableAmount <= 0 {
		return 0, false
	}

	return int64(swappableAmount), true
}

func (s *WalletService) findATAsAvailableForSwap(
	ctx context.Context,
	userID uuid.UUID,
	publicKey solana.PublicKey,
) ([]model.TokenAccount, error) {
	swappable, err := s.WalletTokenRepository.MintsWithAutoswap(ctx, userID)
	if err != nil {
		return nil, apperrors.Internal("failed to get user auto—swappable tokens", err)
	}

	if len(swappable) == 0 {
		return nil, nil
	}

	solSwappable := IsAllowedToSwapSol(swappable)
	req := &model.GetTokenAccountsReq{HideZero: true}
	tokens, err := s.GetTokenAccounts(ctx, publicKey.String(), req)
	if err != nil {
		return nil, err
	}

	// Find intersection of swappable tokens and user's tokens allowed to be swapped,
	// preserving Amount and other TokenAccount fields.
	intersection := intersectTokenAccountsWithMints(tokens.Data, swappable)
	if solSwappable {
		intersection, err = s.addSolToken(ctx, publicKey, intersection)
		if err != nil {
			return nil, err
		}
	}

	if len(intersection) == 0 {
		return nil, nil
	}

	return intersection, nil
}

func IsAllowedToSwapSol(mints []string) bool {
	return slices.Contains(mints, solana.SolMint.String())
}

func (s *WalletService) addSolToken(
	ctx context.Context,
	pubKey solana.PublicKey,
	tokensSwapAvailable []model.TokenAccount,
) ([]model.TokenAccount, error) {
	balance, err := s.GetSolBalance(ctx, pubKey)
	if err != nil {
		return tokensSwapAvailable, err
	}

	solForSwap := model.TokenAccount{
		TokenAddress:  solana.SolMint.String(),
		TokenDecimals: SolMintDecimals,
		Amount:        int64(balance),
	}
	tokensSwapAvailable = append(tokensSwapAvailable, solForSwap)

	return tokensSwapAvailable, nil
}

func TokenMultiplierByDecimals(decimals uint8) float64 {
	return math.Pow10(int(decimals))
}

func (s *WalletService) waitForEnoughTokenBalance(
	ctx context.Context,
	ata solana.PublicKey,
	requiredAmount uint64,
	timeout time.Duration,
	pollInterval time.Duration,
) (bool, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	for {
		hasEnough, err := s.HasEnoughTokenBalanceFinalized(ctx, ata, requiredAmount)
		if err != nil {
			return false, err
		}
		if hasEnough {
			return true, nil
		}

		select {
		case <-ctx.Done():
			return false, apperrors.BadRequest("not enough balance to proceed a transaction (timed out waiting for balance update)")
		case <-time.After(pollInterval):
		}
	}
}

func (s *WalletService) getSwappableATAsOrderByUSDValue(
	swappable []model.TokenAccount,
	prices map[string]jupiter.GetPriceResp,
	params *AutoswapParams,
	usdcNeeded bool,
) []model.TokenWithValue {
	tokensWithValue := make([]model.TokenWithValue, 0, len(swappable))
	for _, tokenAcc := range swappable {
		price, ok := prices[tokenAcc.TokenAddress]
		if !ok || price.UsdPrice <= 0 {
			continue
		}

		availableSwapAmount := tokenAcc.Amount

		if tokenAcc.TokenAddress == USDCMintAddress.String() {
			if usdcNeeded {
				continue
			}
			availableSwapAmount = tokenAcc.Amount - int64(params.RequiredUSDC)

		} else if tokenAcc.TokenAddress == solana.SolMint.String() {
			availableSwapAmount, ok = s.getAvailableSolSwapAmount(tokenAcc, price.UsdPrice)
			if !ok {
				continue
			}
		}

		tokenRawAmount := float64(availableSwapAmount)
		usdcPriceRaw := price.UsdPrice * TokenMultiplierByDecimals(USDCMintDecimals)
		usdValue := usdcPriceRaw * (tokenRawAmount / TokenMultiplierByDecimals(tokenAcc.TokenDecimals))

		tokenWithValue := model.TokenWithValue{
			TokenAccount:        tokenAcc.TokenAccount,
			TokenAddress:        tokenAcc.TokenAddress,
			Amount:              tokenAcc.Amount,
			AvailableSwapAmount: availableSwapAmount,
			TokenDecimals:       tokenAcc.TokenDecimals,
			USDPrice:            price.UsdPrice,
			USDValue:            uint64(usdValue),
		}
		tokensWithValue = append(tokensWithValue, tokenWithValue)
	}

	sort.Slice(tokensWithValue, func(i, j int) bool {
		return tokensWithValue[i].USDValue > tokensWithValue[j].USDValue
	})

	return tokensWithValue
}

func (s *WalletService) prepareTokensForSwap(tokensWithValue []model.TokenWithValue, targetUSDC uint64) ([]model.AutoswapResult, error) {
	usdcAccumulated := uint64(0)
	tokensToSwap := make([]model.AutoswapResult, 0, len(tokensWithValue))

	for _, tokenAcc := range tokensWithValue {
		usdPrice := tokenAcc.USDPrice
		tokenAmount := float64(tokenAcc.Amount)
		tokenDecimals := tokenAcc.TokenDecimals
		usdcDecimalsMultiplier := TokenMultiplierByDecimals(USDCMintDecimals)
		usdcValue := tokenAmount * usdPrice * usdcDecimalsMultiplier / TokenMultiplierByDecimals(tokenDecimals)

		usdcNeeded := float64(targetUSDC - usdcAccumulated)
		if usdcAccumulated >= targetUSDC {
			break
		}

		if usdcValue >= usdcNeeded {
			amountToSwap := usdcNeeded * TokenMultiplierByDecimals(tokenDecimals) / (usdPrice * usdcDecimalsMultiplier)
			autoswapResult := model.AutoswapResultFromTokenWithValue(tokenAcc, uint64(amountToSwap))
			tokensToSwap = append(tokensToSwap, autoswapResult)
			usdcAccumulated += uint64(usdcNeeded)
			break
		} else {
			autoswapResult := model.AutoswapResultFromTokenWithValue(tokenAcc, uint64(tokenAcc.Amount))
			tokensToSwap = append(tokensToSwap, autoswapResult)
			usdcAccumulated += uint64(usdcValue)
		}
	}

	if targetUSDC > usdcAccumulated {
		return nil, apperrors.BadRequest("not enough tokens for usdc autoswap")
	}

	return tokensToSwap, nil
}

// prepareTokensForSolSwap finds tokens worth at least minValueUSD for SOL autoswap
func prepareTokensForSolSwap(
	tokenForSwap *model.TokenWithValue,
	requiredSolUSD float64,
	usdcNeeded bool,
) (*model.TokenWithValue, error) {
	if usdcNeeded && tokenForSwap.TokenAddress == USDCMintAddress.String() {
		return nil, apperrors.BadRequest("insufficient balance: not enough usdc for sol autoswap")
	}

	tokenValueUSD := float64(tokenForSwap.USDValue)

	if tokenValueUSD < requiredSolUSD {
		return nil, apperrors.BadRequest("insufficient balance: not enough tokens for sol autoswap")
	}

	// Calculate how much of this token we need to swap for SOL
	// Add buffer to ensure admin doesn't go negative
	targetTokenValueUSD := requiredSolUSD * 1.05

	// Calculate token amount to swap (in raw units)
	// targetTokenValueUSD is in USD, we need to convert to token raw units
	tokenAmountToSwap := int64(targetTokenValueUSD / tokenForSwap.USDPrice)
	if tokenAmountToSwap > tokenForSwap.AvailableSwapAmount {
		return nil, apperrors.BadRequest("insufficient balance: not enough tokens for sol autoswap")
	}

	// Create a copy of the token with the calculated swap amount
	tokenForSol := *tokenForSwap
	tokenForSol.AvailableSwapAmount = tokenAmountToSwap

	tokenForSwap.Amount = tokenForSwap.Amount - tokenAmountToSwap
	tokenForSwap.AvailableSwapAmount = tokenForSwap.AvailableSwapAmount - tokenAmountToSwap
	tokenForSwap.USDValue = uint64(float64(tokenForSwap.Amount) * tokenForSwap.USDPrice / TokenMultiplierByDecimals(tokenForSwap.TokenDecimals) * USDCRawAmountMultiplierFloat)

	return &tokenForSol, nil

}

func removeUsedTokenAmount(
	tokens []model.TokenWithValue,
	usedToken model.TokenWithValue,
) []model.TokenWithValue {
	var updatedTokens []model.TokenWithValue

	for _, t := range tokens {
		if t.TokenAddress == usedToken.TokenAddress {
			remainingAmount := min(0, t.AvailableSwapAmount-usedToken.AvailableSwapAmount)

			t.AvailableSwapAmount = remainingAmount
			t.Amount = remainingAmount
			t.USDValue = uint64(float64(remainingAmount) * t.USDPrice / TokenMultiplierByDecimals(t.TokenDecimals) * USDCRawAmountMultiplierFloat)

			updatedTokens = append(updatedTokens, t)
		}
	}

	return updatedTokens
}

// canAffordBothAutoswaps checks if user has enough tokens for both SOL and USDC autoswap
func canAffordBothAutoswaps(
	tokens []model.TokenWithValue,
	solNeededUSD uint64,
	usdcNeeded uint64,
) bool {
	var totalValueUSD uint64
	for _, t := range tokens {
		totalValueUSD += t.USDValue
	}

	requiredTotalUSD := solNeededUSD + usdcNeeded

	return totalValueUSD >= requiredTotalUSD
}

func isSolAutoswapNeeded(
	solBalance uint64,
	requiredSolRawAmount uint64,
) bool {
	if solBalance == 0 {
		return true
	}

	if solBalance < requiredSolRawAmount {
		return true
	}

	return false
}

// performSolAutoswap executes SOL autoswap: admin sends SOL to user, user sends token to admin
func (s *WalletService) performSolAutoswap(
	ctx context.Context,
	userPrivateKey solana.PrivateKey,
	adminPrivateKey solana.PrivateKey,
	tokenForSol *model.TokenWithValue,
	solUSDPrice float64,
) (*model.SolAutoswapResult, error) {
	// Calculate SOL amount to send (required USD worth)
	const solAmountUSD = RequiredSolInUSD / USDCRawAmountMultiplierFloat

	// Calculate SOL amount in raw units
	solAmountRaw := uint64(solAmountUSD * TokenMultiplierByDecimals(SolMintDecimals) / solUSDPrice)

	// Check if admin has enough SOL
	adminSolBalance, err := s.GetSolBalance(ctx, adminPrivateKey.PublicKey())
	if err != nil {
		return nil, apperrors.Internal("failed to get admin SOL balance", err)
	}

	if adminSolBalance < solAmountRaw {
		return nil, apperrors.Internal("admin has insufficient SOL for autoswap", err)
	}

	// Create transaction instructions
	instructions := make([]solana.Instruction, 0, 4)

	// 1. Transfer SOL from admin to user
	solTransferInstruction := system.NewTransferInstruction(
		solAmountRaw,
		adminPrivateKey.PublicKey(),
		userPrivateKey.PublicKey(),
	).Build()
	instructions = append(instructions, solTransferInstruction)

	// 2. Transfer token from user to admin
	// First, ensure admin has ATA for the token
	adminTokenMint, err := solana.PublicKeyFromBase58(tokenForSol.TokenAddress)
	if err != nil {
		return nil, apperrors.Internal("invalid token mint address", err)
	}

	adminTokenATA, _, err := solana.FindAssociatedTokenAddress(adminPrivateKey.PublicKey(), adminTokenMint)
	if err != nil {
		return nil, apperrors.Internal("failed to get admin token ATA", err)
	}

	// Check if admin ATA exists, if not create it
	adminATAInfo, err := s.SolanaRPC.GetAccountInfo(ctx, adminTokenATA)
	if err != nil && !errors.Is(err, rpc.ErrNotFound) {
		return nil, apperrors.Internal("failed to check admin token ATA", err)
	}

	var fallbackCULimit uint32 = 10_000
	if adminATAInfo == nil || adminATAInfo.Value == nil || adminATAInfo.Value.Owner == ZeroValuePublicKey {
		createATAInstruction, err := associatedtokenaccount.NewCreateInstruction(
			adminPrivateKey.PublicKey(),
			adminPrivateKey.PublicKey(),
			adminTokenMint,
		).ValidateAndBuild()
		if err != nil {
			return nil, apperrors.Internal("failed to build admin ATA creation instruction", err)
		}

		instructions = append(instructions, createATAInstruction)
		fallbackCULimit = 50_000
	}

	// Transfer token from user to admin
	userTokenATA, _, err := solana.FindAssociatedTokenAddress(userPrivateKey.PublicKey(), adminTokenMint)
	if err != nil {
		return nil, apperrors.Internal("failed to get user token ATA", err)
	}

	tokenTransferInstruction, err := token.NewTransferCheckedInstruction(
		uint64(tokenForSol.AvailableSwapAmount),
		tokenForSol.TokenDecimals,
		userTokenATA,
		adminTokenMint,
		adminTokenATA,
		userPrivateKey.PublicKey(),
		[]solana.PublicKey{userPrivateKey.PublicKey()},
	).ValidateAndBuild()
	if err != nil {
		return nil, apperrors.Internal("failed to build token transfer instruction", err)
	}
	instructions = append(instructions, tokenTransferInstruction)

	tx, err := s.NewTransactionForSimulation(
		instructions,
		txTwoSignersPrivateKeyGetter(adminPrivateKey, userPrivateKey),
		solana.TransactionPayer(adminPrivateKey.PublicKey()),
	)
	if err != nil {
		return nil, apperrors.Internal("failed to create transaction for simulation", err)
	}

	computeUnits, err := s.GetSimulationComputeUnits(ctx, tx)
	if err != nil || computeUnits == 0 {
		computeUnits = fallbackCULimit
	}

	computeUnits = uint32(float64(computeUnits) * CUExtraCapacityCoefficient)

	instructions = append(
		[]solana.Instruction{
			computebudget.NewSetComputeUnitPriceInstructionBuilder().
				SetMicroLamports(s.PriorityTracker.GetMediumPriorityMicroLamports()).
				Build(),
			computebudget.NewSetComputeUnitLimitInstructionBuilder().
				SetUnits(computeUnits).
				Build(),
		},
		instructions...,
	)

	// Create final transaction
	tx, err = solana.NewTransaction(
		instructions,
		solana.Hash{},
		solana.TransactionPayer(adminPrivateKey.PublicKey()),
	)
	if err != nil {
		return nil, apperrors.Internal("failed to create final transaction", err)
	}

	txHash, err := s.sendTxWithTracker(
		ctx,
		tx,
		txTwoSignersPrivateKeyGetter(adminPrivateKey, userPrivateKey),
	)
	if err != nil {
		return nil, apperrors.Internal("failed to send SOL autoswap transaction", err)
	}

	tokenValueUSD := float64(tokenForSol.USDValue) / USDCRawAmountMultiplierFloat

	return &model.SolAutoswapResult{
		TxHash:        txHash.String(),
		TokenMint:     tokenForSol.TokenAddress,
		TokenAmount:   uint64(tokenForSol.Amount),
		SolAmount:     solAmountRaw,
		TokenValueUSD: tokenValueUSD,
	}, nil
}

// performUSDCAutoswap executes USDC autoswap using Jupiter
func (s *WalletService) performUSDCAutoswap(
	ctx context.Context,
	userID uuid.UUID,
	privateKey solana.PrivateKey,
	tokensToSwap []model.AutoswapResult,
) ([]model.AutoswapResult, error) {
	for i, t := range tokensToSwap {
		err := s.TxNotificationStorage.Save(ctx, userID, model.NewTxNotificationStatusPending())
		if err != nil {
			zap.L().Error("failed to save user pending tx notification", zap.Error(err))
		}

		txHash, err := s.swap(
			ctx,
			privateKey,
			t.Mint,
			USDCMintAddress.String(),
			t.SwapAmount,
			int64(s.SwapCommissionCoefficient*CoefficientToBPSRelation),
		)
		if err != nil {
			zap.L().Error("failed usdc autoswap", zap.Error(err))

			notiErr := s.TxNotificationStorage.Save(ctx, userID, model.NewTxNotificationStatusFailed())
			if notiErr != nil {
				zap.L().Error("failed to save user failed tx notification", zap.Error(err))
			}

			return nil, err
		}

		tokensToSwap[i].TxHash = txHash

		err = s.TxNotificationStorage.Save(ctx, userID, model.NewTxNotificationStatusSuccess())
		if err != nil {
			zap.L().Error("failed to save user success tx notification", zap.Error(err))
		}
	}

	return tokensToSwap, nil
}

type AutoswapParams struct {
	UserID       uuid.UUID
	PrivateKey   solana.PrivateKey
	USDCBalance  uint64
	RequiredUSDC uint64
	SolBalance   uint64
}

func (s *WalletService) waitForSolBalanceUpdate(
	ctx context.Context,
	publicKey solana.PublicKey,
	requiredAmount uint64,
	timeout time.Duration,
	pollInterval time.Duration,
) (bool, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	for {
		hasEnough, err := s.HasEnoughSolBalanceFinalized(ctx, publicKey, requiredAmount)
		if err != nil {
			return false, err
		}

		if hasEnough {
			return true, nil
		}

		select {
		case <-ctx.Done():
			return false, apperrors.BadRequest("not enough sol balance to proceed a transaction (timed out waiting for balance update)")
		case <-time.After(pollInterval):
		}
	}
}
