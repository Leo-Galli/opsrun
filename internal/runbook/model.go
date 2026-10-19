// SPDX-License-Identifier: Apache-2.0

// Package runbook defines the domain model produced by parsing a Markdown runbook.
package runbook

import "time"

// Position identifies a location in the source Markdown document, used to produce
// precise parse and validation errors.
type Position struct {
	Line   int
	Column int
}

// VariableType is the declared type of a runbook variable.
type VariableType string

// Supported variable types.
const (
	VariableTypeString   VariableType = "string"
	VariableTypeInt      VariableType = "int"
	VariableTypeBool     VariableType = "bool"
	VariableTypeDuration VariableType = "duration"
)

// Variable describes a runbook input declared in the front matter.
type Variable struct {
	Name       string
	Type       VariableType
	Default    any
	Required   bool
	Validation string // optional regular expression the string form of the value must match
	Position   Position
}

// ApprovalMode controls when a human must approve a step before it runs.
type ApprovalMode string

// Supported approval modes.
const (
	ApprovalModeNone    ApprovalMode = "none"
	ApprovalModePerStep ApprovalMode = "per_step"
	ApprovalModeAll     ApprovalMode = "all"
)

// ApprovalPolicy is the runbook-wide approval configuration, overridable per step.
type ApprovalPolicy struct {
	Mode    ApprovalMode
	Timeout time.Duration
}

// Severity is the declared incident severity of a runbook.
type Severity string

// Supported severities.
const (
	SeverityLow      Severity = "low"
	SeverityMedium   Severity = "medium"
	SeverityHigh     Severity = "high"
	SeverityCritical Severity = "critical"
)

// Target identifies a set of hosts a step or runbook applies to, by explicit host name
// or by inventory group. Exactly one of Host or Group is set.
type Target struct {
	Host  string
	Group string
}

// ExecutionMode controls how a step's target hosts are addressed.
type ExecutionMode string

// Supported execution modes.
const (
	ExecutionModeSerial   ExecutionMode = "serial"
	ExecutionModeParallel ExecutionMode = "parallel"
)

// ActionKind is the role a code block plays within a step.
type ActionKind string

// Supported action kinds.
const (
	ActionKindRun      ActionKind = "run"
	ActionKindCheck    ActionKind = "check"
	ActionKindRollback ActionKind = "rollback"
)

// Action is a single executable command extracted from a step's code block.
type Action struct {
	Kind       ActionKind
	Command    string
	Target     string // inventory group or host this action runs against
	Timeout    time.Duration
	ExpectExit int // only meaningful for ActionKindCheck
	Position   Position
}

// Step is one node of the runbook DAG: a named unit of work with an action, optional
// post-condition checks, an optional rollback action, and dependencies on other steps.
type Step struct {
	ID              string
	Title           string
	Description     string
	DependsOn       []string
	Confirm         bool
	Retries         int
	ContinueOnError bool
	Mode            ExecutionMode
	ReadOnly        bool
	Approval        *ApprovalPolicy // nil means inherit the runbook-wide policy
	Run             *Action
	Checks          []*Action
	Rollback        *Action
	Position        Position
}

// Runbook is the fully parsed representation of a Markdown runbook.
type Runbook struct {
	Name     string
	Version  string
	Service  string
	Severity Severity
	Author   string
	Targets  []Target
	Vars     map[string]Variable
	Approval ApprovalPolicy
	Steps    []*Step
}

// StepByID returns the step with the given id, or nil if no such step exists.
func (r *Runbook) StepByID(id string) *Step {
	for _, s := range r.Steps {
		if s.ID == id {
			return s
		}
	}
	return nil
}
