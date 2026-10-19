// SPDX-License-Identifier: Apache-2.0

package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/Leo-Galli/opsrun/internal/audit"
)

func newAuditCmd() *cobra.Command {
	var logPath string
	var outputFormat string

	cmd := &cobra.Command{
		Use:   "audit",
		Short: "Inspect, verify, and export the audit log",
	}
	cmd.PersistentFlags().StringVar(&logPath, "log", "./audit/opsrun-audit.jsonl", "path to the audit log file")
	cmd.PersistentFlags().StringVar(&outputFormat, "output", "text", "output format: text|json")

	cmd.AddCommand(newAuditListCmd(&logPath, &outputFormat))
	cmd.AddCommand(newAuditShowCmd(&logPath, &outputFormat))
	cmd.AddCommand(newAuditVerifyCmd(&logPath))
	cmd.AddCommand(newAuditExportCmd(&logPath))
	cmd.AddCommand(newAuditKeygenCmd())
	return cmd
}

func newAuditListCmd(logPath, outputFormat *string) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List every run id present in the audit log",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			records, err := audit.ReadAll(*logPath)
			if err != nil {
				return withExitCode(ExitGenericError, err)
			}
			seen := map[string]bool{}
			var runIDs []string
			for _, r := range records {
				if !seen[r.RunID] {
					seen[r.RunID] = true
					runIDs = append(runIDs, r.RunID)
				}
			}
			if *outputFormat == "json" {
				data, _ := json.MarshalIndent(runIDs, "", "  ")
				fmt.Fprintln(cmd.OutOrStdout(), string(data))
				return nil
			}
			for _, id := range runIDs {
				fmt.Fprintln(cmd.OutOrStdout(), id)
			}
			return nil
		},
	}
}

func newAuditShowCmd(logPath, outputFormat *string) *cobra.Command {
	return &cobra.Command{
		Use:   "show <run-id>",
		Short: "Show every audit record for a single run",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			records, err := audit.ReadAll(*logPath)
			if err != nil {
				return withExitCode(ExitGenericError, err)
			}
			var matched []audit.Record
			for _, r := range records {
				if r.RunID == args[0] {
					matched = append(matched, r)
				}
			}
			if *outputFormat == "json" {
				data, _ := json.MarshalIndent(matched, "", "  ")
				fmt.Fprintln(cmd.OutOrStdout(), string(data))
				return nil
			}
			for _, r := range matched {
				exit := ""
				if r.ExitCode != nil {
					exit = fmt.Sprintf(" exit=%d", *r.ExitCode)
				}
				fmt.Fprintf(cmd.OutOrStdout(), "%s %-18s step=%-15s host=%-15s%s %s\n", r.Timestamp, r.Type, r.StepID, r.Host, exit, r.Command)
			}
			return nil
		},
	}
}

func newAuditVerifyCmd(logPath *string) *cobra.Command {
	var publicKeyPath string
	cmd := &cobra.Command{
		Use:   "verify",
		Short: "Verify the audit log's hash chain (and signatures, if a public key is given) for tampering",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			var pub ed25519.PublicKey
			if publicKeyPath != "" {
				var err error
				pub, err = loadAuditPublicKey(publicKeyPath)
				if err != nil {
					return withExitCode(ExitGenericError, err)
				}
			}
			result, err := audit.VerifyFile(*logPath, pub)
			if err != nil {
				return withExitCode(ExitGenericError, err)
			}
			if result.Valid {
				fmt.Fprintf(cmd.OutOrStdout(), "[OK] %d records verified, hash chain intact\n", result.RecordCount)
				return nil
			}
			fmt.Fprintf(cmd.OutOrStdout(), "[FAIL] %d records checked, %d problem(s) found:\n", result.RecordCount, len(result.Violations))
			for _, v := range result.Violations {
				fmt.Fprintf(cmd.OutOrStdout(), "  - %s\n", v)
			}
			return withExitCode(ExitGenericError, fmt.Errorf("audit log %q failed verification", *logPath))
		},
	}
	cmd.Flags().StringVar(&publicKeyPath, "public-key", "", "path to a raw 32-byte Ed25519 public key to also verify signatures")
	return cmd
}

func newAuditExportCmd(logPath *string) *cobra.Command {
	var runID string
	cmd := &cobra.Command{
		Use:   "export",
		Short: "Export the audit log as a Markdown post-mortem timeline",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := audit.ExportMarkdown(*logPath, runID, cmd.OutOrStdout()); err != nil {
				return withExitCode(ExitGenericError, err)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&runID, "run-id", "", "restrict the export to a single run id")
	return cmd
}

func newAuditKeygenCmd() *cobra.Command {
	var outPriv, outPub string
	cmd := &cobra.Command{
		Use:   "keygen",
		Short: "Generate an Ed25519 key pair for signing the audit log",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			pub, priv, err := ed25519.GenerateKey(rand.Reader)
			if err != nil {
				return withExitCode(ExitGenericError, err)
			}
			if err := os.WriteFile(outPriv, priv, 0o600); err != nil {
				return withExitCode(ExitGenericError, fmt.Errorf("write private key: %w", err))
			}
			if err := os.WriteFile(outPub, pub, 0o644); err != nil {
				return withExitCode(ExitGenericError, fmt.Errorf("write public key: %w", err))
			}
			fmt.Fprintf(cmd.OutOrStdout(), "[OK] wrote %s (private, keep secret) and %s (public)\n", outPriv, outPub)
			return nil
		},
	}
	cmd.Flags().StringVar(&outPriv, "out", "audit.key", "output path for the raw private key")
	cmd.Flags().StringVar(&outPub, "out-pub", "audit.pub", "output path for the raw public key")
	return cmd
}
