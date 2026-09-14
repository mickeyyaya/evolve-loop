package subagentrun

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// golden reads a pre-extraction golden with the fixture paths substituted.
func golden(t *testing.T, name string, pairs ...string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	for i := 0; i+1 < len(pairs); i += 2 {
		s = strings.ReplaceAll(s, pairs[i], pairs[i+1])
	}
	return s
}

// Test 34 — composePrompt and the framing constant reproduce the five goldens
// captured on the pre-extraction code, through the dispatcher and directly;
// the framing carries its load-bearing blocks.
func TestPrompt_GoldensReplayAndFramingConstant(t *testing.T) {
	f := newFixture(t)
	pairs := []string{"{WS}", f.ws, "{WORKTREE}", f.worktree, "{ROOT}", f.root}
	auditor := Profile{CLI: "claude", OutputArtifact: ".evolve/runs/cycle-{cycle}/audit.md", Overrides: scoutProfile.Overrides}
	cases := []struct {
		name    string
		agent   string
		cycle   int
		body    string
		profile Profile
		framing bool
		root    string
	}{
		{"prompt-plain.golden.txt", "scout", 5, "Do the thing.\n", scoutProfile, true, f.root},
		{"prompt-auditor.golden.txt", "auditor", 5, "audit body\n", auditor, true, f.root},
		{"prompt-auditor-off.golden.txt", "auditor", 5, "audit body\n", auditor, false, f.root},
		{"prompt-worker.golden.txt", "scout-worker-codebase", 3, "worker body\n", scoutProfile, true, ""},
		{"prompt-no-newline.golden.txt", "scout", 5, "no trailing nl", scoutProfile, true, f.root},
	}
	for _, c := range cases {
		deps := happyDeps(t)
		deps.Profile = func(string) (Profile, error) { return c.profile, nil }
		var got string
		deps.Adapter = AdapterFunc(func(ctx context.Context, e AdapterEnv) (int, error) {
			b, _ := os.ReadFile(e.PromptFile)
			got = string(b)
			return soundAdapter(t).Exec(ctx, e)
		})
		d, _ := observed(t, deps)
		req := f.request()
		req.Agent, req.Cycle, req.Prompt, req.AdversarialAudit, req.ProjectRoot, req.ProfilesDir = c.agent, c.cycle, strings.NewReader(c.body), c.framing, c.root, ""
		if _, err := d.Dispatch(context.Background(), req); err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		if want := golden(t, c.name, pairs...); got != want {
			t.Errorf("%s drifted:\n got %q\nwant %q", c.name, got, want)
		}
	}
	direct := composePrompt("scout", 5, f.ws, filepath.Join(f.root, ".evolve/runs/cycle-5/scout.md"), aaToken, "scout.json", "Do the thing.\n")
	if want := golden(t, "prompt-plain.golden.txt", pairs...); direct != want {
		t.Errorf("composePrompt directly:\n got %q\nwant %q", direct, want)
	}
	if got := composePrompt("x", 0, "/ws", "/a.md", "t", "x.json", "no trailing nl"); !strings.Contains(got, "no trailing nl\n--- END TASK PROMPT ---") {
		t.Errorf("the forced trailing newline: %q", got)
	}
	framing := adversarialAuditFraming()
	if framing != adversarialAuditFramingText || !strings.HasSuffix(golden(t, "prompt-auditor.golden.txt"), framing) {
		t.Fatal("the framing is the constant, appended verbatim")
	}
	for _, w := range []string{"ADVERSARIAL AUDIT MODE", "NO_DEFECT_FOUND", "0.85", "absence of evidence",
		"ADVERSARIAL INPUT TAXONOMY", "Implicit / innocuous-but-harmful", "EMPTY repo", "diversity collapse", "PER-CRITERION EVIDENCE REQUIREMENT"} {
		if !strings.Contains(framing, w) {
			t.Errorf("framing missing %q", w)
		}
	}
}
