//go:build integration

package postgres_test

import (
	"context"
	"database/sql"
	"net/netip"
	"os"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/lilxtent/anti-brute-force/internal/storage/postgres"
)

// defaultDSN matches the postgres service defined in docker-compose.yml.
const defaultDSN = "postgres://user:pass@localhost:5432/db?sslmode=disable"

var (
	dsn     string
	adminDB *sql.DB
)

func TestPostgres(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Postgres SubnetRepository Integration Suite")
}

var _ = BeforeSuite(func() {
	dsn = os.Getenv("ABF_TEST_POSTGRES_DSN")
	if dsn == "" {
		dsn = defaultDSN
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var err error
	adminDB, err = sql.Open("pgx", dsn)
	Expect(err).NotTo(HaveOccurred())
	Expect(adminDB.PingContext(ctx)).To(Succeed(),
		"postgres must be reachable at %q with the migrations applied (`make migrate`, "+
			"or set ABF_TEST_POSTGRES_DSN)", dsn)
})

var _ = AfterSuite(func() {
	if adminDB != nil {
		Expect(adminDB.Close()).To(Succeed())
	}
})

// truncate clears both lists so each spec starts from a clean state.
func truncate() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := adminDB.ExecContext(ctx, "truncate white_list, black_list")
	Expect(err).NotTo(HaveOccurred())
}

var _ = Describe("SubnetRepository", func() {
	var (
		ctx  context.Context
		repo *postgres.SubnetRepository
	)

	BeforeEach(func() {
		ctx = context.Background()
		truncate()
		repo = postgres.New(dsn)
		Expect(repo.Connect()).To(Succeed())
	})

	Describe("Connect", func() {
		It("errors when the DSN is empty", func() {
			err := postgres.New("").Connect()
			Expect(err).To(MatchError(postgres.ErrNotConnected))
		})

		It("is idempotent when already connected", func() {
			Expect(repo.Connect()).To(Succeed())
		})
	})

	DescribeTable("adding and reading back a prefix",
		func(list postgres.ListType, add func(context.Context, netip.Prefix) error) {
			Expect(add(ctx, netip.MustParsePrefix("192.168.1.0/24"))).To(Succeed())

			got, err := repo.GetAll(ctx, list)
			Expect(err).NotTo(HaveOccurred())
			Expect(got).To(ConsistOf(netip.MustParsePrefix("192.168.1.0/24")))
		},
		Entry("white list", postgres.WhiteList, func(c context.Context, p netip.Prefix) error {
			return repo.AddToWhiteList(c, p)
		}),
		Entry("black list", postgres.BlackList, func(c context.Context, p netip.Prefix) error {
			return repo.AddToBlackList(c, p)
		}),
	)

	Describe("GetAll", func() {
		It("returns empty when the list has no rows", func() {
			got, err := repo.GetAll(ctx, postgres.WhiteList)
			Expect(err).NotTo(HaveOccurred())
			Expect(got).To(BeEmpty())
		})

		It("returns every stored prefix", func() {
			Expect(repo.AddToWhiteList(ctx, netip.MustParsePrefix("10.0.0.0/8"))).To(Succeed())
			Expect(repo.AddToWhiteList(ctx, netip.MustParsePrefix("172.16.0.0/16"))).To(Succeed())
			Expect(repo.AddToWhiteList(ctx, netip.MustParsePrefix("2001:db8::/32"))).To(Succeed())

			got, err := repo.GetAll(ctx, postgres.WhiteList)
			Expect(err).NotTo(HaveOccurred())
			Expect(got).To(ConsistOf(
				netip.MustParsePrefix("10.0.0.0/8"),
				netip.MustParsePrefix("172.16.0.0/16"),
				netip.MustParsePrefix("2001:db8::/32"),
			))
		})
	})

	Describe("AddToWhiteList", func() {
		It("stores the masked form of a prefix with host bits set", func() {
			Expect(repo.AddToWhiteList(ctx, netip.MustParsePrefix("192.168.1.42/24"))).To(Succeed())

			got, err := repo.GetAll(ctx, postgres.WhiteList)
			Expect(err).NotTo(HaveOccurred())
			Expect(got).To(ConsistOf(netip.MustParsePrefix("192.168.1.0/24")))
		})

		It("is idempotent on conflict", func() {
			Expect(repo.AddToWhiteList(ctx, netip.MustParsePrefix("10.0.0.0/8"))).To(Succeed())
			Expect(repo.AddToWhiteList(ctx, netip.MustParsePrefix("10.0.0.0/8"))).To(Succeed())

			got, err := repo.GetAll(ctx, postgres.WhiteList)
			Expect(err).NotTo(HaveOccurred())
			Expect(got).To(HaveLen(1))
		})
	})

	Describe("DeleteFromWhiteList", func() {
		It("removes an existing prefix", func() {
			Expect(repo.AddToWhiteList(ctx, netip.MustParsePrefix("10.0.0.0/8"))).To(Succeed())
			Expect(repo.DeleteFromWhiteList(ctx, netip.MustParsePrefix("10.0.0.0/8"))).To(Succeed())

			got, err := repo.GetAll(ctx, postgres.WhiteList)
			Expect(err).NotTo(HaveOccurred())
			Expect(got).To(BeEmpty())
		})

		It("matches on the masked form when deleting", func() {
			Expect(repo.AddToWhiteList(ctx, netip.MustParsePrefix("192.168.1.0/24"))).To(Succeed())
			Expect(repo.DeleteFromWhiteList(ctx, netip.MustParsePrefix("192.168.1.99/24"))).To(Succeed())

			got, err := repo.GetAll(ctx, postgres.WhiteList)
			Expect(err).NotTo(HaveOccurred())
			Expect(got).To(BeEmpty())
		})

		It("is a no-op when the prefix is absent", func() {
			Expect(repo.DeleteFromWhiteList(ctx, netip.MustParsePrefix("10.0.0.0/8"))).To(Succeed())
		})
	})

	Describe("black and white lists are independent", func() {
		It("does not leak entries across lists", func() {
			Expect(repo.AddToWhiteList(ctx, netip.MustParsePrefix("10.0.0.0/8"))).To(Succeed())
			Expect(repo.AddToBlackList(ctx, netip.MustParsePrefix("172.16.0.0/16"))).To(Succeed())

			white, err := repo.GetAll(ctx, postgres.WhiteList)
			Expect(err).NotTo(HaveOccurred())
			Expect(white).To(ConsistOf(netip.MustParsePrefix("10.0.0.0/8")))

			black, err := repo.GetAll(ctx, postgres.BlackList)
			Expect(err).NotTo(HaveOccurred())
			Expect(black).To(ConsistOf(netip.MustParsePrefix("172.16.0.0/16")))
		})
	})
})
