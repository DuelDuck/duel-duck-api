package repository

import (
	"context"
	"github.com/uptrace/bun"
	"gitlab.com/duel-duck/duel-duck-api/internal/model"
	"gitlab.com/duel-duck/duel-duck-api/pkg/repository"
)

type StatsRepository struct {
	DB repository.DB
}

func NewStatsRepository(db repository.DB) *StatsRepository {
	return &StatsRepository{DB: db}
}

func (r *StatsRepository) WithTx(tx bun.Tx) *StatsRepository {
	return &StatsRepository{
		DB: r.DB.WithTx(tx),
	}
}

func (r *StatsRepository) CountTodayNewUsers(ctx context.Context) (int, error) {
	newUsersCount, err := r.DB.NewSelect().
		Model((*model.User)(nil)).
		Where("created_at >= ?", bun.Safe(`NOW() - INTERVAL '24 hours'`)).
		Count(ctx)

	return newUsersCount, err
}

func (r *StatsRepository) CountDuelWinners(ctx context.Context) (int, error) {
	winnersCount, err := r.DB.NewSelect().
		Model((*model.Player)(nil)).
		Where("is_winner = true").
		Count(ctx)

	return winnersCount, err
}

func (r *StatsRepository) CountDuelWinnersToday(ctx context.Context) (int, error) {
	winnersCount, err := r.DB.NewSelect().
		Model((*model.Player)(nil)).
		Where("is_winner = true").
		Where("created_at >= ?", bun.Safe("NOW() - INTERVAL '24 hours'")).
		Count(ctx)

	return winnersCount, err
}

func (r *StatsRepository) CountCompletedDuels(ctx context.Context) (int, error) {
	completedDuels, err := r.DB.NewSelect().
		Model((*model.Duel)(nil)).
		Where(
			"status IN (?)",
			bun.In([]uint8{
				model.DuelStatusResolved,
				model.DuelStatusRefund,
			}),
		).
		Count(ctx)

	return completedDuels, err
}

func (r *StatsRepository) CountCompletedDuelsToday(ctx context.Context) (int, error) {
	completedDuels, err := r.DB.NewSelect().
		Model((*model.Duel)(nil)).
		Where(
			"status IN (?)",
			bun.In([]uint8{
				model.DuelStatusResolved,
				model.DuelStatusRefund,
			}),
		).
		Where("created_at >= ?", bun.Safe("NOW() - INTERVAL '24 hours'")).
		Count(ctx)

	return completedDuels, err
}

func (r *StatsRepository) CountDuels(ctx context.Context) (int, error) {
	duelsCount, err := r.DB.NewSelect().
		Model((*model.Duel)(nil)).
		Where("status >= 1").
		Count(ctx)

	return duelsCount, err
}

func (r *StatsRepository) CountDuelsToday(ctx context.Context) (int, error) {
	duelsCount, err := r.DB.NewSelect().
		Model((*model.Duel)(nil)).
		Where("status >= 1").
		Where("created_at >= ?", bun.Safe("NOW() - INTERVAL '24 hours'")).
		Count(ctx)

	return duelsCount, err
}
