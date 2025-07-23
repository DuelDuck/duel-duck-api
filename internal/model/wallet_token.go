package model

import (
	"github.com/google/uuid"
	"github.com/uptrace/bun"
)

const DuckPointMockMint = "DPOINT"

type WalletToken struct {
	bun.BaseModel `bun:"table:wallet_tokens,alias:wt" json:"-"`

	UserID      uuid.UUID `bun:"" json:"user_id"`
	Mint        string    `bun:"" jons:"mint"`
	Name        string    `bun:"" json:"name"`
	Symbol      string    `bun:"" json:"symbol"`
	ImageURL    string    `bun:"" json:"image_url"`
	IsVisible   bool      `bun:"" json:"is_visible"`
	IsSwappable bool      `bun:"" json:"is_swappable"`
}

type WalletTokenAddReq struct {
	bun.BaseModel `bun:"table:wallet_tokens,alias:wt" json:"-"`

	Mint        string `bun:"" jons:"mint"`
	Name        string `bun:"" json:"name"`
	Symbol      string `bun:"" json:"symbol"`
	ImageURL    string `bun:"" json:"image_url"`
	IsVisible   bool   `bun:"" json:"is_visible"`
	IsSwappable bool   `bun:"" json:"is_swappable"`
}

type WalletTokenEditReq struct {
	bun.BaseModel `bun:"table:wallet_tokens,alias:wt" json:"-"`

	Mint        string `bun:"" jons:"mint"`
	IsVisible   *bool  `bun:"" json:"is_visible"`
	IsSwappable *bool  `bun:"" json:"is_swappable"`
}

type WalletTokenDeleteReq struct {
	Mint string `jons:"mint"`
}

type SwappableTokens struct {
	Mints          []string
	IsSolSwappable bool
}

type AutoswapToken struct {
	Mint         string
	AmountToSwap uint64
}

type TokenWithValue struct {
	TokenAccount        string `json:"token_account"`
	TokenAddress        string `json:"token_address"`
	Amount              int64  `json:"amount"`
	AvailableSwapAmount int64  `json:"available_swap_amount"`
	TokenDecimals       uint8  `json:"token_decimals"`
	USDPrice            float64
	USDValue            uint64
}

type AutoswapResult struct {
	Mint       string  `json:"mint"`
	ATA        string  `json:"ATA"`
	Decimals   uint8   `json:"decimals"`
	Amount     int64   `json:"amount"`
	SwapAmount uint64  `json:"swapped_amount"`
	USCPrice   float64 `json:"usd_price"`
	TxHash     string  `json:"tx_hash"`
}

func AutoswapResultFromTokenWithValue(token TokenWithValue, amountToSwap uint64) AutoswapResult {
	return AutoswapResult{
		Mint:       token.TokenAddress,
		ATA:        token.TokenAccount,
		Decimals:   token.TokenDecimals,
		Amount:     token.Amount,
		SwapAmount: amountToSwap,
		USCPrice:   token.USDPrice,
	}
}
