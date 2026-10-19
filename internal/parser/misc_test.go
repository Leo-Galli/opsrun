// SPDX-License-Identifier: Apache-2.0

package parser

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Leo-Galli/opsrun/internal/runbook"
)

func TestParseErrorError(t *testing.T) {
	e := &ParseError{Message: "boom", Position: runbook.Position{Line: 3, Column: 5}}
	require.Equal(t, "3:5: boom", e.Error())

	e2 := &ParseError{Message: "boom"}
	require.Equal(t, "boom", e2.Error())
}

func TestValidationErrorError(t *testing.T) {
	e := &ValidationError{Message: "something is wrong"}
	require.Equal(t, "something is wrong", e.Error())
}

func TestNewValidatorInvalidDenylistPattern(t *testing.T) {
	_, err := NewValidator([]string{"("})
	require.Error(t, err)
}
