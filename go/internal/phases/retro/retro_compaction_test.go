package retro

import (
	"context"
	"reflect"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

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

func TestRetroPhase_CompactEnabled_StripsBody(t *testing.T) {
	tail := strings.Repeat("on-demand-tail-", 40)
	body := "Operational content.\n\n## Reference Index\n\n" + tail + "\n"

	fb := &fakeBridge{
		writeArtifact: "# Retrospective\n## Root Cause\nx\n## Lessons\ny\n",
		writeLesson:   "id: x\n",
	}

	cfg := Config{Bridge: fb, Prompts: fakePromptsFS(body)}

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

	if strings.Contains(fb.gotReq.Prompt, tail) {
		t.Errorf("retro phase did not strip on-demand tail when CompactPrompts=true: tail still present in bridge prompt (len(prompt)=%d)", len(fb.gotReq.Prompt))
	}
}

func TestRetroPhase_CompactDisabled_BodyIdentical(t *testing.T) {
	tail := strings.Repeat("on-demand-tail-", 40)
	body := "Operational content.\n\n## Reference Index\n\n" + tail + "\n"

	fb := &fakeBridge{
		writeArtifact: "# Retrospective\n## Root Cause\nx\n## Lessons\ny\n",
		writeLesson:   "id: x\n",
	}

	phase := New(Config{Bridge: fb, Prompts: fakePromptsFS(body)})
	ws := t.TempDir()
	_, _ = phase.Run(context.Background(), core.PhaseRequest{
		Cycle:       1,
		ProjectRoot: "/p",
		Workspace:   ws,
		Context:     map[string]string{"previous_verdict": core.VerdictFAIL},
	})

	if !strings.Contains(fb.gotReq.Prompt, tail) {
		t.Errorf("retro phase stripped on-demand tail when CompactPrompts=false (default) — must be byte-identical; identity path is broken")
	}
}
