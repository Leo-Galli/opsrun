// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/Leo-Galli/opsrun/internal/parser"
)

func newValidateCmd(flags *globalFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "validate <runbook.md>",
		Short: "Parse and validate a runbook without executing it",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			rb, err := parseRunbookFile(args[0])
			if err != nil {
				return withExitCode(ExitGenericError, err)
			}

			validator, err := parser.NewValidator(nil)
			if err != nil {
				return withExitCode(ExitGenericError, err)
			}
			findings := validator.Validate(rb)
			printFindings(cmd.OutOrStdout(), findings, flags.output)

			if hasErrors(findings) {
				return withExitCode(ExitValidationFailed, fmt.Errorf("runbook %q failed validation", args[0]))
			}
			if flags.output != "json" {
				fmt.Fprintln(cmd.OutOrStdout(), "[OK] runbook is valid")
			}
			return nil
		},
	}
}
