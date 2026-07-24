//go:build integration

package redis_test

import (
	"context"
	"os"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	goredis "github.com/redis/go-redis/v9"

	"github.com/lilxtent/anti-brute-force/internal/ratelimit"
	redisstore "github.com/lilxtent/anti-brute-force/internal/storage/redis"
)

const (
	defaultAddr     = "localhost:6399"
	defaultPassword = "pass"
)

type manualClock struct{ t time.Time }

func (c *manualClock) Now() time.Time          { return c.t }
func (c *manualClock) Advance(d time.Duration) { c.t = c.t.Add(d) }

var (
	addr     string
	password string
	client   *goredis.Client
)

func TestRedis(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Redis Store Integration Suite")
}

var _ = BeforeSuite(func() {
	addr = os.Getenv("ABF_TEST_REDIS_ADDR")
	if addr == "" {
		addr = defaultAddr
	}
	password = os.Getenv("ABF_TEST_REDIS_PASSWORD")
	if password == "" {
		password = defaultPassword
	}

	client = goredis.NewClient(&goredis.Options{Addr: addr, Password: password})

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	Expect(client.Ping(ctx).Err()).To(Succeed(),
		"redis must be reachable at %q (start it with `docker compose up -d redis` "+
			"or set ABF_TEST_REDIS_ADDR/ABF_TEST_REDIS_PASSWORD)", addr)
})

var _ = AfterSuite(func() {
	if client != nil {
		Expect(client.Close()).To(Succeed())
	}
})

func flush() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	Expect(client.FlushDB(ctx).Err()).To(Succeed())
}

var _ = Describe("Store", func() {
	var (
		ctx   context.Context
		clk   *manualClock
		store *redisstore.Store
		cfg   ratelimit.BucketConfig
	)

	BeforeEach(func() {
		flush()
		ctx = context.Background()
		clk = &manualClock{t: time.Unix(0, 0)}
		store = redisstore.New(client, clk)
		cfg = ratelimit.BucketConfig{Capacity: 3, Window: 3 * time.Second}
	})

	Describe("Allow", func() {
		It("admits up to capacity then rejects", func() {
			for i := 0; i < 3; i++ {
				ok, err := store.Allow(ctx, "login:alice", cfg)
				Expect(err).NotTo(HaveOccurred())
				Expect(ok).To(BeTrue(), "attempt %d should be admitted", i+1)
			}

			ok, err := store.Allow(ctx, "login:alice", cfg)
			Expect(err).NotTo(HaveOccurred())
			Expect(ok).To(BeFalse(), "attempt over capacity should be rejected")
		})

		It("refills fully after a full window elapses", func() {
			for i := 0; i < 3; i++ {
				_, err := store.Allow(ctx, "login:alice", cfg)
				Expect(err).NotTo(HaveOccurred())
			}

			clk.Advance(3 * time.Second) // leaks capacity (3) fully

			ok, err := store.Allow(ctx, "login:alice", cfg)
			Expect(err).NotTo(HaveOccurred())
			Expect(ok).To(BeTrue())
		})

		It("refills proportionally as time passes", func() {
			for i := 0; i < 3; i++ {
				_, err := store.Allow(ctx, "login:alice", cfg)
				Expect(err).NotTo(HaveOccurred())
			}

			clk.Advance(1 * time.Second) // leakRate is 1/s, so one slot frees up

			ok, err := store.Allow(ctx, "login:alice", cfg)
			Expect(err).NotTo(HaveOccurred())
			Expect(ok).To(BeTrue(), "the freed slot should be admitted")

			ok, err = store.Allow(ctx, "login:alice", cfg)
			Expect(err).NotTo(HaveOccurred())
			Expect(ok).To(BeFalse(), "bucket is saturated again")
		})

		It("keeps distinct keys independent", func() {
			for i := 0; i < 3; i++ {
				_, err := store.Allow(ctx, "login:alice", cfg)
				Expect(err).NotTo(HaveOccurred())
			}

			ok, err := store.Allow(ctx, "login:bob", cfg)
			Expect(err).NotTo(HaveOccurred())
			Expect(ok).To(BeTrue(), "a different key has its own bucket")
		})
	})

	Describe("Reset", func() {
		It("drains a saturated bucket", func() {
			for i := 0; i < 3; i++ {
				_, err := store.Allow(ctx, "login:alice", cfg)
				Expect(err).NotTo(HaveOccurred())
			}

			Expect(store.Reset(ctx, "login:alice")).To(Succeed())

			ok, err := store.Allow(ctx, "login:alice", cfg)
			Expect(err).NotTo(HaveOccurred())
			Expect(ok).To(BeTrue(), "reset bucket admits again")
		})

		It("is a no-op for an unknown key", func() {
			Expect(store.Reset(ctx, "login:nobody")).To(Succeed())
		})
	})
})
