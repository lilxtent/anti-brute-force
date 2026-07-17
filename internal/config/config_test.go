package config

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"
)

func TestLoad_Valid(t *testing.T) {
	cfg, err := Load("testdata/valid.yaml")
	require.NoError(t, err)
	require.NotNil(t, cfg)

	want := &Config{
		Server: ServerConfig{HttpPort: 8080},
		Limits: LimitsConfig{
			LoginAttempts:    10,
			PasswordAttempts: 100,
			IPAttempts:       1000,
			WindowTime:       time.Minute,
		},
		Redis: RedisConfig{
			Address:  "localhost:6379",
			Password: "s3cret",
		},
		Postgres: PostgresConfig{
			Dsn: "postgres://user:pass@localhost:5432/db",
		},
	}

	if diff := cmp.Diff(want, cfg); diff != "" {
		t.Errorf("Load() mismatch:\n%s", diff)
	}
}

func TestLoad_FileNotFound(t *testing.T) {
	_, err := Load(filepath.Join(t.TempDir(), "does-not-exist.yaml"))
	require.Error(t, err)
}

func TestLoad_MalformedContent(t *testing.T) {
	_, err := Load("testdata/malformed.yaml")
	require.Error(t, err)
}

func TestLoad_UnsupportedExtension(t *testing.T) {
	_, err := Load("testdata/valid.conf")
	require.Error(t, err)
}

func TestLoad_MissingRequiredFields(t *testing.T) {
	fixtures := []string{
		"missing_server.yaml",
		"missing_http_port.yaml",
		"missing_limits.yaml",
		"missing_redis.yaml",
		"missing_postgres.yaml",
		"empty.yaml",
	}

	for _, name := range fixtures {
		t.Run(name, func(t *testing.T) {
			_, err := Load(filepath.Join("testdata", name))
			require.Error(t, err)
		})
	}
}
