package bridge

// tokenresolver_wiring_test.go — RED contract for cycle-623 task
// token-resolver-production-wiring (inbox
// 2026-07-08T02-10-00Z-token-resolver-production-wiring.json, weight 0.96).
//
// Confirmed bug: `grep -rn TokenResolver go/internal/adapters/bridge/bridge.go`
// returns zero non-test hits — NewDefault's production engineFactory builds a
// gobridge.Deps that never sets TokenResolver, so recordTokenUsage's
// `if e.deps.TokenResolver == nil { return }` guard (internal/bridge/
// engine.go:527) fires on every real launch: token telemetry has been
// silently all-zero since at least cycle 612 (fail-open masks the gap).
//
// Fix contract (Builder implements): a new unexported method
//
//	func (a *Adapter) productionEngineDeps(env map[string]string) gobridge.Deps
//
// that sets TokenResolver: tokenusage.DefaultResolver(configRoot) (configRoot
// resolved from env["HOME"], falling back to os.Getenv("HOME") — same
// precedent as internal/bridge/doctor.go's doctorHome() + ".claude", see
// internal/bridge/billing.go:47 for the exact join). NewDefault's
// engineFactory closure must build its gobridge.Deps via this method (in
// place of the current hand-rolled literal) so the two IDENTICAL DI wiring
// call sites in this file collapse to one. This file — and the
// HasTokenResolver accessor from internal/bridge/hastokenresolver_test.go —
// are both undefined today, so `go vet`/`go build` fails on this package:
// the intended RED signal. DO NOT modify this file; implement production
// code only.
import (
	"os"
	"path/filepath"
	"testing"
	"time"

	gobridge "github.com/mickeyyaya/evolve-loop/go/internal/bridge"
	"github.com/mickeyyaya/evolve-loop/go/internal/tokenusage"
)

// mustParseRFC3339 is a local test helper (this package has no existing
// RFC3339 fixture-time helper to reuse).
func mustParseRFC3339(t *testing.T, s string) time.Time {
	t.Helper()
	ts, err := time.Parse(time.RFC3339, s)
	if err != nil {
		t.Fatalf("parse %s: %v", s, err)
	}
	return ts
}

// TestProductionEngineDeps_WiresNonNilTokenResolver — AC: "Both composition
// roots wire a non-nil resolver via one shared helper" (scout-report.md
// Acceptance Criteria Summary). NewDefault's Deps-building helper must never
// leave TokenResolver nil for a resolvable HOME.
func TestProductionEngineDeps_WiresNonNilTokenResolver(t *testing.T) {
	a := NewDefault(t.TempDir())
	d := a.productionEngineDeps(map[string]string{"HOME": t.TempDir()})
	if d.TokenResolver == nil {
		t.Error("productionEngineDeps(env).TokenResolver is nil — production launches get silent zero telemetry (the cycle-612+ bug)")
	}
}

// TestEngineFactory_WiresTokenResolver is the exact predicate scout-report.md
// names for this task's verifiableBy: "asserts production-built Engine has
// non-nil Deps.TokenResolver". Exercises the REAL engineFactory closure (not
// just the Deps-building helper in isolation) via the HasTokenResolver
// accessor, so a Builder that wires productionEngineDeps into the helper but
// forgets to route engineFactory through it cannot pass by coincidence.
func TestEngineFactory_WiresTokenResolver(t *testing.T) {
	a := NewDefault(t.TempDir())
	built := a.engineFactory(map[string]string{"HOME": t.TempDir()})
	eng, ok := built.(*gobridge.Engine)
	if !ok {
		t.Fatalf("engineFactory returned %T, want *bridge.Engine", built)
	}
	if !eng.HasTokenResolver() {
		t.Error("production engineFactory built an Engine with no TokenResolver wired")
	}
}

// TestProductionEngineDeps_ResolverAppliesRealFixture is the anti-gaming
// counterpart to the two tests above (predicate-quality rule: a wiring check
// that only proves "the field is non-nil" would pass a stub func that always
// returns SourceNone). It proves the wired resolver is genuinely
// tokenusage.DefaultResolver — i.e. it recovers real usage from a real
// on-disk transcript fixture placed under HOME/.claude/projects/... — not a
// disconnected placeholder.
func TestProductionEngineDeps_ResolverAppliesRealFixture(t *testing.T) {
	home := t.TempDir()
	worktree := "/repo/worktrees/cycle-623"
	configRoot := filepath.Join(home, ".claude")
	sessionDir := filepath.Join(configRoot, "projects", "-repo-worktrees-cycle-623")
	if err := os.MkdirAll(sessionDir, 0o755); err != nil {
		t.Fatalf("mkdir fixture session dir: %v", err)
	}
	body := `{"type":"user","cwd":"` + worktree + `","timestamp":"2026-07-08T11:00:01Z","message":{"id":"u1","content":[{"type":"text","text":"start"}]}}
{"type":"assistant","cwd":"` + worktree + `","timestamp":"2026-07-08T11:00:05Z","message":{"id":"m1","usage":{"input_tokens":300,"output_tokens":60,"cache_read_input_tokens":0,"cache_creation_input_tokens":0}}}
`
	if err := os.WriteFile(filepath.Join(sessionDir, "sess1.jsonl"), []byte(body), 0o644); err != nil {
		t.Fatalf("write fixture transcript: %v", err)
	}

	a := NewDefault(t.TempDir())
	d := a.productionEngineDeps(map[string]string{"HOME": home})
	if d.TokenResolver == nil {
		t.Fatal("productionEngineDeps(env).TokenResolver is nil")
	}
	res, err := d.TokenResolver(tokenusage.Window{
		Worktree: worktree,
		Start:    mustParseRFC3339(t, "2026-07-08T10:59:00Z"),
		End:      mustParseRFC3339(t, "2026-07-08T11:01:00Z"),
	})
	if err != nil {
		t.Fatalf("wired resolver returned error against a valid fixture: %v", err)
	}
	if res.Source != tokenusage.SourceTranscript {
		t.Errorf("Source = %q, want %q — the wired resolver must actually scan HOME/.claude, not stub SourceNone", res.Source, tokenusage.SourceTranscript)
	}
}
