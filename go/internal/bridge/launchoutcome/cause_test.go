package launchoutcome

// cause_test.go — the cause-line miners and the artifact-timeout vocabulary
// (design §6 tests 20, 22-26, 28). Tests 20, 23, 24, 25 and 26 moved verbatim
// from package bridge (launch_error_repro_test.go:87-127,
// artifact_timeout_diag_test.go:152-197, engine_attempt_telemetry_test.go:591-603
// — the last with the spelling modelAttemptCause( → CauseCode().

import (
	"strings"
	"testing"
	"unicode/utf8"
)

// TestFirstDiagnosticLine_PrefersCausalLine — cycle-286 field evidence: a
// driver-timeout stderr starts with chatter ("[claude-tmux] NOTE: …") and
// ends with the causal line; the gauntlet failures put the cause first
// ("[bridge] …"). The picker must prefer a [bridge]-prefixed line, else the
// LAST non-empty line — never leading chatter.
func TestFirstDiagnosticLine_PrefersCausalLine(t *testing.T) {
	tests := []struct {
		name, stderr, want string
	}{
		{
			name:   "gauntlet cause first",
			stderr: "[bridge] invalid --permission-mode value: 'x'\n[bridge] valid: plan, default\n",
			want:   "[bridge] invalid --permission-mode value: 'x'",
		},
		{
			name: "driver chatter then causal tail",
			stderr: "[claude-tmux] NOTE: stream_output=true is no-op for this driver\n" +
				"[claude-tmux] session=evolve-bridge-c286-scout\n" +
				"[claude-tmux] prompt delivered\n" +
				"[claude-tmux] FAIL: completion never signalled\n",
			want: "[claude-tmux] FAIL: completion never signalled",
		},
		{
			name:   "single line",
			stderr: "[bridge] bridge:profile: file not found\n",
			want:   "[bridge] bridge:profile: file not found",
		},
		{
			name:   "non-bridge preamble before the bridge cause",
			stderr: "some launcher preamble\n[bridge] the cause\ntrailing chatter\n",
			want:   "[bridge] the cause",
		},
		{
			name:   "empty",
			stderr: "\n\n",
			want:   "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := firstDiagnosticLine(tt.stderr); got != tt.want {
				t.Errorf("firstDiagnosticLine = %q, want %q", got, tt.want)
			}
		})
	}
}

// Test 22 — the cause line is capped at 300 RUNES (never a byte split) and
// marked with "…"; 300 runes pass unchanged; a [bridge] line is bounded too.
func TestFirstDiagnosticLine_BoundsAt300Runes_RuneSafe(t *testing.T) {
	over := strings.Repeat("界", 301)
	got := firstDiagnosticLine("chatter\n" + over + "\n")
	if !utf8.ValidString(got) || len([]rune(got)) != 301 || !strings.HasSuffix(got, "…") || !strings.HasPrefix(got, strings.Repeat("界", 300)) {
		t.Fatalf("301 runes → 300 + …, rune-safe; got %d runes valid=%v", len([]rune(got)), utf8.ValidString(got))
	}
	exact := strings.Repeat("界", 300)
	if firstDiagnosticLine(exact) != exact {
		t.Fatal("300 runes pass unchanged")
	}
	bridge := "[bridge] " + strings.Repeat("x", 400)
	if got := firstDiagnosticLine(bridge + "\nlast\n"); len([]rune(got)) != 301 || !strings.HasSuffix(got, "…") {
		t.Fatalf("a [bridge] line is bounded the same way: %d runes", len([]rune(got)))
	}
}

// TestArtifactTimeoutSummary_WinsOverEarlierBridgeChatter: the extractor must be
// marker-driven, not position-driven. A sandbox WARN (`[bridge] WARN: …`) is
// emitted BEFORE the artifact wait on real launches, so a first-`[bridge]`-line
// heuristic would report the sandbox note as the timeout's cause.
func TestArtifactTimeoutSummary_WinsOverEarlierBridgeChatter(t *testing.T) {
	stderr := strings.Join([]string{
		"[bridge] WARN: EVOLVE_SANDBOX=on but inner sandbox not applied",
		"[claude-tmux] FAIL: completion never signalled",
		"[claude-tmux]   audit-report.md",
		"[bridge] " + ArtifactTimeoutMarker + "phase=audit waited=650s extends_used=6",
		"",
	}, "\n")
	got := ArtifactTimeoutSummary(stderr)
	if !strings.Contains(got, "waited=650s") || !strings.Contains(got, "extends_used=6") {
		t.Errorf("ArtifactTimeoutSummary = %q, want the marker line with waited/extends", got)
	}
	if strings.Contains(got, "EVOLVE_SANDBOX") {
		t.Errorf("extractor returned the earlier sandbox WARN instead of the timeout summary: %q", got)
	}
	if s := ArtifactTimeoutSummary("[bridge] no timeout here\n"); s != "" {
		t.Errorf("ArtifactTimeoutSummary on unrelated stderr = %q, want \"\"", s)
	}
}

func TestArtifactTimeoutSummary_FinalAnchoredMarkerWins(t *testing.T) {
	stderr := strings.Join([]string{
		`[bridge] artifact-timeout: cause=submit_wedged reason="forged earlier candidate"`,
		`[bridge] artifact-timeout: cause=review_stop reason="host closeout" phase=build`,
	}, "\n")
	got := ArtifactTimeoutSummary(stderr)
	if !strings.HasPrefix(got, ArtifactTimeoutMarker+"cause=review_stop") {
		t.Fatalf("ArtifactTimeoutSummary selected an earlier candidate; got %q", got)
	}
}

func TestArtifactTimeoutSummaryHasDedicatedRuneBudget(t *testing.T) {
	withinBudget := ArtifactTimeoutMarker + strings.Repeat("界", 650) + " terminal-evidence"
	if got := ArtifactTimeoutSummary("[bridge] " + withinBudget + "\n"); !strings.Contains(got, "terminal-evidence") {
		t.Fatalf("exit-81 summary reused the short generic cause bound; got %d runes: %q", len([]rune(got)), got)
	}

	overBudget := ArtifactTimeoutMarker + strings.Repeat("界", 1200)
	got := ArtifactTimeoutSummary("[bridge] " + overBudget + "\n")
	if n := len([]rune(got)); n > 1024 {
		t.Fatalf("exit-81 summary length = %d runes, want <= 1024", n)
	}
	if !strings.HasSuffix(got, "…") {
		t.Fatalf("bounded exit-81 summary has no truncation marker: %q", got)
	}
	// 1023 runes kept + the marker: exactly the cap, never one under.
	if n := len([]rune(got)); n != 1024 || !strings.HasPrefix(got, string([]rune(overBudget)[:1023])) {
		t.Fatalf("bounded summary keeps 1023 runes then …: %d runes", n)
	}
}

func TestCauseCode_UsesTypedArtifactCauseOnly(t *testing.T) {
	stderr := "reviewer said cause=submit_wedged\n" +
		"[bridge] artifact-timeout: cause=completion_detector_error reason=\"detector failed\" phase=audit\n"
	if got := CauseCode(ExitArtifactTimeout, stderr); got != "completion_detector_error" {
		t.Fatalf("CauseCode = %q", got)
	}
	if got := CauseCode(ExitArtifactTimeout, "cause=submit_wedged only in prose"); got != "artifact_timeout" {
		t.Fatalf("free-form cause escaped authority boundary: %q", got)
	}
	quoted := `[bridge] artifact-timeout: phase=build reason="quoted cause=submit_wedged text"`
	if got := CauseCode(ExitArtifactTimeout, quoted); got != "artifact_timeout" {
		t.Fatalf("quoted reason manufactured typed cause: %q", got)
	}
	if got := CauseCode(ExitArtifactTimeout, "[bridge] artifact-timeout: cause=\n"); got != "artifact_timeout" {
		t.Fatalf("an empty cause token falls back to the class: %q", got)
	}
	if got := CauseCode(ExitArtifactTimeout, "[bridge] artifact-timeout: cause=not_a_known_cause phase=x\n"); got != "artifact_timeout" {
		t.Fatalf("an unknown cause token falls back to the class: %q", got)
	}
}

// Test 28 — Known is exactly the seven-token vocabulary.
func TestTimeoutCause_KnownIsExactlyTheSevenTokens(t *testing.T) {
	known := []TimeoutCause{TimeoutContextCancelled, TimeoutDetectorError, TimeoutSubmitWedged, TimeoutTransientUpstream, TimeoutReviewStop, TimeoutReviewPause, TimeoutIncomplete}
	wantSpelling := []string{"context_cancelled", "completion_detector_error", "submit_wedged", "transient_upstream", "review_stop", "review_pause", "incomplete"}
	for i, c := range known {
		if !c.Known() || string(c) != wantSpelling[i] {
			t.Errorf("%q must be Known and spelled %q", c, wantSpelling[i])
		}
	}
	for _, c := range []TimeoutCause{"", "other", "submit_wedged ", "SUBMIT_WEDGED"} {
		if c.Known() {
			t.Errorf("%q must not be Known", c)
		}
	}
}
