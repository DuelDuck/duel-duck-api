package model

import (
	"fmt"
	"math"
	"math/rand/v2"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/uptrace/bun"
	"gitlab.com/duel-duck/duel-duck-api/pkg/mtype"
)

const (
	DuelTopicCrypto = "crypto"
	DuelTopicGaming = "gaming"
	DuelTopicSport  = "sport"
	DuelTopicCustom = "custom"
)

const (
	_ = uint8(iota)
	DuelStatusInReview
	DuelStatusAutoCancelled
	DuelStatusAdminCancelled
	DuelStatusInProcess
	DuelStatusResolved
	DuelStatusRefund
)

const (
	DuckPointsDuelMinJoinPrice = 1000
	DuckPointsDuelMaxJoinPrice = 50000

	USDCDuelMinJoinPrice = 1
	USDCDuelMaxJoinPrice = 5000

	PremiumPlayersConsolationReward = 100
)

const (
	SamePredictionCancellationReason     = "All users made the same prediction"
	LackOfParticipantsCancellationReason = "The duel was canceled due to a lack of participants"
)

type Duel struct {
	bun.BaseModel `bun:"table:duels,alias:duels" json:"-"`

	ID                   uuid.UUID         `bun:",pk,type:uuid,default:uuid_generate_v4()" json:"id"`
	OwnerID              uuid.UUID         `bun:"owner_id,type:uuid,notnull" json:"owner_id"`
	ResolvedBy           uuid.UUID         `bun:"resolved_by,type:uuid,default:null" json:"resolved_by"`
	ApprovedBy           uuid.UUID         `bun:"approved_by,type:uuid,default:null" json:"approved_by"`
	RoomNumber           uint64            `bun:"room_number,type:integer,nullzero" json:"room_number"`
	PaymentType          mtype.PaymentType `bun:"payment_type,type:integer,notnull,default:0" json:"payment_type"`
	PlayersCount         uint64            `bun:"players_count,type:integer,notnull,default:0" json:"players_count"`
	RefundedPlayersCount uint64            `bun:"refunded_players_count,type:integer,notnull" json:"refunded_players_count"`
	WinnersCount         uint64            `bun:"winners_count,type:integer,notnull" json:"winners_count"`
	Username             string            `bun:"username,type:varchar(17),notnull" json:"username"`
	Status               uint8             `bun:"status,type:integer,notnull,default:0" json:"status"`
	ImageURL             string            `bun:"image_url,type:text" json:"image_url"`
	BgURL                string            `bun:"bg_url,type:text" json:"bg_url"`
	Topic                string            `bun:"topic,type:varchar(32),notnull" json:"topic"`
	Subtopic             string            `bun:"subtopic,type:varchar(32),nullzero" json:"subtopic"`
	DuelType             string            `bun:"duel_type,type:varchar(32),nullzero" json:"duel_type"`
	Entities             []uint64          `bun:"entities,type:integer[],notnull,default:'{}'" json:"entities"`
	Question             string            `bun:"question,type:text" json:"question"`
	SourceOfTruth        string            `bun:"source_of_truth,type:text" json:"source_of_truth"`
	DuelPrice            uint64            `bun:"duel_price,type:integer,notnull" json:"duel_price"`
	Commission           uint64            `bun:"commission,type:integer,notnull" json:"commission"`
	DuelInfo             map[string]any    `bun:"duel_info,type:json" json:"duel_info"`
	FinalResult          *uint8            `bun:"final_result,type:integer" json:"final_result"`
	CancellationReason   string            `bun:"cancellation_reason,type:text" json:"cancellation_reason"`
	TournamentID         uuid.UUID         `bun:"tournament_id,type:uuid,nullzero" json:"tournament_id"`
	Deadline             time.Time         `bun:"deadline,notnull,default:current_timestamp" json:"deadline"`
	EventDate            time.Time         `bun:"event_date,notnull,default:current_timestamp" json:"event_date"`
	CreatedAt            time.Time         `bun:"created_at,notnull,default:current_timestamp" json:"created_at"`
	UpdatedAt            time.Time         `bun:"updated_at,notnull,default:current_timestamp" json:"updated_at"`
}

type CryptoDuelInfo struct {
	ID    int     `json:"coin_id"`
	Price float64 `json:"target_price"`
	Type  int     `json:"direction"` // 0 -> down, 1 -> up
}

func (c *CryptoDuelInfo) DetermineWinningBet(coinPrice float64) uint8 {
	switch c.Type {
	case 0:
		if coinPrice <= c.Price {
			return 1
		}
	case 1:
		if coinPrice >= c.Price {
			return 1
		}
	}

	return 0
}

func (c *CryptoDuelInfo) ReachedPriceBeforeEventDate(high, low float64) bool {
	switch c.Type {
	case 0:
		if c.Price >= low {
			return true
		}
	case 1:
		if c.Price <= high {
			return true
		}
	}

	return false
}

func (c *CryptoDuelInfo) FormulateDuelQuestion(coin Coin, deadline time.Time) string {
	direction := "at least"
	if c.Type == 0 {
		direction = "less than"
	}

	//return "Will "+coin.Name+" cost "+direction+FloatToString(c.Price)+"$ on the "+FormatDate(deadline)+"?"
	return fmt.Sprintf(
		"Will %s cost %s %s$ on the %s?",
		coin.Name,
		direction,
		FloatToString(c.Price),
		FormatDate(deadline))
}

func (c *CryptoDuelInfo) Map() map[string]any {
	m := make(map[string]any, 3)

	m["coin_id"] = float64(c.ID)
	m["target_price"] = c.Price
	m["direction"] = float64(c.Type)

	return m
}

func GetCryptoDuelInfo(duelInfo map[string]any) (*CryptoDuelInfo, bool) {
	if duelInfo == nil || len(duelInfo) < 3 {
		return nil, false
	}

	info := &CryptoDuelInfo{}
	id, ok := duelInfo["coin_id"]
	if !ok {
		return nil, false
	}

	idFloat, ok := id.(float64)
	if !ok {
		return nil, false
	}
	info.ID = int(idFloat)

	price, ok := duelInfo["target_price"]
	if !ok {
		return nil, false
	}

	if info.Price, ok = price.(float64); !ok {
		return nil, false
	}

	direction, ok := duelInfo["direction"]
	if !ok {
		return nil, false
	}

	directionFloat, ok := direction.(float64)
	if !ok {
		return nil, false
	}
	info.Type = int(directionFloat)

	return info, true
}

func FormatDate(t time.Time) string {
	return fmt.Sprintf("%d of %s", t.Day(), t.Month().String())
}

func RoundToOnePercent(x float64) float64 {
	if x == 0 {
		return 0
	}

	roundingFactor := math.Pow(10, math.Floor(math.Log10(x))-1)

	return math.Round(x/roundingFactor) * roundingFactor
}

func FloatToString(x float64) string {
	if x == 0 {
		return "0"
	}

	roundingFactor := math.Pow(10, math.Floor(math.Log10(x))-1)
	decimalPlaces := int(math.Max(0, -math.Floor(math.Log10(roundingFactor))))

	return strconv.FormatFloat(x, 'f', decimalPlaces, 64)
}

func DuelByCreateReqAdmin(req *CreateDuelReq, user *User) *Duel {
	duel := DuelByCreateReq(req, user)

	duel.Status = DuelStatusInProcess // duel created by admin so no need to review
	duel.Subtopic = req.Subtopic
	duel.DuelType = req.DuelType
	duel.Entities = req.Entities
	duel.TournamentID = req.TournamentID
	if req.ImageURL != "" {
		duel.ImageURL = req.ImageURL
	}
	if req.BgURL != "" {
		duel.BgURL = req.BgURL
	}

	return duel
}

func DuelByCreateReq(req *CreateDuelReq, user *User) *Duel {
	roomNum := uint64(0)
	if req.PaymentType != mtype.PaymentTypeDuck {
		// roomNum is random number in range [10_000: 4_294_967_295], inclusive
		roomNum = uint64(rand.Int64N(math.MaxUint32-10_000) + 10_000 + 1)
	}

	status := DuelStatusInReview
	if req.Topic == DuelTopicCrypto {
		status = DuelStatusInProcess
	}

	now := time.Now()
	return &Duel{
		ID:            uuid.New(),
		RoomNumber:    roomNum,
		PaymentType:   req.PaymentType,
		Username:      user.Username.String(),
		Status:        status,
		OwnerID:       user.ID,
		ImageURL:      "",
		BgURL:         fmt.Sprintf("background_%d.svg", rand.Int64N(5)),
		Topic:         req.Topic,
		Question:      req.Question,
		SourceOfTruth: req.SourceOfTruth,
		Deadline:      req.Deadline,
		EventDate:     req.EventDate,
		DuelPrice:     req.DuelPrice,
		Commission:    req.Commission,
		DuelInfo:      req.DuelInfo,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
}

type DuelShow struct {
	bun.BaseModel `bun:"table:duels,alias:duels" json:"-"`

	Duel
	YesCount     uint64 `bun:",column:yes_count" json:"yes_count"`
	NoCount      uint64 `bun:",column:no_count" json:"no_count"`
	Joined       bool   `bun:",column:joined" json:"joined"`
	YourAnswer   *int   `bun:",column:your_answer" json:"your_answer"`
	PlayerStatus uint8  `bun:",column:player_status" json:"player_status"`
}

type DuelTopicCounter struct {
	Topic      string `bun:",column:topic" json:"topic"`
	TotalDuels uint64 `bun:",column:total_duels" json:"total_duels"`
}

type CreateDuelReq struct {
	PaymentType   mtype.PaymentType `json:"payment_type"`
	ImageURL      string            `json:"image_url"`
	BgURL         string            `json:"bg_url"`
	Topic         string            `json:"topic"`
	Subtopic      string            `json:"subtopic"`
	DuelType      string            `json:"duel_type"`
	Entities      []uint64          `json:"entities"`
	Question      string            `json:"question"`
	SourceOfTruth string            `json:"source_of_truth"`
	Deadline      time.Time         `json:"deadline"`
	EventDate     time.Time         `json:"event_date"`
	DuelPrice     uint64            `json:"duel_price"`
	Commission    uint64            `json:"commission"`
	DuelInfo      map[string]any    `json:"duel_info"` // target_price for cmc resolver
	Answer        uint8             `json:"answer"`
	TournamentID  uuid.UUID         `json:"tournament_id"`
}

type JoinDuelReq struct {
	DuelID uuid.UUID `json:"duel_id"`
	Answer uint8     `json:"answer"`
}

type DuelResolveReq struct {
	DuelID uuid.UUID `json:"duel_id"`
	Answer uint8     `json:"answer"`
}

type DuelResolveParams struct {
	DuelID        uuid.UUID `json:"duel_id"`
	Answer        uint8     `json:"answer"`
	JoinNotBefore time.Time `json:"not_before"`
}

type DuelApproveReq struct {
	DuelID uuid.UUID `json:"duel_id"`
}

type DuelCancelReq struct {
	DuelID             uuid.UUID `json:"duel_id"`
	Status             uint8     `json:"status"`
	CancellationReason string    `json:"cancellation_reason"`
}

type DuelAdminEditReq struct {
	ID            uuid.UUID      `json:"id"`
	SubTopic      string         `json:"subtopic"`
	Question      string         `json:"question"`
	Entities      []uint64       `json:"entities"`
	DuelType      string         `json:"duel_type"`
	SourceOfTruth string         `json:"source_of_truth"`
	ImageURL      string         `json:"image_url"`
	BgURL         string         `json:"bg_url"`
	DuelInfo      map[string]any `json:"duel_info"`
	Deadline      time.Time      `json:"deadline"`
	EventDate     time.Time      `json:"event_date"`
}

type DuelAdminAssignToTournamentReq struct {
	DuelID       uuid.UUID  `json:"duel_id"`
	TournamentID *uuid.UUID `json:"tournament_id"`
}

type DuelDeleteReq struct {
	DuelID uuid.UUID `json:"duel_id"`
}

type DuelInfoEdit struct {
	bun.BaseModel `bun:"table:duels,alias:duels" json:"-"`

	ID          uuid.UUID `bun:",pk,type:uuid,notnull"`
	EntityValue []string  `bun:"entity_value,type:json"`
}

type DuelParams struct {
	Pool         float64
	Commission   float64
	PlayersCount float64
	WinnersCount float64
}

func NewDuelParams(
	duelPrice uint64,
	commission uint64,
	playersCount uint64,
	winnersCount uint64,
) DuelParams {
	return DuelParams{
		Pool:         float64(playersCount) * float64(duelPrice),
		Commission:   float64(commission),
		PlayersCount: float64(playersCount),
		WinnersCount: float64(winnersCount),
	}
}

func (p DuelParams) CalculateCommissionReward() int {
	percentValue := (p.Pool / 100) * p.Commission
	ownerShare := percentValue / 2

	return int(math.Floor(ownerShare))
}

func (p DuelParams) CalculateFinalReward() float64 {
	percentValue := math.Round((p.Pool / 100) * p.Commission)

	finalPool := math.Floor(p.Pool - percentValue)

	finalReward := math.Floor(finalPool / p.WinnersCount)

	return finalReward
}

// USDCPriceMultiplier - USDC in solana blockchain is 6 decimals, 1 USDC is 1_000_000
const USDCPriceMultiplier = 1_000_000

func (p DuelParams) CalculateFinalCryptoReward() uint64 {
	percentValue := p.Pool * p.Commission * USDCPriceMultiplier / 100

	finalPool := p.Pool*USDCPriceMultiplier - percentValue

	return uint64(finalPool / p.WinnersCount)
}

func (p DuelParams) CalculateCryptoCommissionReward() uint64 {
	percentValue := p.Pool * p.Commission * USDCPriceMultiplier / 100

	return uint64(percentValue / 2)
}

type JoinSolanaRoomResp struct {
	TxHash         string           `json:"tx_hash"`
	AutoswapResult []AutoswapResult `json:"autoswap_result"`
}

type CreateCryptoDuelResp struct {
	Duel   *Duel               `json:"duel"`
	Result *JoinSolanaRoomResp `json:"result"`
}

type JoinCryptoDuelResp struct {
	Player *Player             `json:"player"`
	Result *JoinSolanaRoomResp `json:"result"`
}

func AutoCancelReq(duel *Duel) *DuelCancelReq {
	cancellationReason := SamePredictionCancellationReason
	if duel.PlayersCount <= 1 {
		cancellationReason = LackOfParticipantsCancellationReason
	}

	return &DuelCancelReq{
		DuelID:             duel.ID,
		Status:             DuelStatusRefund,
		CancellationReason: cancellationReason,
	}
}
