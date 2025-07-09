package cache

import (
	"context"
	"github.com/redis/go-redis/v9"
	"gitlab.com/duel-duck/duel-duck-api/config"
	"gitlab.com/duel-duck/duel-duck-api/pkg/apperrors"
	"gitlab.com/duel-duck/duel-duck-api/pkg/mtype"
	"time"
)

type CodeStorage struct {
	client  *redis.Client
	conf    *config.Config
	codeTTL time.Duration
}

func NewCodeCacheStorage(
	c *config.Config,
	client *redis.Client,
) *CodeStorage {

	return &CodeStorage{
		client:  client,
		conf:    c,
		codeTTL: c.Auth.TelegramBotCodeTTL,
	}
}

func (s *CodeStorage) Save(ctx context.Context, email mtype.Email, code string) error {
	err := s.client.Set(ctx, email.String(), code, s.codeTTL).Err()
	if err != nil {
		return apperrors.Unauthorized("failed to save user's token", err)
	}

	return nil
}

func (s *CodeStorage) GetCodeByEmail(ctx context.Context, email mtype.Email) (string, error) {
	result, err := s.client.Get(ctx, email.String()).Result()
	if err != nil {
		return "", apperrors.Unauthorized("failed to find code by user's email", err)
	}

	if result == "" {
		return "", apperrors.Unauthorized("token not found")
	}

	return result, nil
}
