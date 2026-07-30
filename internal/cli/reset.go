package cli

import (
	"fmt"
	"net/netip"

	"github.com/spf13/cobra"
)

func newResetCmd(opts *options) *cobra.Command {
	var login, ip string

	cmd := &cobra.Command{
		Use:     "reset",
		Short:   "Clear the rate-limit buckets for a login and an IP",
		Example: "  abf-cli reset --login bob --ip 1.2.3.4",
		Args:    cobra.NoArgs,
		RunE: runE(func(cmd *cobra.Command, _ []string) error {
			if _, err := netip.ParseAddr(ip); err != nil {
				return fmt.Errorf("invalid ip %q: %w", ip, err)
			}

			client, err := opts.client()
			if err != nil {
				return err
			}
			if err := client.Reset(cmd.Context(), login, ip); err != nil {
				return err
			}

			fmt.Fprintf(cmd.OutOrStdout(), "reset: cleared buckets for login=%s ip=%s\n", login, ip)
			return nil
		}),
	}

	cmd.Flags().StringVar(&login, "login", "", "login whose bucket is cleared")
	cmd.Flags().StringVar(&ip, "ip", "", "IP address whose bucket is cleared")
	mustMarkRequired(cmd, "login", "ip")

	return cmd
}

func mustMarkRequired(cmd *cobra.Command, names ...string) {
	for _, name := range names {
		if err := cmd.MarkFlagRequired(name); err != nil {
			panic(fmt.Sprintf("cli: mark %q required: %v", name, err))
		}
	}
}
