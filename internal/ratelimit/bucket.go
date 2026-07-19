package ratelimit

import (
	"math"
	"time"
)

type Bucket struct {
	capacity float64
	leakRate float64
	clock    Clock

	level    float64
	lastLeak time.Time
}

func NewBucket(capacity int, window time.Duration, clock Clock) *Bucket {
	if capacity <= 0 {
		panic("ratelimit: NewBucket capacity must be > 0")
	}
	if window <= 0 {
		panic("ratelimit: NewBucket window must be > 0")
	}
	return &Bucket{
		capacity: float64(capacity),
		leakRate: float64(capacity) / window.Seconds(),
		clock:    clock,
		lastLeak: clock.Now(),
	}
}

func (b *Bucket) Allow() bool {
	b.leak()
	if b.level+1 <= b.capacity {
		b.level++
		return true
	}
	return false
}

func (b *Bucket) Reset() {
	b.level = 0
	b.lastLeak = b.clock.Now()
}

func (b *Bucket) Level() float64 {
	b.leak()
	return b.level
}

func (b *Bucket) leak() {
	now := b.clock.Now()
	elapsed := now.Sub(b.lastLeak).Seconds()
	if elapsed > 0 {
		b.level = math.Max(0, b.level-elapsed*b.leakRate)
		b.lastLeak = now
	}
}
