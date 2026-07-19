package ratelimit

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type manualClock struct{ t time.Time }

func (c *manualClock) Now() time.Time          { return c.t }
func (c *manualClock) Advance(d time.Duration) { c.t = c.t.Add(d) }

func TestBucket_AllowsBurstUpToCapacity(t *testing.T) {
	clk := &manualClock{t: time.Unix(0, 0)}
	b := NewBucket(3, time.Minute, clk)

	for i := 0; i < 3; i++ {
		require.Truef(t, b.Allow(), "attempt %d should be allowed", i)
	}
	require.False(t, b.Allow(), "attempt over capacity should be denied")
}

func TestBucket_LeaksOverTime(t *testing.T) {
	clk := &manualClock{t: time.Unix(0, 0)}
	b := NewBucket(60, time.Minute, clk)

	for i := 0; i < 60; i++ {
		require.True(t, b.Allow())
	}
	require.False(t, b.Allow(), "bucket full")

	clk.Advance(2 * time.Second)
	require.True(t, b.Allow())
	require.True(t, b.Allow())
	require.False(t, b.Allow(), "only 2 drops had leaked")
}

func TestBucket_Reset(t *testing.T) {
	clk := &manualClock{t: time.Unix(0, 0)}
	b := NewBucket(2, time.Minute, clk)

	require.True(t, b.Allow())
	require.True(t, b.Allow())
	require.False(t, b.Allow())

	b.Reset()
	require.True(t, b.Allow(), "reset should empty the bucket")
}

func TestBucket_LevelNeverNegative(t *testing.T) {
	clk := &manualClock{t: time.Unix(0, 0)}
	b := NewBucket(10, time.Minute, clk)

	require.True(t, b.Allow())
	clk.Advance(time.Hour)
	require.Equal(t, 0.0, b.Level())
}

func TestBucket_LevelReflectsFractionalSecondLeak(t *testing.T) {
	clk := &manualClock{t: time.Unix(0, 0)}
	b := NewBucket(1, time.Second, clk)

	require.True(t, b.Allow())
	clk.Advance(500 * time.Millisecond)
	require.InDelta(t, 0.5, b.Level(), 0.01)
}

func TestNewBucket_PanicsOnNonPositiveCapacity(t *testing.T) {
	clk := &manualClock{t: time.Unix(0, 0)}
	require.Panics(t, func() {
		NewBucket(0, time.Minute, clk)
	})
}

func TestNewBucket_PanicsOnNonPositiveWindow(t *testing.T) {
	clk := &manualClock{t: time.Unix(0, 0)}
	require.Panics(t, func() {
		NewBucket(1, 0, clk)
	})
}
