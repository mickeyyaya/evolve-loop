package bridge

import (
	"context"
	"fmt"
	"sort"
	"sync"
)

// Driver is the Strategy for one --cli target (claude-p, claude-tmux,
// codex, codex-tmux, agy, agy-tmux). The Engine owns the CLI-agnostic
// flow (validate → resolve config → preflight → report); a Driver owns
// only the CLI-specific invocation: building the inner argv, dispatching
// the process (or driving a tmux REPL), and waiting for the artifact.
//
// This replaces the bash file-dispatch smell — bin/bridge sourced
// drivers/${cli}.sh and called drv_launch_${cli//-/_} by name-mangling,
// a stringly-typed lookup with no compile-time guarantee the driver
// exists. The Strategy + Registry below makes the driver set
// compile-checked and fail-fast on duplicate registration.
type Driver interface {
	// Name is the --cli value this driver handles (e.g. "claude-p").
	Name() string

	// Launch runs the inner CLI for the fully-resolved config and
	// returns a bridge exit code (one of the Exit* constants). err is
	// non-nil only on unrecoverable harness failures (e.g. context
	// canceled); a CLI that ran but failed returns a non-zero exit code
	// with err == nil — mirroring the bash `set +e; $fn; rc=$?` contract.
	Launch(ctx context.Context, cfg *Config, deps Deps) (int, error)
}

// driverRegistry is the self-registering Driver table. Pattern: Factory
// Method / Registry (GoF) — identical in shape to
// internal/phases/registry so the two stay mentally aligned. Lookups are
// concurrency-safe; registration happens at init() time.
var (
	driverMu sync.RWMutex
	drivers  = map[string]Driver{}
)

// Register publishes a Driver under d.Name(). Panics on an empty name or
// a duplicate so init-time conflicts surface at startup rather than as a
// runtime mystery. Each drivers/<name>.go file calls this exactly once
// from init().
func Register(d Driver) {
	if d == nil {
		panic("bridge: Register requires a non-nil Driver")
	}
	name := d.Name()
	if name == "" {
		panic("bridge: Register requires a non-empty Driver.Name()")
	}
	driverMu.Lock()
	defer driverMu.Unlock()
	if _, exists := drivers[name]; exists {
		panic(fmt.Sprintf("bridge: duplicate Register(%q) — each driver registers exactly once", name))
	}
	drivers[name] = d
}

// LookupDriver returns the Driver for cli. (driver, true) on hit; (nil,
// false) on miss — the Engine returns ExitBadFlags on a miss, matching
// the bash "no driver for cli=..." path.
func LookupDriver(cli string) (Driver, bool) {
	driverMu.RLock()
	defer driverMu.RUnlock()
	d, ok := drivers[cli]
	return d, ok
}

// DriverNames returns a sorted snapshot of registered --cli values.
// Used by probe, usage output, and the docs-contract test.
func DriverNames() []string {
	driverMu.RLock()
	defer driverMu.RUnlock()
	out := make([]string, 0, len(drivers))
	for n := range drivers {
		out = append(out, n)
	}
	sort.Strings(out)
	return out
}

// ResetDriversForTesting clears the registry. ONLY for tests that need a
// controlled driver set; production code MUST NOT call it. The explicit
// suffix makes the intent unmistakable (mirrors phases/registry).
func ResetDriversForTesting() {
	driverMu.Lock()
	defer driverMu.Unlock()
	drivers = map[string]Driver{}
}
