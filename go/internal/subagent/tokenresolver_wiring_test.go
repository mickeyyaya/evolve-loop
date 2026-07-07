package subagent

// tokenresolver_wiring_test.go — RED contract for cycle-623 task
// token-resolver-production-wiring (inbox
// 2026-07-08T02-10-00Z-token-resolver-production-wiring.json, weight 0.96).
//
// Confirmed bug: `grep -rn TokenResolver go/internal/subagent/validateprofile.go`
// returns zero non-test hits — defaultExecAdapter's `gobridge.NewEngine(
// gobridge.Deps{Env: env})` (validateprofile.go:347) never sets
// TokenResolver, so every real (VALIDATE_ONLY=0) subagent dispatch through
// this composition root also gets silent zero telemetry — the second half of
// the same bug fixed on the adapters/bridge side (see
// internal/adapters/bridge/tokenresolver_wiring_test.go).
//
// Fix contract (Builder implements): a new unexported function
//
//	func execAdapterDeps(env map[string]string) gobridge.Deps
//
// that sets TokenResolver: tokenusage.DefaultResolver(configRoot) — SAME
// configRoot-resolution convention as productionEngineDeps in
// internal/adapters/bridge (env["HOME"] falling back to os.Getenv("HOME"),
// joined with ".claude") — the ONE shared tokenusage.DefaultResolver helper,
// two composition roots, both resolving configRoot the same way.
// defaultExecAdapter must build its gobridge.Deps via this function in place
// of the current `gobridge.Deps{Env: env}` literal. execAdapterDeps is
// undefined today, so this package fails to compile — the intended RED
// signal. DO NOT modify this file; implement production code only.
import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/tokenusage"
)

// TestExecAdapterDeps_WiresNonNilTokenResolver — AC: "Both composition roots
// wire a non-nil resolver via one shared helper" (scout-report.md
// Acceptance Criteria Summary), validateprofile.go half.
func TestExecAdapterDeps_WiresNonNilTokenResolver(t *testing.T) {
	d := execAdapterDeps(map[string]string{"HOME": t.TempDir()})
	if d.TokenResolver == nil {
		t.Error("execAdapterDeps(env).TokenResolver is nil — real subagent dispatches get silent zero telemetry")
	}
}

// TestExecAdapterDeps_ResolverAppliesRealFixture — anti-gaming counterpart:
// proves the wired resolver genuinely scans HOME/.claude via
// tokenusage.DefaultResolver, not a disconnected stub.
func TestExecAdapterDeps_ResolverAppliesRealFixture(t *testing.T) {
	home := t.TempDir()
	worktree := "/repo/worktrees/cycle-623-subagent"
	configRoot := filepath.Join(home, ".claude")
	sessionDir := filepath.Join(configRoot, "projects", "-repo-worktrees-cycle-623-subagent")
	if err := os.MkdirAll(sessionDir, 0o755); err != nil {
		t.Fatalf("mkdir fixture session dir: %v", err)
	}
	body := `{"type":"user","cwd":"` + worktree + `","timestamp":"2026-07-08T12:00:01Z","message":{"id":"u1","content":[{"type":"text","text":"start"}]}}
{"type":"assistant","cwd":"` + worktree + `","timestamp":"2026-07-08T12:00:05Z","message":{"id":"m1","usage":{"input_tokens":80,"output_tokens":15,"cache_read_input_tokens":0,"cache_creation_input_tokens":0}}}
`
	if err := os.WriteFile(filepath.Join(sessionDir, "sess1.jsonl"), []byte(body), 0o644); err != nil {
		t.Fatalf("write fixture transcript: %v", err)
	}

	d := execAdapterDeps(map[string]string{"HOME": home})
	if d.TokenResolver == nil {
		t.Fatal("execAdapterDeps(env).TokenResolver is nil")
	}
	res, err := d.TokenResolver(tokenusage.Window{
		Worktree: worktree,
		Start:    mustParseRFC3339Subagent(t, "2026-07-08T11:59:00Z"),
		End:      mustParseRFC3339Subagent(t, "2026-07-08T12:01:00Z"),
	})
	if err != nil {
		t.Fatalf("wired resolver returned error against a valid fixture: %v", err)
	}
	if res.Source != tokenusage.SourceTranscript {
		t.Errorf("Source = %q, want %q — the wired resolver must actually scan HOME/.claude, not stub SourceNone", res.Source, tokenusage.SourceTranscript)
	}
}

// TestExecAdapterDeps_MissingHome_StillReturnsNonNilResolver — edge case: an
// env map with no "HOME" key at all (a stripped/minimal env, which
// ExecAdapter's callers can construct) must not panic and must not leave
// TokenResolver nil; it degrades to os.Getenv("HOME") per the documented
// fallback, and even a totally unresolvable HOME still yields a resolver
// func (Chain/ScanConfigRoot fail open to SourceNone, never a nil func).
func TestExecAdapterDeps_MissingHome_StillReturnsNonNilResolver(t *testing.T) {
	d := execAdapterDeps(map[string]string{})
	if d.TokenResolver == nil {
		t.Error("execAdapterDeps(env without HOME).TokenResolver is nil — must degrade gracefully, never drop telemetry wiring entirely")
	}
}

func mustParseRFC3339Subagent(t *testing.T, s string) time.Time {
	t.Helper()
	ts, err := time.Parse(time.RFC3339, s)
	if err != nil {
		t.Fatalf("parse %s: %v", s, err)
	}
	return ts
}
