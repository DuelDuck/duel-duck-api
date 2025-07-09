package model

import (
	"github.com/uptrace/bun"
	"time"
)

type CMCID uint64

type Coin struct {
	bun.BaseModel `bun:"table:coins,alias:coins" json:"-"`

	ID       uint64 `bun:",pk,type:integer" json:"id"`
	Rank     uint64 `bun:",column:rank,type:integer,notnull" json:"rank"`
	Name     string `bun:",column:name,type:varchar(100),notnull" json:"name"`
	Symbol   string `bun:",column:symbol,type:varchar(50),notnull" json:"symbol"`
	Slug     string `bun:",column:slug,type:varchar(100),notnull" json:"slug"`
	ImageUrl string `bun:",column:image_url,type:varchar(200),notnull" json:"image_url"`
}

type SolanaToken struct {
	bun.BaseModel `bun:"table:solana_tokens,alias:st" json:"-"`

	Mint        string  `bun:",pk" json:"mint"`
	Name        string  `bun:"" json:"name"`
	Symbol      string  `bun:"" json:"symbol"`
	Decimals    uint8   `bun:"" json:"decimals"`
	ImageURL    string  `bun:",nullzero" json:"image_url"`
	DailyVolume float64 `bun:"" json:"daily_volume"`
}

type JupTokensResp struct {
	bun.BaseModel `bun:"table:solana_tokens,alias:st" json:"-"`

	Address     string  `bun:",column:mint" json:"address"`
	Name        string  `bun:",column:name" json:"name"`
	Symbol      string  `bun:",column:symbol" json:"symbol"`
	Decimals    int     `bun:",column:decimals" json:"decimals"`
	LogoURI     string  `bun:",column:image_url" json:"logoURI"`
	DailyVolume float64 `bun:",column:daily_volume" json:"daily_volume"`
}

type CMCCoinResp struct {
	Id       uint64 `json:"id"`
	Rank     uint64 `json:"rank"`
	Name     string `json:"name"`
	Symbol   string `json:"symbol"`
	Slug     string `json:"slug"`
	ImageUrl string `json:"-"`
}

type APIResponse struct {
	Data []CMCCoinResp `json:"data"`
}
type CMCPriceResp struct {
	Data map[uint64]CMCData `json:"data"`
}

type CMCData struct {
	Quote CMCQuote `json:"quote"`
}

type CMCQuote struct {
	USD CMCUSD `json:"USD"`
}

type CMCUSD struct {
	Price float64 `json:"price"`
}

type CMCOHLCVParams struct {
	TimeStart  time.Time
	TimeEnd    time.Time
	TimePeriod string
	Interval   string
}

type CMCOHLCVMultipleTokensResp struct {
	Data map[CMCID]CMCOHLCVData `json:"data"`
}

type CMCOHLCVData struct {
	Quotes []CMCOHLCVQuoteData `json:"quotes"`
	ID     CMCID               `json:"id"`
}

type CMCOHLCVQuoteData struct {
	Quote CMCOHLCVQuoteUSD `json:"quote"`
}

type CMCOHLCVQuoteUSD struct {
	USD CMCOHLCVQuoteUSDValue `json:"USD"`
}
type CMCOHLCVQuoteUSDValue struct {
	Open      float64 `json:"open"`
	High      float64 `json:"high"`
	Low       float64 `json:"low"`
	Close     float64 `json:"close"`
	Volume    float64 `json:"volume"`
	MarketCap float64 `json:"market_cap"`
}

type CMCHighLow struct {
	High float64 `json:"high"`
	Low  float64 `json:"low"`
}

type CMCOHLCVSingleTokenResp struct {
	Data CMCOHLCVData `json:"data"`
}
