package service

import (
	"context"
	"fmt"
	"net/netip"

	"github.com/lilxtent/anti-brute-force/internal/subnet"
)

type Limiter interface {
	Allow(ctx context.Context, login, password, ip string) (bool, error)
	Reset(ctx context.Context, login, ip string) error
}

type SubnetRepository interface {
	GetWhiteList(ctx context.Context) ([]netip.Prefix, error)
	GetBlackList(ctx context.Context) ([]netip.Prefix, error)
	AddToWhiteList(ctx context.Context, prefix netip.Prefix) error
	DeleteFromWhiteList(ctx context.Context, prefix netip.Prefix) error
	AddToBlackList(ctx context.Context, prefix netip.Prefix) error
	DeleteFromBlackList(ctx context.Context, prefix netip.Prefix) error
}

type Service struct {
	limiter   Limiter
	repo      SubnetRepository
	whitelist *subnet.Set
	blacklist *subnet.Set
}

func New(limiter Limiter, repo SubnetRepository) *Service {
	return &Service{
		limiter:   limiter,
		repo:      repo,
		whitelist: &subnet.Set{},
		blacklist: &subnet.Set{},
	}
}

func (s *Service) Load(ctx context.Context) error {
	white, err := s.repo.GetWhiteList(ctx)
	if err != nil {
		return fmt.Errorf("load whitelist: %w", err)
	}
	black, err := s.repo.GetBlackList(ctx)
	if err != nil {
		return fmt.Errorf("load blacklist: %w", err)
	}
	s.whitelist.Load(white)
	s.blacklist.Load(black)
	return nil
}

func (s *Service) Allow(ctx context.Context, login, password, ip string) (bool, error) {
	addr, err := netip.ParseAddr(ip)
	if err != nil {
		return false, fmt.Errorf("parse ip %q: %w", ip, err)
	}
	if s.whitelist.Contains(addr) {
		return true, nil
	}
	if s.blacklist.Contains(addr) {
		return false, nil
	}
	return s.limiter.Allow(ctx, login, password, ip)
}

func (s *Service) Reset(ctx context.Context, login, ip string) error {
	return s.limiter.Reset(ctx, login, ip)
}

func (s *Service) AddToWhitelist(ctx context.Context, prefix netip.Prefix) error {
	if err := s.repo.AddToWhiteList(ctx, prefix); err != nil {
		return err
	}
	s.whitelist.Add(prefix)
	return nil
}

func (s *Service) RemoveFromWhitelist(ctx context.Context, prefix netip.Prefix) error {
	if err := s.repo.DeleteFromWhiteList(ctx, prefix); err != nil {
		return err
	}
	s.whitelist.Remove(prefix)
	return nil
}

func (s *Service) AddToBlacklist(ctx context.Context, prefix netip.Prefix) error {
	if err := s.repo.AddToBlackList(ctx, prefix); err != nil {
		return err
	}
	s.blacklist.Add(prefix)
	return nil
}

func (s *Service) RemoveFromBlacklist(ctx context.Context, prefix netip.Prefix) error {
	if err := s.repo.DeleteFromBlackList(ctx, prefix); err != nil {
		return err
	}
	s.blacklist.Remove(prefix)
	return nil
}
