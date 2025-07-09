package jupiter

import (
	"context"
	"github.com/gagliardetto/solana-go"
	"github.com/go-resty/resty/v2"
	"github.com/goccy/go-json"
	"gitlab.com/duel-duck/duel-duck-api/config"
	"gitlab.com/duel-duck/duel-duck-api/pkg/apperrors"
	"net/http"
	"strconv"
	"strings"
)

type Client struct {
	client *resty.Client
}

type Option func(*Client)

func NewClient(c *config.Config, options ...Option) *Client {
	client := &Client{
		client: resty.New().SetBaseURL(c.Client.JupiterBaseURL),
	}

	for _, option := range options {
		option(client)
	}

	return client
}

type QuoteMetadata json.RawMessage
type GetQuoteParams struct {
	InputMintAddress  string
	OutputMintAddress string

	Amount         int64
	SlippageBps    int64
	PlatformFeeBps int64
}

func (c Client) GetQuote(ctx context.Context, params GetQuoteParams) (*QuoteMetadata, error) {
	const url = "swap/v1/quote"

	resp, err := c.client.R().
		SetContext(ctx).
		SetHeader("Content-Type", "application/json").
		SetQueryParam("inputMint", params.InputMintAddress).
		SetQueryParam("outputMint", params.OutputMintAddress).
		SetQueryParam("amount", strconv.FormatInt(params.Amount, 10)).
		SetQueryParam("slippageBps", strconv.FormatInt(params.SlippageBps, 10)).
		SetQueryParam("maxAccounts", strconv.FormatInt(28, 10)).
		SetQueryParam("platformFeeBps", strconv.FormatInt(params.PlatformFeeBps, 10)).
		Get(url)
	if err != nil {
		return nil, apperrors.ServiceUnavailable("failed to get resp from jupiter: "+url, err)
	}

	if !resp.IsSuccess() {
		if resp.StatusCode() == http.StatusBadRequest {
			return nil, apperrors.BadRequest("bad request from jupiter: " + resp.Request.URL)
		}

		return nil, apperrors.ServiceUnavailable("failed to get resp from jupiter: status is not success")
	}

	metadata := QuoteMetadata(resp.Body())

	return &metadata, nil
}

type GetSwapTransactionRequest struct {
	QuoteMetadata json.RawMessage  `json:"quoteResponse"`
	UserPublicKey solana.PublicKey `json:"userPublicKey"`
	FeeAccount    solana.PublicKey `json:"feeAccount"`
}

type GetSwapTransactionResult struct {
	SwapTransaction *solana.Transaction
}
type GetSwapTransactionResponse struct {
	SwapTransaction string `json:"swapTransaction"`
}

func (c Client) GetSwapTransaction(
	ctx context.Context,
	metadata QuoteMetadata,
	wallet solana.PublicKey,
	adminPublicKey solana.PublicKey,
) (*GetSwapTransactionResult, error) {
	const url = "swap/v1/swap"

	data, err := json.Marshal(GetSwapTransactionRequest{
		QuoteMetadata: json.RawMessage(metadata),
		UserPublicKey: wallet,
		FeeAccount:    adminPublicKey,
	})
	if err != nil {
		return nil, apperrors.Internal("failed to marshal swap transaction", err)
	}

	resp, err := c.client.R().
		SetContext(ctx).
		SetHeader("Content-Type", "application/json").
		SetBody(data).
		SetResult(&GetSwapTransactionResponse{}).
		Post(url)
	if err != nil {
		return nil, apperrors.ServiceUnavailable("failed to get swap tx from jupiter", err)
	}

	if !resp.IsSuccess() {
		if resp.StatusCode() == http.StatusBadRequest {
			return nil, apperrors.BadRequest("received bad request from jupiter "+url, err)
		}
		return nil, apperrors.ServiceUnavailable("failed to get swap tx: non success status received", err)
	}

	res := resp.Result()

	tx, ok := res.(*GetSwapTransactionResponse)
	if !ok {
		return nil, apperrors.ServiceUnavailable("invalid data received from jupiter; cast failed")
	}

	parsedTx, err := solana.TransactionFromBase64(tx.SwapTransaction)
	if err != nil {
		return nil, apperrors.ServiceUnavailable("decode swap transaction", err)
	}

	return &GetSwapTransactionResult{
		SwapTransaction: parsedTx,
	}, nil
}

type GetPriceResp struct {
	UsdPrice       float64 `json:"usdPrice"`
	BlockId        int64   `json:"blockId"`
	Decimals       uint8   `json:"decimals"`
	PriceChange24h float64 `json:"priceChange24h"`
}

func (c Client) Price(
	ctx context.Context,
	tokens ...string,
) (map[string]GetPriceResp, error) {
	const url = "price/v3"

	result := make(map[string]GetPriceResp)
	resp, err := c.client.R().
		SetContext(ctx).
		SetHeader("Content-Type", "application/json").
		SetQueryParam("ids", strings.Join(tokens, ",")).
		SetResult(&result).
		Get(url)
	if err != nil {
		return nil, apperrors.ServiceUnavailable("failed to get token prices from jupiter", err)
	}

	if !resp.IsSuccess() {
		if resp.StatusCode() == http.StatusBadRequest {
			return nil, apperrors.BadRequest("received bad request from jupiter "+url, err)
		}
		return nil, apperrors.ServiceUnavailable("failed to get token prices: non success status received", err)
	}

	return result, nil
}
