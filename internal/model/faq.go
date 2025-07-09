package model

import (
	"mime/multipart"
	"time"

	"github.com/google/uuid"
	"github.com/uptrace/bun"
	"gitlab.com/duel-duck/duel-duck-api/pkg/apperrors"
)

var (
	MediaUrlProduction = "https://duelduck.com/media"
	MediaUrlStage      = "https://stage.duelduck.com/media"

	DefaultLimit  uint32 = 20
	DefaultOffset uint32 = 0

	faqDirectionMap = map[string]bool{
		"asc":  true,
		"desc": true,
		"":     true,
	}

	faqSortMap = map[string]bool{
		"marks_value": true,
		"time":        true,
		"":            true,
	}
)

type FAQ struct {
	IsMarked          bool       `bun:",notnull" json:"is_marked"`
	IsAnswered        bool       `bun:",notnull" json:"is_answered"`
	MarksValue        int        `bun:",notnull" json:"marks_value"`
	AnswerID          uint32     `bun:",notnull" json:"answer_id"`
	IsMine            bool       `bun:"is_me,notnull" json:"is_me"`
	QuestionID        uint32     `bun:"id,notnull" json:"id"`
	Question          string     `bun:",notnull" json:"question"`
	Answer            string     `bun:",notnull" json:"answer"`
	AnswerCreatedAt   *time.Time `bun:",notnull" json:"answer_created_at"`
	QuestionCreatedAt time.Time  `bun:"time,notnull" json:"question_created_at"`
	QuestionImages    []string   `bun:"type:text[]" json:"question_images"`
	AnswerImages      []string   `bun:"type:text[]" json:"answer_images"`
}

type FAQUser struct {
	ID        uuid.UUID `bun:",notnull" json:"-"`
	Anonymous bool      `bun:"-" json:"-"`
}

type FAQAnonymousUser struct {
	bun.BaseModel `bun:"table:faq_anonymous_users"`

	ID        uuid.UUID `bun:",pk,type:uuid,default:uuid_generate_v4()" json:"id"`
	Anonymous bool      `bun:"-" json:"-"`
	IP        string    `bun:",notnull" json:"ip"`
	Agent     string    `bun:",notnull" json:"agent"`
	CreatedAt time.Time `bun:",notnull,default:current_timestamp"  json:"created_at"`
}

type FAQModerator struct {
	bun.BaseModel `bun:"table:faq_moderators"`

	ID         uint32 `bun:"type:serial,pk,autoincrement" json:"id"`
	TelegramID int64  `bun:",notnull" json:"telegram_id"`
}

type FAQQuestionAdd struct {
	Question string   `json:"question" binding:"required,max=1500"`
	Images   []string `json:"images"`
}

type FAQQuestion struct {
	bun.BaseModel `bun:"table:faq_questions,alias:fq"`

	ID              uint32     `bun:"type:serial,pk,autoincrement" json:"id"`
	UserID          *uuid.UUID `bun:",notnull" json:"user_id"`
	AnonymousUserID *uuid.UUID `bun:",notnull" json:"anonymous_user_id"`
	Question        string     `bun:",notnull" json:"question"`
	Answered        bool       `bun:",notnull,default:false" json:"answered"`
	Images          []string   `bun:"type:text[]" json:"images"`
	CreatedAt       time.Time  `bun:",notnull,default:current_timestamp"  json:"created_at"`
}

type CreateFAQ struct {
	Question string                  `json:"question"`
	Images   []*multipart.FileHeader `json:"images"`
}

type FAQAnswer struct {
	bun.BaseModel `bun:"table:faq_answers,alias:fa"`

	ID          uint32    `bun:"type:serial,pk,autoincrement" json:"id"`
	QuestionID  uint32    `bun:",notnull" json:"question_id"`
	ModeratorID *uint32   `bun:",notnull" json:"-" swaggerignore:"true"`
	Answer      string    `bun:",notnull" json:"answer"`
	CreatedAt   time.Time `bun:",notnull,default:current_timestamp"  json:"created_at"`
	Images      []string  `bun:"type:text[]" json:"images"`
}

type AddFAQMarkReq struct {
	QuestionID uint32 `json:"question_id"`
	State      int32  `json:"state"`
}

type FAQMark struct {
	bun.BaseModel `bun:"table:faq_marks"`

	QuestionID      uint32     `bun:"type:serial,pk,autoincrement" json:"question_id"`
	UserID          *uuid.UUID `bun:",notnull,type:uuid" json:"user_id"`
	AnonymousUserID *uuid.UUID `bun:",notnull" json:"anonymous_user_id"`
	State           int32      `bun:",notnull" json:"state"`
	CreatedAt       time.Time  `bun:",notnull,default:current_timestamp"  json:"created_at"`
}

type FAQListQuery struct {
	// Sorting order by parameter : time | marks_value
	Sort string `form:"sort"`

	// Search by
	Search string `form:"search"`

	// Sort by answered FAQs
	IsAnswered *bool `form:"answered"`

	// Sort by user's FAQs
	IsMine *bool `form:"me"`

	// Positions limit per page. By default 20
	Limit *uint32 `form:"limit"`

	// Offset for pagination. By default 0
	Offset *uint32 `form:"offset"`

	// Orders direction. By default applies to 'time' sort parameter.
	// If another sort parameter is used, then its "direction" is applied to that parameter
	// asc - Ascending, from A to Z.
	// desc - Descending, from Z to A.
	Direction string `form:"direction"`
}

func (m *AddFAQMarkReq) Validate() error {
	if m.State != -1 && m.State != 1 {
		return apperrors.BadRequest("invalid mark state")
	}

	return nil
}

func (q *FAQListQuery) Validate() error {
	// Validating sort parameter
	if !faqSortMap[q.Sort] {
		return apperrors.BadRequest("invalid sort parameter")
	}

	if q.Sort == "" {
		q.Sort = "time"
	}

	// Validating direction parameter
	if !faqDirectionMap[q.Direction] {
		return apperrors.BadRequest("invalid direction parameter")
	}

	if q.Direction == "" {
		q.Direction = "desc"
	}

	// Validating pagination
	if q.Limit != nil {
		if *q.Limit > 100 {
			*q.Limit = 20
		}
	} else {
		q.Limit = &DefaultLimit
	}

	if q.Offset == nil {
		q.Offset = &DefaultOffset
	}

	return nil
}
