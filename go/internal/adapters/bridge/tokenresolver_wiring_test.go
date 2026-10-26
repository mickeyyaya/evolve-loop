package bridge

import (
	"os"
	"os/exec"
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
		Collector: tokenusage.FormatClaudeJSONL,
		Worktree:  worktree,
		Start:     mustParseRFC3339(t, "2026-07-08T10:59:00Z"),
		End:       mustParseRFC3339(t, "2026-07-08T11:01:00Z"),
	})
	if err != nil {
		t.Fatalf("wired resolver returned error against a valid fixture: %v", err)
	}
	if res.Source != tokenusage.SourceTranscript {
		t.Errorf("Source = %q, want %q — the wired resolver must actually scan HOME/.claude, not stub SourceNone", res.Source, tokenusage.SourceTranscript)
	}
}

func TestProductionEngineDeps_ResolverReadsTheAgyConversationStoreUnderHome(t *testing.T) {
	if _, err := exec.LookPath("sqlite3"); err != nil {
		t.Skip("sqlite3 not on PATH")
	}
	home := t.TempDir()
	worktree := "/repo/worktrees/cycle-1967"
	store := filepath.Join(home, ".gemini", "antigravity-cli")
	id := "587d3641-24fd-443d-8989-c539a294e080"
	if err := os.MkdirAll(filepath.Join(store, "conversations"), 0o755); err != nil {
		t.Fatal(err)
	}
	seed := func(db, sql string) {
		t.Helper()
		if out, err := exec.Command("sqlite3", db, sql).CombinedOutput(); err != nil {
			t.Fatalf("seed %s: %v %s", db, err, out)
		}
	}
	seed(filepath.Join(store, "conversation_summaries.db"), `create table conversation_summaries(conversation_id text, workspace_uris text, last_user_input_time datetime);
insert into conversation_summaries values ('`+id+`', '["file://`+worktree+`"]', '2026-10-19 07:07:41.401316+00:00');`)
	seed(filepath.Join(store, "conversations", id+".db"), `create table gen_metadata(idx integer, data blob);
insert into gen_metadata values (1, x'0a0b220910d012181e28c0c403');`)

	d := NewDefault(t.TempDir(), nil).productionEngineDeps(map[string]string{"HOME": home})
	got, err := d.TokenResolver(tokenusage.Window{Driver: "agy-tmux", Collector: tokenusage.FormatAgyConversation, Worktree: worktree,
		Start: mustParseRFC3339(t, "2026-10-19T07:07:30Z"), End: mustParseRFC3339(t, "2026-10-19T07:23:34Z")})

	if err != nil || got.Source != tokenusage.SourceAgyConversation || got.Usage.Input != 2384 || got.Usage.Output != 30 || got.Usage.CacheRead != 57920 {
		t.Errorf("resolver = %+v, %v; want the agy conversation under HOME", got, err)
	}
}
