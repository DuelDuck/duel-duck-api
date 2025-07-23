package model

import (
	"mime/multipart"
	"time"

	"github.com/google/uuid"
	"github.com/uptrace/bun"
	"gitlab.com/duel-duck/duel-duck-api/pkg/apperrors"
	repo "gitlab.com/duel-duck/duel-duck-api/pkg/repository"
)

var (
	MediaUrlProduction = "https://api.duelduck.com/media"
	MediaUrlStage      = "https://api-stage.duelduck.com/media"

	faqMarkStateMap = map[int32]bool{
		0:  true,
		1:  true,
		-1: true,
	}

	faqOrderByMap = map[string]bool{
		"marks_value": true,
		"time":        true,
		"":            true,
	}
)

type FAQ struct {
	Username          string     `bun:",notnull" json:"username"`
	ImageURL          string     `bun:"image_url" json:"image_url"`
	Marked            int32      `bun:",notnull" json:"marked"`
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

type GetFAQReq struct {
	QuestionID uint32 `json:"question_id"`
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
	// add validation for
	Opts repo.Options `query:"opts"`

	// Sort by answered FAQs
	IsAnswered *bool `query:"answered"`

	// Sort by user's FAQs
	IsMine *bool `query:"me"`

	// Share question by id
	ShareID *uint32 `query:"share_id"`
}

func (q *FAQListQuery) Validate() error {
	if !q.Opts.Order.IsValid() {
		q.Opts.Order = repo.Order{OrderBy: "time", OrderType: "desc"}
		return nil
	}

	if _, ok := faqOrderByMap[q.Opts.Order.OrderBy]; !ok {
		return apperrors.BadRequest("invalid order by option")
	}

	return nil
}

func (f *CreateFAQ) Validate() error {
	if len(f.Question) == 0 || len(f.Question) > 300 {
		return apperrors.BadRequest("invalid question length")
	}

	if len(f.Images) > 5 {
		return apperrors.BadRequest("no more than 5 images allowed")
	}

	return nil
}

func (m *AddFAQMarkReq) Validate() error {
	if !faqMarkStateMap[m.State] {
		return apperrors.BadRequest("invalid mark state")
	}

	return nil
}
