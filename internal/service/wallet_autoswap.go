package service

import (
	"context"
	"math"
	"slices"
	"sort"
	"time"

	"github.com/gagliardetto/solana-go"
	"github.com/google/uuid"
	"gitlab.com/duel-duck/duel-duck-api/internal/client/jupiter"
	"gitlab.com/duel-duck/duel-duck-api/internal/model"
	"gitlab.com/duel-duck/duel-duck-api/pkg/apperrors"
	"go.uber.org/zap"
)

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

func (s *WalletService) AutoswapUSDC(
	ctx context.Context,
	userID uuid.UUID,
	privateKey solana.PrivateKey,
	requiredAmount uint64,
) ([]model.AutoswapResult, error) {
	if requiredAmount <= 0 {
		return nil, nil
	}

	publicKey := privateKey.PublicKey()
	swappable, err := s.findATAsAvailableForSwap(ctx, userID, publicKey)
	if err != nil {
		return nil, err
	}

	if len(swappable) == 0 {
		return nil, apperrors.BadRequest("insufficient funds: no tokens available for autoswap")
	}

	// Prepare list of token addresses for price lookup
	tokenAddresses := make([]string, 0, len(swappable))
	for _, t := range swappable {
		tokenAddresses = append(tokenAddresses, t.TokenAddress)
	}

	prices, err := s.Jupiter.Price(ctx, tokenAddresses...)
	if err != nil {
		return nil, err
	}

	tokensWithValue := s.getSwappableATAsOrderByUSDValue(swappable, prices)

	// We want to swap enough tokens to get (requiredAmount * 1.1) USDC (raw, 6 decimals)
	targetUSDC := uint64(float64(requiredAmount) * 1.1)

	tokensToSwap, err := s.prepareTokensForSwap(tokensWithValue, targetUSDC)
	if err != nil {
		return nil, err
	}

	for i, t := range tokensToSwap {
		notification := model.NewTxNotificationStatusPending()
		err = s.TxNotificationStorage.Save(ctx, userID, notification)
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

			err = s.TxNotificationStorage.SetStatusFailed(ctx, userID, notification.ID)
			if err != nil {
				zap.L().Error("failed to save user success tx notification", zap.Error(err))
			}

			return nil, err
		}

		tokensToSwap[i].TxHash = txHash

		err = s.TxNotificationStorage.SetStatusSuccess(ctx, userID, notification.ID)
		if err != nil {
			zap.L().Error("failed to save user success tx notification", zap.Error(err))
		}
	}

	return tokensToSwap, nil
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

func (s *WalletService) getSwappableATAsOrderByUSDValue(swappable []model.TokenAccount, prices map[string]jupiter.GetPriceResp) []model.TokenWithValue {
	tokensWithValue := make([]model.TokenWithValue, 0, len(swappable))
	for _, tokenAcc := range swappable {
		price, ok := prices[tokenAcc.TokenAddress]
		if !ok || price.UsdPrice <= 0 || tokenAcc.TokenAddress == USDCMintAddress.String() {
			continue
		}

		availableSwapAmount := tokenAcc.Amount

		if tokenAcc.TokenAddress == solana.SolMint.String() {
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
		return nil, apperrors.PaymentRequired("not enough tokens for usdc autoswap")
	}

	return tokensToSwap, nil
}
