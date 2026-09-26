package bridge

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestAutoResponderTick_BusyGateViaCenter_SuppressesEscalate(t *testing.T) {
	busyPane := "Which absolute path should I write the deliverable to?\n" +
		"⏵⏵ bypass permissions on (shift+tab to cycle) · esc to interrupt\n"

	ws := t.TempDir()
	tmux := &fakeTmux{paneSeq: []string{busyPane}}
	deps := Deps{Tmux: tmux, Sleep: func(time.Duration) {}, LookupEnv: mapLookup(nil)}.withDefaults()
	ar := newAutoResponder("claude-tmux", ws, deps, false, 0)
	ar.prompts = escalatePrompt

	_, rc := ar.tick(context.Background(), "s")
	if rc != 0 {
		t.Errorf("tick() on busy pane with escalate match: rc = %d, want 0 (busy must gate the escalate policy, not fire it)", rc)
	}
}

func TestAutoResponderTick_IdleGateViaCenter_Escalates(t *testing.T) {
	idlePane := "Which absolute path should I write the deliverable to?\n" +
		"⏺ answer complete\n"

	ws := t.TempDir()
	tmux := &fakeTmux{paneSeq: []string{idlePane}}
	deps := Deps{Tmux: tmux, Sleep: func(time.Duration) {}, LookupEnv: mapLookup(nil)}.withDefaults()
	ar := newAutoResponder("claude-tmux", ws, deps, false, 0)
	ar.prompts = escalatePrompt

	_, rc := ar.tick(context.Background(), "s")
	if rc != 85 {
		t.Errorf("tick() on idle pane with escalate match: rc = %d, want 85 (idle must NOT be gated)", rc)
	}
}

// autorespondTickRegionSource returns tick's busy-gate region of autorespond.go, anchored on two unique code lines.
func autorespondTickRegionSource(t *testing.T) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("could not resolve this test file's path via runtime.Caller")
	}
	src, err := os.ReadFile(filepath.Join(filepath.Dir(thisFile), "autorespond.go"))
	if err != nil {
		t.Fatalf("read autorespond.go: %v", err)
	}
	lines := strings.Split(string(src), "\n")

	start, end := -1, -1
	for i, ln := range lines {
		if start == -1 && strings.Contains(ln, "prevCounts := make(map[string]int, len(ar.counts))") {
			start = i
		}
		if start != -1 && strings.Contains(ln, "action, rc := decideAutoRespond(scanPane, ar.prompts, ar.counts, paneBusy)") {
			end = i
			break
		}
	}
	if start == -1 || end == -1 {
		t.Fatal("could not locate the tick() busy-gate region markers in autorespond.go")
	}
	return strings.Join(lines[start:end+1], "\n")
}

func TestAutoResponderTick_NoDirectChromeParse(t *testing.T) {
	region := autorespondTickRegionSource(t)
	if strings.Contains(region, "panestream.PaneBusy(") {
		t.Error("tick() busy-gate region still calls panestream.PaneBusy( directly — must read it via panestream.LivenessCenter.BusyOf instead")
	}
}
