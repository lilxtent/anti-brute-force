//go:build integration

package integration_test

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	goredis "github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"

	"github.com/lilxtent/anti-brute-force/internal/api"
	"github.com/lilxtent/anti-brute-force/internal/app"
	"github.com/lilxtent/anti-brute-force/internal/config"
)

func testConfig() *config.Config {
	redisAddr := os.Getenv("ABF_TEST_REDIS_ADDR")
	if redisAddr == "" {
		redisAddr = "localhost:6399"
	}
	redisPass := os.Getenv("ABF_TEST_REDIS_PASSWORD")
	if redisPass == "" {
		redisPass = "pass"
	}
	dsn := os.Getenv("ABF_TEST_POSTGRES_DSN")
	if dsn == "" {
		dsn = "postgres://user:pass@localhost:5432/db?sslmode=disable"
	}
	return &config.Config{
		Server: config.ServerConfig{HttpPort: 8080},
		Limits: config.LimitsConfig{
			LoginAttempts:    3,
			PasswordAttempts: 1000,
			IPAttempts:       1000,
			WindowTime:       time.Minute,
		},
		Redis:    config.RedisConfig{Address: redisAddr, Password: redisPass},
		Postgres: config.PostgresConfig{Dsn: dsn},
	}
}

func prepareDB(t *testing.T, dsn string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	db, err := sql.Open("pgx", dsn)
	require.NoError(t, err)
	defer db.Close()
	require.NoError(t, db.PingContext(ctx),
		"postgres must be reachable at %q with the migrations applied (make migrate)", dsn)

	_, err = db.ExecContext(ctx, "truncate white_list, black_list")
	require.NoError(t, err)
}

func prepareRedis(t *testing.T, addr, password string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	client := goredis.NewClient(&goredis.Options{Addr: addr, Password: password})
	defer client.Close()
	require.NoError(t, client.FlushDB(ctx).Err(),
		"redis must be reachable at %q", addr)
}

func newTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	cfg := testConfig()
	prepareDB(t, cfg.Postgres.Dsn)
	prepareRedis(t, cfg.Redis.Address, cfg.Redis.Password)

	svc, closer, err := app.Build(cfg)
	require.NoError(t, err)

	ts := httptest.NewServer(api.NewServer(svc, "").Handler)
	t.Cleanup(func() {
		ts.Close()
		require.NoError(t, closer.Close())
	})
	return ts
}

func postJSON(t *testing.T, ts *httptest.Server, method, path, body string) *http.Response {
	t.Helper()
	req, err := http.NewRequest(method, ts.URL+path, bytes.NewBufferString(body))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	resp, err := ts.Client().Do(req)
	require.NoError(t, err)
	return resp
}

func authOK(t *testing.T, ts *httptest.Server, login, password, ip string) bool {
	t.Helper()
	resp := postJSON(t, ts, http.MethodPost, "/auth",
		fmt.Sprintf(`{"login":%q,"password":%q,"ip":%q}`, login, password, ip))
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)
	var out struct {
		OK bool `json:"ok"`
	}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&out))
	return out.OK
}

func TestHealthz(t *testing.T) {
	ts := newTestServer(t)
	resp := postJSON(t, ts, http.MethodGet, "/healthz", "")
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestAuthBlocksAfterLimit(t *testing.T) {
	ts := newTestServer(t)
	const login, password, ip = "alice", "hunter2", "10.10.0.1"

	for i := 0; i < 3; i++ {
		require.True(t, authOK(t, ts, login, password, ip), "attempt %d should be allowed", i+1)
	}
	require.False(t, authOK(t, ts, login, password, ip), "attempt over the limit should be blocked")
}

func TestResetUnblocks(t *testing.T) {
	ts := newTestServer(t)
	const login, password, ip = "bob", "pw", "10.10.0.2"

	for i := 0; i < 3; i++ {
		authOK(t, ts, login, password, ip)
	}
	require.False(t, authOK(t, ts, login, password, ip))

	resp := postJSON(t, ts, http.MethodPost, "/reset",
		fmt.Sprintf(`{"login":%q,"ip":%q}`, login, ip))
	require.Equal(t, http.StatusNoContent, resp.StatusCode)
	resp.Body.Close()

	require.True(t, authOK(t, ts, login, password, ip), "auth should be allowed again after reset")
}

func TestWhitelistAlwaysAllows(t *testing.T) {
	ts := newTestServer(t)
	const login, password, ip = "carol", "pw", "10.20.0.5"

	resp := postJSON(t, ts, http.MethodPost, "/whitelist", `{"subnet":"10.20.0.0/24"}`)
	require.Equal(t, http.StatusNoContent, resp.StatusCode)
	resp.Body.Close()

	for i := 0; i < 6; i++ {
		require.True(t, authOK(t, ts, login, password, ip), "whitelisted ip must always be allowed")
	}
}

func TestBlacklistAlwaysRejects(t *testing.T) {
	ts := newTestServer(t)
	const login, password, ip = "dave", "pw", "10.30.0.7"

	resp := postJSON(t, ts, http.MethodPost, "/blacklist", `{"subnet":"10.30.0.0/24"}`)
	require.Equal(t, http.StatusNoContent, resp.StatusCode)
	resp.Body.Close()

	require.False(t, authOK(t, ts, login, password, ip), "blacklisted ip must be rejected")
}

func TestListDeletion(t *testing.T) {
	ts := newTestServer(t)
	const login, password, ip = "erin", "pw", "10.40.0.9"

	resp := postJSON(t, ts, http.MethodPost, "/blacklist", `{"subnet":"10.40.0.0/24"}`)
	require.Equal(t, http.StatusNoContent, resp.StatusCode)
	resp.Body.Close()
	require.False(t, authOK(t, ts, login, password, ip))

	resp = postJSON(t, ts, http.MethodDelete, "/blacklist", `{"subnet":"10.40.0.0/24"}`)
	require.Equal(t, http.StatusNoContent, resp.StatusCode)
	resp.Body.Close()

	require.True(t, authOK(t, ts, login, password, ip), "auth should be allowed after blacklist entry removed")
}
