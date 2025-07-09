package model

import (
	"time"

	"github.com/google/uuid"
	"github.com/uptrace/bun"
	"gitlab.com/duel-duck/duel-duck-api/pkg/mtype"
)

// Only tasks that need to be completed from back-end mentioned
const (
	TaskSignUp             = 1
	TaskCreateDuel         = 10
	SetUpWallet            = 15
	TaskJoinTournament     = 16
	TaskJoinDuel           = 20
	TaskWinDuel            = 21
	TaskReferAFriend       = 22
	TaskCompletDailyStreak = 23
	TaskFinishTutorial     = 24
)

type Task struct {
	bun.BaseModel `bun:"table:tasks,alias:t"`

	ID                    uint64            `bun:"type:serial,pk,autoincrement" json:"id"`
	Name                  string            `bun:"type:varchar(64),notnull" json:"name"`
	Description           string            `bun:"type:text" json:"description"`
	Completed             bool              `bun:"type:int" json:"completed"`
	RewardClaimed         uint32            `bun:",notnull,default:0" json:"reward_claimed"`
	CompletionCount       uint32            `bun:"type:int" json:"completion_count"`
	Reward                int               `bun:"type:int,notnull" json:"reward"`
	Currency              mtype.PaymentType `bun:"type:int,notnull" json:"currency"`
	XP                    uint32            `bun:"xp,type:int,notnull,default:0" json:"xp"`
	CompletionLimit       uint32            `bun:"type:int" json:"completion_limit"`
	CompletionLimitPeriod uint32            `bun:"type:int" json:"completion_limit_period"`
	Deadline              *uint64           `bun:"-" json:"deadline"`
	Link                  string            `bun:"type:varchar(255)" json:"link"`
	CreatedAt             time.Time         `bun:"default:current_timestamp" json:"created_at"`
}

type CompletedTask struct {
	bun.BaseModel `bun:"table:completed_tasks,alias:c"`

	UserID           uuid.UUID `bun:"type:uuid" json:"user_id"`
	TaskID           uint64    `bun:"type:int" json:"task_id"`
	RewardClaimed    uint32    `bun:"type:int,notnull,default:0" json:"reward_claimed"`
	OverallCount     uint32    `bun:"type:int,notnull,default:1" json:"overall_count"`
	CompletionCount  uint32    `bun:"type:int,notnull,default:1" json:"completion_count"`
	RewardMultiplier float64   `bun:"type:double,notnull,default:1" json:"-"`
	UpdatedAt        time.Time `bun:"default:current_timestamp" json:"updated_at"`
	CreatedAt        time.Time `bun:"default:current_timestamp" json:"created_at"`
}

type CompletedTaskReward struct {
	bun.BaseModel `bun:"table:completed_tasks_rewards,alias:cr"`

	UserID        uuid.UUID `bun:"type:uuid"`
	TaskID        uint64    `bun:"type:int"`
	OverallReward uint32    `bun:"type:int,notnull,default:1"`
	OverallXP     uint32    `bun:"type:int,notnull,default:1"`
	UpdatedAt     time.Time `bun:"default:current_timestamp"`
	CreatedAt     time.Time `bun:"default:current_timestamp"`
}

type ClaimRewardResp struct {
	Task        *Task         `json:"task"`
	UserStats   *UserStats    `json:"user_stats"`
	UserBalance mtype.Balance `json:"user_balance"`
}

func NewCompletedTask(userID uuid.UUID, taskID uint64) *CompletedTask {
	return &CompletedTask{
		UserID:    userID,
		TaskID:    taskID,
		UpdatedAt: time.Now().UTC(),
		CreatedAt: time.Now().UTC(),
	}
}

type CompleteTaskReq struct {
	TaskID uint64 `json:"task_id"`
}
