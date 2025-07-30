package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"gitlab.com/duel-duck/duel-duck-api/config"
	"gitlab.com/duel-duck/duel-duck-api/internal/model"
	"gitlab.com/duel-duck/duel-duck-api/pkg/apperrors"
)

const (
	txNotificationKeyFormat = "user:%s:tx_noti"
	userTxNotificationLimit = 10
	txNotificationTTL       = 20 * time.Hour
)

type TxNotificationStorage struct {
	client *redis.Client
	conf   *config.Config
}

func NewTxNotificationStorage(
	client *redis.Client,
	c *config.Config,
) *TxNotificationStorage {
	return &TxNotificationStorage{
		client: client,
		conf:   c,
	}
}

func getUserTxNotiKey(userID uuid.UUID) string {
	return fmt.Sprintf(txNotificationKeyFormat, userID.String())
}

func (s *TxNotificationStorage) Save(
	ctx context.Context,
	userID uuid.UUID,
	noti *model.TxNotification,
) error {
	key := getUserTxNotiKey(userID)

	// Serialize notification to JSON
	data, err := json.Marshal(noti)
	if err != nil {
		return apperrors.Internal("failed to marshal notification", err)
	}

	// Add to ZSET with score = CreatedAt
	pipe := s.client.Pipeline()
	pipe.ZAdd(ctx, key, redis.Z{
		Score:  float64(noti.CreatedAt),
		Member: string(data),
	})
	// Trim to last 10 (keep newest)
	pipe.ZRemRangeByRank(ctx, key, 0, -(userTxNotificationLimit + 1))
	// Set TTL
	pipe.Expire(ctx, key, txNotificationTTL)
	_, err = pipe.Exec(ctx)
	if err != nil {
		return apperrors.Internal("failed to save notification", err)
	}
	return nil
}

// Get all notifications in order (oldest to newest)
func (s *TxNotificationStorage) GetAll(
	ctx context.Context,
	userID uuid.UUID,
) ([]model.TxNotification, error) {
	key := getUserTxNotiKey(userID)

	vals, err := s.client.ZRange(ctx, key, 0, -1).Result()
	if err != nil {
		return nil, apperrors.Internal("failed to get notifications", err)
	}

	notifications := make([]model.TxNotification, 0, len(vals))
	for _, v := range vals {
		var n model.TxNotification
		if err := json.Unmarshal([]byte(v), &n); err == nil {
			notifications = append(notifications, n)
		}
	}

	return notifications, nil
}

// Delete notification by id
// Delete multiple notifications by IDs for a user
func (s *TxNotificationStorage) Delete(
	ctx context.Context,
	userID uuid.UUID,
	notificationIDs uuid.UUIDs,
) error {
	key := getUserTxNotiKey(userID)

	// Get all notifications
	vals, err := s.client.ZRange(ctx, key, 0, -1).Result()
	if err != nil {
		return apperrors.Internal("failed to get notifications for delete", err)
	}

	// Build a set for fast lookup
	idSet := make(map[uuid.UUID]struct{}, len(notificationIDs))
	for _, id := range notificationIDs {
		idSet[id] = struct{}{}
	}

	// Find all notification JSONs to remove
	toRemove := make([]any, 0, len(notificationIDs))
	for _, v := range vals {
		var n model.TxNotification
		if err := json.Unmarshal([]byte(v), &n); err == nil {
			if _, ok := idSet[n.ID]; ok {
				toRemove = append(toRemove, v)
			}
		}
	}

	if len(toRemove) == 0 {
		return apperrors.NotFound("notification(s) not found")
	}

	if err := s.client.ZRem(ctx, key, toRemove...).Err(); err != nil {
		return apperrors.Internal("failed to delete notification(s)", err)
	}
	return nil
}
