package repository

import (
	"context"
	"fmt"
	"github.com/google/uuid"
	"github.com/uptrace/bun"
	"gitlab.com/duel-duck/duel-duck-api/internal/model"
	"gitlab.com/duel-duck/duel-duck-api/pkg/repository"
)

type TournamentRepository struct {
	repository.Generic[model.Tournament, uuid.UUID]
}

func NewTournamentRepository(
	genericRepository repository.Generic[model.Tournament, uuid.UUID],
) *TournamentRepository {
	return &TournamentRepository{
		Generic: genericRepository,
	}
}

func (r *TournamentRepository) WithTx(tx bun.Tx) *TournamentRepository {
	return &TournamentRepository{Generic: r.Generic.WithTx(tx)}
}

func (r *TournamentRepository) GetAll(
	ctx context.Context,
	opts *repository.Options,
) ([]model.Tournament, error) {
	tournaments := make([]model.Tournament, 0)

	q := r.DB.NewSelect().
		Model(&tournaments)
	q = opts.Apply(q)

	if err := q.Scan(ctx); err != nil {
		return nil, err
	}

	return tournaments, nil
}

func (r *TournamentRepository) UserTournaments(
	ctx context.Context,
	opts *repository.Options,
	userID uuid.UUID,
) ([]model.TournamentUserShow, error) {
	tournaments := make([]model.TournamentUserShow, 0)

	q := r.DB.NewSelect().
		Model(&tournaments).
		ColumnExpr(
			`t.id AS id,
			t.name AS name,
			t.start_date AS start_date,
			t.finish_date AS finish_date,
			t.players_count AS players_count,
			t.image_url AS image_url,
			t.bg_card_url AS bg_card_url,
			t.url as url,
			tl.pnl as pnl,
			tl.total_duels AS total_duels,
			tl.victories AS victories,
			tl.payment_type AS payment_type`,
		).
		Join("INNER JOIN tournaments AS t ON t.id = tp.tournament_id").
		Join(`
			INNER JOIN tournament_leaderboards AS tl 
			ON tl.tournament_id = tp.tournament_id 
			AND tl.user_id = tp.user_id`,
		).
		Where("tp.user_id = ?", userID)
	q = opts.Apply(q)

	if err := q.Scan(ctx); err != nil {
		return nil, err
	}

	return tournaments, nil
}

func (r *TournamentRepository) IncrementPlayersCount(
	ctx context.Context,
	userID uuid.UUID,
	tournamentID uuid.UUID,
) error {
	if tournamentID == uuid.Nil || userID == uuid.Nil {
		return nil
	}

	cte := r.DB.NewInsert().
		Model(&model.TournamentParticipant{
			UserID:       userID,
			TournamentID: tournamentID,
		}).
		Ignore().
		Returning("1")

	_, err := r.DB.NewUpdate().
		TableExpr("tournaments").
		Set("players_count = players_count + 1").
		Where("tournaments.id = ?", tournamentID).
		Where("EXISTS (SELECT 1 FROM ins)").
		With("ins", cte).
		Exec(ctx)

	return err
}

func (r *TournamentRepository) Rewards(
	ctx context.Context,
	tournamentID uuid.UUID,
) (model.TournamentRewards, error) {
	rewards := make(model.TournamentRewards, 0)

	err := r.DB.NewSelect().
		Model(&rewards).
		Where("tournament_id = ?", tournamentID).
		Scan(ctx)
	if err != nil {
		return nil, err
	}

	return rewards, nil
}

func (r *TournamentRepository) UserOwnsTournament(
	ctx context.Context,
	tournamentID uuid.UUID,
	userID uuid.UUID,
) (bool, error) {
	return r.DB.NewSelect().
		Model((*model.Tournament)(nil)).
		Where("tournament_id = ?", tournamentID).
		Where("owner_id = ?", userID).
		Exists(ctx)
}

func (r *TournamentRepository) Leaderboard(
	ctx context.Context,
	tournamentID uuid.UUID,
	opts *repository.Options,
) ([]model.TournamentLeaderboard, int, error) {
	leaders := make([]model.TournamentLeaderboard, 0)

	q := r.DB.NewSelect().
		Model(&leaders).
		Where("tournament_id = ?", tournamentID)
	q = opts.Apply(q)

	leadersCount, err := q.ScanAndCount(ctx)
	if err != nil {
		return nil, 0, err
	}

	return leaders, leadersCount, nil
}

func (r *TournamentRepository) TournamentRank(
	ctx context.Context,
	tournamentID uuid.UUID,
	userID uuid.UUID,
	opts *repository.Options,
) (*model.TournamentRank, error) {
	leader := new(model.TournamentRank)

	cte := r.DB.NewSelect().
		TableExpr("tournament_leaderboards AS tl").
		ColumnExpr("ROW_NUMBER() OVER (ORDER BY ? DESC) AS rank",
			bun.Ident(opts.Order.OrderBy)).
		ColumnExpr("tl.*").
		Where("tournament_id = ?", tournamentID)
	cte = opts.Apply(cte)

	err := r.DB.NewSelect().
		With("cte", cte).
		Table("cte").
		Where("user_id = ?", userID).
		Limit(1).
		Scan(ctx, leader)
	if err != nil {
		if repository.IsErrNoRows(err) {
			return nil, nil
		}

		return nil, err
	}

	return leader, nil
}

func (r *TournamentRepository) AlterLeaderboardSpent(
	ctx context.Context,
	user *model.User,
	duel *model.Duel,
) error {
	if duel.TournamentID == uuid.Nil {
		return nil
	}

	_, err := r.DB.NewInsert().
		Model(model.DefaultTournamentLeaderboardRecord(user, duel)).
		On("CONFLICT (tournament_id, user_id, payment_type) DO UPDATE").
		Set("total_duels = tl.total_duels + EXCLUDED.total_duels").
		Set("spent = tl.spent + EXCLUDED.spent").
		Set("pnl = tl.pnl - EXCLUDED.spent").
		Set("last_predicted_at = NOW()").
		Exec(ctx)

	return err
}

func (r *TournamentRepository) AlterLeaderboardEarned(
	ctx context.Context,
	duel *model.Duel,
	earned float64,
	userIDs uuid.UUIDs,
) error {
	if duel.TournamentID == uuid.Nil {
		return nil
	}

	fmt.Println(userIDs)

	_, err := r.DB.NewUpdate().
		Model((*model.TournamentLeaderboard)(nil)).
		Where("tournament_id = ?", duel.TournamentID).
		Where("user_id IN (?)", bun.In(userIDs)).
		Where("payment_type = ?", duel.PaymentType).
		Set("victories = victories + 1").
		Set("earned = earned + ?", earned).
		Set("pnl = pnl + ?", earned).
		Exec(ctx)

	return err
}

func (r *TournamentRepository) CreateRewards(
	ctx context.Context,
	rewards model.TournamentRewards,
) error {
	_, err := r.DB.NewInsert().
		Model(&rewards).
		Exec(ctx)

	return err
}

func (r *TournamentRepository) DeleteRewards(
	ctx context.Context,
	tournamentID uuid.UUID,
) error {
	_, err := r.DB.NewDelete().
		Model((*model.TournamentReward)(nil)).
		Where("tournament_id = ?", tournamentID).
		Exec(ctx)

	return err
}

func (r *TournamentRepository) CreateOffer(ctx context.Context, offer *model.TournamentOffer) error {
	_, err := r.DB.NewInsert().Model(offer).Exec(ctx)
	return err
}

func (r *TournamentRepository) GetAllOffers(ctx context.Context) ([]model.TournamentOffer, error) {
	offers := make([]model.TournamentOffer, 0)
	err := r.DB.NewSelect().Model(&offers).Order("created_at ASC").Scan(ctx)
	if err != nil {
		return nil, err
	}

	return offers, nil
}
