// Package kerneltest provides a rename-proof test fixture for the transition
// kernel. Tests load the real phase configuration through it and reference
// phases by their STRUCTURAL role/position (first anchor, ship terminal, the
// verdict-branching evaluator, …) — never by a hardcoded phase name. Renaming a
// phase in the registry therefore does NOT require rewriting any test.
//
// It imports only phasespec + config (NOT core), so white-box `package core`
// tests can use it without an import cycle; phase names it returns are converted
// to core.Phase by the caller (via the kernel's own name bridge).
package kerneltest

import (
	"path/filepath"
	"runtime"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasespec"
)

// Fixture is the loaded reference flow. Catalog + Config are the live registry
// data; the accessors resolve specific phases structurally so callers never
// name them.
type Fixture struct {
	Catalog phasespec.Catalog
	Config  config.RoutingConfig
}

// registryPath resolves the shipped registry relative to THIS source file, so
// the fixture works from any package's test working directory.
func registryPath() string {
	_, self, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(self), "..", "..", "..", "docs", "architecture", "phase-registry.json")
}

// Load reads the shipped phase registry into a Fixture. Fatal on a load error —
// a kernel test cannot run without the real flow.
func Load(t testing.TB) *Fixture {
	t.Helper()
	path := registryPath()
	cat, err := phasespec.Load(path)
	if err != nil {
		t.Fatalf("kerneltest: load registry: %v", err)
	}
	cfg, _ := config.Load(path, nil)
	return &Fixture{Catalog: cat, Config: cfg}
}

// Mandatory returns the configured mandatory-anchor names in order.
func (f *Fixture) Mandatory() []string { return f.Config.Mandatory }

// Spine returns the configured linear-spine phase names in order.
func (f *Fixture) Spine() []string { return f.Config.SpineOrder }

// FirstAnchor is the earliest mandatory anchor (the discovery anchor, whatever
// it is named).
func (f *Fixture) FirstAnchor() string {
	if len(f.Config.Mandatory) == 0 {
		return ""
	}
	return f.Config.Mandatory[0]
}

// ShipTerminal is the last mandatory anchor — the phase the floor ships from.
func (f *Fixture) ShipTerminal() string {
	m := f.Config.Mandatory
	if len(m) == 0 {
		return ""
	}
	return m[len(m)-1]
}

// Evaluator is the mandatory phase that gates ship on a verdict — the one whose
// descriptor declares on_pass/on_fail (the audit-class evaluator), resolved
// structurally so a rename does not matter. "" if none.
func (f *Fixture) Evaluator() string {
	for _, name := range f.Config.Mandatory {
		if spec, ok := f.Catalog.Get(name); ok && spec.OnPass != "" && spec.OnFail != "" {
			return name
		}
	}
	return ""
}

// SpineEntry is the first phase on the linear spine.
func (f *Fixture) SpineEntry() string {
	if len(f.Config.SpineOrder) == 0 {
		return ""
	}
	return f.Config.SpineOrder[0]
}
