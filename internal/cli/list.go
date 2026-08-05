package cli

import (
	"context"
	"fmt"
	"net/netip"

	"github.com/spf13/cobra"
)

type subnetFunc func(context.Context, netip.Prefix) error

func newWhitelistCmd(opts *options) *cobra.Command {
	return newListCmd(opts, "whitelist",
		"Subnets that are always allowed",
		func(c Client) subnetFunc { return c.AddWhitelist },
		func(c Client) subnetFunc { return c.RemoveWhitelist },
	)
}

func newBlacklistCmd(opts *options) *cobra.Command {
	return newListCmd(opts, "blacklist",
		"Subnets that are always denied",
		func(c Client) subnetFunc { return c.AddBlacklist },
		func(c Client) subnetFunc { return c.RemoveBlacklist },
	)
}

func newListCmd(opts *options, list, short string, add, remove func(Client) subnetFunc) *cobra.Command {
	cmd := &cobra.Command{
		Use:   list,
		Short: short,
	}
	cmd.AddCommand(
		newSubnetCmd(opts, "add", list, "added", add),
		newSubnetCmd(opts, "remove", list, "removed", remove),
	)
	return cmd
}

func newSubnetCmd(opts *options, verb, list, done string, pick func(Client) subnetFunc) *cobra.Command {
	return &cobra.Command{
		Use:     verb + " <subnet>",
		Short:   fmt.Sprintf("%s a subnet %s the %s", verb, preposition(verb), list),
		Example: fmt.Sprintf("  abf-cli %s %s 192.1.1.0/25", list, verb),
		Args:    cobra.ExactArgs(1),
		RunE: runE(func(cmd *cobra.Command, args []string) error {
			prefix, err := netip.ParsePrefix(args[0])
			if err != nil {
				return fmt.Errorf("invalid subnet %q: %w", args[0], err)
			}

			client, err := opts.client()
			if err != nil {
				return err
			}
			if err := pick(client)(cmd.Context(), prefix); err != nil {
				return err
			}

			fmt.Fprintf(cmd.OutOrStdout(), "%s: %s %s\n", list, done, prefix)
			return nil
		}),
	}
}

func preposition(verb string) string {
	if verb == "add" {
		return "to"
	}
	return "from"
}
