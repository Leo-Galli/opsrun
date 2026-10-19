// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	"github.com/Leo-Galli/opsrun/internal/dag"
	"github.com/Leo-Galli/opsrun/internal/inventory"
	"github.com/Leo-Galli/opsrun/internal/parser"
	"github.com/Leo-Galli/opsrun/internal/runbook"
)

func newPlanCmd(flags *globalFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "plan <runbook.md>",
		Short: "Show the step DAG and interpolated commands without executing anything",
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
			if findings := validator.Validate(rb); hasErrors(findings) {
				printFindings(cmd.ErrOrStderr(), findings, flags.output)
				return withExitCode(ExitValidationFailed, fmt.Errorf("runbook %q failed validation", args[0]))
			}

			cfg, err := loadConfig(flags.config)
			if err != nil {
				return withExitCode(ExitGenericError, err)
			}
			vars, err := resolveVars(flags.varsFile, flags.vars)
			if err != nil {
				return withExitCode(ExitGenericError, err)
			}

			g, err := dag.Build(rb.Steps)
			if err != nil {
				return withExitCode(ExitGenericError, err)
			}
			inv := inventory.NewStatic(cfg.InventoryHosts())

			plan := buildPlan(context.Background(), rb, g, inv, vars)
			if flags.output == "json" {
				data, _ := json.MarshalIndent(plan, "", "  ")
				fmt.Fprintln(cmd.OutOrStdout(), string(data))
				return nil
			}
			printPlanText(cmd.OutOrStdout(), plan)
			return nil
		},
	}
}

type planStep struct {
	ID        string   `json:"id"`
	Title     string   `json:"title"`
	Hosts     []string `json:"hosts"`
	Command   string   `json:"command,omitempty"`
	Checks    []string `json:"checks,omitempty"`
	Rollback  string   `json:"rollback,omitempty"`
	DependsOn []string `json:"depends_on,omitempty"`
	Confirm   bool     `json:"confirm"`
}

type planLevel struct {
	Level int        `json:"level"`
	Steps []planStep `json:"steps"`
}

func buildPlan(ctx context.Context, rb *runbook.Runbook, g *dag.DAG, inv inventory.Inventory, vars map[string]any) []planLevel {
	var levels []planLevel
	for i, level := range g.Levels() {
		pl := planLevel{Level: i + 1}
		for _, step := range level {
			ps := planStep{ID: step.ID, Title: step.Title, Confirm: step.Confirm, DependsOn: step.DependsOn}

			spec := ""
			if step.Run != nil && step.Run.Target != "" {
				spec = step.Run.Target
			} else if len(rb.Targets) > 0 {
				spec = targetString(rb.Targets[0])
			}
			if spec != "" {
				if hosts, err := inv.Resolve(ctx, spec); err == nil {
					for _, h := range hosts {
						ps.Hosts = append(ps.Hosts, h.Name)
					}
				}
			}

			if step.Run != nil {
				ps.Command, _ = parser.InterpolateRedacted(step.Run.Command, rb.Vars, vars)
			}
			for _, c := range step.Checks {
				cmd, _ := parser.InterpolateRedacted(c.Command, rb.Vars, vars)
				ps.Checks = append(ps.Checks, cmd)
			}
			if step.Rollback != nil {
				ps.Rollback, _ = parser.InterpolateRedacted(step.Rollback.Command, rb.Vars, vars)
			}
			pl.Steps = append(pl.Steps, ps)
		}
		levels = append(levels, pl)
	}
	return levels
}

func targetString(t runbook.Target) string {
	if t.Group != "" {
		return t.Group
	}
	return t.Host
}

func printPlanText(w io.Writer, levels []planLevel) {
	for _, level := range levels {
		fmt.Fprintf(w, "Level %d:\n", level.Level)
		for _, s := range level.Steps {
			confirm := ""
			if s.Confirm {
				confirm = " [requires approval]"
			}
			fmt.Fprintf(w, "  Step %s - %s%s\n", s.ID, s.Title, confirm)
			if len(s.DependsOn) > 0 {
				fmt.Fprintf(w, "    depends_on: %s\n", strings.Join(s.DependsOn, ", "))
			}
			fmt.Fprintf(w, "    hosts: %s\n", strings.Join(s.Hosts, ", "))
			if s.Command != "" {
				fmt.Fprintf(w, "    run: %s\n", s.Command)
			}
			for _, c := range s.Checks {
				fmt.Fprintf(w, "    check: %s\n", c)
			}
			if s.Rollback != "" {
				fmt.Fprintf(w, "    rollback: %s\n", s.Rollback)
			}
		}
	}
}
