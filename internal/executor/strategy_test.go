// SPDX-License-Identifier: Apache-2.0

package executor

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Leo-Galli/opsrun/internal/inventory"
)

func hostsN(n int) []inventory.Host {
	hosts := make([]inventory.Host, n)
	for i := range hosts {
		hosts[i] = inventory.Host{Name: string(rune('a' + i))}
	}
	return hosts
}

func TestAllAtOnceStrategy(t *testing.T) {
	s := AllAtOnceStrategy{}
	batches := s.Batches(hostsN(5))
	require.Len(t, batches, 1)
	require.Len(t, batches[0], 5)
	require.Equal(t, 0, s.MaxFailures(5))

	require.Nil(t, s.Batches(nil))
}

func TestRollingStrategy(t *testing.T) {
	s := RollingStrategy{BatchSize: 2, MaxFailurePercent: 25}
	batches := s.Batches(hostsN(5))
	require.Len(t, batches, 3)
	require.Len(t, batches[0], 2)
	require.Len(t, batches[1], 2)
	require.Len(t, batches[2], 1)
	require.Equal(t, 1, s.MaxFailures(5))
}

func TestRollingStrategyDefaultsBatchSize(t *testing.T) {
	s := RollingStrategy{}
	batches := s.Batches(hostsN(3))
	require.Len(t, batches, 3)
	for _, b := range batches {
		require.Len(t, b, 1)
	}
}

func TestCanaryStrategy(t *testing.T) {
	s := CanaryStrategy{CanaryCount: 1}
	batches := s.Batches(hostsN(5))
	require.Len(t, batches, 2)
	require.Len(t, batches[0], 1)
	require.Len(t, batches[1], 4)
	require.Equal(t, 0, s.MaxFailures(5))
}

func TestCanaryStrategyMoreCanariesThanHosts(t *testing.T) {
	s := CanaryStrategy{CanaryCount: 10}
	batches := s.Batches(hostsN(3))
	require.Len(t, batches, 1)
	require.Len(t, batches[0], 3)

	require.Nil(t, s.Batches(nil))
}
