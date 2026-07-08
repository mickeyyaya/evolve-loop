package content

import (
	"strings"
	"testing"
)

// The three interactive labs (inbox / gates / recovery) are part of the page
// contract: the real content file must carry enough material for each to render
// a meaningful interaction, and Validate must fail loudly when it doesn't.

func loadRealSite(t *testing.T) *Site {
	t.Helper()
	site, err := Load("../../shared/content.json")
	if err != nil {
		t.Fatalf("Load real content.json: %v", err)
	}
	return site
}

func TestLoad_RealContentHasInboxLab(t *testing.T) {
	site := loadRealSite(t)
	if site.InboxLab.Heading == "" {
		t.Error("InboxLab.Heading is empty")
	}
	if len(site.InboxLab.Presets) < 3 {
		t.Errorf("len(InboxLab.Presets) = %d, want >= 3", len(site.InboxLab.Presets))
	}
	if len(site.InboxLab.Seed) < 2 {
		t.Errorf("len(InboxLab.Seed) = %d, want >= 2", len(site.InboxLab.Seed))
	}
	for i, p := range site.InboxLab.Presets {
		if p.Weight <= 0 || p.Weight > 1 {
			t.Errorf("InboxLab.Presets[%d] (%q) weight %v outside (0,1]", i, p.ID, p.Weight)
		}
	}
}

func TestLoad_RealContentHasGateLab(t *testing.T) {
	site := loadRealSite(t)
	if site.GateLab.Heading == "" {
		t.Error("GateLab.Heading is empty")
	}
	if site.GateLab.Rule == "" {
		t.Error("GateLab.Rule is empty — the floor predicate is the point of the section")
	}
	if len(site.GateLab.Checks) < 4 {
		t.Errorf("len(GateLab.Checks) = %d, want >= 4", len(site.GateLab.Checks))
	}
	if len(site.GateLab.Scenarios) < 3 {
		t.Errorf("len(GateLab.Scenarios) = %d, want >= 3", len(site.GateLab.Scenarios))
	}
}

func TestLoad_RealContentHasRecoveryLab(t *testing.T) {
	site := loadRealSite(t)
	if site.RecoveryLab.Heading == "" {
		t.Error("RecoveryLab.Heading is empty")
	}
	if site.RecoveryLab.Proof == "" {
		t.Error("RecoveryLab.Proof is empty — the merged real-incident story is part of the section contract")
	}
	if len(site.RecoveryLab.Events) < 4 {
		t.Errorf("len(RecoveryLab.Events) = %d, want >= 4", len(site.RecoveryLab.Events))
	}
	for i, e := range site.RecoveryLab.Events {
		if len(e.Steps) < 2 {
			t.Errorf("RecoveryLab.Events[%d] (%q) needs >= 2 steps, got %d", i, e.Label, len(e.Steps))
		}
	}
}

// The try-it section is the conversion spine: versioned command, authentic
// post-paste terminal output, and the trust rail must all be present.
func TestLoad_RealContentHasTryIt(t *testing.T) {
	site := loadRealSite(t)
	if site.TryIt.Heading == "" {
		t.Error("TryIt.Heading is empty")
	}
	if !strings.Contains(site.TryIt.Command, "install.sh | sh") {
		t.Errorf("TryIt.Command = %q, want the install one-liner", site.TryIt.Command)
	}
	if len(site.TryIt.Terminal) < 4 {
		t.Errorf("len(TryIt.Terminal) = %d, want >= 4 (the what-you-will-see output)", len(site.TryIt.Terminal))
	}
	if len(site.TryIt.Trust) < 2 {
		t.Errorf("len(TryIt.Trust) = %d, want >= 2 (view-script + alternatives)", len(site.TryIt.Trust))
	}
	if len(site.TryIt.NextSteps) < 2 {
		t.Errorf("len(TryIt.NextSteps) = %d, want >= 2", len(site.TryIt.NextSteps))
	}
}

func TestValidate_ReportsMissingTryItCommand(t *testing.T) {
	site := loadRealSite(t)
	site.TryIt.Command = ""
	err := site.Validate()
	if err == nil || !strings.Contains(err.Error(), "tryIt.command") {
		t.Errorf("Validate = %v, want error naming tryIt.command", err)
	}
}

func TestValidate_ReportsMissingInboxLabHeading(t *testing.T) {
	site := loadRealSite(t)
	site.InboxLab.Heading = ""
	err := site.Validate()
	if err == nil || !strings.Contains(err.Error(), "inboxLab.heading") {
		t.Errorf("Validate = %v, want error naming inboxLab.heading", err)
	}
}

func TestValidate_ReportsTooFewGateChecks(t *testing.T) {
	site := loadRealSite(t)
	site.GateLab.Checks = site.GateLab.Checks[:1]
	err := site.Validate()
	if err == nil || !strings.Contains(err.Error(), "gateLab.checks") {
		t.Errorf("Validate = %v, want error naming gateLab.checks", err)
	}
}

// Every gate scenario key must reference a declared check — a typo'd scenario
// must fail the build, not render a toggle that silently does nothing.
func TestValidate_RejectsGateScenarioWithUnknownKey(t *testing.T) {
	site := loadRealSite(t)
	site.GateLab.Scenarios = append(site.GateLab.Scenarios, GateScenario{
		Label: "typo", On: []string{"no-such-check"}, Expect: "ship",
	})
	err := site.Validate()
	if err == nil || !strings.Contains(err.Error(), "no-such-check") {
		t.Errorf("Validate = %v, want error naming the unknown key", err)
	}
}

func TestValidate_RejectsGateScenarioWithBadExpect(t *testing.T) {
	site := loadRealSite(t)
	site.GateLab.Scenarios = append(site.GateLab.Scenarios, GateScenario{
		Label: "bad", On: nil, Expect: "maybe",
	})
	err := site.Validate()
	if err == nil || !strings.Contains(err.Error(), "expect") {
		t.Errorf("Validate = %v, want error naming the bad expect value", err)
	}
}

// Recovery events carry the real failure-adapter vocabulary; anything else is a
// content bug that would teach readers a verdict that doesn't exist.
func TestValidate_RejectsRecoveryEventWithUnknownVerdict(t *testing.T) {
	site := loadRealSite(t)
	site.RecoveryLab.Events = append(site.RecoveryLab.Events, RecoveryEvent{
		Label: "x", Code: "rc=1", Verdict: "SHRUG", Steps: []string{"a", "b"}, Outcome: "y",
	})
	err := site.Validate()
	if err == nil || !strings.Contains(err.Error(), "SHRUG") {
		t.Errorf("Validate = %v, want error naming the unknown verdict", err)
	}
}
