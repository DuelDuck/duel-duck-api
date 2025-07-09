package repository

import (
	"context"
	"github.com/google/uuid"
	"github.com/uptrace/bun"
	"gitlab.com/duel-duck/duel-duck-api/internal/model"
	"gitlab.com/duel-duck/duel-duck-api/pkg/mtype"
	"gitlab.com/duel-duck/duel-duck-api/pkg/repository"
)

type ModeratorStatsRepository struct {
	repository.Generic[model.ModeratorStats, uuid.UUID]
}

func NewModeratorStatsRepository(
	genericRepository repository.Generic[model.ModeratorStats, uuid.UUID],
) *ModeratorStatsRepository {
	return &ModeratorStatsRepository{
		Generic: genericRepository,
	}
}

func (r *ModeratorStatsRepository) WithTx(tx bun.Tx) *ModeratorStatsRepository {
	return &ModeratorStatsRepository{Generic: r.Generic.WithTx(tx)}
}

func (r *ModeratorStatsRepository) Create(ctx context.Context, stats *model.ModeratorStatsCreate) error {
	_, err := r.Generic.DB.NewInsert().Model(stats).Exec(ctx)
	return err
}

func (r *ModeratorStatsRepository) FindByModeratorIDWithOptions(
	ctx context.Context,
	opts *repository.Options,
	moderatorID uuid.UUID,
) ([]model.ModeratorStats, error) {
	stats := make([]model.ModeratorStats, 0)

	q := r.DB.NewSelect().
		With("_duels_data", r.DB.NewSelect().
			Table("duels").
			Column("duels.id").
			Column("duels.payment_type").
			Column("duels.duel_type").
			Column("duels.duel_price").
			Column("duels.question").
			Column("duels.owner_id").
			ColumnExpr("COUNT(players.id) AS player_count").
			Join("LEFT JOIN players ON duels.id = players.duel_id").
			Where("duels.status >= 2").
			Group("duels.id")).
		Model(&stats).
		Column("ms.moderator_id", "ms.duel_id", "ms.action_type", "ms.creation_pay", "ms.created_at").
		ColumnExpr("_duels_data.question AS duel_question").
		ColumnExpr("_duels_data.duel_type IS NULL AS is_custom").
		ColumnExpr(
			`CASE 
				WHEN _duels_data.payment_type = ? OR ms.action_type != ? THEN 0
				ELSE _duels_data.player_count::DOUBLE PRECISION
        	END AS bonuses`,
			mtype.PaymentTypeDuck, model.ActionCreation,
		).
		ColumnExpr("_duels_data.player_count * _duels_data.duel_price AS duel_bank").
		Where("moderator_id = ?", moderatorID).
		Join("LEFT JOIN _duels_data ON ms.duel_id = _duels_data.id")
	q = opts.Apply(q)

	err := q.Scan(ctx)
	if err != nil {
		return nil, err
	}

	return stats, nil
}

func (r *ModeratorStatsRepository) FindAllWithOptions(
	ctx context.Context,
	opts *repository.Options,
) ([]model.ModeratorStats, error) {
	stats := make([]model.ModeratorStats, 0)

	q := r.DB.NewSelect().
		With("_duels_data", r.DB.NewSelect().
			Table("duels").
			Column("duels.id").
			Column("duels.payment_type").
			Column("duels.duel_type").
			Column("duels.duel_price").
			Column("duels.question").
			Column("duels.owner_id").
			ColumnExpr("COUNT(players.id) AS player_count").
			Join("LEFT JOIN players ON duels.id = players.duel_id").
			Where("duels.status >= 2").
			Group("duels.id")).
		Model(&stats).
		Column("ms.moderator_id", "ms.duel_id", "ms.action_type", "ms.creation_pay", "ms.created_at").
		ColumnExpr("_duels_data.question AS duel_question").
		ColumnExpr("_duels_data.duel_type IS NULL AS is_custom").
		ColumnExpr(
			`CASE 
				WHEN _duels_data.payment_type = ? OR ms.action_type != ? THEN 0
				ELSE _duels_data.player_count::DOUBLE PRECISION
        	END AS bonuses`,
			mtype.PaymentTypeDuck, model.ActionCreation,
		).
		ColumnExpr("_duels_data.player_count * _duels_data.duel_price AS duel_bank").
		Join("LEFT JOIN _duels_data ON ms.duel_id = _duels_data.id")
	q = opts.Apply(q)

	err := q.Scan(ctx)
	if err != nil {
		return nil, err
	}

	return stats, nil
}
