package phaseio

import (
	"reflect"
	"strings"
	"testing"
)

func TestEdgeUpstreams_DeclaredEdges(t *testing.T) {
	cases := []struct {
		phase string
		want  []string
	}{
		{"intent", []string{}},
		{"scout", []string{}},
		{"triage", []string{"scout"}},
		{"tdd", []string{"scout", "triage"}},
		{"build-planner", []string{"scout", "triage"}},
		{"swarm-plan", []string{"scout", "triage"}},
		{"build", []string{"scout", "triage"}},
		{"debugger", []string{"triage", "build"}},
		{"audit", []string{}},
	}
	for _, tc := range cases {
		got, declared := EdgeUpstreams(tc.phase)
		if !declared || !reflect.DeepEqual(got, tc.want) {
			t.Errorf("EdgeUpstreams(%q) = %v, %v; want %v, true", tc.phase, got, declared, tc.want)
		}
	}
}

func TestEdgeUpstreams_UndeclaredPhase(t *testing.T) {
	if got, declared := EdgeUpstreams("memo"); declared || got != nil {
		t.Errorf("EdgeUpstreams(memo) = %v, %v; want nil, false", got, declared)
	}
}

func TestEdgeUpstreams_ReturnsACopy(t *testing.T) {
	got, _ := EdgeUpstreams("build")
	got[0] = "audit"
	if again, _ := EdgeUpstreams("build"); again[0] != "scout" {
		t.Errorf("mutating the returned slice changed the edge table: %v", again)
	}
}

func edgeFixture() Handoffs {
	return NewHandoffs(HandoffsInit{
		Scout:    &ScoutView{CycleSizeEstimate: "S", ItemCount: 1},
		Triage:   &TriageView{CycleSize: "M"},
		Build:    &BuildView{Verdict: "PASS"},
		Audit:    &AuditView{Verdict: "FAIL"},
		Generic:  map[string]any{"scout.k": "sv", "triage.k": "tv", "build.k": "bv", "scoutx.k": "nope"},
		Degraded: []string{"scout-report.md"},
	})
}

func TestDigestFor_DebuggerSeesTriageAndBuildOnly(t *testing.T) {
	got := edgeFixture().DigestFor("debugger")
	want := strings.Join([]string{
		`triage: cycle_size="M" phase_skip=[]`,
		`build: verdict="PASS" acs=0/0 red=0 severity=""`,
		`generic "build.k": "bv"`,
		`generic "triage.k": "tv"`,
	}, "\n")
	if got != want {
		t.Errorf("DigestFor(debugger) =\n%s\nwant\n%s", got, want)
	}
}

func TestDigestFor_GenericPrefixNeedsTheDot(t *testing.T) {
	got := edgeFixture().DigestFor("triage")
	if strings.Contains(got, "scoutx") || strings.Contains(got, "degraded") {
		t.Errorf("DigestFor(triage) carries a key outside the scout section:\n%s", got)
	}
	if !strings.Contains(got, `generic "scout.k": "sv"`) {
		t.Errorf("DigestFor(triage) omits the scout generic key:\n%s", got)
	}
}

func TestDigestFor_EmptyEdgesRenderNothing(t *testing.T) {
	for _, phase := range []string{"scout", "audit", "intent", "memo"} {
		if got := edgeFixture().DigestFor(phase); got != "" {
			t.Errorf("DigestFor(%q) = %q; want empty", phase, got)
		}
	}
	if got := NewHandoffs(HandoffsInit{Audit: &AuditView{}}).DigestFor("build"); got != "" {
		t.Errorf("DigestFor(build) with no declared section present = %q; want empty", got)
	}
}

func TestDigestFor_CapsAtThePackageDefault(t *testing.T) {
	h := NewHandoffs(HandoffsInit{
		Scout:   &ScoutView{CycleSizeEstimate: strings.Repeat("s", 4*defaultUpstreamDigestRunes)},
		Triage:  &TriageView{CycleSize: "M"},
		Generic: map[string]any{"triage.big": strings.Repeat("t", 4*defaultUpstreamDigestRunes)},
	})
	got := h.DigestFor("build")
	if n := len([]rune(got)); n > defaultUpstreamDigestRunes {
		t.Fatalf("DigestFor(build) is %d runes; want <= %d", n, defaultUpstreamDigestRunes)
	}
	for _, section := range []string{"scout:", "triage:", `generic "triage.big"`} {
		if !strings.Contains(got, section) {
			t.Errorf("capped digest lost section %q:\n%s", section, got)
		}
	}
}

func TestUpstreamDigest_RendersEverySectionDegradedAndUnencodable(t *testing.T) {
	h := NewHandoffs(HandoffsInit{
		Audit:    &AuditView{Verdict: "FAIL", Confidence: 0.5, RedCount: 2},
		Generic:  map[string]any{"build.fn": func() {}},
		Degraded: []string{"build-report.md"},
	})
	want := strings.Join([]string{
		`audit: verdict="FAIL" confidence=0.5 red=2`,
		`degraded: "build-report.md"`,
		`generic "build.fn": "<unencodable>"`,
	}, "\n")
	if got := h.UpstreamDigest(0); got != want {
		t.Errorf("UpstreamDigest(0) =\n%s\nwant\n%s", got, want)
	}
}

func TestUpstreamDigest_CapBelowTheSeparatorsCutsTheJoinedText(t *testing.T) {
	h := NewHandoffs(HandoffsInit{
		Scout:  &ScoutView{CycleSizeEstimate: "S"},
		Triage: &TriageView{CycleSize: "M"},
		Audit:  &AuditView{Verdict: "PASS"},
	})
	if got := h.UpstreamDigest(1); got != "s" {
		t.Errorf("UpstreamDigest(1) = %q; want the first rune of the joined digest", got)
	}
}
