// SPDX-License-Identifier: Apache-2.0

package parser

import (
	"fmt"
	"regexp"

	"github.com/Leo-Galli/opsrun/internal/dag"
	"github.com/Leo-Galli/opsrun/internal/runbook"
)

var defaultDangerousPatterns = []string{
	`rm\s+-rf\s+/(\s|$)`,
	`rm\s+-rf\s+\*\s*$`,
	`:\(\)\s*\{\s*:\s*\|\s*:\s*&\s*\}\s*;\s*:`,
	`mkfs\.\w+`,
	`dd\s+if=\S+\s+of=/dev/(sd|nvme|hd|xvd)`,
}

var defaultDestructivePatterns = []string{
	`rm\s+-rf?\b`,
	`drop\s+(table|database)`,
	`truncate\s+table`,
	`systemctl\s+stop`,
	`\bshutdown\b`,
	`\breboot\b`,
	`kill\s+-9`,
}

// MarkdownValidator implements Validator for runbooks produced by MarkdownParser.
type MarkdownValidator struct {
	dangerous   []*regexp.Regexp
	destructive []*regexp.Regexp
}

// NewValidator builds a Validator using the built-in dangerous-command patterns plus any
// additional denylist regular expressions supplied by the caller.
func NewValidator(extraDenylist []string) (*MarkdownValidator, error) {
	v := &MarkdownValidator{}
	for _, p := range defaultDangerousPatterns {
		v.dangerous = append(v.dangerous, regexp.MustCompile("(?i)"+p))
	}
	for _, p := range extraDenylist {
		re, err := regexp.Compile("(?i)" + p)
		if err != nil {
			return nil, fmt.Errorf("invalid denylist pattern %q: %w", p, err)
		}
		v.dangerous = append(v.dangerous, re)
	}
	for _, p := range defaultDestructivePatterns {
		v.destructive = append(v.destructive, regexp.MustCompile("(?i)"+p))
	}
	return v, nil
}

// Validate implements Validator.
func (v *MarkdownValidator) Validate(rb *runbook.Runbook) []ValidationError {
	var errs []ValidationError

	ids := make(map[string]bool, len(rb.Steps))
	for _, s := range rb.Steps {
		ids[s.ID] = true
	}

	seen := make(map[string]bool, len(rb.Steps))
	allDepsKnown := true
	for _, s := range rb.Steps {
		if s.ID == "" {
			errs = append(errs, ValidationError{Severity: SeverityError, Message: "step has empty id", Position: s.Position})
			continue
		}
		if seen[s.ID] {
			errs = append(errs, ValidationError{Severity: SeverityError, StepID: s.ID, Position: s.Position, Message: fmt.Sprintf("duplicate step id %q", s.ID)})
		}
		seen[s.ID] = true
		for _, dep := range s.DependsOn {
			if !ids[dep] {
				allDepsKnown = false
				errs = append(errs, ValidationError{Severity: SeverityError, StepID: s.ID, Position: s.Position, Message: fmt.Sprintf("depends on unknown step %q", dep)})
			}
		}
	}

	if allDepsKnown {
		if _, err := dag.Build(rb.Steps); err != nil {
			if cycleErr, ok := err.(*dag.CycleError); ok {
				errs = append(errs, ValidationError{Severity: SeverityError, Message: cycleErr.Error()})
			}
		}
	}

	for _, s := range rb.Steps {
		v.validateStepCommands(rb, s, &errs)
	}

	return errs
}

func (v *MarkdownValidator) validateStepCommands(rb *runbook.Runbook, s *runbook.Step, errs *[]ValidationError) {
	actions := make([]*runbook.Action, 0, 2+len(s.Checks))
	if s.Run != nil {
		actions = append(actions, s.Run)
	}
	actions = append(actions, s.Checks...)
	if s.Rollback != nil {
		actions = append(actions, s.Rollback)
	}

	for _, a := range actions {
		for _, name := range ExtractVarRefs(a.Command) {
			if _, ok := rb.Vars[name]; !ok {
				*errs = append(*errs, ValidationError{Severity: SeverityError, StepID: s.ID, Position: a.Position, Message: fmt.Sprintf("undeclared variable %q", name)})
			}
		}
		for _, pat := range v.dangerous {
			if pat.MatchString(a.Command) {
				*errs = append(*errs, ValidationError{Severity: SeverityError, StepID: s.ID, Position: a.Position, Message: fmt.Sprintf("command matches dangerous pattern %q", pat.String())})
			}
		}
	}

	if s.Run == nil {
		return
	}
	for _, pat := range v.destructive {
		if pat.MatchString(s.Run.Command) && s.Rollback == nil {
			*errs = append(*errs, ValidationError{Severity: SeverityWarning, StepID: s.ID, Position: s.Run.Position, Message: "destructive step has no rollback action"})
			break
		}
	}
}
