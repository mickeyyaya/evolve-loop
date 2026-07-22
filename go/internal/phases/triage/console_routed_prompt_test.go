package triage

// console_routed_prompt_test.go — RED contract for ADR-0074 I1 at the
// visibility seam: a console-routed inbox item must not be OFFERED to lane
// triage at all (an LLM cannot mis-pick what it never sees), and the exclusion
// must be loud in the prompt so triage knows operator-owned work exists.

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/guards"
)

// Explicitly routed items disappear from the batch section and surface only in
// the console_routed exclusion note.
func TestTriageComposePrompt_ExcludesConsoleRoutedItems(t *testing.T) {
	root := t.TempDir()
	writeInboxItem(t, root, "a.json", `{"id":"lane-work","weight":0.9,"campaign":"camp-x"}`)
	writeInboxItem(t, root, "b.json", `{"id":"operator-work","weight":0.96,"route":"console-manual","campaign":"camp-x"}`)

	out := hooks{}.ComposePrompt("BODY", core.PhaseRequest{ProjectRoot: root})
	if !strings.Contains(out, "lane-work") {
		t.Fatalf("dispatchable item missing from prompt:\n%s", out)
	}
	if !strings.Contains(out, "console_routed_excluded") {
		t.Fatalf("prompt must carry a loud console_routed_excluded note:\n%s", out)
	}
	batchSection := out[strings.Index(out, "inbox_batches"):]
	if noteIdx := strings.Index(batchSection, "console_routed_excluded"); noteIdx >= 0 {
		if strings.Contains(batchSection[:noteIdx], "operator-work") {
			t.Errorf("console-routed id must not appear inside the selectable batch listing:\n%s", out)
		}
	}
}

// Derived exclusion uses the REAL guards predicate — pins that a protected
// fix surface (role.go was cycle-1036's burn) auto-routes without any route
// field. Guards-manifest membership is asserted first so a manifest change
// surfaces here as a loud pin move, not a silent pass.
func TestTriageComposePrompt_ProtectedFixSurfaceAutoExcluded(t *testing.T) {
	if !guards.IsProtectedSurface("go/internal/guards/role.go") {
		t.Fatal("pin moved: go/internal/guards/role.go no longer on ProtectedSurfaceManifest — update this test AND the routing rationale")
	}
	root := t.TempDir()
	writeInboxItem(t, root, "a.json", `{"id":"lane-work","weight":0.9}`)
	writeInboxItem(t, root, "b.json", `{"id":"role-gate-fix","weight":0.92,"files":["go/internal/guards/role.go (allowance)"]}`)

	out := hooks{}.ComposePrompt("BODY", core.PhaseRequest{ProjectRoot: root})
	if !strings.Contains(out, "console_routed_excluded") || !strings.Contains(out, "role-gate-fix") {
		t.Fatalf("protected-surface item must be excluded loudly (note naming it):\n%s", out)
	}
}

// The empty-inbox byte-identity pin must survive the partition wiring.
func TestTriageComposePrompt_PartitionKeepsEmptyInboxByteIdentity(t *testing.T) {
	root := t.TempDir()
	req := core.PhaseRequest{ProjectRoot: root}
	a := hooks{}.ComposePrompt("BODY", req)
	if strings.Contains(a, "console_routed_excluded") {
		t.Errorf("empty inbox must not render an exclusion note:\n%s", a)
	}
}
