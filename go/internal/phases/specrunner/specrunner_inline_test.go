package specrunner

import (
	"context"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/mickeyyaya/evolveloop/go/internal/core"
	"github.com/mickeyyaya/evolveloop/go/internal/phasespec"
	"github.com/mickeyyaya/evolveloop/go/internal/prompts"
)

// TestRun_InlinePrompt_NoDiskFile proves a minted phase can ship its prompt
// in-band via Config.PromptBody: with an EMPTY prompts loader (no
// agents/*.md), the phase still dispatches because the runner uses the
// inline body instead of reading from disk.
func TestRun_InlinePrompt_NoDiskFile(t *testing.T) {
	spec := phasespec.PhaseSpec{
		Name:     "minted-reviewer",
		Classify: &phasespec.ClassifyRules{RequireSections: []string{"## Notes"}, VerdictOnPass: core.VerdictPASS},
	}
	fb := &fakeBridge{writeArtifact: "# minted\n## Notes\n- ok\n"}
	emptyLoader := prompts.NewFromFS(fstest.MapFS{}) // any disk read would error
	phase := New(spec, Config{Bridge: fb, Prompts: emptyLoader, PromptBody: "INLINE PERSONA"})

	resp, err := phase.Run(context.Background(), core.PhaseRequest{Cycle: 1, ProjectRoot: "/tmp/p", Workspace: t.TempDir()})
	if err != nil {
		t.Fatalf("inline-prompt spec phase must not read disk; got %v", err)
	}
	if resp.Verdict != core.VerdictPASS {
		t.Errorf("Verdict=%q, want PASS", resp.Verdict)
	}
	if !strings.Contains(fb.gotReq.Prompt, "INLINE PERSONA") {
		t.Errorf("dispatched prompt missing inline body; got:\n%s", fb.gotReq.Prompt)
	}
}

// TestRun_NoInline_LoadsFromDisk proves the default path is unchanged: with
// PromptBody empty, the phase loads agents/<agent>.md exactly as before.
func TestRun_NoInline_LoadsFromDisk(t *testing.T) {
	spec := phasespec.PhaseSpec{
		Name:     "disk-phase",
		Classify: &phasespec.ClassifyRules{VerdictOnPass: core.VerdictPASS},
	}
	fb := &fakeBridge{writeArtifact: "# ok\n"}
	phase := New(spec, Config{Bridge: fb, Prompts: fakePrompts("evolve-disk-phase", "DISK PERSONA")})

	resp, err := phase.Run(context.Background(), core.PhaseRequest{Cycle: 1, ProjectRoot: "/tmp/p", Workspace: t.TempDir()})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if resp.Verdict != core.VerdictPASS {
		t.Errorf("Verdict=%q, want PASS", resp.Verdict)
	}
	if !strings.Contains(fb.gotReq.Prompt, "DISK PERSONA") {
		t.Errorf("dispatched prompt missing on-disk body; got:\n%s", fb.gotReq.Prompt)
	}
}
