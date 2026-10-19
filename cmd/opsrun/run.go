// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"

	"github.com/Leo-Galli/opsrun/internal/approval"
	"github.com/Leo-Galli/opsrun/internal/executor"
	"github.com/Leo-Galli/opsrun/internal/parser"
	"github.com/Leo-Galli/opsrun/internal/runbook"
	"github.com/Leo-Galli/opsrun/internal/tui"
)

func newRunCmd(flags *globalFlags) *cobra.Command {
	var checkpointDir string
	rollout := &rolloutFlags{}

	cmd := &cobra.Command{
		Use:   "run <runbook.md>",
		Short: "Execute a runbook",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			strategy, err := buildRolloutStrategy(rollout)
			if err != nil {
				return withExitCode(ExitGenericError, err)
			}
			return doRun(cmd, flags, args[0], "", checkpointDir, strategy)
		},
	}
	cmd.Flags().StringVar(&checkpointDir, "checkpoint-dir", defaultCheckpointDir(), "directory to write run checkpoints to, for 'opsrun resume'")
	addRolloutFlags(cmd, rollout)
	return cmd
}

func newResumeCmd(flags *globalFlags) *cobra.Command {
	var checkpointDir string
	rollout := &rolloutFlags{}

	cmd := &cobra.Command{
		Use:   "resume <run-id>",
		Short: "Resume a previously interrupted or failed run from its checkpoint",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			strategy, err := buildRolloutStrategy(rollout)
			if err != nil {
				return withExitCode(ExitGenericError, err)
			}
			return doRun(cmd, flags, "", args[0], checkpointDir, strategy)
		},
	}
	cmd.Flags().StringVar(&checkpointDir, "checkpoint-dir", defaultCheckpointDir(), "directory checkpoints were written to by 'opsrun run'")
	addRolloutFlags(cmd, rollout)
	return cmd
}

func defaultCheckpointDir() string {
	return ".opsrun/checkpoints"
}

// doRun implements both `run` (runbookPath set) and `resume` (runID set). It uses the
// Bubble Tea TUI when stdout is a real terminal and --output is not json, falling back
// automatically to plain, line-oriented text otherwise (CI, pipes, --output json).
func doRun(cmd *cobra.Command, flags *globalFlags, runbookPath, runID, checkpointDir string, strategy executor.RolloutStrategy) error {
	cfg, err := loadConfig(flags.config)
	if err != nil {
		return withExitCode(ExitGenericError, err)
	}
	isResume := runID != ""

	vars, err := resolveVars(flags.varsFile, flags.vars)
	if err != nil {
		return withExitCode(ExitGenericError, err)
	}

	// Validate up front for a fresh run, so a bad runbook never gets as far as drawing
	// the TUI. Resume re-validates implicitly: Executor.Resume re-parses the original
	// runbook from the checkpoint before continuing.
	var rb *runbook.Runbook
	if !isResume {
		rb, err = parseRunbookFile(runbookPath)
		if err != nil {
			return withExitCode(ExitGenericError, err)
		}
		validator, verr := parser.NewValidator(nil)
		if verr != nil {
			return withExitCode(ExitGenericError, verr)
		}
		if findings := validator.Validate(rb); hasErrors(findings) {
			printFindings(cmd.ErrOrStderr(), findings, flags.output)
			return withExitCode(ExitValidationFailed, fmt.Errorf("runbook %q failed validation", runbookPath))
		}
	}

	out := cmd.OutOrStdout()
	// Only a real *os.File can be a terminal; a buffer (as in tests, or anywhere
	// cobra's output is redirected) never is, and must not be mistaken for stdout.
	outFile, isRealFile := out.(*os.File)
	if !isRealFile {
		outFile = os.Stderr
	}
	useTUI := flags.output != "json" && isRealFile && isTerminal(outFile)

	var emitter executor.Emitter
	var model *tui.Model
	var program *tea.Program
	if useTUI {
		model = tui.NewModel()
		program = tea.NewProgram(model)
		emitter = model.Emitter()
	} else {
		emitter = NewTextEmitter(out)
	}

	var gate approval.Gate
	switch {
	case flags.nonInteractive:
		gate = NonInteractiveGate{}
	case useTUI:
		gate = model.Gate()
	default:
		gate = &TTYGate{In: os.Stdin, Out: cmd.ErrOrStderr()}
	}

	exec, auditSink, err := buildExecutor(cfg, flags.insecureSkipHostKeyVerification, outFile, emitter)
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
			if !useTUI {
				fmt.Fprintln(cmd.ErrOrStderr(), "\n[WAIT] interrupt received: stopping new steps and rolling back completed ones...")
			}
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
		HostRollout:   strategy,
	}

	var summary *executor.Summary
	var runErr error
	runFunc := func() {
		if isResume {
			summary, runErr = exec.Resume(ctx, runID, opts)
		} else {
			summary, runErr = exec.Run(ctx, rb, opts)
		}
	}

	if useTUI {
		go func() {
			runFunc()
			program.Quit() // in case the model did not already quit on EventRunFinished
		}()
		if _, perr := program.Run(); perr != nil {
			return withExitCode(ExitGenericError, fmt.Errorf("TUI error: %w", perr))
		}
	} else {
		runFunc()
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
