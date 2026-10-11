package router

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolveGeneric_CoercesEachValueKind(t *testing.T) {
	sig := RoutingSignals{Generic: map[string]any{
		"f": 2.5, "i": 7, "s": "small", "t": true, "fa": false, "m": map[string]any{}, "n": nil,
	}}
	cases := []struct {
		field   string
		num     float64
		isNum   bool
		str     string
		present bool
	}{
		{"f", 2.5, true, "", true},
		{"i", 7, true, "", true},
		{"s", 0, false, "small", true},
		{"t", 0, false, "true", true},
		{"fa", 0, false, "false", true},
		{"m", 0, false, "", true},
		{"n", 0, false, "", true},
		{"absent", 0, false, "", false},
	}
	for _, tc := range cases {
		t.Run(tc.field, func(t *testing.T) {
			num, isNum, str, present := resolveGeneric(sig, tc.field)

			if num != tc.num || isNum != tc.isNum || str != tc.str || present != tc.present {
				t.Errorf("resolveGeneric(%q) = (%v, %v, %q, %v), want (%v, %v, %q, %v)",
					tc.field, num, isNum, str, present, tc.num, tc.isNum, tc.str, tc.present)
			}
		})
	}
}

func TestReadReportFallback(t *testing.T) {
	dir := t.TempDir()
	full := filepath.Join(dir, "full.md")
	if err := os.WriteFile(full, []byte("goal_type: fix\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	empty := filepath.Join(dir, "empty.md")
	if err := os.WriteFile(empty, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name         string
		path         string
		want         string
		wantPresent  bool
		wantDegraded string
	}{
		{"present report", full, "goal_type: fix\n", true, ""},
		{"empty report is absent and quiet", empty, "", false, ""},
		{"missing report is absent and quiet", filepath.Join(dir, "missing.md"), "", false, ""},
		{"unreadable report degrades loudly", dir, "", false, "scout: report fallback read: "},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var degraded []string

			got, present := readReportFallback(tc.path, "scout", &degraded)

			if got != tc.want || present != tc.wantPresent {
				t.Errorf("= (%q, %v), want (%q, %v)", got, present, tc.want, tc.wantPresent)
			}
			if tc.wantDegraded == "" && len(degraded) != 0 {
				t.Errorf("degraded = %v, want none", degraded)
			}
			if tc.wantDegraded != "" && (len(degraded) != 1 || !strings.HasPrefix(degraded[0], tc.wantDegraded)) {
				t.Errorf("degraded = %v, want one entry starting %q", degraded, tc.wantDegraded)
			}
		})
	}
}

func TestWalk_SkipsPhasesAlreadyCompletedAfterTheCurrentOne(t *testing.T) {
	in := base("scout")
	in.Cfg.Order = []string{"scout", "build", "audit", "ship"}
	in.Completed = []string{"scout", "build"}

	d := walk(in, nil)

	if d.NextPhase != "audit" {
		t.Errorf("NextPhase = %q (%s), want audit: a completed build must not run twice", d.NextPhase, d.Reason)
	}
}

func TestWalk_EveryLaterPhaseCompletedEndsTheCycle(t *testing.T) {
	in := base("scout")
	in.Cfg.Order = []string{"scout", "build", "audit"}
	in.Completed = []string{"scout", "build", "audit"}

	d := walk(in, nil)

	if d.NextPhase != PhaseEnd || d.Reason != "no-runnable-phase-remaining" {
		t.Errorf("= (%q, %q), want (%q, no-runnable-phase-remaining)", d.NextPhase, d.Reason, PhaseEnd)
	}
}
