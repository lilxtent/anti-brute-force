package ratelimit

import (
	"context"
	"sync"
	"time"
)

type memoryEntry struct {
	bucket     *Bucket
	lastAccess time.Time
}

type MemoryStore struct {
	clock    Clock
	ttl      time.Duration
	mu       sync.Mutex
	buckets  map[string]*memoryEntry
	stop     chan struct{}
	stopOnce sync.Once
}

var _ Store = (*MemoryStore)(nil)

func NewMemoryStore(clock Clock, ttl, sweepInterval time.Duration) *MemoryStore {
	s := &MemoryStore{
		clock:   clock,
		ttl:     ttl,
		buckets: make(map[string]*memoryEntry),
		stop:    make(chan struct{}),
	}
	if sweepInterval > 0 {
		go s.sweepLoop(sweepInterval)
	}
	return s
}

func (s *MemoryStore) Allow(_ context.Context, key string, cfg BucketConfig) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	e, ok := s.buckets[key]
	if !ok {
		e = &memoryEntry{bucket: NewBucket(cfg.Capacity, cfg.Window, s.clock)}
		s.buckets[key] = e
	}
	e.lastAccess = s.clock.Now()
	return e.bucket.Allow(), nil
}

func (s *MemoryStore) Reset(_ context.Context, key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if e, ok := s.buckets[key]; ok {
		e.bucket.Reset()
		e.lastAccess = s.clock.Now()
	}
	return nil
}

func (s *MemoryStore) Close() error {
	s.stopOnce.Do(func() { close(s.stop) })
	return nil
}

func (s *MemoryStore) sweepLoop(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-s.stop:
			return
		case <-ticker.C:
			s.evictIdle()
		}
	}
}

func (s *MemoryStore) evictIdle() {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := s.clock.Now()
	for key, e := range s.buckets {
		if now.Sub(e.lastAccess) >= s.ttl && e.bucket.Level() == 0 {
			delete(s.buckets, key)
		}
	}
}

func (s *MemoryStore) size() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.buckets)
}
