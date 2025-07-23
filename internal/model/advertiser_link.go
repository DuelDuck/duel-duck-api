package model

import (
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/mr-tron/base58"
	"github.com/uptrace/bun"
	"gitlab.com/duel-duck/duel-duck-api/pkg/apperrors"
	repo "gitlab.com/duel-duck/duel-duck-api/pkg/repository"
)

var (
	RedirectEndpoint = "/advertiser-link"

	AppURLPrefix = "https://app."
)

type AdvertiserLink struct {
	bun.BaseModel `bun:"table:advertiser_links,alias:al" swaggerignore:"true"`

	ID             uuid.UUID `bun:",pk,type:uuid,default:uuid_generate_v4()" json:"id"`
	Link           string    `bun:"type:varchar(100),notnull" json:"link"`
	RedirectedLink string    `bun:"type:varchar(100),notnull" json:"redirected_link"`
	Name           string    `bun:"type:varchar(100),notnull" json:"name"`
	Token          string    `bun:"type:varchar(22),notnull" json:"token"`
	VisitorsCount  uint32    `bun:"type:int" json:"visitors_count"`
	UpdatedAt      time.Time `bun:"default:current_timestamp" json:"updated_at"`
	CreatedAt      time.Time `bun:"default:current_timestamp" json:"created_at"`
}

type AdvertiserLinkReferrals struct {
	bun.BaseModel `bun:"table:advertiser_link_referrals,alias:alr" swaggerignore:"true"`

	LinkID    uuid.UUID `bun:"type:uuid" json:"link_id"`
	UserID    uuid.UUID `bun:"type:uuid" json:"user_id"`
	CreatedAt time.Time `bun:"default:current_timestamp" json:"created_at"`
}

type AdvertiserLinkInfo struct {
	bun.BaseModel `bun:"table:advertiser_links" swaggerignore:"true"`

	ID                   uuid.UUID `bun:"type:uuid" json:"id"`
	Link                 string    `bun:"link" json:"link"`
	RedirectedLink       string    `bun:"redirected_link" json:"redirected_link"`
	Name                 string    `bun:"name" json:"name"`
	VisitorsCount        uint32    `bun:"visitors_count" json:"visitors_count"`
	RegisteredUsersCount uint32    `bun:"registered_users_count" json:"registered_users_count"`
	ActiveUsersCount     uint32    `bun:"active_users_count" json:"active_users_count"`
	UsersBalanceDDP      uint64    `bun:"users_balance_ddp" json:"users_balance_ddp"`
	UsersBalanceUSDC     float64   `bun:"users_balance_usdc" json:"users_balance_usdc"`
	DuelsDDPCount        uint64    `bun:"duels_ddp_count" json:"duels_ddp_count"`
	DuelsUSDCCount       uint64    `bun:"duels_usdc_count" json:"duels_usdc_count"`
	DuelsCreatedCount    uint32    `bun:"duels_created_count" json:"duels_created_count"`
	ReferralsCount       uint32    `bun:"referrals_count" json:"referrals_count"`
	UsersPublicAddresses []string  `bun:"users_public_addresses,type:json" json:"-"`
	UpdatedAt            time.Time `bun:"updated_at" json:"updated_at"`
	CreatedAt            time.Time `bun:"created_at" json:"created_at"`
}

type CreateAdvertiserLinkReq struct {
	Link string `json:"link"`
	Name string `json:"name"`
}

func (l *CreateAdvertiserLinkReq) Validate() error {
	if l.Link == "" || l.Name == "" {
		return apperrors.BadRequest("invalid request data")
	}

	return nil
}

type GetAllAdvertiserLinksReq struct {
	Opts repo.Options `query:"opts"`
}

type EditAdvertiserLinkReq struct {
	bun.BaseModel `bun:"table:advertiser_links,alias:al" swaggerignore:"true"`

	ID             uuid.UUID `bun:"id,pk" json:"id"`
	RedirectedLink string    `bun:"redirected_link" json:"redirected_link"`
	Name           string    `bun:"name" json:"name"`
}

type DeleteAdvertiserLinkReq struct {
	ID uuid.UUID `bun:"id" json:"id"`
}

func NewAdvertiserLink(
	req *CreateAdvertiserLinkReq,
	redirectDomain string,
) *AdvertiserLink {

	var (
		linkID         = uuid.New()
		token          = base58.Encode(linkID[:])
		link           = redirectDomain + RedirectEndpoint + "/" + token
		redirectedLink = req.Link + "?link_token=" + token
	)

	if strings.Contains(req.Link, "app") &&
		!strings.Contains(redirectDomain, "stage") {
		link = AppURLPrefix + link
	} else {
		link = "https://" + link
	}

	return &AdvertiserLink{
		ID:             linkID,
		Link:           link,
		RedirectedLink: redirectedLink,
		Name:           req.Name,
		Token:          token,
	}
}
