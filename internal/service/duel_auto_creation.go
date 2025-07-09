package service

import (
	"context"
	"github.com/uptrace/bun"
	"gitlab.com/duel-duck/duel-duck-api/internal/model"
	"gitlab.com/duel-duck/duel-duck-api/pkg/apperrors"
	"gitlab.com/duel-duck/duel-duck-api/pkg/mtype"
	"math/rand"
	"time"
)

func newDuelInfoForAutoCreatedDuels(
	coin model.Coin,
	price float64,
	priceDiffCoefficient float64,
	direction int,
) model.CryptoDuelInfo {
	if direction == 0 {
		priceDiffCoefficient *= -1
	}

	price = model.RoundToOnePercent(price * (1 + priceDiffCoefficient))

	return model.CryptoDuelInfo{
		ID:    int(coin.ID),
		Price: price,
		Type:  direction,
	}
}

func (s *DuelService) CreateDuelsBulk(
	ctx context.Context,
	duels []model.Duel,
) error {
	err := s.DuelRepository.CreateBulk(ctx, duels)
	if err != nil {
		return apperrors.Internal("failed to bulk create crypto duels", err)
	}

	return nil
}

// FlipDaCoin randomly chooses true or false
func FlipDaCoin() bool {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	return r.Intn(2) == 1
}

type AutoDuelsParams struct {
	Owner         *model.User
	EventDate     time.Time
	PaymentType   mtype.PaymentType
	DuelPrice     uint64
	CoinPriceDiff float64
	Commission    uint8
}

func (s *DuelService) CreateDuelsWithCoinPrices(
	coins []model.Coin,
	coinPrices map[uint64]float64,
	params *AutoDuelsParams,
) []model.Duel {
	lessThanDuels := s.createDuelsWithCoinPrices(coins, coinPrices, params, 0)
	atLeastDuels := s.createDuelsWithCoinPrices(coins, coinPrices, params, 1)

	duels := make([]model.Duel, 0, len(lessThanDuels)+len(atLeastDuels))
	duels = append(duels, lessThanDuels...)
	duels = append(duels, atLeastDuels...)

	return duels
}

func (s *DuelService) CreateDuelsWithJoin(
	ctx context.Context,
	duels []model.Duel,
	duelOwner *model.User,
) error {
	answers := make([]uint8, 0, len(duels))
	for range len(duels) {
		if FlipDaCoin() {
			answers = append(answers, 1)
		} else {
			answers = append(answers, 0)
		}
	}

	err := s.TransactionManager.WithinTransaction(ctx, func(ctx context.Context, tx bun.Tx) error {
		err := s.DuelRepository.WithTx(tx).CreateBulk(ctx, duels)
		if err != nil {
			return apperrors.Internal("failed to bulk create duck point duels", err)
		}

		err = s.DuelRepository.WithTx(tx).JoinDuelBulk(ctx, duelOwner.ID, duels, answers)
		if err != nil {
			return apperrors.Internal("failed to bulk join duck point duels", err)
		}

		return nil
	})
	if err != nil {
		return err
	}

	return nil
}

func (s *DuelService) createDuelsWithCoinPrices(
	coins []model.Coin,
	coinPrices map[uint64]float64,
	params *AutoDuelsParams,
	direction int,
) []model.Duel {
	duels := make([]model.Duel, 0, len(coins))

	for _, coin := range coins {
		price, ok := coinPrices[coin.ID]
		if !ok {
			continue
		}

		duelInfo := newDuelInfoForAutoCreatedDuels(coin, price, params.CoinPriceDiff, direction)

		req := &model.CreateDuelReq{
			PaymentType: params.PaymentType,
			Topic:       model.DuelTopicCrypto,
			Question:    duelInfo.FormulateDuelQuestion(coin, params.EventDate),
			Deadline:    params.EventDate.Add(-2 * time.Hour),
			EventDate:   params.EventDate,
			DuelPrice:   params.DuelPrice,
			Commission:  uint64(params.Commission),
			DuelInfo:    duelInfo.Map(),
		}

		duel := model.DuelByCreateReqAdmin(req, params.Owner)
		duels = append(duels, *duel)
	}

	return duels
}
