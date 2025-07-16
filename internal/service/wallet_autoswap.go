package service

import (
	"context"
	"math"
	"slices"
	"sort"
	"time"

	"github.com/gagliardetto/solana-go"
	"github.com/google/uuid"
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

// processSolToken handles special logic for SOL token:
// - If SOL value is less than $1, returns nil (not swappable)
// - If SOL value is >= $1, adjusts amount to leave at least $1 worth of SOL
func (s *WalletService) processSolToken(tokenAcc model.TokenAccount, price float64) *model.TokenAccount {
	if tokenAcc.TokenAddress != solana.SolMint.String() {
		return &tokenAcc
	}

	tokenRawAmount := float64(tokenAcc.Amount)
	tokenDecimals := tokenAcc.TokenDecimals

	// Calculate total SOL USD value
	oneUSDCRaw := TokenMultiplierByDecimals(USDCMintDecimals)
	usdRawPrice := price * oneUSDCRaw
	totalSolUsdRaw := usdRawPrice * (tokenRawAmount / TokenMultiplierByDecimals(tokenDecimals))

	if totalSolUsdRaw < oneUSDCRaw {
		// SOL value less than $1, not swappable
		return nil
	}

	// Calculate $1 worth of SOL in raw units
	oneUsdInSol := 1.0 * TokenMultiplierByDecimals(tokenDecimals) / price

	// Adjust amount to leave at least $1 worth of SOL
	swappableAmount := tokenRawAmount - oneUsdInSol
	if swappableAmount <= 0 {
		return nil
	}

	// Create new token account with adjusted amount
	adjustedToken := tokenAcc
	adjustedToken.Amount = int64(swappableAmount)
	return &adjustedToken
}

// Sort tokens by total USD value (amount * price) desc
type tokenWithValue struct {
	Token    model.TokenAccount
	USDValue uint64
}

func (s *WalletService) AutoswapUSDC(
	ctx context.Context,
	userID uuid.UUID,
	privateKey solana.PrivateKey,
	requiredAmount uint64,
) error {
	if requiredAmount <= 0 {
		return nil
	}

	swappable, err := s.WalletTokenRepository.MintsWithAutoswap(ctx, userID)
	if err != nil {
		return apperrors.Internal("failed to get user auto—swappable tokens", err)
	}

	if len(swappable) == 0 {
		return nil
	}

	solSwappable := IsAllowedToSwapSol(swappable)

	publicKey := privateKey.PublicKey()
	req := &model.GetTokenAccountsReq{HideZero: true}
	tokens, err := s.GetTokenAccounts(ctx, publicKey.String(), req)
	if err != nil {
		return err
	}

	// Find intersection of swappable tokens and user's tokens allowed to be swapped,
	// preserving Amount and other TokenAccount fields.
	intersection := intersectTokenAccountsWithMints(tokens.Data, swappable)
	if solSwappable {
		intersection, err = s.addSolToken(ctx, publicKey, intersection)
		if err != nil {
			return err
		}
	}

	if len(intersection) == 0 {
		return nil
	}

	// Prepare list of token addresses for price lookup
	tokenAddresses := make([]string, 0, len(intersection))
	for _, t := range intersection {
		tokenAddresses = append(tokenAddresses, t.TokenAddress)
	}

	prices, err := s.Jupiter.Price(ctx, tokenAddresses...)
	if err != nil {
		return err
	}

	tokensWithValue := make([]tokenWithValue, 0, len(intersection))
	for _, tokenAcc := range intersection {
		price, ok := prices[tokenAcc.TokenAddress]
		if !ok || price.UsdPrice <= 0 || tokenAcc.TokenAddress == USDCMintAddress.String() {
			continue
		}

		// Process SOL token with special logic
		if tokenAcc.TokenAddress == solana.SolMint.String() {
			processedToken := s.processSolToken(tokenAcc, price.UsdPrice)
			if processedToken == nil {
				continue // SOL not swappable (value < $1)
			}
			tokenAcc = *processedToken
		}

		tokenRawAmount := float64(tokenAcc.Amount)
		tokenDecimals := tokenAcc.TokenDecimals
		usdcPriceRaw := price.UsdPrice * TokenMultiplierByDecimals(USDCMintDecimals)
		usdValue := usdcPriceRaw * (tokenRawAmount / TokenMultiplierByDecimals(tokenDecimals))

		tokensWithValue = append(tokensWithValue, tokenWithValue{
			Token:    tokenAcc,
			USDValue: uint64(usdValue),
		})
	}

	// Sort descending by USDValue
	sort.Slice(tokensWithValue, func(i, j int) bool {
		return tokensWithValue[i].USDValue > tokensWithValue[j].USDValue
	})

	// We want to swap enough tokens to get (requiredAmount * 1.1) USDC (raw, 6 decimals)
	targetUSDC := uint64(float64(requiredAmount) * 1.1)
	usdcAccumulated := uint64(0)
	tokensToSwap := make([]model.AutoswapToken, 0, len(tokensWithValue))

	for _, twv := range tokensWithValue {
		tokenAcc := twv.Token
		price := prices[tokenAcc.TokenAddress]
		tokenAmount := float64(tokenAcc.Amount)
		tokenDecimals := tokenAcc.TokenDecimals
		usdcDecimalsMultiplier := TokenMultiplierByDecimals(USDCMintDecimals)
		usdcValue := tokenAmount * price.UsdPrice * usdcDecimalsMultiplier / TokenMultiplierByDecimals(tokenDecimals)

		usdcNeeded := float64(targetUSDC - usdcAccumulated)
		if usdcAccumulated >= targetUSDC {
			break
		}

		if usdcValue >= usdcNeeded {
			amountToSwap := usdcNeeded * TokenMultiplierByDecimals(tokenDecimals) / (price.UsdPrice * usdcDecimalsMultiplier)
			tokensToSwap = append(tokensToSwap, model.AutoswapToken{
				Mint:         tokenAcc.TokenAddress,
				AmountToSwap: uint64(amountToSwap),
			})
			usdcAccumulated += uint64(usdcNeeded)
			break
		} else {
			tokensToSwap = append(tokensToSwap, model.AutoswapToken{
				Mint:         tokenAcc.TokenAddress,
				AmountToSwap: uint64(tokenAcc.Amount),
			})
			usdcAccumulated += uint64(usdcValue)
		}
	}

	if targetUSDC > usdcAccumulated {
		return apperrors.PaymentRequired("not enough tokens for usdc autoswap")
	}

	for _, t := range tokensToSwap {
		err = s.TxNotificationStorage.Save(ctx, userID, model.NewTxNotificationStatusPending())
		if err != nil {
			zap.L().Error("failed to save user pending tx notification", zap.Error(err))
		}

		txHash, err := s.swap(
			ctx,
			privateKey,
			t.Mint,
			USDCMintAddress.String(),
			t.AmountToSwap,
			int64(s.SwapCommissionCoefficient*CoefficientToBPSRelation),
		)
		if err != nil {
			zap.L().Error("failed usdc autoswap", zap.Error(err))
			err = s.TxNotificationStorage.Save(ctx, userID, model.NewTxNotificationStatusFailed())
			if err != nil {
				zap.L().Error("failed to save user success tx notification", zap.Error(err))
			}

			return err
		}

		err = s.TxNotificationStorage.Save(ctx, userID, model.NewTxNotificationStatusSuccess())
		if err != nil {
			zap.L().Error("failed to save user success tx notification", zap.Error(err))
		}

		zap.L().Info("txHash", zap.String("txHash", txHash))
	}

	return nil
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
		hasEnough, err := s.HasEnoughTokenBalance(ctx, ata, requiredAmount)
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
