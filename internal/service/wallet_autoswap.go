package service

import (
	"context"
	"math"

	"github.com/gagliardetto/solana-go"
	"github.com/google/uuid"
	"gitlab.com/duel-duck/duel-duck-api/internal/model"
	"gitlab.com/duel-duck/duel-duck-api/pkg/apperrors"
)

// intersect finds the intersection of two string slices using a map for O(n+m) performance
func intersect(a, b []string) []string {
	if len(a) == 0 || len(b) == 0 {
		return nil
	}

	// Use smaller slice for map to save memory
	if len(a) > len(b) {
		a, b = b, a
	}

	set := make(map[string]struct{}, len(a))
	for _, v := range a {
		set[v] = struct{}{}
	}

	result := make([]string, 0, len(a))
	for _, v := range b {
		if _, exists := set[v]; exists {
			result = append(result, v)
		}
	}
	return result
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

func (s *WalletService) AutoswapUSDC(
	ctx context.Context,
	userID uuid.UUID,
	user solana.PublicKey,
	requiredAmount uint64,
) error {
	if requiredAmount <= 0 {
		return nil
	}

	swappable, err := s.WalletTokenRepository.MintsWithAutoswap(ctx, userID)
	if err != nil {
		return apperrors.Internal("failed to get user auto—swappable tokens", err)
	}

	solSwappable := false
	for _, t := range swappable {
		if t == solana.SolMint.String() {
			solSwappable = true
			break
		}
	}

	if len(swappable) == 0 {
		return nil
	}

	req := &model.GetTokenAccountsReq{HideZero: true}
	tokens, err := s.GetTokenAccounts(ctx, user.String(), req)
	if err != nil {
		return err
	}

	// Find intersection of swappable tokens and user's tokens allowed to be swapped,
	// preserving Amount and other TokenAccount fields.
	intersection := intersectTokenAccountsWithMints(tokens.Data, swappable)
	if solSwappable {
		balance, err := s.GetSolBalance(ctx, user)
		if err != nil {
			return err
		}

		intersection = append(intersection, model.TokenAccount{
			TokenAddress:  solana.SolMint.String(),
			TokenDecimals: SolMintDecimals,
			Amount:        int64(balance),
		})
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

	// We want to swap enough tokens to get (requiredAmount * 1.1) USDC (raw, 6 decimals)
	targetUSDC := uint64(float64(requiredAmount) * 1.1)
	usdcAccumulated := uint64(0)
	tokensToSwap := make([]model.AutoswapToken, 0, len(intersection))

	for _, tokenAcc := range intersection {
		price, ok := prices[tokenAcc.TokenAddress]
		if !ok || price.UsdPrice <= 0 || tokenAcc.TokenAddress == USDCMintAddress.String() {
			continue
		}

		// Calculate how much USDC (raw) this token account is worth
		// tokenAcc.Amount is in raw units, price.UsdPrice is per 1 token (not raw)
		// Need to adjust for decimals
		// USDC is 6 decimals, tokenAcc.TokenDecimals is token's decimals
		// So, USDC_amount = tokenAcc.Amount * price.UsdPrice * 10^6 / 10^tokenAcc.TokenDecimals

		tokenAmount := float64(tokenAcc.Amount)
		tokenDecimals := tokenAcc.TokenDecimals

		usdcDecimalsMultiplier := math.Pow10(int(USDCMintDecimals))
		usdcValue := tokenAmount * price.UsdPrice * usdcDecimalsMultiplier / math.Pow10(int(tokenDecimals))

		// If we still need more USDC, use as much as needed from this token
		usdcNeeded := float64(targetUSDC - usdcAccumulated)
		if usdcAccumulated >= targetUSDC {
			break
		}

		if usdcValue >= usdcNeeded {
			// Only need part of this token
			// Figure out how much token to swap to get usdcNeeded
			amountToSwap := usdcNeeded * math.Pow10(int(tokenDecimals)) / (price.UsdPrice * usdcDecimalsMultiplier)
			tokensToSwap = append(tokensToSwap, model.AutoswapToken{
				Mint:         tokenAcc.TokenAddress,
				AmountToSwap: uint64(amountToSwap),
			})
			usdcAccumulated += uint64(usdcNeeded)
			break
		} else {
			// Use all of this token
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

	// soon...
	//for _, t := range tokensToSwap {
	//	go func(t model.AutoswapToken) {
	//		_, err := s.swap(
	//			ctx,
	//			solana.MustPrivateKeyFromBase58(""),
	//			t.Mint,
	//			USDCMintAddress.String(),
	//			t.AmountToSwap,
	//			int64(s.SwapCommissionCoefficient*CoefficientToBPSRelation),
	//		)
	//		if err != nil {
	//			zap.L().Error("failed usdc autoswap", zap.Error(err))
	//		}
	//	}(t)
	//}

	return nil
}
