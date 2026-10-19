// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/Leo-Galli/opsrun/internal/inventory"
)

func newHostsCmd(flags *globalFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "hosts",
		Short: "List the hosts known to the configured inventory",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := loadConfig(flags.config)
			if err != nil {
				return withExitCode(ExitGenericError, err)
			}
			inv := inventory.NewStatic(cfg.InventoryHosts())
			allHosts, err := inv.Hosts(context.Background())
			if err != nil {
				return withExitCode(ExitGenericError, err)
			}

			if flags.output == "json" {
				data, _ := json.MarshalIndent(allHosts, "", "  ")
				fmt.Fprintln(cmd.OutOrStdout(), string(data))
				return nil
			}

			if len(allHosts) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No hosts configured.")
				return nil
			}
			for _, h := range allHosts {
				groups := strings.Join(h.Groups, ",")
				fmt.Fprintf(cmd.OutOrStdout(), "%-20s %-20s user=%-10s groups=%s\n", h.Name, h.Address, h.User, groups)
			}
			return nil
		},
	}
}
