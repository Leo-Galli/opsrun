// SPDX-License-Identifier: Apache-2.0

// Package inventory resolves runbook target hosts and groups to concrete connection
// parameters.
package inventory

import "context"

// Host is a single remote target OpsRun can connect to.
type Host struct {
	Name         string
	Address      string
	Port         int
	User         string
	Groups       []string
	IdentityFile string
	ProxyJump    string
}

// Inventory resolves host names and group names declared in a runbook or on the command
// line to concrete hosts.
type Inventory interface {
	// Hosts returns every host known to the inventory.
	Hosts(ctx context.Context) ([]Host, error)

	// Group returns the hosts belonging to the named group.
	Group(ctx context.Context, name string) ([]Host, error)

	// Resolve returns the hosts a target string refers to. A target may be a single
	// host name or a group name.
	Resolve(ctx context.Context, target string) ([]Host, error)
}
