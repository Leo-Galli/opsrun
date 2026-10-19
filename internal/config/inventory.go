// SPDX-License-Identifier: Apache-2.0

package config

import "github.com/Leo-Galli/opsrun/internal/inventory"

// InventoryHosts converts the configured host entries into inventory.Host values.
func (c *Config) InventoryHosts() []inventory.Host {
	hosts := make([]inventory.Host, 0, len(c.Hosts))
	for _, h := range c.Hosts {
		hosts = append(hosts, inventory.Host{
			Name:         h.Name,
			Address:      h.Address,
			Port:         h.Port,
			User:         h.User,
			Groups:       h.Groups,
			IdentityFile: h.IdentityFile,
			ProxyJump:    h.ProxyJump,
		})
	}
	return hosts
}
