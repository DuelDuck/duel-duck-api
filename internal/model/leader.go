package model

import (
	"github.com/google/uuid"
	"github.com/uptrace/bun"
)

type Leader struct {
	bun.BaseModel `bun:"table:leaderboard,alias:ld" json:"-"`

	Rank       uint64    `bun:"rank" json:"rank"`
	UserID     uuid.UUID `bun:"user_id,type:uuid,notnull" json:"user_id"`
	Username   string    `bun:"username" json:"username"`
	TotalDuels uint64    `bun:"total_duels" json:"total_duels"`
	Victories  uint64    `bun:"victories" json:"victories"`
	Spent      float64   `bun:"spent" json:"spent"`
	Earned     float64   `bun:"earned" json:"earned"`
	PNL        float64   `bun:"pnl" json:"pnl"`
}
