package api

import (
	"context"
	"net/http"
	"net/netip"
	"time"
)

type Service interface {
	Allow(ctx context.Context, login, password, ip string) (bool, error)
	Reset(ctx context.Context, login, ip string) error
	AddToWhitelist(ctx context.Context, prefix netip.Prefix) error
	RemoveFromWhitelist(ctx context.Context, prefix netip.Prefix) error
	AddToBlacklist(ctx context.Context, prefix netip.Prefix) error
	RemoveFromBlacklist(ctx context.Context, prefix netip.Prefix) error
}

func NewServer(svc Service, addr string) *http.Server {
	h := &handler{svc: svc}

	mux := http.NewServeMux()
	mux.HandleFunc("POST /auth", h.auth)
	mux.HandleFunc("POST /reset", h.reset)
	mux.HandleFunc("POST /whitelist", h.addWhitelist)
	mux.HandleFunc("DELETE /whitelist", h.removeWhitelist)
	mux.HandleFunc("POST /blacklist", h.addBlacklist)
	mux.HandleFunc("DELETE /blacklist", h.removeBlacklist)
	mux.HandleFunc("GET /healthz", h.healthz)

	return &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}
}
