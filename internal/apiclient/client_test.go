package apiclient_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/lilxtent/anti-brute-force/internal/apiclient"
)

const timeout = 2 * time.Second

type capture struct {
	method string
	path   string
	body   string
}

func newServer(t *testing.T, status int, body string) (*apiclient.Client, *capture) {
	t.Helper()

	got := &capture{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		got.method, got.path, got.body = r.Method, r.URL.Path, string(raw)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		if body != "" {
			_, _ = w.Write([]byte(body))
		}
	}))
	t.Cleanup(srv.Close)

	client, err := apiclient.New(srv.URL, timeout)
	require.NoError(t, err)
	return client, got
}

func prefix(t *testing.T, s string) netip.Prefix {
	t.Helper()
	p, err := netip.ParsePrefix(s)
	require.NoError(t, err)
	return p
}

func TestNewRejectsInvalidBaseURL(t *testing.T) {
	for name, addr := range map[string]string{
		"empty":      "",
		"no scheme":  "localhost:8080",
		"bad scheme": "ftp://localhost:8080",
		"malformed":  "http://%zz",
	} {
		t.Run(name, func(t *testing.T) {
			_, err := apiclient.New(addr, timeout)
			require.Error(t, err)
		})
	}
}

func TestNewTrimsTrailingSlash(t *testing.T) {
	got := &capture{}
	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		got.path = r.URL.Path
	}))
	defer srv.Close()

	client, err := apiclient.New(srv.URL+"/", timeout)
	require.NoError(t, err)
	require.NoError(t, client.Health(context.Background()))
	require.Equal(t, "/healthz", got.path)
}

func TestCheck(t *testing.T) {
	t.Run("allowed", func(t *testing.T) {
		client, got := newServer(t, http.StatusOK, `{"ok":true}`)

		ok, err := client.Check(context.Background(), "bob", "s3cret", "1.2.3.4")

		require.NoError(t, err)
		require.True(t, ok)
		require.Equal(t, http.MethodPost, got.method)
		require.Equal(t, "/auth", got.path)
		require.JSONEq(t, `{"login":"bob","password":"s3cret","ip":"1.2.3.4"}`, got.body)
	})

	t.Run("denied", func(t *testing.T) {
		client, _ := newServer(t, http.StatusOK, `{"ok":false}`)

		ok, err := client.Check(context.Background(), "bob", "s3cret", "1.2.3.4")

		require.NoError(t, err)
		require.False(t, ok)
	})
}

func TestReset(t *testing.T) {
	client, got := newServer(t, http.StatusNoContent, "")

	require.NoError(t, client.Reset(context.Background(), "bob", "1.2.3.4"))

	require.Equal(t, http.MethodPost, got.method)
	require.Equal(t, "/reset", got.path)
	require.JSONEq(t, `{"login":"bob","ip":"1.2.3.4"}`, got.body)
}

func TestSubnetMethods(t *testing.T) {
	tests := []struct {
		name   string
		call   func(*apiclient.Client, context.Context, netip.Prefix) error
		method string
		path   string
	}{
		{"add whitelist", (*apiclient.Client).AddWhitelist, http.MethodPost, "/whitelist"},
		{"remove whitelist", (*apiclient.Client).RemoveWhitelist, http.MethodDelete, "/whitelist"},
		{"add blacklist", (*apiclient.Client).AddBlacklist, http.MethodPost, "/blacklist"},
		{"remove blacklist", (*apiclient.Client).RemoveBlacklist, http.MethodDelete, "/blacklist"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, got := newServer(t, http.StatusNoContent, "")

			require.NoError(t, tt.call(client, context.Background(), prefix(t, "192.1.1.0/25")))

			require.Equal(t, tt.method, got.method)
			require.Equal(t, tt.path, got.path)
			require.JSONEq(t, `{"subnet":"192.1.1.0/25"}`, got.body)
		})
	}
}

func TestHealth(t *testing.T) {
	client, got := newServer(t, http.StatusOK, `{"status":"ok"}`)

	require.NoError(t, client.Health(context.Background()))

	require.Equal(t, http.MethodGet, got.method)
	require.Equal(t, "/healthz", got.path)
	require.Empty(t, got.body)
}

func TestServerErrorResponses(t *testing.T) {
	t.Run("json error body surfaces the message", func(t *testing.T) {
		client, _ := newServer(t, http.StatusBadRequest, `{"error":"invalid subnet: bad prefix"}`)

		err := client.AddBlacklist(context.Background(), prefix(t, "10.0.0.0/8"))

		require.ErrorContains(t, err, "invalid subnet: bad prefix")
		require.ErrorContains(t, err, "400")
	})

	t.Run("non-json body falls back to the status", func(t *testing.T) {
		client, _ := newServer(t, http.StatusBadGateway, "<html>bad gateway</html>")

		err := client.Health(context.Background())

		require.ErrorContains(t, err, "502")
	})

	t.Run("unexpected status on check", func(t *testing.T) {
		client, _ := newServer(t, http.StatusInternalServerError, `{"error":"internal error"}`)

		_, err := client.Check(context.Background(), "bob", "s3cret", "1.2.3.4")

		require.ErrorContains(t, err, "internal error")
	})
}

func TestUnreachableServer(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	addr := srv.URL
	srv.Close()

	client, err := apiclient.New(addr, timeout)
	require.NoError(t, err)

	require.Error(t, client.Health(context.Background()))
}

func TestContextCancellation(t *testing.T) {
	client, _ := newServer(t, http.StatusOK, `{"status":"ok"}`)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	require.ErrorIs(t, client.Health(ctx), context.Canceled)
}
