package service

import (
	"context"
	"fmt"
	"github.com/gagliardetto/solana-go"
	"github.com/goccy/go-json"
	"github.com/uptrace/bun"
	"gitlab.com/duel-duck/duel-duck-api/config"
	"gitlab.com/duel-duck/duel-duck-api/internal/model"
	"gitlab.com/duel-duck/duel-duck-api/internal/storage/repository"
	"gitlab.com/duel-duck/duel-duck-api/pkg/apperrors"
	repo "gitlab.com/duel-duck/duel-duck-api/pkg/repository"
	"go.uber.org/zap"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type CoinService struct {
	Client             *http.Client
	CoinRepository     *repository.CoinRepository
	TransactionManager *repo.TransactionManager
	CMCApiKey          string
}

func NewCoinService(
	c *config.Config,
	coinRepository *repository.CoinRepository,
	transactionManager *repo.TransactionManager,
) *CoinService {
	return &CoinService{
		Client:             http.DefaultClient,
		CoinRepository:     coinRepository,
		TransactionManager: transactionManager,
		CMCApiKey:          c.App.CMCApiKey,
	}
}

func (s *CoinService) UpdateAllCoins(ctx context.Context) error {
	params := url.Values{}
	params.Add("limit", "5000")
	params.Add("sort", "cmc_rank")
	finalURL := fmt.Sprintf("https://pro-api.coinmarketcap.com/v1/cryptocurrency/map?%s", params.Encode())

	req, err := http.NewRequest("GET", finalURL, nil)
	if err != nil {
		return apperrors.Internal("cannot create request", err)
	}

	req.Header.Add("X-CMC_PRO_API_KEY", s.CMCApiKey)

	resp, err := s.Client.Do(req)
	if err != nil {
		return apperrors.Internal("request failed", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return apperrors.Internal("failed to read response", err)
	}

	var apiResponse model.APIResponse

	if err = json.Unmarshal(body, &apiResponse); err != nil {
		return apperrors.Internal("failed to unmarshal", err)
	}

	for i := range apiResponse.Data {
		imageUrl := fmt.Sprintf("https://s2.coinmarketcap.com/static/img/coins/64x64/%d.png", apiResponse.Data[i].Id)
		apiResponse.Data[i].ImageUrl = imageUrl
	}

	err = s.TransactionManager.WithinTransaction(ctx,
		func(ctx context.Context, tx bun.Tx) error {
			for i := range apiResponse.Data {
				coin := model.Coin{
					ID:       apiResponse.Data[i].Id,
					Rank:     apiResponse.Data[i].Rank,
					Name:     apiResponse.Data[i].Name,
					Symbol:   apiResponse.Data[i].Symbol,
					Slug:     apiResponse.Data[i].Slug,
					ImageUrl: apiResponse.Data[i].ImageUrl,
				}
				exist, err := s.CoinRepository.WithTx(tx).Exists(ctx, &coin)
				if err != nil {
					return err
				}

				if !exist {
					err = s.CoinRepository.WithTx(tx).Create(ctx, &coin)
					if err != nil {
						return err
					}
					continue
				}

				err = s.CoinRepository.WithTx(tx).Update(ctx, &coin)
				if err != nil {
					return err
				}
			}

			return nil
		})

	if err != nil {
		return apperrors.Internal("failed to reward referrers", err)
	}

	return nil
}

func (s *CoinService) GetAllCoins(ctx context.Context, options *repo.Options) ([]model.Coin, error) {
	coins, err := s.CoinRepository.GetAllCoins(ctx, options)
	if err != nil {
		return nil, apperrors.Internal("failed to get coins", err)
	}

	return coins, nil
}

func (s *CoinService) GetCurrentCoinPriceByID(id uint64) (float64, error) {
	// TODO: Move to the v2 endpoint api version as v1 /v1/cryptocurrency/quotes/latest
	// considered to be deprecated according to
	// https://coinmarketcap.com/api/documentation/v1/#operation/getV1CryptocurrencyQuotesLatest

	const apiURL = "https://pro-api.coinmarketcap.com/v2/cryptocurrency/quotes/latest"
	var coinMarketCapID = strconv.FormatUint(id, 10)

	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return 0, apperrors.ServiceUnavailable("Failed to create request: %v", err)
	}

	q := req.URL.Query()
	q.Add("id", coinMarketCapID)
	req.URL.RawQuery = q.Encode()
	req.Header.Add("X-CMC_PRO_API_KEY", s.CMCApiKey)
	req.Header.Add("Accept", "application/json")

	resp, err := s.Client.Do(req)
	if err != nil {
		return 0, apperrors.ServiceUnavailable("Error making the API request: %v", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		status := strconv.FormatInt(int64(resp.StatusCode), 10)
		return 0, apperrors.ServiceUnavailable("Non-OK HTTP status from CoinMarketCap: "+status, err)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, apperrors.Internal("failed to read response", err)
	}

	var apiResponse model.CMCPriceResp
	if err = json.Unmarshal(body, &apiResponse); err != nil {
		return 0, apperrors.Internal("failed to unmarshal", err)
	}

	coinData, ok := apiResponse.Data[id]
	if !ok {
		return 0, apperrors.ServiceUnavailable("price is not provided")
	}

	return coinData.Quote.USD.Price, nil
}

func (s *CoinService) GetCurrentCoinsPriceByIDs(ids ...uint64) (map[uint64]float64, error) {
	const apiURL = "https://pro-api.coinmarketcap.com/v2/cryptocurrency/quotes/latest"

	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return nil, apperrors.ServiceUnavailable("Failed to create request: %v", err)
	}

	queryIDs := make([]string, 0, len(ids))
	for _, id := range ids {
		queryIDs = append(queryIDs, strconv.FormatUint(id, 10))
	}

	q := req.URL.Query()
	q.Add("id", strings.Join(queryIDs, ","))

	req.URL.RawQuery = q.Encode()
	req.Header.Add("X-CMC_PRO_API_KEY", s.CMCApiKey)
	req.Header.Add("Accept", "application/json")

	resp, err := s.Client.Do(req)
	if err != nil {
		return nil, apperrors.ServiceUnavailable("Error making the API request: %v", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		status := strconv.FormatInt(int64(resp.StatusCode), 10)
		return nil, apperrors.ServiceUnavailable("Non-OK HTTP status from CoinMarketCap: "+status, err)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, apperrors.Internal("failed to read response", err)
	}

	var apiResponse model.CMCPriceResp
	if err = json.Unmarshal(body, &apiResponse); err != nil {
		return nil, apperrors.Internal("failed to unmarshal", err)
	}

	coinPrices := make(map[uint64]float64, len(apiResponse.Data))
	for k, coin := range apiResponse.Data {
		coinPrices[k] = coin.Quote.USD.Price
	}

	return coinPrices, nil
}

func (s *CoinService) GetCoinsByCMCIDs(ctx context.Context, cmcIDs []uint64) ([]model.Coin, error) {
	coins, err := s.CoinRepository.GetCoinsByCMCIDs(ctx, cmcIDs)
	if err != nil {
		return nil, apperrors.Internal("failed to get coins by cmc ids", err)
	}

	return coins, nil
}

func (s *CoinService) RefreshSolanaTokens(ctx context.Context) error {
	tokens, err := s.GetAllJupVerifiedTokens()
	if err != nil {
		return err
	}

	if err = s.CoinRepository.RefreshSolanaTokens(ctx, tokens); err != nil {
		return apperrors.Internal("failed to refresh solana tokens", err)
	}

	return nil
}

func (s *CoinService) GetSolanaTokenByName(ctx context.Context, token string) ([]model.SolanaToken, error) {
	tokenInfo, err := s.CoinRepository.GetSolanaTokenByName(ctx, token)
	if err != nil {
		return nil, apperrors.Internal("failed to find any token by name or symbol", err)
	}

	return tokenInfo, nil
}

func (s *CoinService) GetSolanaTokenByMint(ctx context.Context, mint string) (*model.SolanaToken, error) {
	if _, err := solana.PublicKeyFromBase58(mint); err != nil {
		return nil, apperrors.BadRequest("failed to parse mint address", err)
	}

	token, err := s.CoinRepository.FindSolanaTokenByMint(ctx, mint)
	if err != nil {
		return nil, apperrors.Internal("failed to get token by mint", err)
	}

	if token == nil {
		return nil, apperrors.NotFound("token with provided mint not found")
	}

	return token, nil
}

func (s *CoinService) GetAllJupVerifiedTokens() ([]model.JupTokensResp, error) {
	const apiURL = "https://tokens.jup.ag/tokens"
	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return nil, apperrors.ServiceUnavailable("Failed to create request: %v", err)
	}

	q := req.URL.Query()
	q.Add("tags", "verified")
	req.URL.RawQuery = q.Encode()
	req.Header.Add("Accept", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, apperrors.ServiceUnavailable("Error making the API request to jup", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		status := strconv.FormatInt(int64(resp.StatusCode), 10)
		return nil, apperrors.ServiceUnavailable("Non-OK HTTP status from jup: "+status, err)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, apperrors.Internal("failed to read response", err)
	}

	var tokens []model.JupTokensResp
	if err = json.Unmarshal(body, &tokens); err != nil {
		return nil, apperrors.Internal("failed to unmarshal", err)
	}

	return tokens, nil
}

func (s *CoinService) GetCurrentSolanaPrice() (float64, error) {
	const solanaCoinMarketCapID = 5426

	price, err := s.GetCurrentCoinPriceByID(solanaCoinMarketCapID)
	if err != nil {
		return 0, err
	}

	return price, nil
}

func (s *CoinService) GetOpenHighLowCloseVolume(
	params *model.CMCOHLCVParams,
	ids ...model.CMCID,
) (map[model.CMCID]model.CMCOHLCVData, error) {
	const apiURL = "https://pro-api.coinmarketcap.com/v2/cryptocurrency/ohlcv/historical"
	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return nil, apperrors.ServiceUnavailable("Failed to create request: %v", err)
	}

	queryIDs := make([]string, 0, len(ids))
	for _, id := range ids {
		queryIDs = append(queryIDs, strconv.FormatUint(uint64(id), 10))
	}

	q := req.URL.Query()
	q.Add("id", strings.Join(queryIDs, ","))
	q.Add("time_start", params.TimeStart.Format(time.RFC3339))
	q.Add("time_end", params.TimeEnd.Format(time.RFC3339))
	q.Add("time_period", params.TimePeriod)
	q.Add("interval", params.Interval)

	req.URL.RawQuery = q.Encode()
	req.Header.Add("X-CMC_PRO_API_KEY", s.CMCApiKey)
	req.Header.Add("Accept", "application/json")

	resp, err := s.Client.Do(req)
	if err != nil {
		return nil, apperrors.ServiceUnavailable("Error making the API request: %v", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		status := strconv.FormatInt(int64(resp.StatusCode), 10)
		return nil, apperrors.ServiceUnavailable("Non-OK HTTP status from CoinMarketCap "+apiURL+": "+status, err)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, apperrors.Internal("failed to read response", err)
	}

	var apiResponse model.CMCOHLCVMultipleTokensResp
	if err = json.Unmarshal(body, &apiResponse); err != nil {

		var apiResponse model.CMCOHLCVSingleTokenResp
		if err = json.Unmarshal(body, &apiResponse); err != nil {
			return nil, apperrors.Internal("failed to unmarshal response", err)
		}

		return map[model.CMCID]model.CMCOHLCVData{
			apiResponse.Data.ID: apiResponse.Data,
		}, nil
	}

	return apiResponse.Data, nil
}

func (s *CoinService) GetHighLowHourly(ids ...model.CMCID) (map[model.CMCID]model.CMCHighLow, error) {
	now := time.Now().UTC().Truncate(time.Hour)

	params := &model.CMCOHLCVParams{
		// if current was 17:30, TimeStart would be equal to 15:59. We want to get data for the latest hour
		// and according to CoinMarketCap doc we must pass time_start exclusively
		// https://coinmarketcap.com/api/documentation/v1/#operation/getV2CryptocurrencyOhlcvLatest
		TimeStart:  now.Add(-time.Hour - time.Minute),
		TimeEnd:    now,
		TimePeriod: "hourly",
		Interval:   "hourly",
	}

	data, err := s.GetOpenHighLowCloseVolume(params, ids...)
	if err != nil {
		return nil, err
	}

	highLow := make(map[model.CMCID]model.CMCHighLow, len(data))
	for _, ohlcv := range data {
		if len(ohlcv.Quotes) == 0 {
			zap.L().Error("no data provided for coin in CMC OHLCV resp")
			continue
		}

		quote := ohlcv.Quotes[len(ohlcv.Quotes)-1]
		hl := model.CMCHighLow{
			High: quote.Quote.USD.High,
			Low:  quote.Quote.USD.Low,
		}

		highLow[ohlcv.ID] = hl
	}

	return highLow, nil
}
