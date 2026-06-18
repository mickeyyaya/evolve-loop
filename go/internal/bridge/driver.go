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

// CLIPreflight is an OPTIONAL Driver capability for per-CLI prep work that
// must complete BEFORE the inner CLI process is launched. The Engine
// dispatches it via type assertion (`driver.(CLIPreflight)`) so a driver
// that needs no prep work simply omits the method — no no-op stubs in every
// concrete driver. Establishing this seam (cycle-124 G3, redesign of the
// inline pretrust call at the top of codexTmuxDriver.Launch) gives every
// CLI a uniform place to mutate config files / refresh credentials / probe
// the binary BEFORE the user-visible launch path runs. Today only
// codex-tmux implements it (pre-trust worktree + workspace paths in
// ~/.codex/config.toml per cycle-122 Fix 1); claude-tmux / agy-tmux /
// ollama-tmux opt out by not declaring the method.
//
// Semantics: best-effort. The Engine LOGS a non-nil error to stderr but
// continues to Launch — this matches the existing inline call's posture
// (Fix 2's extended fallback trigger list is the downstream defense
// against any preflight failure). A driver that needs Preflight to be
// load-bearing (abort launch on failure) MUST encode that in its own
// Launch body, not here.
type CLIPreflight interface {
	Preflight(ctx context.Context, cfg *Config, deps Deps) error
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

// bareDriverMap is the single source of truth for the BARE CLI name →
// registered driver projection. The dispatch path (subagent.Run /
// ValidateProfile, consensusdispatch) historically shelled `bash <cli>.sh`
// for both bare names ("claude") and driver names ("claude-tmux"). Routing
// through the bridge instead means a bare name must first be projected onto
// a registered driver, because LookupDriver keys on the exact driver name.
//
// Resolved CLI names (claude-tmux/codex-tmux/agy-tmux) are already driver
// names and pass through DriverFor unchanged. The bare names:
//   - claude → claude-tmux  (production subscription path; CLAUDE.md)
//   - gemini → claude-tmux  (gemini.sh's HYBRID mode delegated to claude.sh;
//     mapping to claude-tmux preserves that delegation)
//   - codex → codex-tmux, agy → agy-tmux are listed for completeness, but they
//     are DORMANT: "codex" and "agy" are themselves registered (headless)
//     drivers, so DriverFor's already-a-driver pass-through wins and the map
//     entry is never consulted. They fire only if those drivers were ever
//     unregistered, keeping the projection total.
//
// "antigravity" is normalized to "agy" upstream (run.go / validateprofile.go),
// so it never reaches here as a bare name.
var bareDriverMap = map[string]string{
	"claude": "claude-tmux",
	"gemini": "claude-tmux",
	"codex":  "codex-tmux",
	"agy":    "agy-tmux",
}

// DriverFor projects a resolved-or-bare CLI name onto a registered driver
// name. A name that is already a registered driver passes through unchanged;
// a known bare name maps via bareDriverMap; anything else is returned as-is
// so the caller's LookupDriver miss surfaces the original (unknown) name in
// its diagnostic. This is the single source for the bare→driver projection
// shared by every dispatch site (subagent + consensusdispatch).
func DriverFor(cli string) string {
	if _, ok := LookupDriver(cli); ok {
		return cli
	}
	if mapped, ok := bareDriverMap[cli]; ok {
		return mapped
	}
	return cli
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
