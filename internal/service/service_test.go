package service

import (
	"context"
	"errors"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/require"
)

type fakeLimiter struct {
	allowCalled bool
	allowResult bool
	allowErr    error

	resetCalled bool
	resetLogin  string
	resetIP     string
	resetErr    error
}

func (f *fakeLimiter) Allow(_ context.Context, _, _, _ string) (bool, error) {
	f.allowCalled = true
	return f.allowResult, f.allowErr
}

func (f *fakeLimiter) Reset(_ context.Context, login, ip string) error {
	f.resetCalled = true
	f.resetLogin, f.resetIP = login, ip
	return f.resetErr
}

type fakeRepo struct {
	white    []netip.Prefix
	black    []netip.Prefix
	whiteErr error
	blackErr error
	mutErr   error

	whiteAdd []netip.Prefix
	whiteDel []netip.Prefix
	blackAdd []netip.Prefix
	blackDel []netip.Prefix
}

func (f *fakeRepo) GetWhiteList(_ context.Context) ([]netip.Prefix, error) {
	if f.whiteErr != nil {
		return nil, f.whiteErr
	}
	return f.white, nil
}

func (f *fakeRepo) GetBlackList(_ context.Context) ([]netip.Prefix, error) {
	if f.blackErr != nil {
		return nil, f.blackErr
	}
	return f.black, nil
}

func (f *fakeRepo) AddToWhiteList(_ context.Context, p netip.Prefix) error {
	if f.mutErr != nil {
		return f.mutErr
	}
	f.whiteAdd = append(f.whiteAdd, p)
	return nil
}

func (f *fakeRepo) DeleteFromWhiteList(_ context.Context, p netip.Prefix) error {
	if f.mutErr != nil {
		return f.mutErr
	}
	f.whiteDel = append(f.whiteDel, p)
	return nil
}

func (f *fakeRepo) AddToBlackList(_ context.Context, p netip.Prefix) error {
	if f.mutErr != nil {
		return f.mutErr
	}
	f.blackAdd = append(f.blackAdd, p)
	return nil
}

func (f *fakeRepo) DeleteFromBlackList(_ context.Context, p netip.Prefix) error {
	if f.mutErr != nil {
		return f.mutErr
	}
	f.blackDel = append(f.blackDel, p)
	return nil
}

func mustPrefix(t *testing.T, s string) netip.Prefix {
	t.Helper()
	p, err := netip.ParsePrefix(s)
	require.NoError(t, err)
	return p
}

func loadedService(t *testing.T, lim Limiter, repo SubnetRepository) *Service {
	t.Helper()
	svc := New(lim, repo)
	require.NoError(t, svc.Load(context.Background()))
	return svc
}

func TestLoad_PopulatesBothLists(t *testing.T) {
	repo := &fakeRepo{
		white: []netip.Prefix{mustPrefix(t, "10.0.0.0/8")},
		black: []netip.Prefix{mustPrefix(t, "192.168.0.0/16")},
	}
	lim := &fakeLimiter{allowResult: false}
	svc := loadedService(t, lim, repo)

	ok, err := svc.Allow(context.Background(), "u", "p", "10.1.1.1")
	require.NoError(t, err)
	require.True(t, ok, "loaded whitelist entry must be allowed")

	ok, err = svc.Allow(context.Background(), "u", "p", "192.168.5.5")
	require.NoError(t, err)
	require.False(t, ok, "loaded blacklist entry must be denied")

	require.False(t, lim.allowCalled, "list hits must not consult the limiter")
}

func TestLoad_WhitelistErrorPropagates(t *testing.T) {
	sentinel := errors.New("get whitelist boom")
	svc := New(&fakeLimiter{}, &fakeRepo{whiteErr: sentinel})

	require.ErrorIs(t, svc.Load(context.Background()), sentinel)
}

func TestLoad_BlacklistErrorPropagates(t *testing.T) {
	sentinel := errors.New("get blacklist boom")
	svc := New(&fakeLimiter{}, &fakeRepo{blackErr: sentinel})

	require.ErrorIs(t, svc.Load(context.Background()), sentinel)
}

func TestAllow_WhitelistWinsWithoutTouchingLimiter(t *testing.T) {
	repo := &fakeRepo{
		white: []netip.Prefix{mustPrefix(t, "10.0.0.0/8")},
		black: []netip.Prefix{mustPrefix(t, "10.1.2.3/32")}, // also blacklisted; whitelist must win
	}
	lim := &fakeLimiter{allowResult: false}
	svc := loadedService(t, lim, repo)

	ok, err := svc.Allow(context.Background(), "u", "p", "10.1.2.3")
	require.NoError(t, err)
	require.True(t, ok)
	require.False(t, lim.allowCalled, "limiter must not be consulted on a whitelist hit")
}

func TestAllow_BlacklistBlocksWithoutTouchingLimiter(t *testing.T) {
	repo := &fakeRepo{black: []netip.Prefix{mustPrefix(t, "192.168.0.0/16")}}
	lim := &fakeLimiter{allowResult: true}
	svc := loadedService(t, lim, repo)

	ok, err := svc.Allow(context.Background(), "u", "p", "192.168.5.5")
	require.NoError(t, err)
	require.False(t, ok)
	require.False(t, lim.allowCalled, "limiter must not be consulted on a blacklist hit")
}

func TestAllow_FallsThroughToLimiter(t *testing.T) {
	for _, tc := range []struct {
		name   string
		result bool
	}{
		{"limiter allows", true},
		{"limiter denies", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			lim := &fakeLimiter{allowResult: tc.result}
			svc := New(lim, &fakeRepo{})

			ok, err := svc.Allow(context.Background(), "u", "p", "8.8.8.8")
			require.NoError(t, err)
			require.Equal(t, tc.result, ok)
			require.True(t, lim.allowCalled, "limiter must decide when neither list matches")
		})
	}
}

func TestAllow_LimiterErrorPropagates(t *testing.T) {
	sentinel := errors.New("limiter boom")
	lim := &fakeLimiter{allowErr: sentinel}
	svc := New(lim, &fakeRepo{})

	ok, err := svc.Allow(context.Background(), "u", "p", "8.8.8.8")
	require.ErrorIs(t, err, sentinel)
	require.False(t, ok)
}

func TestAllow_InvalidIPErrorsWithoutTouchingLimiter(t *testing.T) {
	lim := &fakeLimiter{allowResult: true}
	svc := New(lim, &fakeRepo{})

	ok, err := svc.Allow(context.Background(), "u", "p", "not-an-ip")
	require.Error(t, err)
	require.False(t, ok)
	require.False(t, lim.allowCalled)
}

func TestReset_DelegatesToLimiter(t *testing.T) {
	lim := &fakeLimiter{}
	svc := New(lim, &fakeRepo{})

	require.NoError(t, svc.Reset(context.Background(), "u", "1.2.3.4"))
	require.True(t, lim.resetCalled)
	require.Equal(t, "u", lim.resetLogin)
	require.Equal(t, "1.2.3.4", lim.resetIP)
}

func TestReset_ErrorPropagates(t *testing.T) {
	sentinel := errors.New("reset boom")
	lim := &fakeLimiter{resetErr: sentinel}
	svc := New(lim, &fakeRepo{})

	require.ErrorIs(t, svc.Reset(context.Background(), "u", "1.2.3.4"), sentinel)
}

func TestAddToWhitelist_WritesThroughToRepoAndSet(t *testing.T) {
	repo := &fakeRepo{}
	lim := &fakeLimiter{allowResult: false}
	svc := New(lim, repo)
	p := mustPrefix(t, "203.0.113.0/24")

	require.NoError(t, svc.AddToWhitelist(context.Background(), p))
	require.Equal(t, []netip.Prefix{p}, repo.whiteAdd, "repo must be written")

	ok, err := svc.Allow(context.Background(), "u", "p", "203.0.113.9")
	require.NoError(t, err)
	require.True(t, ok, "added prefix must now be whitelisted in memory")
	require.False(t, lim.allowCalled, "a whitelist hit must not consult the limiter")
}

func TestAddToBlacklist_WritesThroughToRepoAndSet(t *testing.T) {
	repo := &fakeRepo{}
	lim := &fakeLimiter{allowResult: true}
	svc := New(lim, repo)
	p := mustPrefix(t, "203.0.113.0/24")

	require.NoError(t, svc.AddToBlacklist(context.Background(), p))
	require.Equal(t, []netip.Prefix{p}, repo.blackAdd, "repo must be written")

	ok, err := svc.Allow(context.Background(), "u", "p", "203.0.113.9")
	require.NoError(t, err)
	require.False(t, ok, "added prefix must now be blacklisted in memory")
	require.False(t, lim.allowCalled, "a blacklist hit must not consult the limiter")
}

func TestRemoveFromWhitelist_WritesThroughToRepoAndSet(t *testing.T) {
	p := mustPrefix(t, "203.0.113.0/24")
	repo := &fakeRepo{white: []netip.Prefix{p}}
	lim := &fakeLimiter{allowResult: false}
	svc := loadedService(t, lim, repo)

	require.NoError(t, svc.RemoveFromWhitelist(context.Background(), p))
	require.Equal(t, []netip.Prefix{p}, repo.whiteDel, "repo must be written")

	ok, err := svc.Allow(context.Background(), "u", "p", "203.0.113.9")
	require.NoError(t, err)
	require.False(t, ok, "removed prefix must no longer be whitelisted")
	require.True(t, lim.allowCalled, "after removal the IP falls through to the limiter")
}

func TestRemoveFromBlacklist_WritesThroughToRepoAndSet(t *testing.T) {
	p := mustPrefix(t, "203.0.113.0/24")
	repo := &fakeRepo{black: []netip.Prefix{p}}
	lim := &fakeLimiter{allowResult: true}
	svc := loadedService(t, lim, repo)

	require.NoError(t, svc.RemoveFromBlacklist(context.Background(), p))
	require.Equal(t, []netip.Prefix{p}, repo.blackDel, "repo must be written")

	ok, err := svc.Allow(context.Background(), "u", "p", "203.0.113.9")
	require.NoError(t, err)
	require.True(t, ok, "removed prefix must no longer be blacklisted")
	require.True(t, lim.allowCalled, "after removal the IP falls through to the limiter")
}

func TestMutation_RepoErrorLeavesSetUnchanged(t *testing.T) {
	sentinel := errors.New("repo boom")
	p := mustPrefix(t, "203.0.113.0/24")
	const ip = "203.0.113.9"

	tests := []struct {
		name          string
		preloadWhite  bool
		preloadBlack  bool
		mutate        func(svc *Service) error
		limiterResult bool
		wantAllow     bool
		wantLimiter   bool
	}{
		{
			name:          "AddToWhitelist",
			mutate:        func(s *Service) error { return s.AddToWhitelist(context.Background(), p) },
			limiterResult: false,
			wantAllow:     false,
			wantLimiter:   true,
		},
		{
			name:          "AddToBlacklist",
			mutate:        func(s *Service) error { return s.AddToBlacklist(context.Background(), p) },
			limiterResult: true,
			wantAllow:     true,
			wantLimiter:   true,
		},
		{
			name:          "RemoveFromWhitelist",
			preloadWhite:  true,
			mutate:        func(s *Service) error { return s.RemoveFromWhitelist(context.Background(), p) },
			limiterResult: false,
			wantAllow:     true,
			wantLimiter:   false,
		},
		{
			name:          "RemoveFromBlacklist",
			preloadBlack:  true,
			mutate:        func(s *Service) error { return s.RemoveFromBlacklist(context.Background(), p) },
			limiterResult: true,
			wantAllow:     false,
			wantLimiter:   false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			repo := &fakeRepo{mutErr: sentinel}
			if tc.preloadWhite {
				repo.white = []netip.Prefix{p}
			}
			if tc.preloadBlack {
				repo.black = []netip.Prefix{p}
			}
			lim := &fakeLimiter{allowResult: tc.limiterResult}
			svc := loadedService(t, lim, repo)

			require.ErrorIs(t, tc.mutate(svc), sentinel)

			ok, err := svc.Allow(context.Background(), "u", "p", ip)
			require.NoError(t, err)
			require.Equal(t, tc.wantAllow, ok, "set must not change when the repo write fails")
			require.Equal(t, tc.wantLimiter, lim.allowCalled)
		})
	}
}
