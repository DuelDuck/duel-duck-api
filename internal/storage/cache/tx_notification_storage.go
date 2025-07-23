package cache

import (
	"context"
	"fmt"
	"time"

	"github.com/goccy/go-json"

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
		if err = json.Unmarshal([]byte(v), &n); err == nil {
			notifications = append(notifications, n)
		}
	}

	return notifications, nil
}

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

func (s *TxNotificationStorage) update(
	ctx context.Context,
	userID uuid.UUID,
	notificationID uuid.UUID,
	newStatus uint8,
	newCreatedAt uint64,
) error {
	key := getUserTxNotiKey(userID)

	// Get all notifications
	vals, err := s.client.ZRange(ctx, key, 0, -1).Result()
	if err != nil {
		return apperrors.Internal("failed to get notifications for update", err)
	}

	var (
		oldValue    string
		updatedNoti model.TxNotification
		found       bool
	)

	for _, v := range vals {
		var n model.TxNotification
		if err := json.Unmarshal([]byte(v), &n); err == nil {
			if n.ID == notificationID {
				// Update fields
				n.Status = newStatus
				n.CreatedAt = newCreatedAt
				updatedNoti = n
				oldValue = v
				found = true
				break
			}
		}
	}

	if !found {
		return apperrors.NotFound("notification not found")
	}

	if err := s.client.ZRem(ctx, key, oldValue).Err(); err != nil {
		return apperrors.Internal("failed to remove old notification", err)
	}

	data, err := json.Marshal(updatedNoti)
	if err != nil {
		return apperrors.Internal("failed to marshal updated notification", err)
	}

	// Add updated notification with new score
	pipe := s.client.Pipeline()
	pipe.ZAdd(ctx, key, redis.Z{
		Score:  float64(updatedNoti.CreatedAt),
		Member: string(data),
	})

	// Trim to last 10 (keep newest)
	pipe.ZRemRangeByRank(ctx, key, 0, -(userTxNotificationLimit + 1))
	pipe.Expire(ctx, key, txNotificationTTL)

	_, err = pipe.Exec(ctx)
	if err != nil {
		return apperrors.Internal("failed to update notification", err)
	}

	return nil
}

func (s *TxNotificationStorage) SetStatusSuccess(
	ctx context.Context,
	userID uuid.UUID,
	notificationID uuid.UUID,
) error {
	return s.update(
		ctx,
		userID,
		notificationID,
		model.TxNotificationStatusSuccess,
		uint64(time.Now().UnixMilli()),
	)
}

func (s *TxNotificationStorage) SetStatusFailed(
	ctx context.Context,
	userID uuid.UUID,
	notificationID uuid.UUID,
) error {
	return s.update(
		ctx,
		userID,
		notificationID,
		model.TxNotificationStatusFailed,
		uint64(time.Now().UnixMilli()),
	)
}
