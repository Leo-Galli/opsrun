// SPDX-License-Identifier: Apache-2.0

package inventory

import (
	"context"
	"fmt"
)

// static is an in-memory Inventory backed by a fixed list of hosts, typically loaded
// from opsrun.yaml.
type static struct {
	hosts  []Host
	byName map[string]Host
	groups map[string][]Host
}

// NewStatic builds an Inventory from a fixed list of hosts.
func NewStatic(hosts []Host) Inventory {
	s := &static{
		hosts:  hosts,
		byName: make(map[string]Host, len(hosts)),
		groups: make(map[string][]Host),
	}
	for _, h := range hosts {
		s.byName[h.Name] = h
		for _, g := range h.Groups {
			s.groups[g] = append(s.groups[g], h)
		}
	}
	return s
}

// Hosts implements Inventory.
func (s *static) Hosts(_ context.Context) ([]Host, error) {
	return append([]Host(nil), s.hosts...), nil
}

// Group implements Inventory.
func (s *static) Group(_ context.Context, name string) ([]Host, error) {
	hosts, ok := s.groups[name]
	if !ok {
		return nil, fmt.Errorf("unknown group %q", name)
	}
	return append([]Host(nil), hosts...), nil
}

// Resolve implements Inventory.
func (s *static) Resolve(ctx context.Context, target string) ([]Host, error) {
	if h, ok := s.byName[target]; ok {
		return []Host{h}, nil
	}
	if hosts, ok := s.groups[target]; ok {
		return append([]Host(nil), hosts...), nil
	}
	return nil, fmt.Errorf("unknown host or group %q", target)
}
