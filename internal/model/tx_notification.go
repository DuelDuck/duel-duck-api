package model

import (
	"time"

	"github.com/google/uuid"
)

type TxNotification struct {
	ID        uuid.UUID `json:"id"`
	Status    uint8     `json:"status"` // pending, success, failed
	CreatedAt uint64    `json:"created_at"`
}

const (
	TxNotificationStatusPending = 1
	TxNotificationStatusSuccess = 2
	TxNotificationStatusFailed  = 3
)

func newTxNotification(status uint8) *TxNotification {
	return &TxNotification{
		ID:        uuid.New(),
		Status:    status,
		CreatedAt: uint64(time.Now().UnixMilli()),
	}
}

func NewTxNotificationStatusPending() *TxNotification {
	return newTxNotification(TxNotificationStatusPending)
}

func NewTxNotificationStatusSuccess() *TxNotification {
	return newTxNotification(TxNotificationStatusSuccess)
}

func NewTxNotificationStatusFailed() *TxNotification {
	return newTxNotification(TxNotificationStatusFailed)
}

type TxNotificationDeleteReq struct {
	NotificationIDs []uuid.UUID `json:"notification_ids"`
}
