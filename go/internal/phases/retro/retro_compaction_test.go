package retro

import (
	"context"
	"reflect"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

// retro_compaction_test.go — RED contract for cycle-421 task retro-phase-compaction-wiring.
//
// RED state (before Builder):
//   - retro.Config has no CompactPrompts bool field → reflect FieldByName returns zero Value → FAIL
//   - retro.go line 80 loads agent.Body raw (no StripOnDemandSections call) → bridge gets unstripped body → FAIL
//   - TestRetroPhase_CompactDisabled_BodyIdentical: current behavior is no-strip → pre-existing GREEN
//
// Builder must:
//   1. Add CompactPrompts bool to retro.Config.
//   2. In retro.go, after Agent() load, call StripOnDemandSections(agent.Body) when p.compactPrompts.
//   3. In cmd_cycle.go:358, pass CompactPrompts: cfg.CompactPrompts to retro.New.

// TestRetroPhase_ConfigHasCompactPrompts asserts that retro.Config has a CompactPrompts bool
// field, making retro the compaction-aware custom phase (mirrors BaseRunner-hosted phases).
// acs-predicate: config-check (inherent Config-struct field presence; no behavioral probe possible
// before the field exists — B1 registry-class fix mandates this structural assertion).
// RED: field absent → reflect FieldByName("CompactPrompts").IsValid() == false → t.Fatal.
func TestRetroPhase_ConfigHasCompactPrompts(t *testing.T) {
	ct := reflect.TypeOf(Config{})
	f, ok := ct.FieldByName("CompactPrompts")
	if !ok {
		t.Fatal("retro.Config has no CompactPrompts bool field — Builder must add: CompactPrompts bool")
	}
	if f.Type.Kind() != reflect.Bool {
		t.Fatalf("retro.Config.CompactPrompts type is %v, want bool", f.Type)
	}
}

// TestRetroPhase_CompactEnabled_StripsBody asserts that when CompactPrompts=true the retro
// phase strips the on-demand tail from the agent body before dispatching to the bridge.
// Uses reflect to set CompactPrompts (avoids compile error in RED state when field is absent).
// RED state (step 1): reflect finds no field → t.Fatal("retro.Config has no CompactPrompts field").
// RED state (step 2, after field added but before stripping wired): tail still present in prompt → t.Errorf.
func TestRetroPhase_CompactEnabled_StripsBody(t *testing.T) {
	tail := strings.Repeat("on-demand-tail-", 40) // ~600B distinctive tail
	body := "Operational content.\n\n## Reference Index\n\n" + tail + "\n"

	fb := &fakeBridge{
		writeArtifact: "# Retrospective\n## Root Cause\nx\n## Lessons\ny\n",
		writeLesson:   "id: x\n",
	}

	cfg := Config{Bridge: fb, Prompts: fakePromptsFS(body)}

	// Set CompactPrompts via reflect — compiles regardless of field existence.
	// If field absent: IsValid() == false → t.Fatal (step 1 RED).
	v := reflect.ValueOf(&cfg).Elem()
	f := v.FieldByName("CompactPrompts")
	if !f.IsValid() {
		t.Fatal("retro.Config has no CompactPrompts field — wiring not yet implemented")
	}
	f.SetBool(true)

	phase := New(cfg)
	ws := t.TempDir()
	_, _ = phase.Run(context.Background(), core.PhaseRequest{
		Cycle:       1,
		ProjectRoot: "/p",
		Workspace:   ws,
		Context:     map[string]string{"previous_verdict": core.VerdictFAIL},
	})

	// On-demand tail must NOT appear in the prompt sent to the bridge.
	// RED (step 2): retro.go doesn't call StripOnDemandSections → tail present → FAIL.
	// GREEN: stripping wired → tail stripped from agent.Body before composePrompt → absent.
	if strings.Contains(fb.gotReq.Prompt, tail) {
		t.Errorf("retro phase did not strip on-demand tail when CompactPrompts=true: tail still present in bridge prompt (len(prompt)=%d)", len(fb.gotReq.Prompt))
	}
}

// TestRetroPhase_CompactDisabled_BodyIdentical asserts that when CompactPrompts=false (default),
// the retro phase passes the raw body unchanged to the bridge — identity path.
// Pre-existing GREEN: current retro.go strips nothing; on-demand tail is present in bridge prompt.
// Regression guard: fires if Builder accidentally makes stripping unconditional (always-strip bug).
func TestRetroPhase_CompactDisabled_BodyIdentical(t *testing.T) {
	tail := strings.Repeat("on-demand-tail-", 40) // same tail as enabled test
	body := "Operational content.\n\n## Reference Index\n\n" + tail + "\n"

	fb := &fakeBridge{
		writeArtifact: "# Retrospective\n## Root Cause\nx\n## Lessons\ny\n",
		writeLesson:   "id: x\n",
	}

	// Config without explicit CompactPrompts — zero value (false).
	phase := New(Config{Bridge: fb, Prompts: fakePromptsFS(body)})
	ws := t.TempDir()
	_, _ = phase.Run(context.Background(), core.PhaseRequest{
		Cycle:       1,
		ProjectRoot: "/p",
		Workspace:   ws,
		Context:     map[string]string{"previous_verdict": core.VerdictFAIL},
	})

	// On-demand tail MUST be present in the prompt when CompactPrompts=false.
	// If tail is absent, stripping ran unconditionally — regression.
	if !strings.Contains(fb.gotReq.Prompt, tail) {
		t.Errorf("retro phase stripped on-demand tail when CompactPrompts=false (default) — must be byte-identical; identity path is broken")
	}
}
