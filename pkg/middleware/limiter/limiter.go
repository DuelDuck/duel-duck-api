package limiter

import (
	"github.com/gofiber/fiber/v3"
)

const (
	// X-RateLimit-* headers
	xRateLimitLimit     = "X-RateLimit-Limit"
	xRateLimitRemaining = "X-RateLimit-Remaining"
	xRateLimitReset     = "X-RateLimit-Reset"
)

func NewSlidingWindow(config ...Config) fiber.Handler {
	cfg := configDefault(config...)
	return newSlidingWindow(cfg)
}
