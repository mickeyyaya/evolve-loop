package retro

// retro_compaction_amplified_test.go — Adversarial amplification for cycle-421
// retro-phase-compaction-wiring (behavioral gaps in retro package).
//
// Probes gaps NOT covered by retro_compaction_test.go (3 tests: ConfigHasCompactPrompts,
// CompactEnabled_StripsBody, CompactDisabled_BodyIdentical):
//
//   - CompactPrompts=true must NOT break the PASS verdict short-circuit.
//     Existing TestRun_PreviousPASS_SKIPPEDWithoutBridgeCall uses default Config
//     (CompactPrompts=false). This amplifier probes whether wiring the flag accidentally
//     removes the guard that skips bridge calls for PASS verdicts.
//   - CompactEnabled produces a strictly shorter prompt than CompactDisabled.
//     C421_005 / TestRetroPhase_CompactEnabled_StripsBody verifies the tail is ABSENT
//     (content check). This test adds a SIZE assertion: if the tail is stripped, the
//     total prompt length must be strictly less (guards against "absent in content
//     search but still injected elsewhere" bugs).

import (
	"context"
	"reflect"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

// TestRetroPhase_CompactEnabled_PreviousPASS_StillSkips asserts that setting
// CompactPrompts=true does not break the PASS-verdict short-circuit: retro still
// returns SKIPPED without calling the bridge when previous_verdict=PASS.
//
// Amplification angle: the existing TestRun_PreviousPASS_SKIPPEDWithoutBridgeCall
// uses a default Config (CompactPrompts=false zero-value). If the compaction wiring
// accidentally changed the skip predicate (e.g., a wrong `if p.compactPrompts { ... }`
// placement that gates the verdict check), the PASS short-circuit would break with
// CompactPrompts=true while the existing test still passes.
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

// TestRetroPhase_CompactEnabled_PromptSizeStrictlySmaller asserts that when
// CompactPrompts=true, the prompt sent to the bridge is strictly smaller (in bytes)
// than when CompactPrompts=false, given an agent body containing an on-demand tail.
//
// Amplification angle: C421_005 / TestRetroPhase_CompactEnabled_StripsBody performs a
// content check (tail string absent). This test adds a SIZE check: total prompt length
// must be strictly less. These are complementary guards — content-absent is necessary
// but not sufficient; a bug could strip the exact tail string while injecting it via
// a different code path, passing the content check but not the size check.
func TestRetroPhase_CompactEnabled_PromptSizeStrictlySmaller(t *testing.T) {
	tail := strings.Repeat("on-demand-tail-", 40) // ~600B distinctive tail
	body := "Operational content.\n\n## Reference Index\n\n" + tail + "\n"
	writeArtifact := "# Retrospective\n## Root Cause\nx\n## Lessons\ny\n"
	writeLesson := "id: x\n"

	// Run with compact=false (default zero value).
	fbFull := &fakeBridge{writeArtifact: writeArtifact, writeLesson: writeLesson}
	phaseDefault := New(Config{Bridge: fbFull, Prompts: fakePromptsFS(body)})
	_, _ = phaseDefault.Run(context.Background(), core.PhaseRequest{
		Cycle:       1,
		ProjectRoot: "/p",
		Workspace:   t.TempDir(),
		Context:     map[string]string{"previous_verdict": core.VerdictFAIL},
	})
	fullLen := len(fbFull.gotReq.Prompt)

	// Run with compact=true via reflect.
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
