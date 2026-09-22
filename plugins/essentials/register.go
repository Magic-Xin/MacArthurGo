package essentials

import "fmt"

// RegisterBuiltins registers the always-available administrative plugins.
func RegisterBuiltins() error {
	registrations := []struct {
		name string
		fn   func() error
	}{
		{name: "ban", fn: registerBan},
		{name: "info", fn: registerInfo},
		{name: "update", fn: registerUpdate},
	}
	for _, registration := range registrations {
		if err := registration.fn(); err != nil {
			return fmt.Errorf("register builtin %s: %w", registration.name, err)
		}
	}
	return nil
}
