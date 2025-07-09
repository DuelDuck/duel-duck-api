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
