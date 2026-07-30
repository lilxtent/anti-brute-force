package cli

import (
	"context"
	"errors"
	"net/netip"
	"os"
	"time"

	"github.com/spf13/cobra"
)

const (
	addrEnv        = "ABF_ADDR"
	defaultAddr    = "http://localhost:8080"
	defaultTimeout = 5 * time.Second
)

var ErrDenied = errors.New("authorization attempt denied")

type Client interface {
	Check(ctx context.Context, login, password, ip string) (bool, error)
	Reset(ctx context.Context, login, ip string) error
	AddWhitelist(ctx context.Context, p netip.Prefix) error
	RemoveWhitelist(ctx context.Context, p netip.Prefix) error
	AddBlacklist(ctx context.Context, p netip.Prefix) error
	RemoveBlacklist(ctx context.Context, p netip.Prefix) error
	Health(ctx context.Context) error
}

type ClientFactory func(addr string, timeout time.Duration) (Client, error)

type options struct {
	addr      string
	timeout   time.Duration
	newClient ClientFactory
}

func (o *options) client() (Client, error) {
	return o.newClient(o.addr, o.timeout)
}

func NewRootCmd(newClient ClientFactory) *cobra.Command {
	opts := &options{newClient: newClient}

	root := &cobra.Command{
		Use:           "abf-cli",
		Short:         "Administer the anti-bruteforce service",
		SilenceErrors: true,
	}

	addr := os.Getenv(addrEnv)
	if addr == "" {
		addr = defaultAddr
	}
	root.PersistentFlags().StringVar(&opts.addr, "addr", addr,
		"service address (overrides the "+addrEnv+" environment variable)")
	root.PersistentFlags().DurationVar(&opts.timeout, "timeout", defaultTimeout,
		"per-request timeout")

	root.AddCommand(
		newWhitelistCmd(opts),
		newBlacklistCmd(opts),
		newResetCmd(opts),
		newCheckCmd(opts),
		newHealthCmd(opts),
	)

	return root
}

func runE(fn func(cmd *cobra.Command, args []string) error) func(*cobra.Command, []string) error {
	return func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true
		return fn(cmd, args)
	}
}
