package bridge

import (
	"strings"
	"testing"
	"time"
)

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

func TestResolveSessionNoncePreservesRunScopePrefix(t *testing.T) {
	deps := Deps{Now: time.Now}.withDefaults()
	cfg := &Config{Cycle: 12, Agent: "build", RunID: "01ARZ3NDEKTSV4RRFFQ69G5FAV"}
	got, _ := resolveSession(cfg, deps, "evolve-bridge-")
	if !strings.HasPrefix(got, "evolve-bridge-r01ARZ3ND-c12-build-") {
		t.Errorf("session=%q lost the run-scope prefix", got)
	}
}
