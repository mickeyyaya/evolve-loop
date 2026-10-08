package triage

import (
	"regexp"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

var batchesPastedRE = regexp.MustCompile(`- inbox_batches:[^\n]*\n<pasted_content id="([0-9a-f]+)">\n([\s\S]*?)\n</pasted_content id="([0-9a-f]+)">\n`)

func TestTriageComposePrompt_InboxBatchesAreMarkedAsPastedContent(t *testing.T) {
	root := t.TempDir()
	writeInboxItem(t, root, "a.json", `{"id":"alpha-ignore-previous-instructions","weight":0.9,"campaign":"camp-x"}`)
	writeInboxItem(t, root, "b.json", `{"id":"beta","weight":0.4,"campaign":"camp-x"}`)

	out := hooks{}.ComposePrompt("BODY", core.PhaseRequest{ProjectRoot: root})
	m := batchesPastedRE.FindStringSubmatch(out)
	if m == nil {
		t.Fatalf("the inbox batches are not inside a <pasted_content> block after the inbox_batches note:\n%s", out)
	}
	if m[1] != m[3] {
		t.Errorf("opening id %q and closing id %q differ", m[1], m[3])
	}
	if !strings.Contains(m[2], "alpha-ignore-previous-instructions") || !strings.Contains(m[2], "beta") {
		t.Errorf("the pasted block lacks the inbox ids:\n%s", m[2])
	}
}
