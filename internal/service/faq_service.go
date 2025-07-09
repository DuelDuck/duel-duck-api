package service

import (
	"context"

	"gitlab.com/duel-duck/duel-duck-api/config"
	"gitlab.com/duel-duck/duel-duck-api/internal/model"
	"gitlab.com/duel-duck/duel-duck-api/internal/storage/repository"
	"gitlab.com/duel-duck/duel-duck-api/pkg/apperrors"
	repo "gitlab.com/duel-duck/duel-duck-api/pkg/repository"
)

type FAQService struct {
	FAQRepository      *repository.FAQRepository
	FileService        *FileService
	TransactionManager *repo.TransactionManager

	MediaUrl   string
	ChanFAQBot chan *model.FAQQuestion
}

func NewFAQService(
	c *config.Config,
	faqRepository *repository.FAQRepository,
	fileService *FileService,
	transactionManager *repo.TransactionManager,
) *FAQService {

	var mediaUrl string

	if c.App.Environment == config.EnvironmentProduction {
		mediaUrl = model.MediaUrlProduction
	} else if c.App.Environment == config.EnvironmentStage {
		mediaUrl = model.MediaUrlStage
	} else {
		mediaUrl = "https://c79c-2a00-1858-102d-82bd-f557-b06a-ec49-f97e.ngrok-free.app/media" // todo: remove
	}

	return &FAQService{
		FAQRepository:      faqRepository,
		FileService:        fileService,
		TransactionManager: transactionManager,
		ChanFAQBot:         make(chan *model.FAQQuestion, 50),
		MediaUrl:           mediaUrl,
	}
}

func (s *FAQService) GetAllFAQ(
	ctx context.Context,
	user *model.FAQUser,
	faqParams *model.FAQListQuery,
) ([]model.FAQ, uint64, error) {
	faqs, err := s.FAQRepository.GetAllFAQ(ctx, user, faqParams)
	if err != nil {
		return nil, 0, apperrors.Internal("failed to get all faq", err)
	}

	countTotal, err := s.FAQRepository.GetFAQsTotals(ctx, user, faqParams)
	if err != nil {
		return nil, 0, apperrors.Internal("failed to get total faq count", err)
	}

	return faqs, countTotal, nil
}

func (s *FAQService) AddFAQQuestion(
	ctx context.Context,
	user *model.FAQUser,
	data *model.CreateFAQ,
) (*model.FAQQuestion, error) {

	var (
		err error

		faq = &model.FAQQuestion{Question: data.Question}
	)

	if len(data.Images) > 0 {
		faq.Images, err = s.FileService.SaveFAQQuestionImages(s.MediaUrl, data.Images)
		if err != nil {
			return nil, apperrors.Internal("failed to save uploaded faq images", err)
		}
	}

	if user.Anonymous {
		faq.AnonymousUserID = &user.ID
	} else {
		faq.UserID = &user.ID
	}

	err = s.FAQRepository.CreateFAQQuestion(ctx, faq)
	if err != nil {
		return nil, apperrors.Internal("failed to create faq", err)
	}

	go func() {
		s.ChanFAQBot <- faq
	}()

	return faq, nil
}

func (s *FAQService) AddFAQMark(
	ctx context.Context,
	user *model.FAQUser,
	mark *model.FAQMark,
) error {

	// Check for faq existence
	exists, err := s.FAQRepository.FAQExists(ctx, mark.QuestionID)
	if err != nil {
		return apperrors.Internal("failed to check if the faq exists by id")
	}

	if !exists {
		return apperrors.AlreadyExist("faq not found by id")
	}

	if user.Anonymous {
		mark.AnonymousUserID = &user.ID
	} else {
		mark.UserID = &user.ID
	}

	// Check for mark existence
	exists, err = s.FAQRepository.MarkExists(ctx, mark)
	if err != nil {
		return apperrors.Internal("failed to check if the mark exists")
	}

	if !exists {
		err = s.FAQRepository.CreateFAQMark(ctx, mark)
		if err != nil {
			return apperrors.Internal("failed to create mark")
		}

		return nil
	}

	err = s.FAQRepository.UpdateFAQMarkByID(ctx, mark)
	if err != nil {
		return apperrors.Internal("failed to add mark faq", err)
	}

	return nil
}

func (s *FAQService) GetFAQAAnonymousUser(
	ctx context.Context,
	user *model.FAQAnonymousUser,
) (*model.FAQAnonymousUser, error) {
	return s.FAQRepository.GetFAQAAnonymousUser(ctx, user)
}

func (s *FAQService) SaveFAQAAnonymousUser(
	ctx context.Context,
	user *model.FAQAnonymousUser,
) (*model.FAQAnonymousUser, error) {
	return s.FAQRepository.SaveFAQAAnonymousUser(context.TODO(), user)
}

func (s *FAQService) stop(_ context.Context) error {
	close(s.ChanFAQBot)

	return nil
}
