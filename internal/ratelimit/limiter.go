package ratelimit

import (
	"context"
	"time"
)

type Limits struct {
	LoginAttempts    int
	PasswordAttempts int
	IPAttempts       int
	Window           time.Duration
}

type Limiter struct {
	store  Store
	limits Limits
}

func NewLimiter(store Store, limits Limits) *Limiter {
	if limits.LoginAttempts <= 0 {
		panic("ratelimit: NewLimiter limits.LoginAttempts must be > 0")
	}
	if limits.PasswordAttempts <= 0 {
		panic("ratelimit: NewLimiter limits.PasswordAttempts must be > 0")
	}
	if limits.IPAttempts <= 0 {
		panic("ratelimit: NewLimiter limits.IPAttempts must be > 0")
	}
	if limits.Window <= 0 {
		panic("ratelimit: NewLimiter limits.Window must be > 0")
	}
	return &Limiter{store: store, limits: limits}
}

func (l *Limiter) Allow(ctx context.Context, login, password, ip string) (bool, error) {
	okLogin, errLogin := l.store.Allow(ctx, "login:"+login,
		BucketConfig{Capacity: l.limits.LoginAttempts, Window: l.limits.Window})
	okPassword, errPassword := l.store.Allow(ctx, "password:"+password,
		BucketConfig{Capacity: l.limits.PasswordAttempts, Window: l.limits.Window})
	okIP, errIP := l.store.Allow(ctx, "ip:"+ip,
		BucketConfig{Capacity: l.limits.IPAttempts, Window: l.limits.Window})

	if err := firstErr(errLogin, errPassword, errIP); err != nil {
		return false, err
	}
	return okLogin && okPassword && okIP, nil
}

func (l *Limiter) Reset(ctx context.Context, login, ip string) error {
	if err := l.store.Reset(ctx, "login:"+login); err != nil {
		return err
	}
	return l.store.Reset(ctx, "ip:"+ip)
}

func firstErr(errs ...error) error {
	for _, err := range errs {
		if err != nil {
			return err
		}
	}
	return nil
}
