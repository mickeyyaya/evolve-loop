package core

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

func TestWallReportPauseText_PinsEachShape(t *testing.T) {
	t.Parallel()
	login := FamilyEnd{CLI: "claude", Pattern: "auth_recheck", Evidence: "ws/claude-report.json"}
	cases := []struct {
		name string
		wall Wall
		want string
	}{
		{"all login with evidence", Wall{}.Observe(login),
			"cycle 7 phase audit: claude needs a login. Evidence: ws/claude-report.json. Impact: the cycle is paused; claims released. Time alone does not clear this. Next: open claude and run /login. Then evolve loop --resume."},
		{"mixed does not claim time is useless", Wall{}.Observe(login).Observe(FamilyEnd{CLI: "agy", Pattern: "rate_limit"}),
			"cycle 7 phase audit: claude needs a login; agy is on quota, reset unknown. Evidence: ws/claude-report.json. Impact: the cycle is paused; claims released. Next: open claude and run /login, or wait for agy, reset unknown. Then evolve loop --resume."},
		{"a row with no record stops the sentence", Wall{}.Observe(login).Observe(FamilyEnd{CLI: "codex"}),
			"cycle 7 phase audit: claude needs a login; codex ended with no recorded cause. Evidence: ws/claude-report.json. Impact: the cycle is paused; claims released. Next: open claude and run /login. Then evolve loop --resume."},
	}
	for _, tc := range cases {
		if got := (WallReport{Cycle: 7, Phase: "audit", Wall: tc.wall, Workspace: t.TempDir()}).PauseText("claims released"); got != tc.want {
			t.Errorf("%s:\n got %q\nwant %q", tc.name, got, tc.want)
		}
	}
}

func TestWallPredicates(t *testing.T) {
	t.Parallel()
	login := Wall{}.Observe(FamilyEnd{CLI: "claude", Pattern: "auth_recheck"})
	quota := Wall{}.Observe(FamilyEnd{CLI: "agy", Pattern: "rate_limit"})
	if !login.HasLogin() || login.HasQuota() || !login.AllLogin() {
		t.Errorf("login wall: login=%v quota=%v all=%v", login.HasLogin(), login.HasQuota(), login.AllLogin())
	}
	if quota.HasLogin() || !quota.HasQuota() || quota.AllLogin() {
		t.Errorf("quota wall: login=%v quota=%v all=%v", quota.HasLogin(), quota.HasQuota(), quota.AllLogin())
	}
	if (Wall{}).AllLogin() {
		t.Error("an empty wall holds no login")
	}
}

func TestWallReportPauseEvent_CarriesTheModuleCodeAndStopReason(t *testing.T) {
	t.Parallel()
	wall := Wall{}.Observe(FamilyEnd{CLI: "claude", Pattern: "auth_recheck"})
	e := WallReport{Cycle: 7, Phase: "audit", Wall: wall, Workspace: t.TempDir()}.PauseEvent("claims released", "origin")
	if e.Kind != signalcenter.KindLoginRequired || e.Code != CodeLoginRequired || e.Module != signalcenter.ModuleBridge ||
		e.Severity != signalcenter.SeverityWarn || e.Cycle != 7 || e.Phase != "audit" || e.Origin != "origin" ||
		e.Fields["stop_reason"] != "login-required" || e.Fields["phase"] != "audit" {
		t.Errorf("event %+v", e)
	}
}
