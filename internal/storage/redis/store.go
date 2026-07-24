package redis

import (
	"context"
	_ "embed"

	goredis "github.com/redis/go-redis/v9"

	"github.com/lilxtent/anti-brute-force/internal/ratelimit"
)

const keyPrefix = "abf:rl:"

//go:embed lua/allow.lua
var allowScriptSrc string

var allowScript = goredis.NewScript(allowScriptSrc)

type Store struct {
	client *goredis.Client
	clock  ratelimit.Clock
}

func New(client *goredis.Client, clock ratelimit.Clock) *Store {
	return &Store{client: client, clock: clock}
}

var _ ratelimit.Store = (*Store)(nil)

func (s *Store) Allow(ctx context.Context, key string, cfg ratelimit.BucketConfig) (bool, error) {
	res, err := allowScript.Run(ctx, s.client,
		[]string{keyPrefix + key},
		s.clock.Now().UnixMilli(),
		cfg.Capacity,
		cfg.Window.Milliseconds(),
	).Int()
	if err != nil {
		return false, err
	}
	return res == 1, nil
}

func (s *Store) Reset(ctx context.Context, key string) error {
	return s.client.Del(ctx, keyPrefix+key).Err()
}

func (s *Store) Close() error {
	return s.client.Close()
}
