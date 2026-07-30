package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newHealthCmd(opts *options) *cobra.Command {
	return &cobra.Command{
		Use:     "health",
		Short:   "Check that the service is reachable",
		Example: "  abf-cli health",
		Args:    cobra.NoArgs,
		RunE: runE(func(cmd *cobra.Command, _ []string) error {
			client, err := opts.client()
			if err != nil {
				return err
			}
			if err := client.Health(cmd.Context()); err != nil {
				return err
			}

			fmt.Fprintln(cmd.OutOrStdout(), "ok")
			return nil
		}),
	}
}
