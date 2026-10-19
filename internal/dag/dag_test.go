// SPDX-License-Identifier: Apache-2.0

package dag

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Leo-Galli/opsrun/internal/runbook"
)

func step(id string, deps ...string) *runbook.Step {
	return &runbook.Step{ID: id, DependsOn: deps}
}

func TestBuildSimple(t *testing.T) {
	steps := []*runbook.Step{step("a"), step("b", "a"), step("c", "a")}
	g, err := Build(steps)
	require.NoError(t, err)
	require.NotNil(t, g.Step("a"))
	require.Nil(t, g.Step("missing"))
}

func TestBuildUnknownDependency(t *testing.T) {
	steps := []*runbook.Step{step("a", "nope")}
	_, err := Build(steps)
	require.Error(t, err)
	var unknown *UnknownDependencyError
	require.ErrorAs(t, err, &unknown)
	require.Equal(t, "a", unknown.StepID)
	require.Equal(t, "nope", unknown.DependencyID)
}

func TestBuildCycle(t *testing.T) {
	steps := []*runbook.Step{step("a", "c"), step("b", "a"), step("c", "b")}
	_, err := Build(steps)
	require.Error(t, err)
	var cycle *CycleError
	require.ErrorAs(t, err, &cycle)
	require.NotEmpty(t, cycle.Cycle)
}

func TestBuildSelfCycle(t *testing.T) {
	steps := []*runbook.Step{step("a", "a")}
	_, err := Build(steps)
	require.Error(t, err)
	var cycle *CycleError
	require.ErrorAs(t, err, &cycle)
}

func TestLevels(t *testing.T) {
	// a -> b -> d
	// a -> c -> d
	steps := []*runbook.Step{
		step("a"),
		step("b", "a"),
		step("c", "a"),
		step("d", "b", "c"),
	}
	g, err := Build(steps)
	require.NoError(t, err)
	levels := g.Levels()
	require.Len(t, levels, 3)
	require.Len(t, levels[0], 1)
	require.Equal(t, "a", levels[0][0].ID)
	require.Len(t, levels[1], 2)
	ids := []string{levels[1][0].ID, levels[1][1].ID}
	require.ElementsMatch(t, []string{"b", "c"}, ids)
	require.Len(t, levels[2], 1)
	require.Equal(t, "d", levels[2][0].ID)
}

func TestLevelsIndependentSteps(t *testing.T) {
	steps := []*runbook.Step{step("a"), step("b"), step("c")}
	g, err := Build(steps)
	require.NoError(t, err)
	levels := g.Levels()
	require.Len(t, levels, 1)
	require.Len(t, levels[0], 3)
}

func TestDependents(t *testing.T) {
	steps := []*runbook.Step{step("a"), step("b", "a"), step("c", "a")}
	g, err := Build(steps)
	require.NoError(t, err)
	require.Equal(t, []string{"b", "c"}, g.Dependents("a"))
	require.Empty(t, g.Dependents("b"))
}
