package cli

import (
	"fmt"
	"net/netip"

	"github.com/spf13/cobra"
)

func newCheckCmd(opts *options) *cobra.Command {
	var login, password, ip string

	cmd := &cobra.Command{
		Use:     "check",
		Short:   "Ask the service whether an authorization attempt is allowed",
		Example: "  abf-cli check --login bob --password s3cret --ip 1.2.3.4",
		Args:    cobra.NoArgs,
		RunE: runE(func(cmd *cobra.Command, _ []string) error {
			if _, err := netip.ParseAddr(ip); err != nil {
				return fmt.Errorf("invalid ip %q: %w", ip, err)
			}

			client, err := opts.client()
			if err != nil {
				return err
			}
			ok, err := client.Check(cmd.Context(), login, password, ip)
			if err != nil {
				return err
			}

			if !ok {
				fmt.Fprintln(cmd.OutOrStdout(), "denied")
				return ErrDenied
			}
			fmt.Fprintln(cmd.OutOrStdout(), "allowed")
			return nil
		}),
	}

	cmd.Flags().StringVar(&login, "login", "", "login to check")
	cmd.Flags().StringVar(&password, "password", "", "password to check")
	cmd.Flags().StringVar(&ip, "ip", "", "IP address to check")
	mustMarkRequired(cmd, "login", "password", "ip")

	return cmd
}
