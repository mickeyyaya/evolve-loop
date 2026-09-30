package retro

import (
	"context"
	"reflect"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

func TestRetroPhase_CompactEnabled_PreviousPASS_StillSkips(t *testing.T) {
	fb := &fakeBridge{}
	cfg := Config{Bridge: fb, Prompts: fakePromptsFS("body")}

	v := reflect.ValueOf(&cfg).Elem()
	f := v.FieldByName("CompactPrompts")
	if !f.IsValid() {
		t.Skip("CompactPrompts field absent — covered by C421_004; skipping interaction test")
	}
	f.SetBool(true)

	phase := New(cfg)
	resp, err := phase.Run(context.Background(), core.PhaseRequest{
		Cycle: 1, ProjectRoot: "/p", Workspace: t.TempDir(),
		Context: map[string]string{"previous_verdict": core.VerdictPASS},
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if resp.Verdict != core.VerdictSKIPPED {
		t.Errorf("Verdict=%q with CompactPrompts=true and previous=PASS; want SKIPPED — compaction flag must not alter the short-circuit", resp.Verdict)
	}
	if fb.gotReq.Cycle != 0 {
		t.Error("bridge.Launch was called when previous=PASS and CompactPrompts=true — PASS short-circuit is broken by compaction flag")
	}
}

func TestRetroPhase_CompactEnabled_PromptSizeStrictlySmaller(t *testing.T) {
	tail := strings.Repeat("on-demand-tail-", 40)
	body := "Operational content.\n\n## Reference Index\n\n" + tail + "\n"
	writeArtifact := "# Retrospective\n## Root Cause\nx\n## Lessons\ny\n"
	writeLesson := "id: x\n"

	fbFull := &fakeBridge{writeArtifact: writeArtifact, writeLesson: writeLesson}
	phaseDefault := New(Config{Bridge: fbFull, Prompts: fakePromptsFS(body)})
	_, _ = phaseDefault.Run(context.Background(), core.PhaseRequest{
		Cycle:       1,
		ProjectRoot: "/p",
		Workspace:   t.TempDir(),
		Context:     map[string]string{"previous_verdict": core.VerdictFAIL},
	})
	fullLen := len(fbFull.gotReq.Prompt)

	fbCompact := &fakeBridge{writeArtifact: writeArtifact, writeLesson: writeLesson}
	cfg := Config{Bridge: fbCompact, Prompts: fakePromptsFS(body)}
	v := reflect.ValueOf(&cfg).Elem()
	f := v.FieldByName("CompactPrompts")
	if !f.IsValid() {
		t.Skip("CompactPrompts field absent — covered by C421_004; skipping size comparison")
	}
	f.SetBool(true)
	phaseCompact := New(cfg)
	_, _ = phaseCompact.Run(context.Background(), core.PhaseRequest{
		Cycle:       1,
		ProjectRoot: "/p",
		Workspace:   t.TempDir(),
		Context:     map[string]string{"previous_verdict": core.VerdictFAIL},
	})
	compactLen := len(fbCompact.gotReq.Prompt)

	if fullLen == 0 {
		t.Fatal("bridge was not called with default config — test infrastructure broken")
	}
	if compactLen >= fullLen {
		t.Errorf("compact prompt (%d bytes) not strictly smaller than full prompt (%d bytes) — stripping must reduce total prompt size when on-demand tail is present", compactLen, fullLen)
	}
}
