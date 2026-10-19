// SPDX-License-Identifier: Apache-2.0

package runbook

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestStepByID(t *testing.T) {
	rb := &Runbook{Steps: []*Step{{ID: "a"}, {ID: "b"}}}
	require.Equal(t, "a", rb.StepByID("a").ID)
	require.Nil(t, rb.StepByID("missing"))
}
