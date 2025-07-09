package service

import (
	"context"
	"github.com/google/uuid"
	"gitlab.com/duel-duck/duel-duck-api/internal/storage/cache"
	"gitlab.com/duel-duck/duel-duck-api/pkg/apperrors"
	"gitlab.com/duel-duck/duel-duck-api/pkg/mailer"
	"gitlab.com/duel-duck/duel-duck-api/pkg/mtype"
)

const (
	EmailVerificationCodeTopic = "DuelDuck: Verification Code"
)

type AuthService struct {
	CodeStorage     *cache.CodeStorage
	TelegramStorage *cache.TelegramStorage
	Mailer          mailer.Mailer
}

func NewAuthService(
	codeStorage *cache.CodeStorage,
	telegramStorage *cache.TelegramStorage,
	mailer mailer.Mailer,
) *AuthService {
	return &AuthService{
		CodeStorage:     codeStorage,
		TelegramStorage: telegramStorage,
		Mailer:          mailer,
	}
}

func (s *AuthService) SendCodeOnEmail(ctx context.Context, template string, email mtype.Email) error {
	code, err := generateVerificationCode()
	if err != nil {
		return err
	}

	err = s.CodeStorage.Save(ctx, email, code)
	if err != nil {
		return err
	}

	mail := mailer.NewMail(email, template, EmailVerificationCodeTopic, code)

	err = s.Mailer.SendEmail(ctx, mail)
	if err != nil {
		return err
	}

	return nil
}

func (s *AuthService) CheckUsersEmailCode(ctx context.Context, email mtype.Email, code string) (bool, error) {
	storedCode, err := s.CodeStorage.GetCodeByEmail(ctx, email)
	if err != nil {
		return false, err
	}

	if storedCode != code {
		return false, nil
	}

	return true, nil
}

func (s *AuthService) SaveTelegramCode(ctx context.Context, telegramID string, code uuid.UUID) error {
	err := s.TelegramStorage.Save(ctx, telegramID, code)
	if err != nil {
		return err
	}

	return nil
}

const (
	maxTGBotLoginLinkUsages = 2
)

func (s *AuthService) CheckUsersTelegramCode(ctx context.Context, telegramID string, code uuid.UUID) (bool, error) {
	storedCode, err := s.TelegramStorage.GetCodeTelegramID(ctx, telegramID)
	if err != nil {
		return false, err
	}

	if storedCode.Code != code {
		return false, nil
	}

	if storedCode.ReadCounter >= maxTGBotLoginLinkUsages {
		if err = s.DeleteCodeByTelegramID(ctx, telegramID); err != nil {
			return false, err
		}

		return false, apperrors.BadRequest("telegram bot link can be used only twice")
	} else {

	}

	return true, nil
}

func (s *AuthService) DeleteCodeByTelegramID(ctx context.Context, telegramID string) error {
	return s.TelegramStorage.DeleteCodeByTelegramID(ctx, telegramID)
}
