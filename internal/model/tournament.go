package model

import (
	"fmt"
	"github.com/google/uuid"
	"github.com/uptrace/bun"
	"gitlab.com/duel-duck/duel-duck-api/pkg/mtype"
	repo "gitlab.com/duel-duck/duel-duck-api/pkg/repository"
	"time"
)

type Tournament struct {
	bun.BaseModel `bun:"table:tournaments" json:"-"`

	ID             uuid.UUID `bun:",pk" json:"id"`
	OwnerID        uuid.UUID `bun:",notnull" json:"owner_id"`
	RewardPoolDP   uint64    `bun:"reward_pool_dp,notnull" json:"reward_pool_dp"`
	RewardPoolUSDC uint64    `bun:"reward_pool_usdc,notnull" json:"reward_pool_usdc"`
	PlayersCount   uint64    `bun:"players_count,notnull" json:"players_count"`
	Name           string    `bun:",notnull" json:"name"`
	Description    string    `bun:"" json:"description"`
	Author         string    `bun:",notnull" json:"author"`
	ImageURL       string    `bun:"" json:"image_url"`
	BgURL          string    `bun:"" json:"bg_url"`
	BgCardURL      string    `bun:"" json:"bg_card_url"`
	XURL           string    `bun:"x_url" json:"x_url"`
	InstagramURL   string    `bun:"" json:"instagram_url"`
	YoutubeURL     string    `bun:"" json:"youtube_url"`
	URL            string    `bun:",unique,notnull" json:"url"`
	StartDate      time.Time `bun:"" json:"start_date"`
	FinishDate     time.Time `bun:"" json:"finish_date"`
	CreatedAt      time.Time `bun:"" json:"created_at"`
}

type TournamentReward struct {
	bun.BaseModel `bun:"table:tournament_rewards" json:"-"`

	TournamentID uuid.UUID `bun:",notnull" json:"tournament_id"`
	FromRank     uint64    `bun:",notnull" json:"from_rank"`
	ToRank       uint64    `bun:",notnull" json:"to_rank"`
	DPAmount     float64   `bun:",notnull" json:"dp_amount"`
	USDCAmount   float64   `bun:",notnull" json:"usdc_amount"`
}

type TournamentParticipant struct {
	bun.BaseModel `bun:"table:tournament_participants" json:"-"`

	TournamentID uuid.UUID `bun:",notnull" json:"tournament_id"`
	UserID       uuid.UUID `bun:",notnull" json:"user_id"`
}

type TournamentUserShow struct {
	bun.BaseModel `bun:"table:tournament_participants,alias:tp" json:"-"`

	ID           uuid.UUID         `bun:",pk" json:"id"`
	PNL          float64           `bun:",notnull" json:"pnl"`
	TotalDuels   uint64            `bun:"" json:"total_duels"`
	Victories    uint64            `bun:"" json:"victories"`
	PlayersCount uint64            `bun:",notnull" json:"players_count"`
	Name         string            `bun:",notnull" json:"name"`
	ImageURL     string            `bun:"" json:"image_url"`
	BgCardURL    string            `bun:"" json:"bg_card_url"`
	URL          string            `bun:",unique,notnull" json:"url"`
	PaymentType  mtype.PaymentType `bun:"" json:"payment_type"`
	StartDate    time.Time         `bun:"" json:"start_date"`
	FinishDate   time.Time         `bun:"" json:"finish_date"`
}

type TournamentRank struct {
	bun.BaseModel `bun:"table:tournament_leaderboards,alias:tl" json:"-"`

	Rank            uint64            `bun:"" json:"rank"`
	TournamentID    uuid.UUID         `bun:"tournament_id,notnull" json:"tournament_id"`
	UserID          uuid.UUID         `bun:"user_id,notnull" json:"user_id"`
	Username        mtype.Username    `bun:"username" json:"username"`
	PaymentType     mtype.PaymentType `bun:"payment_type,notnull" json:"payment_type"`
	TotalDuels      uint64            `bun:"total_duels,notnull" json:"total_duels"`
	Victories       uint64            `bun:"victories,notnull" json:"victories"`
	Spent           uint64            `bun:"spent,notnull" json:"spent"`
	Earned          float64           `bun:"earned,notnull" json:"earned"`
	PNL             float64           `bun:"pnl,notnull" json:"pnl"`
	LastPredictedAt time.Time         `bun:"last_predicted_at" json:"last_predicted_at"`
}

type TournamentLeaderboard struct {
	bun.BaseModel `bun:"table:tournament_leaderboards,alias:tl" json:"-"`

	TournamentID    uuid.UUID         `bun:"tournament_id,notnull" json:"tournament_id"`
	UserID          uuid.UUID         `bun:"user_id,notnull" json:"user_id"`
	Username        mtype.Username    `bun:"username" json:"username"`
	PaymentType     mtype.PaymentType `bun:"payment_type,notnull" json:"payment_type"`
	TotalDuels      uint64            `bun:"total_duels,notnull" json:"total_duels"`
	Victories       uint64            `bun:"victories,notnull" json:"victories"`
	Spent           uint64            `bun:"spent,notnull" json:"spent"`
	Earned          float64           `bun:"earned,notnull" json:"earned"`
	PNL             float64           `bun:"pnl,notnull" json:"pnl"`
	LastPredictedAt time.Time         `bun:"last_predicted_at" json:"last_predicted_at"`
}

func DefaultTournamentLeaderboardRecord(user *User, duel *Duel) *TournamentLeaderboard {
	return &TournamentLeaderboard{
		TournamentID: duel.TournamentID,
		UserID:       user.ID,
		Username:     user.Username,
		PaymentType:  duel.PaymentType,
		TotalDuels:   1,
		Spent:        duel.DuelPrice,
		PNL:          -float64(duel.DuelPrice),
	}
}

type TournamentRewards []TournamentReward

func (rs TournamentRewards) Valid() error {
	if len(rs) == 0 {
		return fmt.Errorf("rewards are empty")
	}

	for i := 0; i < len(rs); i++ {
		r := rs[i]

		if r.FromRank > r.ToRank {
			return fmt.Errorf("invalid range: from_rank (%d) > to_rank (%d)", r.FromRank, r.ToRank)
		}

		if i > 0 {
			prev := rs[i-1]

			if r.FromRank <= prev.FromRank {
				return fmt.Errorf("rewards not sorted by from_rank: %d before %d", prev.FromRank, r.FromRank)
			}

			if r.FromRank <= prev.ToRank {
				return fmt.Errorf("overlap between ranges: [%d-%d] and [%d-%d]",
					prev.FromRank, prev.ToRank, r.FromRank, r.ToRank)
			}

			if r.FromRank != prev.ToRank+1 {
				return fmt.Errorf("gap between ranges: [%d-%d] and [%d-%d]",
					prev.FromRank, prev.ToRank, r.FromRank, r.ToRank)
			}
		}
	}

	return nil
}

type TournamentCreateReq struct {
	Tournament TournamentCreateData `json:"tournament"`
	Rewards    TournamentRewards    `json:"rewards"`
}

type TournamentCreateData struct {
	RewardPoolDP   uint64    `json:"reward_pool_dp"`
	RewardPoolUSDC uint64    `json:"reward_pool_usdc"`
	Name           string    `json:"name"`
	Description    string    `json:"description"`
	Author         string    `json:"author"`
	ImageURL       string    `json:"image_url"`
	BgURL          string    `json:"bg_url"`
	BgCardURL      string    `json:"bg_card_url"`
	XURL           string    `json:"x_url"`
	InstagramURL   string    `json:"instagram_url"`
	YoutubeURL     string    `json:"youtube_url"`
	URL            string    `json:"url"`
	StartDate      time.Time `json:"start_date"`
	FinishDate     time.Time `json:"finish_date"`
}

type TournamentRewardsReq struct {
	TournamentID uuid.UUID `query:"tournament_id"`
}

type TournamentRewardsUpdateReq struct {
	TournamentID uuid.UUID         `json:"tournament_id"`
	Rewards      TournamentRewards `json:"rewards"`
}

func TournamentByCreateReq(
	req *TournamentCreateData,
	ownerID uuid.UUID,
) *Tournament {
	return &Tournament{
		ID:             uuid.New(),
		OwnerID:        ownerID,
		RewardPoolDP:   req.RewardPoolDP,
		RewardPoolUSDC: req.RewardPoolUSDC,
		Name:           req.Name,
		Description:    req.Description,
		Author:         req.Author,
		ImageURL:       req.ImageURL,
		BgURL:          req.BgURL,
		BgCardURL:      req.BgCardURL,
		XURL:           req.XURL,
		InstagramURL:   req.InstagramURL,
		YoutubeURL:     req.YoutubeURL,
		URL:            req.URL,
		StartDate:      req.StartDate,
		FinishDate:     req.FinishDate,
	}
}

type TournamentOffer struct {
	bun.BaseModel `bun:"table:tournament_offers" json:"-"`

	ID          uuid.UUID `bun:",pk" json:"id"`
	Name        string    `bun:",notnull" json:"name"`
	Contact     string    `bun:",notnull" json:"contact"`
	ProjectLink string    `bun:",notnull" json:"project_link"`
	Description string    `bun:"" json:"description"`
	CreatedAt   time.Time `bun:"" json:"created_at"`
}

func (b *TournamentOffer) Notification() string {
	return fmt.Sprintf("ID: %s\nName: %s\nContact: %s\nProjectLink: %s\nDescription: %s\n",
		b.ID.String(), b.Name, b.Contact, b.ProjectLink, b.Description)
}

type TournamentLeaderboardReq struct {
	TournamentID uuid.UUID    `query:"tournament_id"`
	Opts         repo.Options `query:"opts"`
}

type TournamentOfferReq struct {
	bun.BaseModel `bun:"table:tournament_offers" json:"-"`

	Name        string `bun:",notnull" json:"name"`
	Contact     string `bun:",notnull" json:"contact"`
	ProjectLink string `bun:",notnull" json:"project_link"`
	Description string `json:"description"`
}

func NewTournamentOffer(offerReq *TournamentOfferReq) *TournamentOffer {
	return &TournamentOffer{
		ID:          uuid.New(),
		Name:        offerReq.Name,
		Contact:     offerReq.Contact,
		ProjectLink: offerReq.ProjectLink,
		Description: offerReq.Description,
	}
}
