package redis

import (
	"context"
	"errors"

	"github.com/lilxtent/anti-brute-force/internal/ratelimit"
)

var errNotImplemented = errors.New("redis store: not implemented")

type Store struct{}

func New() *Store { return &Store{} }

var _ ratelimit.Store = (*Store)(nil)

func (s *Store) Allow(_ context.Context, _ string, _ ratelimit.BucketConfig) (bool, error) {
	return false, errNotImplemented
}

func (s *Store) Reset(_ context.Context, _ string) error {
	return errNotImplemented
}

func (s *Store) Close() error { return nil }
