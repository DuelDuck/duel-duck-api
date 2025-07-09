package middleware

import (
	"errors"
	"net/http"
	"time"

	"github.com/gofiber/fiber/v3"
	"gitlab.com/duel-duck/duel-duck-api/config"
	"gitlab.com/duel-duck/duel-duck-api/pkg/apperrors"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func NewLoggingMiddleware(l *zap.Logger) *LogMiddleware {
	return &LogMiddleware{l: l}
}

type LogMiddleware struct {
	l *zap.Logger
}

func (h *LogMiddleware) RegisterLogger(c *config.Config, app *fiber.App) {
	app.Use(h.LogRequests)

	switch c.App.Environment {
	case config.EnvironmentProduction, config.EnvironmentStage:
		app.Use(h.LogErrorsProduction)
	default:
		app.Use(h.LogErrorsDevelopment)
	}
}

func (h *LogMiddleware) LogRequests(c fiber.Ctx) error {
	start := time.Now()
	err := c.Next()
	stop := time.Now()

	status := c.Response().StatusCode()
	latency := stop.Sub(start).String()
	ip := c.IP()
	method := c.Method()
	path := c.Path()

	level := zapcore.InfoLevel
	if status >= 400 {
		level = zapcore.ErrorLevel
	}

	h.l.Log(
		level,
		"request",
		zap.Int("status", status),
		zap.String("latency", latency),
		zap.String("ip", ip),
		zap.String("method", method),
		zap.String("path", path),
	)

	return err
}

func (h *LogMiddleware) LogErrorsProduction(c fiber.Ctx) error {
	err := c.Next()
	if err == nil {
		return nil
	}

	var fiberErr *fiber.Error
	if errors.As(err, &fiberErr) {
		c.Status(fiberErr.Code)
		return fiberErr
	}

	appErr, ok := apperrors.IsAppError(err)
	if !ok {
		c.Status(http.StatusInternalServerError)
		h.l.Error("unhandled error", zap.Error(err))
		return fiber.NewError(http.StatusInternalServerError, "internal server error")
	}

	if appErr.BaseError != nil {
		h.l.Error(
			appErr.Message,
			zap.String("err", appErr.BaseError.Error()),
			zap.String("occurred", appErr.Path()),
		)
	} else {
		h.l.Error(
			appErr.Message,
			zap.String("occurred", appErr.Path()),
		)
	}

	c.Status(appErr.Status)
	return fiber.NewError(appErr.Status, appErr.Error())
}

func (h *LogMiddleware) LogErrorsDevelopment(c fiber.Ctx) error {
	err := c.Next()
	if err == nil {
		return nil
	}

	var fiberErr *fiber.Error
	if errors.As(err, &fiberErr) {
		c.Status(fiberErr.Code)
		return fiberErr
	}

	appErr, ok := apperrors.IsAppError(err)
	if !ok || appErr == nil {
		h.l.Error(err.Error())
		c.Status(http.StatusInternalServerError)

		err = apperrors.Internal("internal server error")
		return fiber.NewError(http.StatusInternalServerError, err.Error())
	}

	if appErr.BaseError != nil {
		h.l.Error(
			appErr.Message+" "+appErr.Path(),
			zap.String("err", appErr.BaseError.Error()),
		)
	} else {
		h.l.Error(
			appErr.Message + " " + appErr.Path(),
		)
	}

	c.Status(appErr.Status)
	return fiber.NewError(appErr.Status, appErr.Error())
}
