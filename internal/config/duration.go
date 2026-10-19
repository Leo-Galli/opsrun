// SPDX-License-Identifier: Apache-2.0

package config

import (
	"fmt"
	"time"
)

// Duration wraps time.Duration so it can be parsed from a YAML string such as "30s".
type Duration time.Duration

// UnmarshalYAML implements yaml.Unmarshaler.
func (d *Duration) UnmarshalYAML(unmarshal func(any) error) error {
	var s string
	if err := unmarshal(&s); err != nil {
		return err
	}
	parsed, err := time.ParseDuration(s)
	if err != nil {
		return fmt.Errorf("invalid duration %q: %w", s, err)
	}
	*d = Duration(parsed)
	return nil
}

// MarshalYAML implements yaml.Marshaler.
func (d Duration) MarshalYAML() (any, error) {
	return time.Duration(d).String(), nil
}

// Value returns the underlying time.Duration.
func (d Duration) Value() time.Duration {
	return time.Duration(d)
}
