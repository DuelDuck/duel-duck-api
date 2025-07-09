package cron

import (
	"context"
	"github.com/google/uuid"
	rcron "github.com/robfig/cron/v3"
	"gitlab.com/duel-duck/duel-duck-api/config"
	"gitlab.com/duel-duck/duel-duck-api/internal/model"
	"gitlab.com/duel-duck/duel-duck-api/internal/service"
	"gitlab.com/duel-duck/duel-duck-api/pkg/apperrors"
	"gitlab.com/duel-duck/duel-duck-api/pkg/mtype"
	"go.uber.org/zap"
	"strconv"
	"time"
)

type DuelCron struct {
	Log                       *zap.Logger
	Cron                      *rcron.Cron
	DuelService               *service.DuelService
	CoinService               *service.CoinService
	AutoDuelsConf             *config.Duels
	duelsResolveJoinNotBefore time.Duration
}

const (
	RunningEveryHour = "@hourly"
)

func NewDuelCron(
	c *config.Config,
	l *zap.Logger,
	cron *rcron.Cron,
	duelService *service.DuelService,
	coinService *service.CoinService,
) (*DuelCron, error) {
	duelCron := &DuelCron{
		Log:                       l,
		Cron:                      cron,
		DuelService:               duelService,
		CoinService:               coinService,
		AutoDuelsConf:             &c.Duels,
		duelsResolveJoinNotBefore: c.Duels.ResolveJoinNotBefore,
	}

	_, err := duelCron.Cron.AddFunc(RunningEveryHour, duelCron.updateLeaderboard)
	if err != nil {
		return nil, err
	}

	_, err = duelCron.Cron.AddFunc(c.App.CryptoDuelsResolveInterval, duelCron.resolveCryptoOnEventDate)
	if err != nil {
		return nil, err
	}

	_, err = duelCron.Cron.AddFunc(c.App.CryptoDuelsResolveBeforeInterval, duelCron.resolveCryptoBeforeEventDate)
	if err != nil {
		return nil, err
	}

	_, err = duelCron.Cron.AddFunc(c.App.CryptoDuelsResolveBeforeInterval, duelCron.resolveDuckPointOnEventDate)
	if err != nil {
		return nil, err
	}

	_, err = duelCron.Cron.AddFunc(c.App.CryptoDuelsResolveBeforeInterval, duelCron.resolveDuckPointBeforeEventDate)
	if err != nil {
		return nil, err
	}

	_, err = duelCron.Cron.AddFunc(c.App.CryptoDuelsResolveBeforeInterval, duelCron.resolveDuckPointBeforeEventDate)
	if err != nil {
		return nil, err
	}

	_, err = duelCron.Cron.AddFunc(c.App.CryptoDuelsResolveBeforeInterval, duelCron.autoCancelOldDuels)
	if err != nil {
		return nil, err
	}

	_, err = duelCron.Cron.AddFunc(duelCron.AutoDuelsConf.ShortTerm.RunParams, duelCron.autoCreateDuelsShortTerm)
	if err != nil {
		return nil, err
	}

	_, err = duelCron.Cron.AddFunc(duelCron.AutoDuelsConf.LongTerm.RunParams, duelCron.autoCreateDuelsLongTerm)
	if err != nil {
		return nil, err
	}

	return duelCron, nil
}

func (c *DuelCron) updateLeaderboard() {
	err := c.DuelService.RefreshLeaderboard(context.Background())
	if err != nil {
		LogErr(c.Log, err)
	} else {
		c.Log.Debug("duel cron: successfully refreshed leaderboard")
	}

	// temporary solution  for tournaments, mustn't be stored as view
	err = c.DuelService.RefreshLeaderboardTournament(context.Background(), "tournament_leaderboard")
	if err != nil {
		LogErr(c.Log, err)
	} else {
		c.Log.Debug("duel cron: successfully refreshed leaderboard")
	}

	err = c.DuelService.RefreshLeaderboardTournament(context.Background(), "tournament_leaderboard_dd")
	if err != nil {
		LogErr(c.Log, err)
	} else {
		c.Log.Debug("duel cron: successfully refreshed leaderboard")
	}
}

func (c *DuelCron) resolveCryptoOnEventDate() {
	duelResolveHashes, err := c.DuelService.ResolveCryptoOnEventDate(context.Background())
	if err != nil {
		LogErr(c.Log, err)
	} else if len(duelResolveHashes) > 0 {
		duelsN := strconv.FormatUint(uint64(len(duelResolveHashes)), 10)

		c.Log.Info("on_event_date crypto resolve cron: resolved " + duelsN + " duels")
		for duelID, hashes := range duelResolveHashes {
			c.Log.Info("duel was resolved on deadline",
				zap.String("duel_id", duelID.String()),
				zap.Strings("hashes", hashes))
		}
	}
}

func (c *DuelCron) resolveCryptoBeforeEventDate() {
	duelResolveHashes, err := c.DuelService.ResolveCryptoBeforeEventDate(
		context.Background(),
		c.duelsResolveJoinNotBefore)
	if err != nil {
		LogErr(c.Log, err)
		return
	}

	duelsN := strconv.FormatUint(uint64(len(duelResolveHashes)), 10)
	c.Log.Info("before_event_date crypto resolve cron: resolved " + duelsN + " duels")

	for duelID, hashes := range duelResolveHashes {
		c.Log.Info("duel: "+duelID.String(), zap.Strings("hashes", hashes))
	}
}

func (c *DuelCron) resolveDuckPointOnEventDate() {
	resolvedDuels, err := c.DuelService.ResolveDuckPointOnEventDate(context.Background())
	if err != nil {
		LogErr(c.Log, err)
	} else if len(resolvedDuels) > 0 {
		duelsN := strconv.FormatUint(uint64(len(resolvedDuels)), 10)

		c.Log.Info("on_event_date duck point resolve cron: resolved " + duelsN + " duels")
		for _, id := range resolvedDuels {
			c.Log.Info("duel was resolved on event_date",
				zap.String("duel_id", id.String()))
		}
	}
}

func (c *DuelCron) resolveDuckPointBeforeEventDate() {
	duelResolveHashes, err := c.DuelService.ResolveDuckPointBeforeEventDate(
		context.Background(),
		c.duelsResolveJoinNotBefore)
	if err != nil {
		LogErr(c.Log, err)
		return
	}

	duelsN := strconv.FormatUint(uint64(len(duelResolveHashes)), 10)
	c.Log.Info("before_event_date duck point resolve cron: resolved "+duelsN+" duels",
		zap.Any("ids", duelResolveHashes))
}

func (c *DuelCron) autoCancelOldDuels() {
	ids, err := c.DuelService.AutoCancelDuels(context.Background())
	if err != nil {
		LogErr(c.Log, err)
		return
	}

	duelsN := strconv.FormatUint(uint64(len(ids)), 10)
	c.Log.Info("auto_cancel_duels cron: cancelled "+duelsN+"duels", zap.Any("ids", ids))
}

var CMCIDs = map[uint64]string{
	1:     "BTC",
	5426:  "SOL",
	1027:  "ETH",
	1839:  "BNB",
	1975:  "LINK",
	74:    "DOGE",
	29210: "JUP",
	22974: "TAO",
	6535:  "NEAR",
	20947: "SUI",
	3794:  "ATOM",
	52:    "XPR",
	22691: "STRK",
}

func (c *DuelCron) autoCreateDuelsShortTerm() {
	ctx := context.Background()

	err := c.createDuelsWithTopicCrypto(ctx, &c.AutoDuelsConf.ShortTerm, CMCIDs)
	if err != nil {
		LogErr(c.Log, err)
	}
}

func (c *DuelCron) autoCreateDuelsLongTerm() {
	ctx := context.Background()

	err := c.createDuelsWithTopicCrypto(ctx, &c.AutoDuelsConf.LongTerm, CMCIDs)
	if err != nil {
		LogErr(c.Log, err)
	}
}

func (c *DuelCron) createDuelsWithTopicCrypto(
	ctx context.Context,
	params *config.AutoCreatedDuels,
	cmcIDs map[uint64]string,
) error {
	ids := make([]uint64, 0, len(cmcIDs))
	for id := range cmcIDs {
		ids = append(ids, id)
	}

	coins, err := c.CoinService.GetCoinsByCMCIDs(ctx, ids)
	if err != nil {
		return err
	}

	coinPrices, err := c.CoinService.GetCurrentCoinsPriceByIDs(ids...)
	if err != nil {
		return apperrors.ServiceUnavailable("failed to get token prices", err)
	}

	duelOwner := &model.User{
		ID:       uuid.MustParse("17ba4ff3-0fd5-4b9e-aa2c-5c91d126678e"),
		Username: "DuckVader",
		Role:     mtype.AdminRole,
	}

	now := time.Now().UTC()
	duelsEventDate := time.Date(
		now.Year(), now.Month(), now.Day()+params.DaysToEventDate,
		0, 0, 0, 0, now.Location())

	duelParams := &service.AutoDuelsParams{
		DuelPrice:     params.CryptoPrice,
		Owner:         duelOwner,
		EventDate:     duelsEventDate,
		PaymentType:   mtype.PaymentTypeUSDC,
		Commission:    params.Commission,
		CoinPriceDiff: params.PriceDiffCoefficient,
	}

	// crypto duels creation
	duels := c.DuelService.CreateDuelsWithCoinPrices(coins, coinPrices, duelParams)
	err = c.DuelService.CreateDuelsBulk(ctx, duels)
	if err != nil {
		return err
	}

	// duck point duels creation
	duelParams.PaymentType = mtype.PaymentTypeDuck
	duelParams.DuelPrice = params.DuckPointPrice

	duels = c.DuelService.CreateDuelsWithCoinPrices(coins, coinPrices, duelParams)
	err = c.DuelService.CreateDuelsWithJoin(ctx, duels, duelOwner)
	if err != nil {
		return err
	}

	return nil
}

func (c *DuelCron) start(_ context.Context) error {
	c.Log.Info("duel cron started")
	c.Cron.Start()
	return nil
}

func (c *DuelCron) stop(_ context.Context) error {
	c.Log.Info("duel cron stopped")
	c.Cron.Stop()
	return nil
}
