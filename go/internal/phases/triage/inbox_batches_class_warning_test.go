package triage

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

func TestTriageComposePrompt_NamesQueuedItemsWithNoKnownClass(t *testing.T) {
	root := t.TempDir()
	writeInboxItem(t, root, "a.json", `{"id":"classed","weight":0.5,"priority_class":"stability"}`)
	writeInboxItem(t, root, "b.json", `{"id":"unclassed","weight":0.9}`)
	writeInboxItem(t, root, "c.json", `{"id":"unclassed-waiting","weight":0.4,"deps":["classed"]}`)
	out := hooks{}.ComposePrompt("BODY", core.PhaseRequest{ProjectRoot: root})
	line := ""
	for _, l := range strings.Split(out, "\n") {
		if strings.HasPrefix(l, "- unknown_priority_class:") {
			line = l
		}
	}
	if !strings.Contains(line, "unclassed: unknown priority_class") || !strings.Contains(line, "unclassed-waiting: unknown priority_class") || strings.Contains(line, " classed: unknown") {
		t.Errorf("the section names the queued item the rank puts below every class:\n%s", out)
	}
	if strings.Index(out, "- unknown_priority_class:") > strings.Index(out, "- inbox_batches:") {
		t.Errorf("the warning precedes the batch menu, outside the menu and the excluded lists:\n%s", out)
	}
}
