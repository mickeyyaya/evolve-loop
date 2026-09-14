package subagentrun

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Test 29 — the artifact placement table: workers under the workspace, the
// template expanded under the root, an absolute template kept, an empty
// template proceeds with "" (quirk Q1).
func TestPrepare_ArtifactPathTable(t *testing.T) {
	f := newFixture(t)
	if got := ResolveArtifactPath("", 7, f.root); got != "" {
		t.Fatalf("empty template ⇒ %q", got)
	}
	if got := ResolveArtifactPath("/abs/{cycle}/{cycle}.md", 7, f.root); got != "/abs/7/7.md" {
		t.Fatalf("absolute kept, every {cycle} expanded: %q", got)
	}
	cases := []struct {
		agent, template, want string
	}{
		{"scout-worker-codebase", ".evolve/runs/cycle-{cycle}/scout.md", filepath.Join(f.ws, "workers", "scout-worker-codebase.md")},
		{"scout", ".evolve/runs/cycle-{cycle}/scout.md", filepath.Join(f.root, ".evolve", "runs", "cycle-7", "scout.md")},
		{"scout", "", ""},
	}
	for _, c := range cases {
		deps := happyDeps(t)
		deps.Profile = func(string) (Profile, error) {
			p := scoutProfile
			p.OutputArtifact = c.template
			return p, nil
		}
		var got string
		deps.Adapter = AdapterFunc(func(_ context.Context, e AdapterEnv) (int, error) { got = e.ArtifactPath; return 0, nil })
		d, _ := observed(t, deps)
		req := f.request()
		req.Agent, req.Cycle = c.agent, 7
		out, err := d.Dispatch(context.Background(), req)
		if err != nil || got != c.want || out.ArtifactPath != c.want {
			t.Errorf("%s %q: %v got %q, want %q", c.agent, c.template, err, got, c.want)
		}
		if c.want == "" && out.Verdict != VerdictIntegrityFail {
			t.Errorf("the empty template proceeds to INTEGRITY_FAIL: %+v", out)
		}
	}
}

// Test 30 — a regular file where the artifact directory goes is
// `mkdir artifact dir: <err>` with step=artifact_dir.
func TestPrepare_MkdirErrorIsPrepareFailed(t *testing.T) {
	f := newFixture(t)
	if err := os.WriteFile(filepath.Join(f.root, ".evolve"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	d, r := observed(t, happyDeps(t))
	_, err := d.Dispatch(context.Background(), f.request())
	if err == nil || !strings.HasPrefix(err.Error(), "subagent/run: mkdir artifact dir: ") {
		t.Fatalf("%v", err)
	}
	if e := r.only(t, CodePrepareFailed); e.Fields["step"] != "artifact_dir" || !strings.HasPrefix(e.Reason, "mkdir artifact dir: ") || e.Fields["artifact"] == "" {
		t.Fatalf("%+v", e)
	}
}

// Test 31 — the token: the override verbatim without touching the entropy,
// a mint of 16 hex, the entropy error, the unprefixed short-read text.
func TestMintToken_LengthRandErrPartialReadAndOverride(t *testing.T) {
	tok, err := MintToken(func(b []byte) (int, error) {
		for i := range b {
			b[i] = byte(i)
		}
		return len(b), nil
	})
	if err != nil || len(tok) != ChallengeTokenBytes*2 || tok != "0001020304050607" {
		t.Fatalf("%q %v", tok, err)
	}
	if _, err := MintToken(func([]byte) (int, error) { return 0, errors.New("boom") }); err == nil || err.Error() != "boom" {
		t.Fatalf("the entropy error propagates bare: %v", err)
	}
	if _, err := MintToken(func(b []byte) (int, error) { return 4, nil }); err == nil || err.Error() != "rand returned 4 bytes, want 8" {
		t.Fatalf("the short read: %v", err)
	}

	f := newFixture(t)
	d, _ := observed(t, happyDeps(t), WithRand(func([]byte) (int, error) { t.Fatal("the override never touches the entropy"); return 0, nil }))
	req := f.request()
	req.ChallengeTokenOverride = "parent-tok-worker-x"
	if out, err := d.Dispatch(context.Background(), req); err != nil || out.ChallengeToken != "parent-tok-worker-x" {
		t.Fatalf("%v %+v", err, out)
	}
	d, r := observed(t, happyDeps(t), WithRand(func([]byte) (int, error) { return 0, errors.New("entropy depleted") }))
	if _, err := d.Dispatch(context.Background(), f.request()); err == nil || err.Error() != "subagent/run: token: entropy depleted" {
		t.Fatalf("%v", err)
	}
	if e := r.only(t, CodePrepareFailed); e.Fields["step"] != "token" || e.Reason != "token: entropy depleted" {
		t.Fatalf("%+v", e)
	}
}

// Test 32 — the git fallback: an error or an empty value stamps "unknown"
// and is ONE GIT_STATE_UNKNOWN; both present is silent.
func TestPrepare_GitStateUnknownSignalsOnce(t *testing.T) {
	f := newFixture(t)
	cases := []struct {
		name           string
		state          func(context.Context, string) (string, string, error)
		head, tree     string
		signal         bool
		reasonContains string
	}{
		{"error", func(context.Context, string) (string, string, error) { return "", "", errors.New("no git") }, "unknown", "unknown", true, "no git; empty head; empty tree diff"},
		{"empty diff", func(context.Context, string) (string, string, error) { return "h", "", nil }, "h", "unknown", true, "(empty tree diff)"},
		{"both present", func(context.Context, string) (string, string, error) { return "h", "t", nil }, "h", "t", false, ""},
	}
	for _, c := range cases {
		deps := happyDeps(t)
		deps.GitState = c.state
		d, r := observed(t, deps)
		req := f.request()
		req.LedgerPath = filepath.Join(t.TempDir(), "ledger.jsonl")
		if _, err := d.Dispatch(context.Background(), req); err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		line, _ := os.ReadFile(req.LedgerPath)
		if want := `"git_head":"` + c.head + `","tree_state_sha":"` + c.tree + `"`; !strings.Contains(string(line), want) {
			t.Errorf("%s: line %s lacks %s", c.name, line, want)
		}
		if !c.signal {
			if len(r.events) != 0 {
				t.Errorf("%s: silent, got %v", c.name, r.codes())
			}
			continue
		}
		e := r.only(t, CodeGitStateUnknown)
		if e.Fields["step"] != "provenance" || e.Fields["head"] != c.head || e.Fields["tree_state"] != c.tree || e.Fields["project_root"] != f.root ||
			!strings.Contains(e.Reason, c.reasonContains) || !strings.HasSuffix(e.Reason, `); ledger stamps "unknown"`) {
			t.Errorf("%s: %+v", c.name, e)
		}
	}
}

// Test 33 — a failing prompt reader is `read prompt: <err>` with step=prompt.
func TestPrepare_PromptReadError(t *testing.T) {
	f := newFixture(t)
	d, r := observed(t, happyDeps(t))
	req := f.request()
	req.Prompt = readerFunc(func([]byte) (int, error) { return 0, errors.New("pipe broken") })
	if _, err := d.Dispatch(context.Background(), req); err == nil || err.Error() != "subagent/run: read prompt: pipe broken" {
		t.Fatalf("%v", err)
	}
	if e := r.only(t, CodePrepareFailed); e.Fields["step"] != "prompt" || e.Reason != "read prompt: pipe broken" {
		t.Fatalf("%+v", e)
	}
}
