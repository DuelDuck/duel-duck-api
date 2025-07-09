package cron

import (
	"gitlab.com/duel-duck/duel-duck-api/pkg/apperrors"
	"go.uber.org/zap"
)

func LogErr(l *zap.Logger, err error) {
	appErr, ok := apperrors.IsAppError(err)
	if !ok {
		l.Error("cron job failed", zap.Error(err))
		return
	}

	if appErr.BaseError != nil {
		l.Error(
			appErr.Message,
			zap.String("err", appErr.BaseError.Error()),
			zap.String("occurred", appErr.Path()),
		)
	} else {
		l.Error(
			appErr.Message,
			zap.String("occurred", appErr.Path()),
		)
	}
}
