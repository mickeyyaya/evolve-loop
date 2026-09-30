package triage

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/guards"
)

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

// Guards-manifest membership is asserted first so a manifest change surfaces
// as a loud pin move, not a silent pass.
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

func TestTriageComposePrompt_PartitionKeepsEmptyInboxByteIdentity(t *testing.T) {
	root := t.TempDir()
	req := core.PhaseRequest{ProjectRoot: root}
	a := hooks{}.ComposePrompt("BODY", req)
	if strings.Contains(a, "console_routed_excluded") {
		t.Errorf("empty inbox must not render an exclusion note:\n%s", a)
	}
}

func TestTriageComposePrompt_HoldsBackItemsWaitingOnADependency(t *testing.T) {
	root := t.TempDir()
	writeInboxItem(t, root, "a.json", `{"id":"lane-work","weight":0.9}`)
	writeInboxItem(t, root, "b.json", `{"id":"operator-work","weight":0.96,"route":"console-manual"}`)
	writeInboxItem(t, root, "c.json", `{"id":"blocked-work","weight":0.95,"deps":["operator-work"]}`)

	out := hooks{}.ComposePrompt("BODY", core.PhaseRequest{ProjectRoot: root})
	note := strings.Index(out, "dependency_blocked")
	if note < 0 || !strings.Contains(out[note:], "blocked-work: deps unmet: needs operator-work") {
		t.Fatalf("an item waiting on a dependency must be listed loudly as not selectable:\n%s", out)
	}
	if batches := out[strings.Index(out, "inbox_batches"):note]; strings.Contains(batches, "blocked-work") {
		t.Errorf("an item waiting on a dependency must not appear inside the selectable batch listing:\n%s", out)
	}
}
