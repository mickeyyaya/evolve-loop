package triage

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/test/fixtures"
)

func TestTriageComposePrompt_BatchesFollowTheRankAndShowEachScoreAndTopFactor(t *testing.T) {
	root := t.TempDir()
	writeInboxItem(t, root, "a.json", `{"id":"heavier-security","weight":0.6,"priority_class":"security"}`)
	writeInboxItem(t, root, "b.json", `{"id":"lighter-correctness","weight":0.55,"priority_class":"correctness"}`)
	writeInboxItem(t, root, "c.json", `{"id":"blocker","weight":0.5,"priority_class":"correctness"}`)
	writeInboxItem(t, root, "d.json", `{"id":"waits-on-blocker","weight":0.9,"priority_class":"stability","deps":["blocker"]}`)
	out := hooks{}.ComposePrompt("BODY", core.PhaseRequest{ProjectRoot: root})
	first := strings.Index(out, "- batch 1 (")
	want := "- batch 1 (no shared signal): blocker\n  - blocker: score 0.4750, top factor base\n" +
		"- batch 2 (no shared signal): lighter-correctness\n  - lighter-correctness: score 0.4475, top factor base\n"
	if first < 0 || !strings.HasPrefix(out[first:], want) {
		t.Fatalf("the lighter item a waiting item depends on leads (the rank counts the whole queue), then the next score, not the heavier weight:\n%s", out)
	}
	if !strings.Contains(out, "  - heavier-security: score 0.2950, top factor base\n") {
		t.Errorf("every candidate shows its score:\n%s", out)
	}
	if !strings.Contains(out, "evolve inbox rank") {
		t.Errorf("the section names the rank it is ordered by:\n%s", out)
	}
}

func TestTriageComposePrompt_RanksWithTheEvolveDirsPolicy(t *testing.T) {
	root := t.TempDir()
	writeInboxItem(t, root, "a.json", `{"id":"heavier-security","weight":0.6,"priority_class":"security"}`)
	writeInboxItem(t, root, "b.json", `{"id":"lighter-correctness","weight":0.55,"priority_class":"correctness"}`)
	fixtures.MustWrite(t, filepath.Join(root, ".evolve", "policy.json"), `{"inbox_priority":{"class_order":["security","stability","correctness"]}}`)
	out := hooks{}.ComposePrompt("BODY", core.PhaseRequest{ProjectRoot: root})
	if !strings.Contains(out, "- batch 1 (no shared signal): heavier-security\n") {
		t.Errorf("the evolve dir's class_order puts security first:\n%s", out)
	}
}
