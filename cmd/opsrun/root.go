// SPDX-License-Identifier: Apache-2.0

package main

import (
	"github.com/spf13/cobra"
)

// globalFlags holds every persistent (root-level) flag, set once by newRootCmd and read
// by every subcommand.
type globalFlags struct {
	config                          string
	vars                            []string
	varsFile                        string
	dryRun                          bool
	interactive                     bool
	nonInteractive                  bool
	concurrency                     int
	output                          string
	insecureSkipHostKeyVerification bool
}

func newRootCmd() *cobra.Command {
	flags := &globalFlags{}

	root := &cobra.Command{
		Use:           "opsrun",
		Short:         "Turn Markdown runbooks into safe, concurrent incident response workflows over SSH",
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	root.PersistentFlags().StringVar(&flags.config, "config", "", "path to opsrun.yaml (defaults to built-in safe defaults)")
	root.PersistentFlags().StringArrayVar(&flags.vars, "var", nil, "set a runbook variable: key=value (repeatable)")
	root.PersistentFlags().StringVar(&flags.varsFile, "vars-file", "", "path to a YAML file of variable values")
	root.PersistentFlags().BoolVar(&flags.dryRun, "dry-run", false, "show what would run without executing or connecting to any host")
	root.PersistentFlags().BoolVar(&flags.interactive, "interactive", false, "require approval before every step, regardless of the runbook's policy")
	root.PersistentFlags().BoolVar(&flags.nonInteractive, "non-interactive", false, "never prompt for approval; abort instead if one would be required")
	root.PersistentFlags().IntVar(&flags.concurrency, "concurrency", 0, "max number of independent steps to run at once within a DAG level (0 = unlimited)")
	root.PersistentFlags().StringVar(&flags.output, "output", "text", "output format: text|json")
	root.PersistentFlags().BoolVar(&flags.insecureSkipHostKeyVerification, "insecure-skip-host-key-verification", false, "DANGEROUS: accept any SSH host key without verification")

	root.AddCommand(
		newRunCmd(flags),
		newValidateCmd(flags),
		newPlanCmd(flags),
		newResumeCmd(flags),
		newAuditCmd(),
		newHostsCmd(flags),
		newVersionCmd(),
	)
	return root
}
