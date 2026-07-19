package ratelimit

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type fakeStore struct {
	calls   []string
	resets  []string
	allowFn func(key string) (bool, error)
}

func (f *fakeStore) Allow(_ context.Context, key string, _ BucketConfig) (bool, error) {
	f.calls = append(f.calls, key)
	if f.allowFn != nil {
		return f.allowFn(key)
	}
	return true, nil
}

func (f *fakeStore) Reset(_ context.Context, key string) error {
	f.resets = append(f.resets, key)
	return nil
}

func (f *fakeStore) Close() error { return nil }

func testLimits() Limits {
	return Limits{LoginAttempts: 10, PasswordAttempts: 100, IPAttempts: 1000, Window: time.Minute}
}

func TestLimiter_AllowTrueWhenAllUnderLimit(t *testing.T) {
	f := &fakeStore{}
	l := NewLimiter(f, testLimits())

	ok, err := l.Allow(context.Background(), "u", "p", "1.2.3.4")
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, []string{"login:u", "password:p", "ip:1.2.3.4"}, f.calls)
}

func TestLimiter_AllowFalseWhenAnyOverLimit_AllStillCharged(t *testing.T) {
	f := &fakeStore{allowFn: func(key string) (bool, error) {
		return key != "password:p", nil
	}}
	l := NewLimiter(f, testLimits())

	ok, err := l.Allow(context.Background(), "u", "p", "1.2.3.4")
	require.NoError(t, err)
	require.False(t, ok)
	require.Equal(t, []string{"login:u", "password:p", "ip:1.2.3.4"}, f.calls,
		"all three buckets must be charged even when one denies")
}

func TestLimiter_AllowChargesAllThreeOnError(t *testing.T) {
	sentinel := errors.New("store boom")
	f := &fakeStore{allowFn: func(key string) (bool, error) {
		if key == "login:u" {
			return false, sentinel
		}
		return true, nil
	}}
	l := NewLimiter(f, testLimits())

	ok, err := l.Allow(context.Background(), "u", "p", "1.2.3.4")
	require.ErrorIs(t, err, sentinel)
	require.False(t, ok)
	require.Equal(t, []string{"login:u", "password:p", "ip:1.2.3.4"}, f.calls,
		"all three attempted despite an error")
}

func TestLimiter_EnforcesLoginLimitEndToEnd(t *testing.T) {
	clk := &manualClock{t: time.Unix(0, 0)}
	s := NewMemoryStore(clk, time.Minute, 0)
	defer s.Close()
	l := NewLimiter(s, Limits{LoginAttempts: 2, PasswordAttempts: 100, IPAttempts: 100, Window: time.Minute})
	ctx := context.Background()

	ok, err := l.Allow(ctx, "u", "p1", "1.1.1.1")
	require.NoError(t, err)
	require.True(t, ok)
	ok, _ = l.Allow(ctx, "u", "p2", "1.1.1.2")
	require.True(t, ok)
	ok, _ = l.Allow(ctx, "u", "p3", "1.1.1.3")
	require.False(t, ok, "third attempt exceeds login limit of 2")
}

func TestNewLimiter_PanicsOnZeroLimits(t *testing.T) {
	f := &fakeStore{}
	require.Panics(t, func() {
		NewLimiter(f, Limits{})
	})
}

func TestNewLimiter_PanicsOnZeroWindow(t *testing.T) {
	f := &fakeStore{}
	require.Panics(t, func() {
		NewLimiter(f, Limits{LoginAttempts: 10, PasswordAttempts: 100, IPAttempts: 1000, Window: 0})
	})
}

func TestLimiter_EnforcesPasswordLimitEndToEnd(t *testing.T) {
	clk := &manualClock{t: time.Unix(0, 0)}
	s := NewMemoryStore(clk, time.Minute, 0)
	defer s.Close()
	l := NewLimiter(s, Limits{LoginAttempts: 100, PasswordAttempts: 2, IPAttempts: 100, Window: time.Minute})
	ctx := context.Background()

	ok, err := l.Allow(ctx, "u1", "p", "1.1.1.1")
	require.NoError(t, err)
	require.True(t, ok)
	ok, _ = l.Allow(ctx, "u2", "p", "1.1.1.2")
	require.True(t, ok)
	ok, _ = l.Allow(ctx, "u3", "p", "1.1.1.3")
	require.False(t, ok, "third attempt exceeds password limit of 2")
}

func TestLimiter_ResetClearsLoginAndIPOnly(t *testing.T) {
	f := &fakeStore{}
	l := NewLimiter(f, testLimits())

	require.NoError(t, l.Reset(context.Background(), "u", "1.2.3.4"))
	require.Equal(t, []string{"login:u", "ip:1.2.3.4"}, f.resets,
		"reset must not touch the password bucket")
}
