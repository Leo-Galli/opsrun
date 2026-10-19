// SPDX-License-Identifier: Apache-2.0

package executor

import "github.com/Leo-Galli/opsrun/internal/inventory"

// RolloutStrategy decides how a step's target hosts are grouped into sequential
// batches, and how many host-level failures a step tolerates before the strategy gives
// up on remaining batches.
type RolloutStrategy interface {
	// Batches splits hosts into ordered groups. Hosts within a batch run concurrently;
	// batches run one after another.
	Batches(hosts []inventory.Host) [][]inventory.Host

	// MaxFailures returns how many host failures, out of total, this strategy
	// tolerates before the step is considered failed and later batches are skipped.
	MaxFailures(total int) int
}

// AllAtOnceStrategy runs a step against every target host concurrently, in a single
// batch, and tolerates no failures: any failed host fails the step.
type AllAtOnceStrategy struct{}

// Batches implements RolloutStrategy.
func (AllAtOnceStrategy) Batches(hosts []inventory.Host) [][]inventory.Host {
	if len(hosts) == 0 {
		return nil
	}
	return [][]inventory.Host{hosts}
}

// MaxFailures implements RolloutStrategy.
func (AllAtOnceStrategy) MaxFailures(int) int { return 0 }

// RollingStrategy runs a step in fixed-size batches, proceeding to the next batch only
// if the failure rate so far is within MaxFailurePercent.
type RollingStrategy struct {
	BatchSize         int
	MaxFailurePercent float64 // 0-100
}

// Batches implements RolloutStrategy.
func (r RollingStrategy) Batches(hosts []inventory.Host) [][]inventory.Host {
	size := r.BatchSize
	if size <= 0 {
		size = 1
	}
	var batches [][]inventory.Host
	for i := 0; i < len(hosts); i += size {
		end := i + size
		if end > len(hosts) {
			end = len(hosts)
		}
		batches = append(batches, hosts[i:end])
	}
	return batches
}

// MaxFailures implements RolloutStrategy.
func (r RollingStrategy) MaxFailures(total int) int {
	return int(float64(total) * r.MaxFailurePercent / 100.0)
}

// CanaryStrategy runs a step against a small canary batch first, then the remaining
// hosts as a second batch, tolerating no failures at either stage.
type CanaryStrategy struct {
	CanaryCount int
}

// Batches implements RolloutStrategy.
func (c CanaryStrategy) Batches(hosts []inventory.Host) [][]inventory.Host {
	n := c.CanaryCount
	if n <= 0 {
		n = 1
	}
	if n >= len(hosts) {
		if len(hosts) == 0 {
			return nil
		}
		return [][]inventory.Host{hosts}
	}
	return [][]inventory.Host{hosts[:n], hosts[n:]}
}

// MaxFailures implements RolloutStrategy.
func (CanaryStrategy) MaxFailures(int) int { return 0 }
