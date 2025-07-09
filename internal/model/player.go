package model

import (
	"github.com/google/uuid"
	"github.com/uptrace/bun"
	"time"
)

const (
	PlayerStatusActive   uint8 = 0
	PlayerStatusResolved uint8 = 1
	PlayerStatusRefunded uint8 = 2
)

type Player struct {
	bun.BaseModel `bun:"table:players,alias:players" json:"-"`

	ID          uuid.UUID `bun:",pk,type:uuid,default:uuid_generate_v4()" json:"id"`
	UserID      uuid.UUID `bun:"type:uuid" json:"user_id"`
	DuelID      uuid.UUID `bun:"type:uuid" json:"duel_id"`
	WinAmount   float64   `bun:"type:int" json:"win_amount"`
	Answer      uint8     `bun:"type:int" json:"answer"`
	FinalStatus uint8     `bun:"type:smallint" json:"final_status"`
	IsWinner    bool      `bun:"type:bool" json:"is_winner"`
	CreatedAt   time.Time `bun:",column:created_at,notnull,default:current_timestamp" json:"created_at"`
}

type PlayerShow struct {
	bun.BaseModel `bun:"table:players,alias:players" json:"-"`

	Player
	Username string `bun:",column:username,type:text" json:"username"`
	ImageUrl string `bun:",column:image_url,type:text" json:"image_url"`
}

type CryptoDuelPlayer struct {
	bun.BaseModel `bun:"table:players,alias:players" json:"-"`

	ID            uuid.UUID `bun:",pk,type:uuid,default:uuid_generate_v4()" json:"id"`
	UserID        uuid.UUID `bun:"type:uuid" json:"user_id"`
	PublicAddress string    `bun:",type:varchar(44),unique,nullzero" json:"public_address"`
}
