package model

import (
	"github.com/google/uuid"
	"github.com/uptrace/bun"
	"gitlab.com/duel-duck/duel-duck-api/pkg/apperrors"
	"time"
)

const (
	ActionCreation = "creation"
	ActionModerate = "moderation"
	ActionClosing  = "closing"
)

const (
	CreationPay   float64 = 0.5
	ModerationPay float64 = 0.2
	ClosingPay    float64 = 0.3
)

type ModeratorStats struct {
	bun.BaseModel `bun:"table:moderator_stats,alias:ms" json:"-"`

	ModeratorID  uuid.UUID `bun:"moderator_id" json:"moderator_id"`
	DuelID       uuid.UUID `bun:"duel_id" json:"duel_id"`
	ActionType   string    `bun:"action_type,type:varchar(32),notnull" json:"action_type"`
	DuelQuestion string    `bun:"duel_question" json:"duel_question"`
	IsCustom     bool      `bun:"is_custom" json:"is_custom"`
	DuelBank     uint64    `bun:"duel_bank" json:"duel_bank"`
	CreationPay  float64   `bun:"creation_pay,type:double precision,notnull" json:"creation_pay"`
	Bonuses      float64   `bun:"bonuses,type:double precision" json:"bonuses"`
	CreatedAt    time.Time `bun:"created_at,notnull,default:current_timestamp" json:"created_at"`
}

type ModeratorStatsCreate struct {
	bun.BaseModel `bun:"table:moderator_stats,alias:ms" json:"-"`

	ModeratorID uuid.UUID `bun:"moderator_id" json:"moderator_id"`
	DuelID      uuid.UUID `bun:"duel_id" json:"duel_id"`
	ActionType  string    `bun:"action_type,type:varchar(32),notnull" json:"action_type"`
	CreationPay float64   `bun:"creation_pay,type:double precision,notnull" json:"creation_pay"`
}

func NewModeratorStatsCreate(
	moderatorID uuid.UUID,
	duelID uuid.UUID,
	duelStatus uint8,
	newDuelStatus uint8,
) (*ModeratorStatsCreate, error) {
	stats := &ModeratorStatsCreate{
		ModeratorID: moderatorID,
		DuelID:      duelID,
	}

	if duelStatus == DuelStatusInReview && newDuelStatus == DuelStatusAdminCancelled {
		stats.ActionType = ActionModerate
		stats.CreationPay = ModerationPay

		return stats, nil
	}

	duelStatusClosed := newDuelStatus == DuelStatusRefund || newDuelStatus == DuelStatusResolved
	if duelStatus == DuelStatusInProcess && duelStatusClosed {
		stats.ActionType = ActionClosing
		stats.CreationPay = ClosingPay

		return stats, nil
	}

	return nil, apperrors.BadRequest("unable to perform an action with the duel")
}
