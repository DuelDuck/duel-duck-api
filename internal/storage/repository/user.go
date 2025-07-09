package repository

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/uptrace/bun"

	"gitlab.com/duel-duck/duel-duck-api/internal/model"
	"gitlab.com/duel-duck/duel-duck-api/pkg/mtype"
	"gitlab.com/duel-duck/duel-duck-api/pkg/repository"
)

type UserRepository struct {
	repository.Generic[model.User, uuid.UUID]
}

func (r *UserRepository) WithTx(tx bun.Tx) *UserRepository {
	return &UserRepository{Generic: r.Generic.WithTx(tx)}
}

func NewUserRepository(
	genericRepository repository.Generic[model.User, uuid.UUID],
) *UserRepository {
	return &UserRepository{
		Generic: genericRepository,
	}
}

func (r *UserRepository) Edit(
	ctx context.Context,
	userID uuid.UUID,
	userData *model.UserEditReq,
) error {
	_, err := r.DB.NewUpdate().
		Model(userData).
		Where("id = ?", userID).
		OmitZero().
		Exec(ctx)
	return err
}

func (r *UserRepository) FindByReferralToken(
	ctx context.Context,
	referralToken string,
) (*model.User, error) {
	var user = new(model.User)

	err := r.DB.NewSelect().
		Model(user).
		Where("referral_token = ?", referralToken).
		Scan(ctx)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}

		return nil, err
	}

	return user, nil
}

func (r *UserRepository) FindByPublicAddress(
	ctx context.Context,
	publicAddress string,
) (*model.User, error) {
	var user = new(model.User)

	err := r.DB.NewSelect().
		Model(user).
		Where("public_address = ?", publicAddress).
		Scan(ctx)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}

		return nil, err
	}

	return user, nil
}

func (r *UserRepository) FindByEmail(
	ctx context.Context,
	email mtype.Email,
) (*model.User, error) {
	var user = new(model.User)

	err := r.DB.NewSelect().
		Model(user).
		Where("email = ?", email.String()).
		Scan(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}

	if err != nil {
		return nil, err
	}

	return user, nil
}

func (r *UserRepository) GetByPublicAddress(
	ctx context.Context,
	address string,
) (*model.User, error) {
	var user = new(model.User)

	err := r.DB.NewSelect().
		Model(user).
		Where("public_address = ?", address).
		Scan(ctx)
	if err != nil {
		return nil, err
	}

	return user, nil
}

func (r *UserRepository) ReferralStats(
	ctx context.Context,
	id uuid.UUID,
) (*model.UserReferralStats, error) {
	user := new(model.UserReferralStats)

	err := r.DB.
		NewSelect().
		Model(user).
		Where("id = ?", id).
		Scan(ctx)
	if err != nil {
		return nil, err
	}

	return user, nil
}

func (r *UserRepository) UserReferrals(
	ctx context.Context,
	id uuid.UUID,
	opts *repository.Options,
) ([]*model.UserReferralShow, error) {
	stats := make([]*model.UserReferralShow, 0)

	q := r.DB.
		NewSelect().
		Model(&stats).
		Column(
			"ur.referrer_id",
			"ur.referral_id",
			"ur.income_usdc",
			"ur.income_dp",
			"ur.referral_commission_end",
			"ur.created_at",
		).
		ColumnExpr("u.username AS username").
		Join("INNER JOIN users AS u ON u.id = ur.referral_id").
		Where("ur.referrer_id = ?", id)

	q = opts.Apply(q)
	if err := q.Scan(ctx); err != nil {
		return nil, err
	}

	return stats, nil
}

func (r *UserRepository) FindByTelegramID(
	ctx context.Context,
	telegramID string,
) (*model.User, error) {
	var user = new(model.User)

	err := r.DB.NewSelect().
		Model(user).
		Where("telegram_id = ?", telegramID).
		Scan(ctx)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}

		return nil, err
	}

	return user, nil
}

func (r *UserRepository) AlterBalanceByID(
	ctx context.Context,
	id uuid.UUID,
	amount int,
) (mtype.Balance, error) {
	var balance mtype.Balance

	err := r.DB.NewUpdate().
		Model((*model.User)(nil)).
		Set("balance = balance + ?", amount).
		Where("id = ?", id).
		Returning("balance").
		Scan(ctx, &balance)
	if err != nil {
		return 0, err
	}

	return balance, err
}

func (r *UserRepository) GetLevelAndXPByID(
	ctx context.Context,
	id uuid.UUID,
) (uint32, uint32, error) {
	var (
		xp    uint32
		level uint32
	)

	err := r.DB.NewSelect().
		Model((*model.User)(nil)).
		Column("level").
		Column("current_xp").
		Where("id = ?", id).
		Scan(ctx, &level, &xp)
	if err != nil {
		return 0, 0, err
	}

	return level, xp, err
}

func (r *UserRepository) GiveRewardByID(
	ctx context.Context,
	id uuid.UUID,
	amount int,
	newLevel uint32,
	newXP uint32,
) (mtype.Balance, error) {
	var balance mtype.Balance

	err := r.DB.NewUpdate().
		Model((*model.User)(nil)).
		Set("balance = balance + ?", amount).
		Set("current_xp = ?", newXP).
		Set("level = ?", newLevel).
		Where("id = ?", id).
		Returning("balance").
		Scan(ctx, &balance)
	if err != nil {
		return 0, err
	}

	return balance, nil
}

func (r *UserRepository) UpdateDuelWinnersBalance(
	ctx context.Context,
	duelID uuid.UUID,
	correctAnswer uint8,
	amount uint64,
	joinNotBefore time.Time,
) error {
	_, err := r.DB.NewUpdate().
		Model((*model.User)(nil)).
		TableExpr("players AS p").
		Set("balance = balance + ?", amount).
		Where("p.user_id = u.id").
		Where("p.duel_id = ?", duelID).
		Where("p.answer = ?", correctAnswer).
		Where("p.created_at <= ?", joinNotBefore).
		Exec(ctx)
	if err != nil {
		return err
	}

	_, err = r.DB.NewUpdate().
		Model((*model.User)(nil)).
		TableExpr("players AS p").
		Set("balance = balance + ?", model.PremiumPlayersConsolationReward).
		Where("p.user_id = u.id").
		Where("p.duel_id = ?", duelID).
		Where("u.is_premium = true").
		Where("p.answer != ?", correctAnswer).
		Where("p.created_at <= ?", joinNotBefore).
		Exec(ctx)

	return err
}

func (r *UserRepository) PartialDuelRefund(
	ctx context.Context,
	duelID uuid.UUID,
	duelPrice uint64,
	joinNotBefore time.Time,
) error {
	_, err := r.DB.NewUpdate().
		Model((*model.User)(nil)).
		TableExpr("players AS p").
		Set("balance = balance + ?", duelPrice).
		Where("p.user_id = u.id").
		Where("p.duel_id = ?", duelID).
		Where("p.created_at > ?", joinNotBefore).
		Exec(ctx)

	return err
}

func (r *UserRepository) RefundOnDuelCancel(
	ctx context.Context,
	duelID uuid.UUID,
	amount uint64,
) error {
	_, err := r.DB.NewUpdate().
		Model((*model.User)(nil)).
		TableExpr("players AS p").
		Set("balance = balance + ?", amount).
		Where("p.user_id = u.id").
		Where("p.duel_id = ?", duelID).
		Exec(ctx)

	return err
}

func (r *UserRepository) CountUsers(
	ctx context.Context,
) (int, error) {
	userCount, err := r.DB.NewSelect().
		Model((*model.User)(nil)).
		Count(ctx)

	return userCount, err
}

func (r *UserRepository) AddDuckPointsTransactionToHistory(
	ctx context.Context,
	transaction *model.DuckPointsTransaction,
) error {
	_, err := r.DB.NewInsert().Model(transaction).Exec(ctx)
	return err
}

func (r *UserRepository) GetDuckPointsTransferHistory(
	ctx context.Context,
	publicAddress string,
	opts repository.Options,
) ([]model.DuckPointsTransaction, error) {
	transactions := make([]model.DuckPointsTransaction, 0)
	q := r.DB.NewSelect().
		Model(&transactions).
		Where("(dpt.sender_address = ? OR dpt.recipient_address = ?)", publicAddress, publicAddress)

	q = opts.Apply(q)
	if err := q.Scan(ctx); err != nil {
		return nil, err
	}

	return transactions, nil
}

func (r *UserRepository) FindStatsByID(
	ctx context.Context,
	id uuid.UUID,
) (*model.UserStats, error) {
	var userStats = new(model.UserStats)

	// todo: add columns for users (ddp_earned, usdc_earned)
	err := r.DB.NewSelect().
		Model(userStats).
		Column("u.id").
		Column("u.level").
		Column("u.current_xp").
		ColumnExpr(` 
        (
            SELECT COALESCE(SUM(cr.overall_reward), 0)
            FROM completed_tasks_rewards AS cr
            JOIN tasks AS t ON t.id = cr.task_id
            WHERE cr.user_id = u.id
              AND t.currency = 0      
        ) AS ddp_earned`,
		).
		ColumnExpr(`
        (
            SELECT COALESCE(SUM(cr.overall_reward), 0)
            FROM completed_tasks_rewards AS cr
            JOIN tasks AS t ON t.id = cr.task_id
            WHERE cr.user_id = u.id
              AND t.currency = 1  
        ) AS usdc_earned`,
		).
		Where("u.id = ?", id).
		Scan(ctx)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}

		return nil, err
	}

	return userStats, nil
}

func (r *UserRepository) FindByUsername(ctx context.Context, username mtype.Username) (*model.User, error) {
	user := new(model.User)

	err := r.DB.NewSelect().
		Model(user).
		Where("username = ?", username).
		Scan(ctx)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}

		return nil, err
	}

	return user, nil
}
