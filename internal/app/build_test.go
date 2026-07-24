//go:build integration

package app_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/lilxtent/anti-brute-force/internal/app"
	"github.com/lilxtent/anti-brute-force/internal/config"
)

func testRedisConfig() config.RedisConfig {
	addr := os.Getenv("ABF_TEST_REDIS_ADDR")
	if addr == "" {
		addr = "localhost:6399"
	}
	password := os.Getenv("ABF_TEST_REDIS_PASSWORD")
	if password == "" {
		password = "pass"
	}
	return config.RedisConfig{Address: addr, Password: password}
}

func baseConfig() *config.Config {
	return &config.Config{
		Server: config.ServerConfig{HttpPort: 8080},
		Limits: config.LimitsConfig{
			LoginAttempts:    3,
			PasswordAttempts: 3,
			IPAttempts:       3,
			WindowTime:       time.Minute,
		},
		Redis: testRedisConfig(),
	}
}

func TestBuild_ReturnsWorkingLimiter(t *testing.T) {
	limiter, closer, err := app.Build(baseConfig())
	require.NoError(t, err)
	require.NotNil(t, limiter)
	require.NotNil(t, closer)
	t.Cleanup(func() { require.NoError(t, closer.Close()) })

	ok, err := limiter.Allow(context.Background(), "alice", "hunter2", "10.0.0.1")
	require.NoError(t, err)
	require.True(t, ok)
}

func TestBuild_ErrorsWhenRedisUnreachable(t *testing.T) {
	cfg := baseConfig()
	cfg.Redis.Address = "127.0.0.1:1"

	limiter, closer, err := app.Build(cfg)
	require.Error(t, err)
	require.Nil(t, limiter)
	require.Nil(t, closer)
}
