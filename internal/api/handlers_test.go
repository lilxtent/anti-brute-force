package api_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/lilxtent/anti-brute-force/internal/api"
)

type fakeService struct {
	allowOK   bool
	allowErr  error
	resetErr  error
	subnetErr error

	lastPrefix netip.Prefix
	resetCalls int
}

func (f *fakeService) Allow(_ context.Context, _, _, _ string) (bool, error) {
	return f.allowOK, f.allowErr
}
func (f *fakeService) Reset(_ context.Context, _, _ string) error {
	f.resetCalls++
	return f.resetErr
}
func (f *fakeService) AddToWhitelist(_ context.Context, p netip.Prefix) error {
	f.lastPrefix = p
	return f.subnetErr
}
func (f *fakeService) RemoveFromWhitelist(_ context.Context, p netip.Prefix) error {
	f.lastPrefix = p
	return f.subnetErr
}
func (f *fakeService) AddToBlacklist(_ context.Context, p netip.Prefix) error {
	f.lastPrefix = p
	return f.subnetErr
}
func (f *fakeService) RemoveFromBlacklist(_ context.Context, p netip.Prefix) error {
	f.lastPrefix = p
	return f.subnetErr
}

func do(t *testing.T, svc api.Service, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	srv := api.NewServer(svc, "")
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	rec := httptest.NewRecorder()
	srv.Handler.ServeHTTP(rec, req)
	return rec
}

func TestAuth(t *testing.T) {
	t.Run("allowed returns ok true", func(t *testing.T) {
		rec := do(t, &fakeService{allowOK: true},
			http.MethodPost, "/auth", `{"login":"a","password":"p","ip":"10.0.0.1"}`)
		require.Equal(t, http.StatusOK, rec.Code)
		require.JSONEq(t, `{"ok":true}`, rec.Body.String())
	})

	t.Run("blocked returns ok false", func(t *testing.T) {
		rec := do(t, &fakeService{allowOK: false},
			http.MethodPost, "/auth", `{"login":"a","password":"p","ip":"10.0.0.1"}`)
		require.Equal(t, http.StatusOK, rec.Code)
		require.JSONEq(t, `{"ok":false}`, rec.Body.String())
	})

	t.Run("missing field is 400", func(t *testing.T) {
		rec := do(t, &fakeService{},
			http.MethodPost, "/auth", `{"login":"a","ip":"10.0.0.1"}`)
		require.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("invalid ip is 400", func(t *testing.T) {
		rec := do(t, &fakeService{},
			http.MethodPost, "/auth", `{"login":"a","password":"p","ip":"nope"}`)
		require.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("malformed json is 400", func(t *testing.T) {
		rec := do(t, &fakeService{}, http.MethodPost, "/auth", `{`)
		require.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("downstream error is 500", func(t *testing.T) {
		rec := do(t, &fakeService{allowErr: errors.New("redis down")},
			http.MethodPost, "/auth", `{"login":"a","password":"p","ip":"10.0.0.1"}`)
		require.Equal(t, http.StatusInternalServerError, rec.Code)
		require.NotContains(t, rec.Body.String(), "redis down")
	})
}

func TestReset(t *testing.T) {
	t.Run("success is 204 and calls service", func(t *testing.T) {
		f := &fakeService{}
		rec := do(t, f, http.MethodPost, "/reset", `{"login":"a","ip":"10.0.0.1"}`)
		require.Equal(t, http.StatusNoContent, rec.Code)
		require.Equal(t, 1, f.resetCalls)
	})

	t.Run("missing field is 400", func(t *testing.T) {
		rec := do(t, &fakeService{}, http.MethodPost, "/reset", `{"login":"a"}`)
		require.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("invalid ip is 400", func(t *testing.T) {
		rec := do(t, &fakeService{}, http.MethodPost, "/reset", `{"login":"a","ip":"nope"}`)
		require.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("downstream error is 500", func(t *testing.T) {
		rec := do(t, &fakeService{resetErr: errors.New("redis down")},
			http.MethodPost, "/reset", `{"login":"a","ip":"10.0.0.1"}`)
		require.Equal(t, http.StatusInternalServerError, rec.Code)
		require.NotContains(t, rec.Body.String(), "redis down")
	})
}

func TestSubnetLists(t *testing.T) {
	paths := []struct {
		method, path string
	}{
		{http.MethodPost, "/whitelist"},
		{http.MethodDelete, "/whitelist"},
		{http.MethodPost, "/blacklist"},
		{http.MethodDelete, "/blacklist"},
	}
	for _, p := range paths {
		t.Run(p.method+" "+p.path+" valid subnet is 204", func(t *testing.T) {
			f := &fakeService{}
			rec := do(t, f, p.method, p.path, `{"subnet":"192.1.1.0/25"}`)
			require.Equal(t, http.StatusNoContent, rec.Code)
			require.Equal(t, "192.1.1.0/25", f.lastPrefix.String())
		})
		t.Run(p.method+" "+p.path+" invalid subnet is 400", func(t *testing.T) {
			rec := do(t, &fakeService{}, p.method, p.path, `{"subnet":"nope"}`)
			require.Equal(t, http.StatusBadRequest, rec.Code)
		})
		t.Run(p.method+" "+p.path+" downstream error is 500", func(t *testing.T) {
			rec := do(t, &fakeService{subnetErr: errors.New("pg down")},
				p.method, p.path, `{"subnet":"192.1.1.0/25"}`)
			require.Equal(t, http.StatusInternalServerError, rec.Code)
			require.NotContains(t, rec.Body.String(), "pg down")
		})
	}
}

func TestHealthz(t *testing.T) {
	rec := do(t, &fakeService{}, http.MethodGet, "/healthz", "")
	require.Equal(t, http.StatusOK, rec.Code)
	require.JSONEq(t, `{"status":"ok"}`, rec.Body.String())
}

func TestMethodNotAllowed(t *testing.T) {
	rec := do(t, &fakeService{}, http.MethodGet, "/auth", "")
	require.Equal(t, http.StatusMethodNotAllowed, rec.Code)
}
