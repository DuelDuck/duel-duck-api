package repository

import (
	"context"
	"database/sql"
	"errors"

	"github.com/google/uuid"
	"github.com/uptrace/bun"
	"gitlab.com/duel-duck/duel-duck-api/internal/model"
	"gitlab.com/duel-duck/duel-duck-api/pkg/repository"
)

type ReferralRepository struct {
	DB repository.DB
}

func NewReferralRepository(
	db repository.DB,
) *ReferralRepository {
	return &ReferralRepository{
		DB: db,
	}
}

func (r *ReferralRepository) WithTx(tx bun.Tx) *ReferralRepository {
	return &ReferralRepository{
		DB: r.DB.WithTx(tx),
	}
}

func (r *ReferralRepository) Create(
	ctx context.Context,
	referrerToken string,
	referralID uuid.UUID,
	reward uint64,
) error {
	var referrerID uuid.UUID
	err := r.DB.NewUpdate().
		Model((*model.User)(nil)).
		Set("referral_count = referral_count + 1").
		Set("referral_income_dp = referral_income_dp + ?", reward).
		Set("balance = balance + ?", reward).
		Where("referral_token = ?", referrerToken).
		Returning("id").
		Scan(ctx, &referrerID)
	if err != nil && referrerID == uuid.Nil {
		return err
	}

	referral := model.NewUserReferral(referrerID, referralID, reward)
	_, err = r.DB.NewInsert().
		Model(referral).
		Exec(ctx)

	return err
}

const (
	tgChannelReferralsTable = "tg_channel_referrals"
)

func (r *ReferralRepository) CheckIfInvitedByTGChannel(ctx context.Context, referrerToken string) (bool, error) {
	res, err := r.DB.NewUpdate().
		Table(tgChannelReferralsTable).
		Where("referral_token = ?", referrerToken).
		Set("referral_count = referral_count + 1").
		Exec(ctx)
	if err != nil {
		return false, err
	}

	rowsAffected, err := res.RowsAffected()
	if err != nil {
		return false, err
	}

	return rowsAffected > 0, nil
}

func (r *ReferralRepository) ReferrerWalletByUserID(
	ctx context.Context,
	userID uuid.UUID,
) (*model.ReferrerWallet, error) {
	referrer := new(model.ReferrerWallet)

	err := r.DB.NewSelect().
		Model(referrer).
		ColumnExpr(
			`u.public_address as referrer_public_address,
			ur.referrer_id,
			ur.referral_id`,
		).
		Join("INNER JOIN user_referrals ur ON ur.referrer_id = u.id").
		Where("ur.referral_id = ?", userID).
		Where("ur.referral_commission_end >= NOW()").
		Scan(ctx)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}

		return nil, err
	}

	return referrer, nil
}

func (r *ReferralRepository) UpdateReferrerUSDCIncomeByReferralID(
	ctx context.Context,
	referrerID uuid.UUID,
	commissionEarned float64,
) error {
	userUpdate := r.DB.NewUpdate().
		Table("users").
		Set("referral_income_usdc = referral_income_usdc + ?", commissionEarned).
		Where("id = ?", referrerID).
		Returning("id")

	_, err := r.DB.NewUpdate().
		Model((*model.UserReferral)(nil)).
		Set("income_usdc = income_usdc + ?", commissionEarned).
		Where("referrer_id = ?", referrerID).
		Where("referral_commission_end > NOW()").
		With("user_update", userUpdate).
		Where("EXISTS (SELECT 1 FROM user_update)").
		Exec(ctx)

	return err
}

func (r *ReferralRepository) UpdateReferrerDPIncomeByReferralID(
	ctx context.Context,
	referralID uuid.UUID,
	commissionEarned uint64,
) error {
	refUpd := r.DB.NewUpdate().
		Model((*model.UserReferral)(nil)).
		Set("income_dp = income_dp + ?", commissionEarned).
		Where("ur.referral_id = ?", referralID).
		Where("ur.referral_commission_end > NOW()").
		Returning("ur.referrer_id")

	_, err := r.DB.NewUpdate().
		Model((*model.User)(nil)).
		TableExpr("ref_upd").
		Set("referral_income_dp = u.referral_income_dp + ?", commissionEarned).
		Where("u.id = ref_upd.referrer_id").
		With("ref_upd", refUpd).
		Exec(ctx)

	return err
}

func (r *ReferralRepository) GetReferrerID(
	ctx context.Context,
	referralID uuid.UUID,
) (uuid.UUID, error) {
	var referrerID uuid.UUID

	err := r.DB.NewSelect().
		Model((*model.UserReferral)(nil)).
		Column("referrer_id").
		Where("referral_id = ?", referralID).
		Scan(ctx, &referrerID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return uuid.Nil, nil
		}

		return uuid.Nil, err
	}

	return referrerID, nil
}
