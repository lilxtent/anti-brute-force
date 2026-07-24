package app

import (
	"context"
	"fmt"
	"io"
	"time"

	goredis "github.com/redis/go-redis/v9"

	"github.com/lilxtent/anti-brute-force/internal/config"
	"github.com/lilxtent/anti-brute-force/internal/ratelimit"
	redisstore "github.com/lilxtent/anti-brute-force/internal/storage/redis"
)

const pingTimeout = 5 * time.Second

func Build(cfg *config.Config) (*ratelimit.Limiter, io.Closer, error) {
	client := goredis.NewClient(&goredis.Options{
		Addr:     cfg.Redis.Address,
		Password: cfg.Redis.Password,
	})

	ctx, cancel := context.WithTimeout(context.Background(), pingTimeout)
	defer cancel()
	if err := client.Ping(ctx).Err(); err != nil {
		_ = client.Close()
		return nil, nil, fmt.Errorf("connect to redis at %q: %w", cfg.Redis.Address, err)
	}

	store := redisstore.New(client, ratelimit.RealClock{})
	limiter := ratelimit.NewLimiter(store, ratelimit.Limits{
		LoginAttempts:    cfg.Limits.LoginAttempts,
		PasswordAttempts: cfg.Limits.PasswordAttempts,
		IPAttempts:       cfg.Limits.IPAttempts,
		Window:           cfg.Limits.WindowTime,
	})

	return limiter, store, nil
}
