package cache

import (
	"context"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"gitlab.com/duel-duck/duel-duck-api/config"
	"gitlab.com/duel-duck/duel-duck-api/internal/model"
	"gitlab.com/duel-duck/duel-duck-api/pkg/apperrors"
	"strconv"
	"strings"
	"time"
)

const (
	codeAndUsageCounterSep = ":"
)

type TelegramStorage struct {
	client  *redis.Client
	conf    *config.Config
	codeTTL time.Duration
}

func NewTelegramCacheStorage(
	c *config.Config,
	client *redis.Client,
) *TelegramStorage {

	return &TelegramStorage{
		client:  client,
		conf:    c,
		codeTTL: c.Auth.TelegramBotCodeTTL,
	}
}

func (s *TelegramStorage) Save(ctx context.Context, telegramID string, code uuid.UUID) error {
	value := code.String() + codeAndUsageCounterSep + "0"
	err := s.client.Set(ctx, telegramID, value, s.codeTTL).Err()
	if err != nil {
		return apperrors.Unauthorized("failed to save user's telegram id", err)
	}

	return nil
}

func (s *TelegramStorage) GetCodeTelegramID(ctx context.Context, telegramID string) (*model.SignInWithTelegramWebCache, error) {
	codeAndUsageCounterStr, err := s.client.Get(ctx, telegramID).Result()
	if err != nil {
		return nil, apperrors.Unauthorized("failed to find code by user's telegram id", err)
	}

	if codeAndUsageCounterStr == "" {
		return nil, apperrors.Unauthorized("code not found")
	}

	codeAndUsageCounter := strings.Split(codeAndUsageCounterStr, codeAndUsageCounterSep)
	if len(codeAndUsageCounter) != 2 {
		return nil, apperrors.Internal("failed to get code and read counter")
	}

	code, err := uuid.Parse(codeAndUsageCounter[0])
	if err != nil {
		return nil, apperrors.Unauthorized("stored code is not valid")
	}

	readCounter, err := strconv.ParseUint(codeAndUsageCounter[1], 10, 64)
	if err != nil {
		return nil, apperrors.Internal("failed to parse get number of cache reads", err)
	}

	incrementedValue := code.String() + codeAndUsageCounterSep + strconv.FormatInt(int64(readCounter+1), 10)
	err = s.client.Set(ctx, telegramID, incrementedValue, s.codeTTL).Err()
	if err != nil {
		return nil, apperrors.Unauthorized("failed to save user's telegram id", err)
	}

	return &model.SignInWithTelegramWebCache{
		Code:        code,
		ReadCounter: uint8(readCounter),
	}, nil
}

func (s *TelegramStorage) DeleteCodeByTelegramID(ctx context.Context, telegramID string) error {
	_, err := s.client.Del(ctx, telegramID).Result()
	if err != nil {
		return apperrors.Unauthorized("failed to delete code by user's telegram id", err)
	}

	return nil
}
