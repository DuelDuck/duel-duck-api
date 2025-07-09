package model

import (
	"github.com/google/uuid"
	"github.com/uptrace/bun"
	"gitlab.com/duel-duck/duel-duck-api/pkg/mtype"
	"time"
)

type UserReferral struct {
	bun.BaseModel `bun:"table:user_referrals,alias:ur" json:"-"`

	ReferrerID            uuid.UUID `bun:"referrer_id,notnull" json:"referrer_id"`
	ReferralID            uuid.UUID `bun:"referral_id,pk,notnull,unique" json:"referral_id"`
	IncomeUSDC            float64   `bun:"income_usdc,notnull" json:"income_usdc"`
	IncomeDP              uint64    `bun:"income_dp,notnull" json:"income_dp"`
	ReferralCommissionEnd time.Time `bun:"referral_commission_end,notnull" json:"referral_commission_end"`
	CreatedAt             time.Time `bun:"created_at,notnull,nullzero,default:current_timestamp" json:"created_at"`
}

func NewUserReferral(referrerID, referralID uuid.UUID, rewardDP uint64) *UserReferral {
	now := time.Now().UTC()

	return &UserReferral{
		ReferrerID:            referrerID,
		ReferralID:            referralID,
		ReferralCommissionEnd: now.AddDate(0, 6, 0),
		IncomeUSDC:            0,
		IncomeDP:              rewardDP,
		CreatedAt:             now,
	}
}

type UserReferralStats struct {
	bun.BaseModel `bun:"table:users,alias:u" json:"-"`

	ID                 uuid.UUID `bun:",pk" json:"id"`
	ReferralToken      string    `bun:"" json:"referral_token"`
	ReferralCount      uint64    `bun:"" json:"referral_count"`
	ReferralIncomeUSDC float64   `bun:"" json:"referral_income_usdc"`
	ReferralIncomeDP   uint64    `bun:"" json:"referral_income_dp"`
}

type UserReferralShow struct {
	bun.BaseModel `bun:"table:user_referrals,alias:ur" json:"-"`

	ReferrerID            uuid.UUID      `bun:"referrer_id,notnull" json:"referrer_id"`
	ReferralID            uuid.UUID      `bun:"referral_id,pk,notnull,unique" json:"referral_id"`
	Username              mtype.Username `bun:"username" json:"username"`
	IncomeUSDC            float64        `bun:"income_usdc,notnull" json:"income_usdc"`
	IncomeDP              uint64         `bun:"income_dp,notnull" json:"income_dp"`
	ReferralCommissionEnd time.Time      `bun:"referral_commission_end,notnull" json:"referral_commission_end"`
	CreatedAt             time.Time      `bun:"created_at,notnull,nullzero,default:current_timestamp" json:"created_at"`
}
type ReferrerWallet struct {
	bun.BaseModel `bun:"table:users,alias:u" json:"-"`

	ReferrerID            uuid.UUID `bun:"" json:"referrer_id"`
	ReferralID            uuid.UUID `bun:"" json:"referral_id"`
	ReferrerPublicAddress string    `bun:"" json:"referrer_public_address"`
}
