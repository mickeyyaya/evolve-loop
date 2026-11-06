package core

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

func TestWallReportFailText_NamesTheFailureTheWallAndTheLoginStep(t *testing.T) {
	t.Parallel()
	wall := Wall{}.Observe(FamilyEnd{CLI: "claude-tmux", Pattern: "auth_recheck"})
	got := WallReport{Cycle: 7, Phase: "audit", Wall: wall, Workspace: t.TempDir()}.FailText()
	for _, want := range []string{"cycle 7 phase audit", "claude needs a login", "the phase failed and the cycle ends in failure", "open claude and run /login"} {
		if !strings.Contains(got, want) {
			t.Errorf("FailText lacks %q: %s", want, got)
		}
	}
}

func TestWallReportFailEvent_CarriesTheFailTextTheCodeAndTheStopReason(t *testing.T) {
	t.Parallel()
	wall := Wall{}.Observe(FamilyEnd{CLI: "claude-tmux", Pattern: "auth_recheck"})
	ws := t.TempDir()
	e := WallReport{Cycle: 7, Phase: "audit", Wall: wall, Workspace: ws}.FailEvent("test.origin")
	if e.Reason != (WallReport{Cycle: 7, Phase: "audit", Wall: wall, Workspace: ws}.FailText()) {
		t.Errorf("reason %q is not the fail text", e.Reason)
	}
	if e.Kind != signalcenter.KindLoginRequired || e.Code != CodeLoginRequired || e.Cycle != 7 || e.Origin != "test.origin" {
		t.Errorf("event %+v", e)
	}
	if e.Fields["stop_reason"] != "login-required" || e.Fields["phase"] != "audit" {
		t.Errorf("fields %v", e.Fields)
	}
}
