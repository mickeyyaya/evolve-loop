// driver_tmux_repl_nonce_test.go — ADR-0049 N15: ephemeral tmux session names
// must be unique even when two are minted at the SAME wall-clock instant.
// Concurrent fleet cycles (and same-phase retries within a cycle) can dispatch
// in the same second; a second-granularity timestamp alone collides, and tmux
// would then have two cycles fighting over one session.
package bridge

import (
	"strings"
	"testing"
	"time"
)

// Two ephemeral sessions minted under a FROZEN clock, same run/cycle/agent,
// must get distinct names. A timestamp (even UnixNano under a fixed clock)
// cannot guarantee this; a per-process nonce can.
func TestResolveSessionUniqueUnderSameClock(t *testing.T) {
	frozen := time.Unix(1_700_000_000, 0)
	deps := Deps{Now: func() time.Time { return frozen }}.withDefaults()
	cfg := &Config{Cycle: 7, Agent: "build", RunID: "01ARZ3NDEKTSV4RRFFQ69G5FAV"}

	a, _ := resolveSession(cfg, deps, "evolve-bridge-")
	b, _ := resolveSession(cfg, deps, "evolve-bridge-")
	if a == b {
		t.Errorf("two ephemeral sessions under the same clock got identical names %q — concurrent fleet dispatches would collide on one tmux session", a)
	}
}

// The nonce must survive truncation: even for the longest realistic agent +
// high cycle/pid, the unique tail must not be chopped by truncate64. Mint two
// long-name sessions under a frozen clock and require they still differ AND fit.
func TestResolveSessionNonceSurvivesTruncation(t *testing.T) {
	frozen := time.Unix(1_700_000_000, 0)
	deps := Deps{Now: func() time.Time { return frozen }}.withDefaults()
	cfg := &Config{Cycle: 9999, Agent: "build-planner", RunID: "01ARZ3NDEKTSV4RRFFQ69G5FAV"}

	a, _ := resolveSession(cfg, deps, "evolve-bridge-")
	b, _ := resolveSession(cfg, deps, "evolve-bridge-")
	if len(a) > 64 || len(b) > 64 {
		t.Errorf("session names exceed tmux-safe 64: %d/%d (%q,%q)", len(a), len(b), a, b)
	}
	if a == b {
		t.Errorf("long-agent sessions collided after truncation: %q", a)
	}
}

// The run-scope prefix contract (CB.5) must still hold with the nonce in place.
func TestResolveSessionNoncePreservesRunScopePrefix(t *testing.T) {
	deps := Deps{Now: time.Now}.withDefaults()
	cfg := &Config{Cycle: 12, Agent: "build", RunID: "01ARZ3NDEKTSV4RRFFQ69G5FAV"}
	got, _ := resolveSession(cfg, deps, "evolve-bridge-")
	if !strings.HasPrefix(got, "evolve-bridge-r01ARZ3ND-c12-build-") {
		t.Errorf("session=%q lost the run-scope prefix", got)
	}
}
