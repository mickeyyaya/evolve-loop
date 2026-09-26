package bridge

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	gobridge "github.com/mickeyyaya/evolve-loop/go/internal/bridge"
	"github.com/mickeyyaya/evolve-loop/go/internal/tokenusage"
)

func mustParseRFC3339(t *testing.T, s string) time.Time {
	t.Helper()
	ts, err := time.Parse(time.RFC3339, s)
	if err != nil {
		t.Fatalf("parse %s: %v", s, err)
	}
	return ts
}

func TestProductionEngineDeps_WiresNonNilTokenResolver(t *testing.T) {
	a := NewDefault(t.TempDir(), nil)
	d := a.productionEngineDeps(map[string]string{"HOME": t.TempDir()})
	if d.TokenResolver == nil {
		t.Error("productionEngineDeps(env).TokenResolver is nil — production launches get silent zero telemetry (the cycle-612+ bug)")
	}
}

func TestEngineFactory_WiresTokenResolver(t *testing.T) {
	a := NewDefault(t.TempDir(), nil)
	built := a.engineFactory(map[string]string{"HOME": t.TempDir()})
	eng, ok := built.(*gobridge.Engine)
	if !ok {
		t.Fatalf("engineFactory returned %T, want *bridge.Engine", built)
	}
	if !eng.HasTokenResolver() {
		t.Error("production engineFactory built an Engine with no TokenResolver wired")
	}
}

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

	a := NewDefault(t.TempDir(), nil)
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
