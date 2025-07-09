package limiter

import (
	"github.com/gofiber/fiber/v3"
	"time"
)

type Config struct {
	// Next defines a function to skip this middleware when returned true.
	//
	// Optional. Default: nil
	Next func(c fiber.Ctx) bool

	// Skip allows you to skip requests so they are not being counter if returned true
	//
	// Optional. Default: nil
	Skip func(c fiber.Ctx) bool

	// A function to dynamically calculate the max requests supported by the rate limiter middleware
	//
	// Default: func(c fiber.Ctx) int {
	//   return c.Max
	// }
	MaxFunc func(c fiber.Ctx) int

	// KeyGenerator allows you to generate custom keys, by default c.IP() is used
	//
	// Default: func(c fiber.Ctx) string {
	//   return c.IP()
	// }
	KeyGenerator func(fiber.Ctx) string

	// LimitReached is called when a request hits the limit
	//
	// Default: func(c fiber.Ctx) error {
	//   return c.SendStatus(fiber.StatusTooManyRequests)
	// }
	LimitReached fiber.Handler

	// Max number of recent connections during `Expiration` seconds before sending a 429 response
	//
	// Default: 5
	Max int

	// Expiration is the time on how long to keep records of requests in memory
	//
	// Default: 1 * time.Minute
	Expiration time.Duration
}

// ConfigDefault is the default config
var ConfigDefault = Config{
	Max:        1,
	Expiration: 2 * time.Second,
	KeyGenerator: func(c fiber.Ctx) string {
		return c.IP()
	},
	LimitReached: func(c fiber.Ctx) error {
		return c.SendStatus(fiber.StatusTooManyRequests)
	},
}

func configDefault(config ...Config) Config {
	// Return default config if nothing provided
	if len(config) < 1 {
		return ConfigDefault
	}

	// Override default config
	cfg := config[0]

	// Set default values
	if cfg.Next == nil {
		cfg.Next = ConfigDefault.Next
	}
	if cfg.Skip == nil {
		cfg.Next = ConfigDefault.Next
	}
	if cfg.Max <= 0 {
		cfg.Max = ConfigDefault.Max
	}
	if int(cfg.Expiration.Seconds()) <= 0 {
		cfg.Expiration = ConfigDefault.Expiration
	}
	if cfg.KeyGenerator == nil {
		cfg.KeyGenerator = ConfigDefault.KeyGenerator
	}
	if cfg.LimitReached == nil {
		cfg.LimitReached = ConfigDefault.LimitReached
	}
	if cfg.MaxFunc == nil {
		cfg.MaxFunc = func(_ fiber.Ctx) int {
			return cfg.Max
		}
	}
	return cfg
}
