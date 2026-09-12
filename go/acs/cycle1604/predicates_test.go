//go:build acs

// Package cycle1604 materialises the cycle-1604 acceptance criteria for the
// single fleet-scoped inbox item `tokenopt-handoff-digests` (cycle-1603
// carryover: an audit-reported PASS for this exact contract was never landed
// on the live tree — cross-worktree mismatch, see scout-report.md Key
// Finding 2 — so this cycle re-authors the RED contract against the current
// worktree rather than trusting the stale report).
//
// API this cycle materialises (Builder implements exactly this shape):
//
//   - phaseio.Handoffs.UpstreamDigest(capRunes int) string — a deterministic,
//     rune-bounded narrative rendering of the sealed upstream view.
//     capRunes<=0 falls back to a package default. The zero-value Handoffs
//     (no upstream phase completed) renders "" — absence stays absence, never
//     fabricated content. A present-but-zero-value view (e.g. BuildView{})
//     still renders its section — "present but empty" must never collapse
//     into "absent". Degraded()-listed edges are named in the output, never
//     silently dropped (R5's read-miss-vs-absence distinction must survive
//     the digest).
//   - The tdd phase's real ComposePrompt (reachable only via tdd.New(...).
//     ComposePrompt, the exported runner.BaseRunner entry point — never a
//     direct call into the unexported `hooks` type) renders the digest of
//     req.Input.Upstream() when req.Input.Active(), and the cap holds on that
//     live path: a huge raw upstream value must never appear verbatim in the
//     rendered prompt. An inactive PhaseInput (Active()==false) must leave
//     the prompt byte-identical to the legacy (no-digest) rendering.
//
// Predicate strategy — each predicate below EXERCISES the system under test
// (direct calls into phaseio's real exported API, or the tdd phase's real
// exported ComposePrompt entry point) and asserts on the returned string, per
// the cycle-85 behavioral-predicate rule. None is a source-grep of production
// code.
//
//   - 001 proves the cap is a hard ceiling against oversized upstream data.
//   - 002 proves a zero-value Handoffs (no upstream at all) is absence, not a
//     fabricated digest.
//   - 003 proves a read-miss recorded in Degraded() surfaces in the digest
//     rather than vanishing (the R5 contract).
//   - 004 proves the digest is deterministic across repeated calls on
//     identical input (map-iteration-order regression guard: Generic is a
//     map[string]any).
//   - 005 proves present-but-zero-value is distinguished from genuinely
//     absent, at the digest layer, not just at Handoffs.Build()'s ok bool.
//   - 006 is the reachability/caller proof (house rule): the REAL tdd
//     ComposePrompt, driven through the production PhaseInput channel, must
//     render upstream digest content — not a predicate calling the seam
//     directly.
//   - 007 is the negative/anti-no-op proof paired with 006: an oversized raw
//     upstream value must never leak verbatim into the live tdd prompt.
//   - 008 proves the byte-identical-inactive-path AC: an inactive PhaseInput
//     (the Active()==false / legacy Context-map path) is untouched by the
//     digest hook.
package cycle1604

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/phaseio"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/tdd"
)

// --- Task: phaseio-handoff-digest-contract (digest projection) --------------

// TestC1604_001_UpstreamDigestBoundedByCap proves the digest never exceeds the
// requested cap even when the sealed upstream view carries far more text than
// that — the structural size cap the scout report requires in place of an
// unmeasured token-savings claim.
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

// TestC1604_002_UpstreamDigestAbsentForZeroValueHandoffs proves the zero-value
// Handoffs (no upstream phase has completed) yields an empty digest rather
// than fabricated content — absent must stay absent.
func TestC1604_002_UpstreamDigestAbsentForZeroValueHandoffs(t *testing.T) {
	var h phaseio.Handoffs // zero value: nothing upstream has completed
	if got := h.UpstreamDigest(512); got != "" {
		t.Errorf("zero-value Handoffs must yield an empty digest, got %q", got)
	}
}

// TestC1604_003_UpstreamDigestSurfacesDegradedReads proves a recorded
// read-miss (R5: failed-for-a-reason-other-than-absence) is named in the
// digest instead of being silently dropped — a degraded upstream read must
// never present as a clean absence to a downstream prompt consumer.
//
// Cycle-1632 audit-repair extension (cycle-1604 df875c36…/d69bd9f3…, M1): the
// degraded-only fixture could not observe the blind post-render truncation —
// with a typed view rendered FIRST whose agent-authored scalar is oversized,
// the degraded row was evicted wholesale. The second case pins that shape:
// the read-miss must survive an oversized ScoutView under the same cap.
func TestC1604_003_UpstreamDigestSurfacesDegradedReads(t *testing.T) {
	for _, tc := range []struct {
		name string
		init phaseio.HandoffsInit
		edge string // the degraded edge the digest must still name
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

// TestC1604_004_UpstreamDigestDeterministicAcrossCalls proves the digest is
// stable across repeated calls on identical input. Generic is a
// map[string]any, so a naive implementation that ranges it directly into the
// rendered string would be nondeterministic between calls — a real defect a
// grep-only predicate could never catch.
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

// TestC1604_005_UpstreamDigestDistinguishesPresentZeroFromAbsent proves a
// present-but-zero-value view (e.g. a build phase that just started, 0 ACS
// run yet) still renders its section, distinct from an upstream phase that
// never ran at all — mirroring the P5 present-vs-absent contract Handoffs'
// accessors already enforce, now at the digest layer too.
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

// --- Task: runner-consumes-handoff-digests (live TDD composer) -------------

// TestC1604_006_TDDComposePromptRendersUpstreamDigestFromRealCaller is the
// house-rule caller proof: it drives the REAL production entry point
// (tdd.New(...).ComposePrompt, the exported runner.BaseRunner method — never
// a predicate calling phaseio directly) with a PhaseInput built the way the
// dispatch seam builds one, and asserts the rendered prompt actually carries
// upstream digest content. A predicate that only exercised phaseio would pass
// on dead code; this one fails until the tdd phase hook is actually wired.
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

// TestC1604_007_TDDComposePromptCapsDigestNoRawArtifactLeak is the negative
// pair to 006: an oversized raw upstream value fed through the same real
// production path must never appear verbatim in the rendered prompt — proof
// the cap actually governs the live prompt, not just the phaseio unit tests.
func TestC1604_007_TDDComposePromptCapsDigestNoRawArtifactLeak(t *testing.T) {
	huge := strings.Repeat("RAWARTIFACT", 500) // 5500 chars — far past any sane prompt digest cap
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

// TestC1604_008_TDDComposePromptByteIdenticalWhenInactive proves the inactive
// PhaseInput path (Active()==false — off/shadow PhaseIO stages, or any
// hand-built request without Phase stamped) is untouched by the digest hook:
// rendering with a populated-but-inactive Upstream must equal rendering with
// a completely zero-value Input, so the legacy dispatch stays byte-identical
// until the seam actually activates the envelope.
func TestC1604_008_TDDComposePromptByteIdenticalWhenInactive(t *testing.T) {
	phase := tdd.New(tdd.Config{})

	zeroReq := core.PhaseRequest{}
	zeroGot := phase.ComposePrompt("BODY", zeroReq)

	// An inactive PhaseInput built with Upstream populated but Phase unset
	// (Active()==false) must render identically to the fully zero request —
	// proof the digest hook gates on Active(), not on Upstream() being
	// non-empty.
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
