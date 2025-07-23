package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/uptrace/bun"
	"gitlab.com/duel-duck/duel-duck-api/internal/model"
	"gitlab.com/duel-duck/duel-duck-api/pkg/repository"
)

type FAQRepository struct {
	DB repository.DB
}

func NewFAQRepository(
	db repository.DB,
) *FAQRepository {
	return &FAQRepository{
		DB: db,
	}
}

func (r *FAQRepository) WithTx(tx bun.Tx) *FAQRepository {
	return &FAQRepository{
		DB: r.DB.WithTx(tx),
	}
}

func (r *FAQRepository) GetTelegramModeratorIDs(
	ctx context.Context,
) ([]int64, error) {
	telegramIDs := make([]int64, 0)

	err := r.DB.NewSelect().
		Model((*model.FAQModerator)(nil)).
		Column("telegram_id").
		Scan(ctx, &telegramIDs)
	if err != nil {
		return nil, err
	}

	return telegramIDs, nil
}

func (r *FAQRepository) GetFAQAAnonymousUser(
	ctx context.Context,
	user *model.FAQAnonymousUser,
) (*model.FAQAnonymousUser, error) {

	err := r.DB.NewSelect().
		Model(user).
		Where("ip = ?", user.IP).
		Where("agent = ?", user.Agent).
		Scan(ctx, user)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return user, nil
		}
		return user, err
	}

	return user, nil
}

func (r *FAQRepository) GetAllFAQ(
	ctx context.Context,
	user *model.FAQUser,
	faqParams *model.FAQListQuery,
) ([]model.FAQ, error) {
	faqs := make([]model.FAQ, 0)

	var userID, anonymousUserID uuid.UUID
	if user.Anonymous {
		anonymousUserID = user.ID
	} else {
		userID = user.ID
	}

	// Calculate question marks
	marksQuery := r.DB.
		NewSelect().
		Model((*model.FAQMark)(nil)).
		ColumnExpr("question_id, SUM(state) AS marks_value").
		Group("question_id")

	// Get user's marks
	userMarksQuery := r.DB.
		NewSelect().
		Model((*model.FAQMark)(nil)).
		Column("question_id").
		Column("user_id").
		Column("anonymous_user_id").
		Column("state").
		Where("user_id = ? OR anonymous_user_id = ?", userID, anonymousUserID)

	// Get answers
	answersQuery := r.DB.
		NewSelect().
		Model((*model.FAQAnswer)(nil))

	query := r.DB.
		NewSelect().
		Model((*model.FAQQuestion)(nil)).
		ColumnExpr(`
			fq.id                AS "id",
			fq.question          AS "question",
			fq.created_at        AS "time",
			COALESCE(m.marks_value, 0) AS "marks_value",
			CASE
				WHEN fq.user_id = ? OR fq.anonymous_user_id = ? THEN TRUE
				ELSE FALSE
			END AS "is_me",
			ul.state             AS "marked",
			fq.answered          AS "is_answered",
			fa.answer            AS "answer",
			fa.id                AS "answer_id",
			fa.created_at        AS "answer_created_at",
			fq.images            AS "question_images",
			fa.images            AS "answer_images",
			CASE
				WHEN fq.user_id IS NOT NULL THEN u.username
				ELSE 'Anonymous'
			END AS "username",
			CASE
				WHEN fq.user_id IS NOT NULL THEN u.image_url
				ELSE ''
			END AS "image_url"
		`, userID, anonymousUserID).
		Join("LEFT JOIN (?) AS m  ON m.question_id  = fq.id", marksQuery).
		Join("LEFT JOIN (?) AS ul ON ul.question_id = fq.id", userMarksQuery).
		Join("LEFT JOIN (?) AS fa ON fa.question_id = fq.id", answersQuery).
		Join("LEFT JOIN users AS u ON u.id = fq.user_id").
		GroupExpr(`
			fq.id, fq.question, fq.created_at,
			fq.user_id, fq.anonymous_user_id,
			ul.user_id, ul.anonymous_user_id, ul.state,
			fq.answered,
			fa.answer, fa.id, fa.created_at,
			fq.images, fa.images,
			m.marks_value,
			u.username,
			u.image_url
		`)

	// Get user's questions or shared question
	if faqParams.ShareID != nil {
		query.Where("fq.id = ?", *faqParams.ShareID)
	} else {
		// Get user's questions
		if faqParams.IsMine != nil && *faqParams.IsMine {
			query.Where("fq.user_id = ? OR fq.anonymous_user_id = ?", userID, anonymousUserID)

			if faqParams.IsAnswered != nil {
				query.Where("fq.answered = ?", *faqParams.IsAnswered)
			}
		} else {
			query.Where("fq.answered = TRUE")
		}
	}
	query = faqParams.Opts.Apply(query)

	err := query.
		Scan(ctx, &faqs)
	if err != nil {
		return nil, err
	}

	return faqs, nil
}

func (r *FAQRepository) GetFAQQuestionByID(
	ctx context.Context,
	id uint32,
) (*model.FAQQuestion, error) {
	var faq model.FAQQuestion

	err := r.DB.NewSelect().
		Model(&faq).
		Where("id = ?", id).
		Scan(ctx, &faq)
	if err != nil {
		return nil, err
	}

	return &faq, nil
}

func (r *FAQRepository) GetFAQAnswerByFAQQuestionID(
	ctx context.Context,
	questionID uint32,
) (*model.FAQAnswer, error) {
	var faq model.FAQAnswer

	err := r.DB.NewSelect().
		Model(&faq).
		Where("question_id = ?", questionID).
		Scan(ctx, &faq)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return &model.FAQAnswer{}, nil
		}
		return nil, err
	}

	return &faq, nil
}

func (r *FAQRepository) GetModeratorByTelegramID(
	ctx context.Context,
	telegramID uint64,
) (*model.FAQModerator, error) {
	var moderator model.FAQModerator

	err := r.DB.NewSelect().
		Model(&moderator).
		Where("telegram_id = ?", telegramID).
		Scan(ctx, &moderator)
	if err != nil {
		return nil, err
	}

	return &moderator, nil
}

func (r *FAQRepository) DeleteFAQQuestion(
	ctx context.Context,
	id uint32,
) error {

	_, err := r.DB.NewDelete().
		Model((*model.FAQQuestion)(nil)).
		Where("id = ?", id).
		Exec(ctx)
	if err != nil {
		return err
	}

	return nil
}

func (r *FAQRepository) AddFAQAnswer(
	ctx context.Context,
	answer *model.FAQAnswer,
) error {

	_, err := r.DB.NewUpdate().
		Model((*model.FAQQuestion)(nil)).
		Where("id = ?", answer.QuestionID).
		Set("answered = true").
		Exec(ctx)
	if err != nil {
		return err
	}

	_, err = r.DB.NewInsert().
		Model(answer).
		Exec(ctx)

	return err
}

func (r *FAQRepository) UpdateFAQAnswer(
	ctx context.Context,
	answer *model.FAQAnswer,
) error {
	_, err := r.DB.NewUpdate().
		Model(answer).
		Where("id = ?", answer.ID).
		Exec(ctx)

	return err
}

func (r *FAQRepository) SaveFAQAAnonymousUser(
	ctx context.Context,
	user *model.FAQAnonymousUser,
) (*model.FAQAnonymousUser, error) {
	_, err := r.DB.NewInsert().
		Model(user).
		Returning("*").
		Exec(ctx, user)

	if err != nil {
		return nil, err
	}

	return user, nil
}

func (r *FAQRepository) CreateFAQQuestion(
	ctx context.Context,
	faq *model.FAQQuestion,
) error {
	_, err := r.DB.NewInsert().
		Model(faq).
		Returning("*").
		Exec(ctx)

	return err
}

func (r *FAQRepository) FAQExists(
	ctx context.Context,
	questionID uint32,
) (bool, error) {
	exists, err := r.DB.NewSelect().
		Model((*model.FAQQuestion)(nil)).
		Where("id = ?", questionID).
		Exists(ctx)
	if err != nil {
		return false, err
	}

	return exists, nil
}

func (r *FAQRepository) MarkExists(
	ctx context.Context,
	mark *model.FAQMark,
) (bool, error) {
	exists, err := r.DB.NewSelect().
		Model(mark).
		Where("question_id = ?", mark.QuestionID).
		Where("user_id = ? OR anonymous_user_id = ?", mark.UserID, mark.AnonymousUserID).
		Exists(ctx)
	if err != nil {
		fmt.Println(err)
		return false, err
	}

	return exists, nil
}

func (r *FAQRepository) UpdateFAQMarkByID(
	ctx context.Context,
	mark *model.FAQMark,
) error {
	_, err := r.DB.NewUpdate().
		Model(mark).
		Set("state = ?", mark.State).
		Where("question_id = ?", mark.QuestionID).
		Where("user_id = ? OR anonymous_user_id = ?", mark.UserID, mark.AnonymousUserID).
		Exec(ctx)
	if err != nil {
		return err
	}

	return nil
}

func (r *FAQRepository) CreateFAQMark(
	ctx context.Context,
	mark *model.FAQMark,
) error {
	_, err := r.DB.NewInsert().
		Model(mark).
		Exec(ctx)
	if err != nil {
		return err
	}

	return nil
}
