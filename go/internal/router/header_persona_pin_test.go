package router

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The reference file is stripped from dispatched prompts, so the dispatched scout persona must name each header too.
func TestPersonaTemplates_CarryTheHeaderLines(t *testing.T) {
	agents := filepath.Join("..", "..", "..", "agents")
	read := func(name string) string {
		t.Helper()
		b, err := os.ReadFile(filepath.Join(agents, name))
		if err != nil {
			t.Fatal(err)
		}
		return string(b)
	}
	scout, reference, triage := read("evolve-scout.md"), read("evolve-scout-reference.md"), read("evolve-triage.md")
	for _, h := range []string{HeaderGoalType, HeaderDeliverableKind} {
		if !strings.Contains(scout, "`"+h) {
			t.Errorf("dispatched scout persona lacks the %q header directive", h)
		}
		if !strings.Contains(reference, "\n"+h) {
			t.Errorf("scout output template lacks a %q header line", h)
		}
	}
	for _, h := range []string{HeaderCycleSize, HeaderDeliverableKind} {
		if !strings.Contains(triage, "\n"+h) {
			t.Errorf("triage persona lacks a %q header line", h)
		}
	}
}
