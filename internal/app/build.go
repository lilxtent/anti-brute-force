package app

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	goredis "github.com/redis/go-redis/v9"

	"github.com/lilxtent/anti-brute-force/internal/config"
	"github.com/lilxtent/anti-brute-force/internal/ratelimit"
	"github.com/lilxtent/anti-brute-force/internal/service"
	"github.com/lilxtent/anti-brute-force/internal/storage/postgres"
	redisstore "github.com/lilxtent/anti-brute-force/internal/storage/redis"
)

const pingTimeout = 5 * time.Second
const loadTimeout = 5 * time.Second

type multiCloser []io.Closer

func (m multiCloser) Close() error {
	var err error
	for i := len(m) - 1; i >= 0; i-- {
		if cerr := m[i].Close(); cerr != nil {
			err = errors.Join(err, cerr)
		}
	}
	return err
}

func Build(cfg *config.Config) (*service.Service, io.Closer, error) {
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

	repo := postgres.New(cfg.Postgres.Dsn)
	if err := repo.Connect(); err != nil {
		_ = store.Close()
		return nil, nil, fmt.Errorf("connect to postgres: %w", err)
	}

	svc := service.New(limiter, repo)
	loadCtx, loadCancel := context.WithTimeout(context.Background(), loadTimeout)
	defer loadCancel()
	if err := svc.Load(loadCtx); err != nil {
		_ = repo.Close()
		_ = store.Close()
		return nil, nil, fmt.Errorf("load lists: %w", err)
	}

	return svc, multiCloser{store, repo}, nil
}
