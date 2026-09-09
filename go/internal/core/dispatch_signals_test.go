package core

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
)

func writeDomainJSON(t *testing.T, root, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(root, ".evolve"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".evolve", "domain.json"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func writeReport(t *testing.T, ws, name, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(ws, name), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestDispatchSignals — ADR-0099 slice 3: core is the ONE digester feeding the
// dispatch-time selectors (the skill-overlay `when` rule). The kind is the
// DECLARED one when a report spoke (triage > scout), else the project default
// (.evolve/domain.json), else code — so a document-domain project's very
// first scout dispatch (no report exists yet) already carries the solution
// persona, and a code project stays byte-identical.
func TestDispatchSignals(t *testing.T) {
	scoutDoc := "<!-- challenge-token: x -->\ngoal_type: partnership-deal\ndeliverable_kind: document\n\n## Selected Tasks\n"
	triageCode := "<!-- challenge-token: x -->\ncycle_size_estimate: small\ndeliverable_kind: code\n\n## top_n\n- a\n"
	scoutNoKind := "<!-- challenge-token: x -->\ngoal_type: partnership-deal\n\n## Selected Tasks\n"
	for _, tc := range []struct {
		name         string
		phase        Phase
		domain       string
		scout        string
		triage       string
		wantKind     string
		wantGoalType string
	}{
		{name: "scout dispatch, no reports, no domain ⇒ code", phase: PhaseScout, wantKind: config.DeliverableKindCode},
		{name: "scout dispatch, writing domain ⇒ the project default", phase: PhaseScout, domain: `{"domain":"writing"}`, wantKind: config.DeliverableKindDocument},
		{name: "triage dispatch, scout declared no kind, writing domain ⇒ the project default", phase: PhaseTriage, domain: `{"domain":"writing"}`, scout: scoutNoKind, wantKind: config.DeliverableKindDocument, wantGoalType: "partnership-deal"},
		{name: "build dispatch, nothing declared, writing domain ⇒ code (what the floor reads)", phase: PhaseBuild, domain: `{"domain":"writing"}`, scout: scoutNoKind, wantKind: config.DeliverableKindCode, wantGoalType: "partnership-deal"},
		{name: "scout declaration beats the domain default", phase: PhaseTriage, domain: `{"domain":"coding"}`, scout: scoutDoc, wantKind: config.DeliverableKindDocument, wantGoalType: "partnership-deal"},
		{name: "triage declaration beats scout and domain", phase: PhaseBuild, domain: `{"domain":"writing"}`, scout: scoutDoc, triage: triageCode, wantKind: config.DeliverableKindCode, wantGoalType: "partnership-deal"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root, ws := t.TempDir(), t.TempDir()
			if tc.domain != "" {
				writeDomainJSON(t, root, tc.domain)
			}
			if tc.scout != "" {
				writeReport(t, ws, "scout-report.md", tc.scout)
			}
			if tc.triage != "" {
				writeReport(t, ws, "triage-report.md", tc.triage)
			}
			got := dispatchSignals(tc.phase, ws, root)
			if got[config.SignalDeliverableKind] != tc.wantKind {
				t.Errorf("%s = %q, want %q (all: %v)", config.SignalDeliverableKind, got[config.SignalDeliverableKind], tc.wantKind, got)
			}
			if gt, present := got[config.SignalGoalType]; gt != tc.wantGoalType || (present && tc.wantGoalType == "") {
				t.Errorf("%s = %q (present=%v), want %q", config.SignalGoalType, gt, present, tc.wantGoalType)
			}
		})
	}
}

// TestDispatchSignals_DegradedDigestIsLoud: a report that exists but cannot be
// read (here: a directory in its place) is reported on stderr with the
// kernel's own reason, and the projection stays on the conservative side —
// even on a writing-domain project's own triage dispatch, the default never
// reclassifies a torn cycle.
func TestDispatchSignals_DegradedDigestIsLoud(t *testing.T) {
	root, ws := t.TempDir(), t.TempDir()
	writeDomainJSON(t, root, `{"domain":"writing"}`)
	if err := os.Mkdir(filepath.Join(ws, "triage-report.md"), 0o755); err != nil {
		t.Fatal(err)
	}
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	orig := os.Stderr
	os.Stderr = w
	got := dispatchSignals(PhaseTriage, ws, root)
	os.Stderr = orig
	_ = w.Close()
	logged, _ := io.ReadAll(r)
	_ = r.Close()
	if got[config.SignalDeliverableKind] != config.DeliverableKindCode {
		t.Errorf("degraded digest must stay on the conservative kind, never the domain default; got %v", got)
	}
	if !strings.Contains(string(logged), "WARN") || !strings.Contains(string(logged), "triage") {
		t.Errorf("degraded digest must WARN naming the torn report; stderr=%q", logged)
	}
}
