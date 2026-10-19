// SPDX-License-Identifier: Apache-2.0

package inventory

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewStatic(t *testing.T) {
	inv := NewStatic([]Host{
		{Name: "web-1", Address: "10.0.0.1", Groups: []string{"web"}},
		{Name: "web-2", Address: "10.0.0.2", Groups: []string{"web"}},
		{Name: "db-1", Address: "10.0.0.3", Groups: []string{"db"}},
	})

	hosts, err := inv.Hosts(context.Background())
	require.NoError(t, err)
	require.Len(t, hosts, 3)

	group, err := inv.Group(context.Background(), "web")
	require.NoError(t, err)
	require.Len(t, group, 2)

	_, err = inv.Group(context.Background(), "missing")
	require.Error(t, err)

	resolved, err := inv.Resolve(context.Background(), "web-1")
	require.NoError(t, err)
	require.Len(t, resolved, 1)
	require.Equal(t, "web-1", resolved[0].Name)

	resolved, err = inv.Resolve(context.Background(), "db")
	require.NoError(t, err)
	require.Len(t, resolved, 1)
	require.Equal(t, "db-1", resolved[0].Name)

	_, err = inv.Resolve(context.Background(), "nope")
	require.Error(t, err)
}
