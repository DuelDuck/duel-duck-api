package cache

import (
	"context"
	"github.com/redis/go-redis/v9"
	"gitlab.com/duel-duck/duel-duck-api/config"
	"net"
	"time"
)

func CreateRedisClient(c *config.Config) (*redis.Client, error) {
	redisAddress := net.JoinHostPort(c.Redis.Host, c.Redis.Port)

	redisCl := redis.NewClient(&redis.Options{
		Addr:     redisAddress,
		Password: c.Redis.Password,
		DB:       c.Redis.DB,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if _, err := redisCl.Ping(ctx).Result(); err != nil {
		return nil, err
	}

	return redisCl, nil
}
