// SPDX-License-Identifier: Apache-2.0

// Package parser defines the contract for turning Markdown runbook source into the
// runbook domain model, and for validating the result. The goldmark-based implementation
// lives in parser_markdown.go.
package parser

import (
	"context"
	"io"

	"github.com/Leo-Galli/opsrun/internal/runbook"
)

// RunbookParser parses Markdown runbook source into the domain model. Implementations
// must walk the Markdown AST rather than matching on raw text, so that positions
// reported in errors are accurate.
type RunbookParser interface {
	Parse(ctx context.Context, r io.Reader) (*runbook.Runbook, error)
}

// Severity classifies a validation finding.
type Severity string

// Supported validation severities.
const (
	SeverityError   Severity = "error"
	SeverityWarning Severity = "warning"
)

// ValidationError describes a single problem found while validating a parsed runbook.
type ValidationError struct {
	Severity Severity
	Message  string
	StepID   string
	Position runbook.Position
}

// Error implements the error interface so a ValidationError can be returned directly.
func (e *ValidationError) Error() string {
	return e.Message
}

// Validator checks a parsed runbook for structural and semantic problems that the
// parser itself cannot catch in a single pass: duplicate step ids, missing or cyclic
// dependencies, missing rollback actions on destructive steps, undeclared variable
// references, and dangerous commands matched against a configurable denylist.
type Validator interface {
	Validate(rb *runbook.Runbook) []ValidationError
}
