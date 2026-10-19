// SPDX-License-Identifier: Apache-2.0

// Package dag builds a directed acyclic graph from runbook steps and produces an
// execution order that respects dependencies while maximizing parallelism.
package dag

import (
	"fmt"
	"strings"

	"github.com/Leo-Galli/opsrun/internal/runbook"
)

// CycleError reports a dependency cycle detected while building a DAG.
type CycleError struct {
	Cycle []string
}

// Error implements the error interface.
func (e *CycleError) Error() string {
	return fmt.Sprintf("dependency cycle detected: %s", strings.Join(e.Cycle, " -> "))
}

// UnknownDependencyError reports a step that depends on an id that does not exist.
type UnknownDependencyError struct {
	StepID       string
	DependencyID string
}

// Error implements the error interface.
func (e *UnknownDependencyError) Error() string {
	return fmt.Sprintf("step %q depends on unknown step %q", e.StepID, e.DependencyID)
}

// DAG is the dependency graph of a runbook's steps.
type DAG struct {
	steps map[string]*runbook.Step
	edges map[string][]string // stepID -> ids it depends on
}

// Build constructs a DAG from steps, validating that every dependency refers to a known
// step id and that no cycle exists.
func Build(steps []*runbook.Step) (*DAG, error) {
	g := &DAG{
		steps: make(map[string]*runbook.Step, len(steps)),
		edges: make(map[string][]string, len(steps)),
	}
	for _, s := range steps {
		g.steps[s.ID] = s
		g.edges[s.ID] = append([]string(nil), s.DependsOn...)
	}
	for id, deps := range g.edges {
		for _, dep := range deps {
			if _, ok := g.steps[dep]; !ok {
				return nil, &UnknownDependencyError{StepID: id, DependencyID: dep}
			}
		}
	}
	if cycle := g.findCycle(); cycle != nil {
		return nil, &CycleError{Cycle: cycle}
	}
	return g, nil
}

// findCycle performs a DFS looking for a back edge, returning the cycle path if found.
func (g *DAG) findCycle() []string {
	const (
		white = 0
		gray  = 1
		black = 2
	)
	color := make(map[string]int, len(g.steps))
	var path []string
	var visit func(id string) []string
	visit = func(id string) []string {
		color[id] = gray
		path = append(path, id)
		for _, dep := range g.edges[id] {
			switch color[dep] {
			case gray:
				start := 0
				for i, p := range path {
					if p == dep {
						start = i
						break
					}
				}
				cycle := append([]string(nil), path[start:]...)
				return append(cycle, dep)
			case white:
				if c := visit(dep); c != nil {
					return c
				}
			}
		}
		path = path[:len(path)-1]
		color[id] = black
		return nil
	}
	ids := g.orderedIDs()
	for _, id := range ids {
		if color[id] == white {
			if c := visit(id); c != nil {
				return c
			}
		}
	}
	return nil
}

// orderedIDs returns step ids in their original declaration order, for deterministic
// traversal and error messages.
func (g *DAG) orderedIDs() []string {
	ids := make([]string, 0, len(g.steps))
	seen := make(map[string]bool, len(g.steps))
	var walk []string
	for id := range g.steps {
		walk = append(walk, id)
	}
	// Preserve a stable order based on step slice order captured at Build time via the
	// steps map is not guaranteed; fall back to a stable sort-free pass using the
	// insertion order recorded in edges construction is not available either, so sort
	// lexically for determinism.
	sortStrings(walk)
	for _, id := range walk {
		if !seen[id] {
			seen[id] = true
			ids = append(ids, id)
		}
	}
	return ids
}

func sortStrings(s []string) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j-1] > s[j]; j-- {
			s[j-1], s[j] = s[j], s[j-1]
		}
	}
}

// Levels returns the steps grouped into ordered batches: every step in a batch has all
// of its dependencies satisfied by steps in earlier batches, so steps within a batch can
// run concurrently.
func (g *DAG) Levels() [][]*runbook.Step {
	remaining := make(map[string][]string, len(g.edges))
	for id, deps := range g.edges {
		remaining[id] = append([]string(nil), deps...)
	}
	done := make(map[string]bool, len(g.steps))
	var levels [][]*runbook.Step
	for len(done) < len(g.steps) {
		var batchIDs []string
		for _, id := range g.orderedIDs() {
			if done[id] {
				continue
			}
			ready := true
			for _, dep := range remaining[id] {
				if !done[dep] {
					ready = false
					break
				}
			}
			if ready {
				batchIDs = append(batchIDs, id)
			}
		}
		if len(batchIDs) == 0 {
			// Build already rejected cycles, so this should be unreachable.
			break
		}
		batch := make([]*runbook.Step, 0, len(batchIDs))
		for _, id := range batchIDs {
			batch = append(batch, g.steps[id])
			done[id] = true
		}
		levels = append(levels, batch)
	}
	return levels
}

// Step returns the step with the given id, or nil if it does not exist in the graph.
func (g *DAG) Step(id string) *runbook.Step {
	return g.steps[id]
}

// Dependents returns the ids of steps that directly depend on id.
func (g *DAG) Dependents(id string) []string {
	var out []string
	for sid, deps := range g.edges {
		for _, dep := range deps {
			if dep == id {
				out = append(out, sid)
				break
			}
		}
	}
	sortStrings(out)
	return out
}
