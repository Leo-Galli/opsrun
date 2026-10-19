// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/Leo-Galli/opsrun/internal/approval"
	"github.com/Leo-Galli/opsrun/internal/executor"
	"github.com/Leo-Galli/opsrun/internal/parser"
)

func newRunCmd(flags *globalFlags) *cobra.Command {
	var checkpointDir string

	cmd := &cobra.Command{
		Use:   "run <runbook.md>",
		Short: "Execute a runbook",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return doRun(cmd, flags, args[0], "", checkpointDir)
		},
	}
	cmd.Flags().StringVar(&checkpointDir, "checkpoint-dir", defaultCheckpointDir(), "directory to write run checkpoints to, for 'opsrun resume'")
	return cmd
}

func newResumeCmd(flags *globalFlags) *cobra.Command {
	var checkpointDir string

	cmd := &cobra.Command{
		Use:   "resume <run-id>",
		Short: "Resume a previously interrupted or failed run from its checkpoint",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return doRun(cmd, flags, "", args[0], checkpointDir)
		},
	}
	cmd.Flags().StringVar(&checkpointDir, "checkpoint-dir", defaultCheckpointDir(), "directory checkpoints were written to by 'opsrun run'")
	return cmd
}

func defaultCheckpointDir() string {
	return ".opsrun/checkpoints"
}

// doRun implements both `run` (runbookPath set) and `resume` (runID set).
func doRun(cmd *cobra.Command, flags *globalFlags, runbookPath, runID, checkpointDir string) error {
	cfg, err := loadConfig(flags.config)
	if err != nil {
		return withExitCode(ExitGenericError, err)
	}

	isResume := runID != ""

	vars, err := resolveVars(flags.varsFile, flags.vars)
	if err != nil {
		return withExitCode(ExitGenericError, err)
	}

	out := cmd.OutOrStdout()
	outFile, _ := out.(*os.File)
	if outFile == nil {
		outFile = os.Stdout
	}

	var gate approval.Gate
	switch {
	case flags.nonInteractive:
		gate = NonInteractiveGate{}
	default:
		gate = &TTYGate{In: os.Stdin, Out: cmd.ErrOrStderr()}
	}

	textEmitter := NewTextEmitter(out)
	exec, auditSink, err := buildExecutor(cfg, flags.insecureSkipHostKeyVerification, outFile, textEmitter)
	if err != nil {
		return withExitCode(ExitGenericError, err)
	}
	if auditSink != nil {
		defer auditSink.Close()
	}
	exec.Approval = gate

	// signal.Notify (rather than signal.NotifyContext) so the "interrupt received"
	// message prints only for a real signal, never when ctx is canceled by the
	// deferred cancel() below at the end of a normal run.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(sigCh)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		select {
		case <-sigCh:
			fmt.Fprintln(cmd.ErrOrStderr(), "\n[WAIT] interrupt received: stopping new steps and rolling back completed ones...")
			cancel()
		case <-ctx.Done():
		}
	}()

	opts := executor.Options{
		RunbookPath:   runbookPath,
		DryRun:        flags.dryRun,
		Interactive:   flags.interactive,
		Concurrency:   flags.concurrency,
		Vars:          vars,
		CheckpointDir: checkpointDir,
	}

	var summary *executor.Summary
	var runErr error
	if isResume {
		summary, runErr = exec.Resume(ctx, runID, opts)
	} else {
		runbook, perr := parseRunbookFile(runbookPath)
		if perr != nil {
			return withExitCode(ExitGenericError, perr)
		}
		validator, verr := parser.NewValidator(nil)
		if verr != nil {
			return withExitCode(ExitGenericError, verr)
		}
		if findings := validator.Validate(runbook); hasErrors(findings) {
			printFindings(cmd.ErrOrStderr(), findings, flags.output)
			return withExitCode(ExitValidationFailed, fmt.Errorf("runbook %q failed validation", runbookPath))
		}
		summary, runErr = exec.Run(ctx, runbook, opts)
	}

	if summary != nil {
		printSummary(out, summary, flags.output)
	}

	switch {
	case summary == nil:
		return withExitCode(ExitGenericError, runErr)
	case summary.Failed && ctx.Err() != nil:
		return withExitCode(ExitInterrupted, runErr)
	case summary.Failed:
		return withExitCode(ExitRunFailed, runErr)
	default:
		return nil
	}
}
