package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/uptrace/bun"
	"gitlab.com/duel-duck/duel-duck-api/internal/model"
	"gitlab.com/duel-duck/duel-duck-api/pkg/mtype"
	"gitlab.com/duel-duck/duel-duck-api/pkg/repository"
	"go.uber.org/zap"
)

type DuelRepository struct {
	repository.Generic[model.Duel, uuid.UUID]
}

func NewDuelRepository(
	genericRepository repository.Generic[model.Duel, uuid.UUID],
) *DuelRepository {
	return &DuelRepository{
		Generic: genericRepository,
	}
}

func (r *DuelRepository) WithTx(tx bun.Tx) *DuelRepository {
	return &DuelRepository{Generic: r.Generic.WithTx(tx)}
}

func (r *DuelRepository) CreateBulk(ctx context.Context, duels []model.Duel) error {
	_, err := r.DB.NewInsert().Model(&duels).Exec(ctx)
	return err
}

func (r *DuelRepository) GetAllDuels(
	ctx context.Context,
	userID uuid.UUID,
	options *repository.Options,
) ([]model.DuelShow, error) {
	duels := make([]model.DuelShow, 0)

	q := r.DB.NewSelect().
		Model(&duels).
		ColumnExpr("duels.*").
		ColumnExpr("(p.user_id IS NOT NULL) AS joined").
		ColumnExpr("p.final_status as player_status").
		ColumnExpr("p.answer AS your_answer").
		ColumnExpr("COALESCE(yes_counts.yes_count, 0) AS yes_count").
		ColumnExpr("duels.players_count - COALESCE(yes_counts.yes_count, 0) as no_count").
		Join("left join players p on p.duel_id = duels.id AND p.user_id = ?", userID).
		Join("left join (select duel_id, COUNT(*) AS yes_count FROM players WHERE answer = 1 GROUP BY duel_id) AS yes_counts ON yes_counts.duel_id = duels.id")
	q = options.Apply(q)

	if err := q.Scan(ctx); err != nil {
		return nil, err
	}

	return duels, nil
}

func (r *DuelRepository) GetAllDuelsWhereParticipate(
	ctx context.Context,
	userID uuid.UUID,
	options *repository.Options,
) ([]model.DuelShow, error) {
	duels := make([]model.DuelShow, 0)

	q := r.DB.NewSelect().
		Model(&duels).
		ColumnExpr("duels.*").
		ColumnExpr("(p.user_id IS NOT NULL) AS joined").
		ColumnExpr("p.final_status as player_status").
		ColumnExpr("p.answer AS your_answer").
		ColumnExpr("COALESCE(yes_counts.yes_count, 0) AS yes_count").
		ColumnExpr("duels.players_count - COALESCE(yes_counts.yes_count, 0) as no_count").
		Join("inner join players p on p.duel_id = duels.id and p.user_id = ?", userID).
		Join("left join (select duel_id, COUNT(*) AS yes_count FROM players WHERE answer = 1 GROUP BY duel_id) AS yes_counts ON yes_counts.duel_id = duels.id")
	q = options.Apply(q)

	if err := q.Scan(ctx); err != nil {
		return nil, err
	}

	return duels, nil
}

func (r *DuelRepository) GetDuelShowByID(
	ctx context.Context,
	userID uuid.UUID,
	duelID uuid.UUID,
) (*model.DuelShow, error) {
	duel := new(model.DuelShow)

	q := r.DB.NewSelect().
		Model(duel).
		ColumnExpr("duels.*").
		ColumnExpr("(p.user_id IS NOT NULL) AS joined").
		ColumnExpr("p.final_status as player_status").
		ColumnExpr("p.answer AS your_answer").
		ColumnExpr("COALESCE(yes_counts.yes_count, 0) AS yes_count").
		ColumnExpr("duels.players_count - COALESCE(yes_counts.yes_count, 0) as no_count").
		Join("left join players p on p.duel_id = duels.id AND p.user_id = ?", userID).
		Join("left join (select duel_id, COUNT(*) AS yes_count FROM players WHERE answer = 1 GROUP BY duel_id) AS yes_counts ON yes_counts.duel_id = duels.id").
		Where("duels.id = ?", duelID)

	if err := q.Scan(ctx); err != nil {
		return nil, err
	}

	return duel, nil
}

func (r *DuelRepository) GetHistoryByUserID(
	ctx context.Context,
	userID uuid.UUID,
	options *repository.Options,
) ([]model.DuelShow, error) {
	duels := make([]model.DuelShow, 0)

	q := r.DB.NewSelect().
		Model(&duels).
		ColumnExpr("duels.*").
		ColumnExpr("(p.user_id IS NOT NULL) AS joined").
		ColumnExpr("p.final_status as player_status").
		ColumnExpr("p.answer as your_answer").
		ColumnExpr("COALESCE(yes_counts.yes_count, 0) AS yes_count").
		ColumnExpr("duels.players_count - COALESCE(yes_counts.yes_count, 0) as no_count").
		Join("left join players p on p.duel_id = duels.id AND p.user_id = ?", userID).
		Join("left join (select duel_id, COUNT(*) AS yes_count FROM players WHERE answer = 1 GROUP BY duel_id) AS yes_counts ON yes_counts.duel_id = duels.id").
		Where("p.user_id = ? and duels.status in (?, ?, ?, ?)",
			userID,
			model.DuelStatusAutoCancelled,
			model.DuelStatusAdminCancelled,
			model.DuelStatusResolved,
			model.DuelStatusRefund,
		)
	q = options.Apply(q)

	if err := q.Scan(ctx); err != nil {
		return nil, err
	}

	return duels, nil
}

func (r *DuelRepository) GetUserDuels(
	ctx context.Context,
	userID uuid.UUID,
	options *repository.Options,
) ([]model.DuelShow, error) {
	duels := make([]model.DuelShow, 0)

	q := r.DB.NewSelect().
		Model(&duels).
		ColumnExpr("duels.*").
		ColumnExpr("(p.user_id IS NOT NULL) as joined").
		ColumnExpr("p.final_status as player_status").
		ColumnExpr("p.answer as your_answer").
		ColumnExpr("COALESCE(yes_counts.yes_count, 0) AS yes_count").
		ColumnExpr("duels.players_count - COALESCE(yes_counts.yes_count, 0) as no_count").
		Join("left join players p on p.duel_id = duels.id AND p.user_id = ?", userID).
		Join("left join (select duel_id, COUNT(*) AS yes_count FROM players WHERE answer = 1 GROUP BY duel_id) AS yes_counts ON yes_counts.duel_id = duels.id").
		Where("duels.owner_id = ?", userID)
	q = options.Apply(q)

	if err := q.Scan(ctx); err != nil {
		return nil, err
	}

	return duels, nil
}

func (r *DuelRepository) JoinDuel(
	ctx context.Context,
	userID uuid.UUID,
	req *model.JoinDuelReq,
	duel *model.Duel,
) (*model.Player, error) {
	player := &model.Player{
		ID:        uuid.New(),
		UserID:    userID,
		DuelID:    req.DuelID,
		Answer:    req.Answer,
		CreatedAt: time.Now(),
	}
	_, err := r.DB.NewInsert().Model(player).Exec(ctx)
	if err != nil {
		return nil, err
	}

	_, err = r.DB.NewUpdate().
		Model((*model.Duel)(nil)).
		Set("players_count = players_count + 1").
		Set("status = ?", duel.Status).
		Where("id = ?", req.DuelID).
		Exec(ctx)
	if err != nil {
		return nil, err
	}

	return player, nil
}

func (r *DuelRepository) JoinDuelBulk(
	ctx context.Context,
	userID uuid.UUID,

	duels []model.Duel,
	answers []uint8,
) error {
	go func() {
		err := recover()
		zap.L().Error("error caught", zap.Any("err", err))
	}()
	if len(duels) != len(answers) {
		return fmt.Errorf("number of duels does not match number of answers")
	}

	players := make([]model.Player, 0, len(duels))
	for i, duel := range duels {
		player := model.Player{
			ID:        uuid.New(),
			UserID:    userID,
			DuelID:    duel.ID,
			Answer:    answers[i],
			CreatedAt: time.Now(),
		}

		players = append(players, player)
	}

	_, err := r.DB.NewInsert().Model(&players).Exec(ctx)
	if err != nil {
		return err
	}

	values := r.DB.NewValues(&players)

	_, err = r.DB.NewUpdate().
		With("_data", values).
		Model((*model.Duel)(nil)).
		TableExpr("_data").
		Set("players_count = players_count + 1").
		Where("duels.id = _data.duel_id").
		Exec(ctx)
	if err != nil {
		return err
	}

	return nil
}

func (r *DuelRepository) GetTopicCounters(
	ctx context.Context,
	opts *repository.Options,
) ([]model.DuelTopicCounter, error) {

	result := make([]model.DuelTopicCounter, 0)

	q := r.DB.NewSelect().
		Table("duels").
		Column("topic").
		ColumnExpr("coalesce(COUNT(*), 0) AS total_duels").
		Group("topic")
	q = opts.Apply(q)
	if err := q.Scan(ctx, &result); err != nil {
		return nil, err
	}

	return result, nil
}

func (r *DuelRepository) GetIndividualTopicCounters(
	ctx context.Context,
	opts repository.Options,
	userID uuid.UUID,
) ([]model.DuelTopicCounter, error) {
	result := make([]model.DuelTopicCounter, 0)

	q := r.DB.NewSelect().
		Table("duels").
		Column("topic").
		ColumnExpr("COALESCE(COUNT(duels.id), 0) AS total_duels").
		Join("LEFT JOIN players on players.duel_id = duels.id AND players.user_id = ?", userID).
		Where("players.id is NULL").
		Group("topic")

	q = opts.Apply(q)

	if err := q.Scan(ctx, &result); err != nil {
		return nil, err
	}

	return result, nil
}

func (r *DuelRepository) GetAllByDuelInfoEntity(
	ctx context.Context,
	subtopic string,
	entityType string,
) ([]model.DuelInfoEdit, error) {
	result := make([]model.DuelInfoEdit, 0)

	err := r.DB.NewSelect().
		Model(&result).
		Column("id").
		ColumnExpr(
			"?->? AS entity_value",
			bun.Ident("duel_info"),
			entityType,
		).
		Where("subtopic = ?", subtopic).
		Where("duel_info::jsonb ? ?", bun.Safe("?"), entityType).
		Scan(ctx)

	if err != nil {
		return nil, err
	}

	return result, nil
}

func (r *DuelRepository) UpdateDuelInfoWithNewEntityName(
	ctx context.Context,
	entityType string,
	duels []model.DuelInfoEdit,
) error {
	values := r.DB.NewValues(&duels)

	_, err := r.DB.NewUpdate().
		With("_data", values).
		Model((*model.DuelInfoEdit)(nil)).
		TableExpr("_data").
		SetColumn(
			"duel_info",
			"jsonb_set(duel_info::jsonb, ?, _data.entity_value::jsonb, ?)",
			fmt.Sprintf("{%s}", entityType),
			false, // 'create_if_missing' flag. No need to create a field if didn't exist
		).
		Where("duels.id = _data.id").
		Exec(ctx)

	return err
}

func (r *DuelRepository) RefreshMaterializedView(
	ctx context.Context,
	viewName string,
) error {
	_, err := r.DB.NewRaw("REFRESH MATERIALIZED VIEW ?", bun.Ident(viewName)).Exec(ctx)

	return err
}

func (r *DuelRepository) GetTotalLeaderboard(
	ctx context.Context,
	opts repository.Options,
	leaderboardName string,
) ([]model.Leader, int, error) {
	leaders := make([]model.Leader, 0)

	q := r.DB.NewSelect().
		TableExpr("?", bun.Safe(leaderboardName)).
		ColumnExpr("ROW_NUMBER() OVER (ORDER BY SUM(pnl) DESC) AS rank").
		Column("user_id").
		Column("username").
		ColumnExpr("SUM(total_duels) AS total_duels").
		ColumnExpr("SUM(victories) AS victories").
		ColumnExpr("SUM(spent) AS spent").
		ColumnExpr("SUM(earned) AS earned").
		ColumnExpr("SUM(pnl) AS pnl").
		Group("user_id", "username")

	q = opts.Apply(q)
	count, err := q.ScanAndCount(ctx, &leaders)
	if err != nil {
		return nil, 0, err
	}

	return leaders, count, nil
}

func (r *DuelRepository) FindUserRank(
	ctx context.Context,
	opts repository.Options,
	userID uuid.UUID,
	leaderboardName string,
) (*model.Leader, error) {
	leader := new(model.Leader)

	cteQuery := r.DB.NewSelect().
		ColumnExpr("ROW_NUMBER() OVER (ORDER BY SUM(?) DESC) AS rank",
			bun.Ident(opts.Order.OrderBy)).
		Column("user_id").
		Column("username").
		ColumnExpr("SUM(total_duels) AS total_duels").
		ColumnExpr("SUM(victories) AS victories").
		ColumnExpr("SUM(spent) AS spent").
		ColumnExpr("SUM(earned) AS earned").
		ColumnExpr("SUM(pnl) AS pnl").
		Table(leaderboardName).
		Group("user_id", "username")
	cteQuery = opts.Apply(cteQuery)

	err := r.DB.NewSelect().
		With("ranked_leaderboard", cteQuery).
		Table("ranked_leaderboard").
		Where("user_id = ?", userID).
		Scan(ctx, leader)
	if repository.IsErrNoRows(err) {
		return nil, nil
	}

	if err != nil {
		return nil, err
	}

	return leader, nil
}

func (r *DuelRepository) GetActiveCryptoDuelsWithTopicCrypto(
	ctx context.Context,
	paymentType mtype.PaymentType,
) ([]model.Duel, error) {
	duels := make([]model.Duel, 0)

	err := r.DB.NewSelect().
		Model(&duels).
		Where("duels.payment_type = ?", paymentType).
		Where("duels.status = ?", model.DuelStatusInProcess).
		Where("duels.topic = ?", model.DuelTopicCrypto).
		Where("duels.event_date > ?", time.Now()).
		Scan(ctx)
	if err != nil {
		return nil, err
	}

	return duels, nil
}

func (r *DuelRepository) GetEndedDuelsWithTopicCrypto(
	ctx context.Context,
	paymentType mtype.PaymentType,
) ([]model.Duel, error) {
	duels := make([]model.Duel, 0)

	err := r.DB.NewSelect().
		Model(&duels).
		Where("duels.payment_type = ?", paymentType).
		Where("duels.status = ?", model.DuelStatusInProcess).
		Where("duels.topic = ?", model.DuelTopicCrypto).
		Where("duels.event_date < ?", time.Now()).
		Scan(ctx)
	if err != nil {
		return nil, err
	}

	return duels, nil
}

const TournamentID = "d11d0ee6-e74d-4844-b22f-3a4962e57f9e"

func (r *DuelRepository) JoinedTournamentDuelFirstTime(
	ctx context.Context,
	userID uuid.UUID,
) (bool, error) {
	joinedTournamentDuelsCount, err := r.DB.NewSelect().
		Model((*model.Duel)(nil)).
		Where("tournament_id = ?", TournamentID).
		Where("status >=  ?", model.DuelStatusInProcess).
		Join("INNER JOIN players ON duels.id = players.duel_id AND players.user_id = ?", userID).
		Count(ctx)
	if err != nil {
		return false, err
	}

	return joinedTournamentDuelsCount == 1, nil
}

func (r *DuelRepository) GetAllCoinsFromActiveDuels(
	ctx context.Context,
) ([]model.CMCID, error) {
	var coinIDs []model.CMCID

	err := r.DB.NewSelect().
		Table("duels").
		Where("duels.status = ?", model.DuelStatusInProcess).
		Where("duels.topic = ?", model.DuelTopicCrypto).
		ColumnExpr("DISTINCT duel_info->>'coin_id'").
		Scan(ctx, &coinIDs)
	if err != nil {
		return nil, err
	}

	return coinIDs, nil
}

func (r *DuelRepository) GetOldDuelsInReview(
	ctx context.Context,
	paymentType mtype.PaymentType,
) ([]model.Duel, error) {
	duels := make([]model.Duel, 0)

	err := r.DB.NewSelect().
		Model(&duels).
		Where("duels.payment_type = ?", paymentType).
		Where("duels.status <= ?", model.DuelStatusInReview).
		WhereGroup(" AND ", func(q *bun.SelectQuery) *bun.SelectQuery {
			return q.
				WhereOr("duels.created_at < ?", time.Now().AddDate(0, 0, -7)).
				WhereOr("duels.event_date < ?", time.Now())
		}).
		Scan(ctx)
	if err != nil {
		return nil, err
	}

	return duels, nil
}

func (r *DuelRepository) FindOwnerIDByDuelID(
	ctx context.Context,
	duelID uuid.UUID,
) (uuid.UUID, error) {
	var ownerID uuid.UUID

	err := r.DB.NewSelect().
		Table("duels").
		Column("owner_id").
		Where("id = ?", duelID).
		Scan(ctx, &ownerID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return uuid.Nil, nil
		}
		return uuid.Nil, err
	}

	return ownerID, nil
}

func (r *DuelRepository) SetTournamentID(
	ctx context.Context,
	duelID uuid.UUID,
	tournamentID *uuid.UUID,
) error {
	_, err := r.DB.NewUpdate().
		Model((*model.Duel)(nil)).
		Set("tournament_id = ?", tournamentID).
		Where("id = ?", duelID).
		Exec(ctx)

	return err
}
