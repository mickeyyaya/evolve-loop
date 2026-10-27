// Package kerneltest loads the shipped phase registry as a fixture for kernel tests.
// Tests reach phases by structural role, never by a hardcoded name.
package kerneltest

import (
	"path/filepath"
	"runtime"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasespec"
)

type Fixture struct {
	Catalog phasespec.Catalog
	Config  config.RoutingConfig
}

func registryPath() string {
	_, self, _, _ := runtime.Caller(0)
	return config.RegistryPath(filepath.Join(filepath.Dir(self), "..", "..", ".."))
}

func Load(t testing.TB) *Fixture {
	t.Helper()
	return loadFrom(t, registryPath())
}

func loadFrom(t testing.TB, path string) *Fixture {
	t.Helper()
	cat, err := phasespec.Load(path)
	if err != nil {
		t.Fatalf("kerneltest: load registry: %v", err)
		return nil
	}
	cfg, _ := config.Load(path)
	return &Fixture{Catalog: cat, Config: cfg}
}

func (f *Fixture) Mandatory() []string { return f.Config.Mandatory }

func (f *Fixture) Spine() []string { return f.Config.SpineOrder }

func (f *Fixture) FirstAnchor() string {
	if len(f.Config.Mandatory) == 0 {
		return ""
	}
	return f.Config.Mandatory[0]
}

func (f *Fixture) ShipTerminal() string {
	m := f.Config.Mandatory
	if len(m) == 0 {
		return ""
	}
	return m[len(m)-1]
}

func (f *Fixture) Evaluator() string {
	for _, name := range f.Config.Mandatory {
		if spec, ok := f.Catalog.Get(name); ok && spec.OnPass != "" && spec.OnFail != "" {
			return name
		}
	}
	return ""
}

func (f *Fixture) SpineEntry() string {
	if len(f.Config.SpineOrder) == 0 {
		return ""
	}
	return f.Config.SpineOrder[0]
}
