package specrunner

import (
	"context"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasespec"
	"github.com/mickeyyaya/evolve-loop/go/internal/prompts"
)

func TestRun_InlinePrompt_NoDiskFile(t *testing.T) {
	spec := phasespec.PhaseSpec{
		Name:     "minted-reviewer",
		Classify: &phasespec.ClassifyRules{RequireSections: []string{"## Notes"}, VerdictOnPass: core.VerdictPASS},
	}
	fb := &fakeBridge{writeArtifact: "# minted\n## Notes\n- ok\n"}
	emptyLoader := prompts.NewFromFS(fstest.MapFS{})
	phase := New(spec, Config{Bridge: fb, Prompts: emptyLoader, PromptBody: "INLINE PERSONA"})

	resp, err := phase.Run(context.Background(), core.PhaseRequest{Cycle: 1, ProjectRoot: t.TempDir(), Workspace: t.TempDir()})
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

func TestRun_NoInline_LoadsFromDisk(t *testing.T) {
	spec := phasespec.PhaseSpec{
		Name:     "disk-phase",
		Classify: &phasespec.ClassifyRules{VerdictOnPass: core.VerdictPASS},
	}
	fb := &fakeBridge{writeArtifact: "# ok\n"}
	phase := New(spec, Config{Bridge: fb, Prompts: fakePrompts("evolve-disk-phase", "DISK PERSONA")})

	resp, err := phase.Run(context.Background(), core.PhaseRequest{Cycle: 1, ProjectRoot: t.TempDir(), Workspace: t.TempDir()})
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
