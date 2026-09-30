//go:build acs

package cycle1604

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/phaseio"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/tdd"
)

func TestC1604_001_UpstreamDigestBoundedByCap(t *testing.T) {
	long := strings.Repeat("x", 5000)
	h := phaseio.NewHandoffs(phaseio.HandoffsInit{
		Generic: map[string]any{"scout.notes": long},
	})
	const cap = 64
	got := h.UpstreamDigest(cap)
	if n := len([]rune(got)); n > cap {
		t.Errorf("UpstreamDigest(%d) returned %d runes, want <= %d: %q", cap, n, cap, got)
	}
	if got == "" {
		t.Errorf("UpstreamDigest must not be empty when real upstream data is present")
	}
}

func TestC1604_002_UpstreamDigestAbsentForZeroValueHandoffs(t *testing.T) {
	var h phaseio.Handoffs
	if got := h.UpstreamDigest(512); got != "" {
		t.Errorf("zero-value Handoffs must yield an empty digest, got %q", got)
	}
}

func TestC1604_003_UpstreamDigestSurfacesDegradedReads(t *testing.T) {
	for _, tc := range []struct {
		name string
		init phaseio.HandoffsInit
		edge string
	}{
		{
			name: "degraded-only",
			init: phaseio.HandoffsInit{Degraded: []string{"scout: handoff-scout.json: permission denied"}},
			edge: "scout",
		},
		{
			name: "oversized-scout-view-before-degraded",
			init: phaseio.HandoffsInit{
				Scout:    &phaseio.ScoutView{CycleSizeEstimate: strings.Repeat("x", 4000)},
				Degraded: []string{"build: handoff-build.json: permission denied"},
			},
			edge: "build",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := phaseio.NewHandoffs(tc.init).UpstreamDigest(512)
			if n := len([]rune(got)); n > 512 {
				t.Errorf("digest is %d runes, cap 512", n)
			}
			if !strings.Contains(strings.ToLower(got), "degrad") {
				t.Errorf("digest must surface a read-miss as degraded, not drop it silently: %q", got)
			}
			if !strings.Contains(got, tc.edge) {
				t.Errorf("digest must name which upstream edge degraded (%s): %q", tc.edge, got)
			}
		})
	}
}

func TestC1604_004_UpstreamDigestDeterministicAcrossCalls(t *testing.T) {
	h := phaseio.NewHandoffs(phaseio.HandoffsInit{
		Scout: &phaseio.ScoutView{CycleSizeEstimate: "large", ItemCount: 4, CarryoverCount: 2, BacklogSize: 9},
		Build: &phaseio.BuildView{Verdict: "PASS", ACSGreen: 3, ACSTotal: 3},
		Generic: map[string]any{
			"scout.a": 1.0, "scout.b": 2.0, "scout.c": 3.0, "scout.d": 4.0, "scout.e": 5.0,
		},
	})
	first := h.UpstreamDigest(512)
	for i := 0; i < 5; i++ {
		if got := h.UpstreamDigest(512); got != first {
			t.Fatalf("UpstreamDigest is nondeterministic across identical calls (map-order leak?) on attempt %d:\n first=%q\n   got=%q", i, first, got)
		}
	}
}

func TestC1604_005_UpstreamDigestDistinguishesPresentZeroFromAbsent(t *testing.T) {
	withBuild := phaseio.NewHandoffs(phaseio.HandoffsInit{Build: &phaseio.BuildView{}})
	without := phaseio.NewHandoffs(phaseio.HandoffsInit{})

	gotWith := withBuild.UpstreamDigest(512)
	gotWithout := without.UpstreamDigest(512)

	if !strings.Contains(gotWith, "build") {
		t.Errorf("a present-but-zero-value BuildView must still render a build section, got %q", gotWith)
	}
	if strings.Contains(gotWithout, "build") {
		t.Errorf("an absent build handoff must not render a build section, got %q", gotWithout)
	}
}

func TestC1604_006_TDDComposePromptRendersUpstreamDigestFromRealCaller(t *testing.T) {
	const marker = "large-marker-c1604-digest"
	req := core.PhaseRequest{
		Input: phaseio.NewPhaseInput(phaseio.PhaseInputInit{
			Phase: string(core.PhaseTDD),
			Upstream: phaseio.NewHandoffs(phaseio.HandoffsInit{
				Scout: &phaseio.ScoutView{CycleSizeEstimate: marker},
			}),
		}),
	}
	phase := tdd.New(tdd.Config{})
	got := phase.ComposePrompt("BODY", req)
	if !strings.Contains(got, marker) {
		t.Errorf("tdd's real ComposePrompt must render the upstream digest reachable from the production PhaseInput channel; marker %q missing from:\n%s", marker, got)
	}
}

func TestC1604_007_TDDComposePromptCapsDigestNoRawArtifactLeak(t *testing.T) {
	huge := strings.Repeat("RAWARTIFACT", 500)
	req := core.PhaseRequest{
		Input: phaseio.NewPhaseInput(phaseio.PhaseInputInit{
			Phase: string(core.PhaseTDD),
			Upstream: phaseio.NewHandoffs(phaseio.HandoffsInit{
				Generic: map[string]any{"scout.raw": huge},
			}),
		}),
	}
	phase := tdd.New(tdd.Config{})
	got := phase.ComposePrompt("BODY", req)
	if strings.Contains(got, huge) {
		t.Errorf("tdd's real ComposePrompt must never embed the full raw upstream value verbatim — the bounded digest must truncate it")
	}
}

func TestC1604_008_TDDComposePromptByteIdenticalWhenInactive(t *testing.T) {
	phase := tdd.New(tdd.Config{})

	zeroReq := core.PhaseRequest{}
	zeroGot := phase.ComposePrompt("BODY", zeroReq)

	inactiveReq := core.PhaseRequest{
		Input: phaseio.NewPhaseInput(phaseio.PhaseInputInit{
			Upstream: phaseio.NewHandoffs(phaseio.HandoffsInit{
				Scout: &phaseio.ScoutView{CycleSizeEstimate: "should-never-render"},
			}),
		}),
	}
	inactiveGot := phase.ComposePrompt("BODY", inactiveReq)

	if inactiveGot != zeroGot {
		t.Errorf("inactive PhaseInput must render byte-identical to zero-value Input; got:\n%s\nwant:\n%s", inactiveGot, zeroGot)
	}
	if strings.Contains(inactiveGot, "should-never-render") {
		t.Errorf("inactive PhaseInput must never render Upstream content: %q", inactiveGot)
	}
}
