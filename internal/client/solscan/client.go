package solscan

import (
	"context"
	"github.com/go-resty/resty/v2"
	"github.com/goccy/go-json"
	"gitlab.com/duel-duck/duel-duck-api/config"
	"gitlab.com/duel-duck/duel-duck-api/internal/model"
	"gitlab.com/duel-duck/duel-duck-api/pkg/apperrors"
	"go.uber.org/zap"
	"net/http"
	"strconv"
)

type Client struct {
	client *resty.Client
	apiKey string
}

func (c *Client) R() *resty.Request {
	return c.client.R().SetHeaders(map[string]string{
		"Accept":       "application/json",
		"Content-Type": "application/json",
		"token":        c.apiKey,
	})
}

const baseURL = "https://pro-api.solscan.io/v2.0/"

type Option func(*Client)

func NewClient(c *config.Config, options ...Option) *Client {
	client := &Client{
		client: resty.New().SetBaseURL(baseURL),
		apiKey: c.Client.SolscanAPIKey,
	}

	for _, option := range options {
		option(client)
	}

	return client
}

func (c *Client) GetTransactions(
	ctx context.Context,
	publicAddress string,
	reqData *model.GetTransactionHistoryReq,
) ([]model.Transaction, error) {
	const url = "account/balance_change"

	req := c.R().
		SetContext(ctx).
		SetQueryParam("address", publicAddress).
		SetQueryParam("remove_spam", strconv.FormatBool(reqData.RemoveSpam))

	if reqData.PageSize != 0 && reqData.PageNumber != 0 {
		req.SetQueryParam("page", strconv.FormatUint(reqData.PageNumber, 10)).
			SetQueryParam("page_size", strconv.FormatUint(reqData.PageSize, 10))
	}

	resp, err := req.Get(url)
	if err != nil {
		return nil, apperrors.ServiceUnavailable("failed to get resp from jupiter: "+url, err)
	}

	if !resp.IsSuccess() {
		if resp.StatusCode() == http.StatusBadRequest {
			zap.L().Error("bad request from solscan",
				zap.String("url", resp.Request.URL),
				zap.ByteString("body", resp.Body()),
			)

			return nil, apperrors.BadRequest("bad request from solscan: " + resp.Request.URL)
		}

		return nil, apperrors.ServiceUnavailable("failed to get resp from solscan: status is not success")
	}

	solscanTxs := &model.SolscanBalanceChangeResp{}
	if err = json.UnmarshalContext(ctx, resp.Body(), &solscanTxs); err != nil {
		return nil, apperrors.Internal("failed to unmarshal solscan resp: "+url, err)
	}

	return solscanTxs.Data, nil
}

func (c *Client) GetTokenAccounts(
	ctx context.Context,
	publicAddress string,
	reqData *model.GetTokenAccountsReq,
) (*model.SolscanTokenAccountsResp, error) {
	const url = "account/token-accounts"

	req := c.R().
		SetContext(ctx).
		SetQueryParam("address", publicAddress).
		SetQueryParam("type", reqData.Type).
		SetQueryParam("hide_zero", strconv.FormatBool(reqData.HideZero))

	if reqData.PageSize != 0 && reqData.PageNumber != 0 {
		req.SetQueryParam("page", strconv.FormatUint(reqData.PageNumber, 10)).
			SetQueryParam("page_size", strconv.FormatUint(reqData.PageSize, 10))
	}

	if reqData.Type == "" {
		req.SetQueryParam("type", "token")
	}

	resp, err := req.Get(url)
	if err != nil {
		return nil, apperrors.ServiceUnavailable("failed to get resp from jupiter: "+url, err)
	}

	if !resp.IsSuccess() {
		if resp.StatusCode() == http.StatusBadRequest {
			return nil, apperrors.BadRequest("bad request from solscan: " + resp.Request.URL)
		}

		return nil, apperrors.ServiceUnavailable("failed to get resp from solscan: status is not success")
	}

	tokenAccounts := &model.SolscanTokenAccountsResp{}
	if err = json.UnmarshalContext(ctx, resp.Body(), &tokenAccounts); err != nil {
		return nil, apperrors.Internal("failed to unmarshal solscan resp: "+url, err)
	}

	return tokenAccounts, nil
}
