// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/Leo-Galli/opsrun/internal/executor"
)

// rolloutFlags holds the --strategy family of flags, shared by run and resume.
type rolloutFlags struct {
	strategy          string
	batchSize         int
	maxFailurePercent float64
	canaryCount       int
}

func addRolloutFlags(cmd *cobra.Command, f *rolloutFlags) {
	cmd.Flags().StringVar(&f.strategy, "strategy", "all-at-once", "host rollout strategy for every step: all-at-once|rolling|canary")
	cmd.Flags().IntVar(&f.batchSize, "batch-size", 1, "hosts per batch for --strategy rolling")
	cmd.Flags().Float64Var(&f.maxFailurePercent, "max-failure-percent", 0, "tolerated host failure percentage (0-100) for --strategy rolling before aborting the step")
	cmd.Flags().IntVar(&f.canaryCount, "canary-count", 1, "number of canary hosts for --strategy canary")
}

// buildRolloutStrategy translates rolloutFlags into an executor.RolloutStrategy.
func buildRolloutStrategy(f *rolloutFlags) (executor.RolloutStrategy, error) {
	switch f.strategy {
	case "", "all-at-once":
		return executor.AllAtOnceStrategy{}, nil
	case "rolling":
		return executor.RollingStrategy{BatchSize: f.batchSize, MaxFailurePercent: f.maxFailurePercent}, nil
	case "canary":
		return executor.CanaryStrategy{CanaryCount: f.canaryCount}, nil
	default:
		return nil, fmt.Errorf("unknown --strategy %q: expected all-at-once, rolling, or canary", f.strategy)
	}
}
