package llmcalls

import (
	"os"
	"strings"
	"testing"
)

func TestAppendTurnsEdge_NilToolsWritesOneLine(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()
	if err := AppendTurns(ws, "call-nil", []Turn{{Index: 1, TS: "2026-10-27T09:00:00Z"}}); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(TurnsPath(ws))
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Count(string(raw), "\n"); got != 1 {
		t.Fatalf("want 1 line, got %d: %q", got, raw)
	}
	if !strings.Contains(string(raw), "call-nil") {
		t.Fatalf("call id missing: %q", raw)
	}
}

func TestAppendTurnsEdge_TwoCallsKeepTheirOwnCallIDs(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()
	for _, id := range []string{"call-a", "call-b"} {
		if err := AppendTurns(ws, id, []Turn{{Index: 1}, {Index: 2}}); err != nil {
			t.Fatal(err)
		}
	}
	raw, err := os.ReadFile(TurnsPath(ws))
	if err != nil {
		t.Fatal(err)
	}
	s := string(raw)
	if strings.Count(s, "call-a") != 2 || strings.Count(s, "call-b") != 2 {
		t.Fatalf("each call must own 2 lines: %q", s)
	}
}
