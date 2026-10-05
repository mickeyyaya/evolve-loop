package bridge

import (
	"os"
	"strings"
	"testing"
)

const selfMatchPaneDepth = 64

func requireNoRuleFiresAtAnyPaneBottom(t *testing.T, rules []ManifestPrompt, files []string) {
	t.Helper()
	for _, p := range rules {
		if p.TailLines <= 0 || p.TailLines > selfMatchPaneDepth {
			t.Fatalf("rule %s has tail_lines %d; a %d-line pane must hold every rule's whole tail, or this guard sees less than the rule does", p.Name, p.TailLines, selfMatchPaneDepth)
		}
	}
	for _, f := range files {
		body, err := os.ReadFile(f)
		if err != nil {
			t.Fatalf("read %s: %v (this guard is worthless if it cannot read its own sources)", f, err)
		}
		lines := strings.Split(string(body), "\n")
		for end := range lines {
			pane := strings.Join(lines[max(0, end+1-selfMatchPaneDepth):end+1], "\n")
			if a, rc := decideAutoRespond(pane, rules, map[string]int{}, false); rc != 0 || a != "noop" {
				t.Fatalf("(%q, rc %d) on a pane whose bottom line is %s:%d: an agent that prints this tracked file can stop its output there, so its text must never take the live dialog's shape", a, rc, f, end+1)
			}
		}
	}
}
