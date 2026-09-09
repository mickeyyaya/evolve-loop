package router

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestPersonaTemplates_CarryTheHeaderLines — ADR-0099 slice 3: the words the
// kernel READS (HeaderGoalType, HeaderDeliverableKind, HeaderCycleSize) are the
// words the scout and triage personas WRITE. Two pins per header: the
// DISPATCHED persona names it as a directive (agents/evolve-scout.md's
// operational body — the reference file is stripped from dispatched prompts),
// and the output template carries it as a line-start header.
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
