//go:build e2e && !evolve_test_phases

package a

import "example.com/fix/f"

var F = f.Name
