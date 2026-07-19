package ratelimit

import (
	"context"
	"time"
)

type BucketConfig struct {
	Capacity int
	Window   time.Duration
}

type Store interface {
	Allow(ctx context.Context, key string, cfg BucketConfig) (bool, error)
	Reset(ctx context.Context, key string) error
	Close() error
}
