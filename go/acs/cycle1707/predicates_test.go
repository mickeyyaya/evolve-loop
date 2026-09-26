//go:build acs

// Package cycle1707 materialises the acceptance criteria for
// lineage-datestamp-normalization: LineageKey must additionally strip
// calendar-year-shaped date runs so same-line dated snapshots
// (gpt-4o-2024-08-06, gpt-4o-2024-11-20) share a lineage bucket and
// PromoteLatest can pick the newer one, while size/tag digit suffixes
// (:8b, :70b, 32b) stay untouched and the reuse-gate decisionVersion ratchet
// is bumped for the semantics change.
package cycle1707

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/modelquery"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

// TestC1707_001_DatedSnapshotsShareLineageKey is AC1: gpt-4o-2024-08-06 and
// gpt-4o-2024-11-20 are the same model line at different dated snapshots and
// must bucket under the same LineageKey so PromoteLatest can act on them.
func TestC1707_001_DatedSnapshotsShareLineageKey(t *testing.T) {
	a, b := modelquery.LineageKey("gpt-4o-2024-08-06"), modelquery.LineageKey("gpt-4o-2024-11-20")
	if a != b {
		t.Errorf("LineageKey(gpt-4o-2024-08-06)=%q != LineageKey(gpt-4o-2024-11-20)=%q — dated snapshots of the same line must share a key", a, b)
	}
}

// TestC1707_002_SizeAndTagSuffixesStayDistinct is AC2, the adversarial
// negative guarding against an over-broad date regex: capability-bearing
// digit suffixes (:8b vs :70b, and the 32b in qwen2.5-coder:32b) must NOT be
// swallowed by the new date-run strip. A no-op or over-broad fix that
// collapses these into one key must fail this predicate.
func TestC1707_002_SizeAndTagSuffixesStayDistinct(t *testing.T) {
	mustDiffer := [][2]string{
		{"llama3.1:8b", "llama3.1:70b"},
		{"qwen2.5-coder:32b", "qwen2.5-coder:70b"},
	}
	for _, p := range mustDiffer {
		ka, kb := modelquery.LineageKey(p[0]), modelquery.LineageKey(p[1])
		if ka == kb {
			t.Errorf("LineageKey(%q) == LineageKey(%q) = %q — capability-class suffix collapsed by the date/version strip", p[0], p[1], ka)
		}
	}
}

// TestC1707_003_NewestInLineageOrdersDatedSnapshots is AC3: given both dated
// snapshots of one line, NewestInLineage (or its composed comparator) must
// pick the later calendar date, not the first-listed id.
func TestC1707_003_NewestInLineageOrdersDatedSnapshots(t *testing.T) {
	got := modelquery.NewestInLineage([]string{"gpt-4o-2024-08-06", "gpt-4o-2024-11-20"})
	if got != "gpt-4o-2024-11-20" {
		t.Errorf("NewestInLineage([gpt-4o-2024-08-06, gpt-4o-2024-11-20]) = %q, want gpt-4o-2024-11-20 (the later date)", got)
	}
	// Order must not matter — the newer date wins regardless of input position.
	got2 := modelquery.NewestInLineage([]string{"gpt-4o-2024-11-20", "gpt-4o-2024-08-06"})
	if got2 != "gpt-4o-2024-11-20" {
		t.Errorf("NewestInLineage([gpt-4o-2024-11-20, gpt-4o-2024-08-06]) = %q, want gpt-4o-2024-11-20 (order must not flip the winner)", got2)
	}
}

// TestC1707_004_KnownLimitationPinMigratedToEquality is AC4: the pinned
// known-limitation test (lineage_test.go) currently asserts these two dated
// ids keep DIFFERENT keys — its own migration note says to move this case
// into mustMatch once normalization ships. This predicate demands the
// opposite of the old pin (equality) directly from the production function,
// so it stays RED under today's code and only goes GREEN once the migration
// described in the note has actually happened in behavior, not just in the
// test file.
func TestC1707_004_KnownLimitationPinMigratedToEquality(t *testing.T) {
	a, b := modelquery.LineageKey("gpt-4o-2024-08-06"), modelquery.LineageKey("gpt-4o-2024-11-20")
	if a != b {
		t.Errorf("known-limitation pin not yet migrated: LineageKey(gpt-4o-2024-08-06)=%q still differs from LineageKey(gpt-4o-2024-11-20)=%q", a, b)
	}
}

// oldFingerprintV1 is the Fingerprint() output for a fixed FingerprintInput
// computed against the CURRENT (unfixed) decision surface, where
// decisionVersion is still "v1". TestC1707_005 demands the fixed input no
// longer reproduces this value, which only happens once decisionVersion is
// bumped in fingerprint.go (AC5) — a behavioral proxy for the hand-bumped
// ratchet constant that never appears in this package's public API.
const oldFingerprintV1 = "sha256:441aa02fd68539bda9f2826334d8165598de47251384bf564244f616b8722a3b"

// TestC1707_005_DecisionVersionBumpedForSemanticsChange is AC5 (first half):
// LineageKey's semantics changed (AC1-3), so decisionVersion must be bumped —
// otherwise a cached pre-fix classification could be silently reused for a
// CLI whose candidate list is unchanged (fingerprint.go's own stated
// purpose). Fingerprint() for a fixed input must therefore stop reproducing
// the pinned v1 baseline.
func TestC1707_005_DecisionVersionBumpedForSemanticsChange(t *testing.T) {
	in := modelquery.FingerprintInput{
		CLI:        "acs-cycle1707-baseline",
		Candidates: []string{"gpt-4o-2024-08-06", "gpt-4o-2024-11-20"},
		Policy:     modelquery.FreshnessPolicy{},
		Tiers:      []string{"fast", "deep"},
	}
	got := modelquery.Fingerprint(in)
	if got == oldFingerprintV1 {
		t.Errorf("Fingerprint(%+v) = %s still matches the pinned v1 baseline — decisionVersion was not bumped for the lineage semantics change", in, got)
	}
}

// TestC1707_006_DecisionSurfacePinTracksTheChange is AC5 (second half): the
// ratchet test in decisionversion_pin_test.go must be kept in sync with
// whatever lineage.go/fingerprint.go end up looking like — it is designed to
// fail the moment the decision surface changes until the pin is updated by
// hand, forcing the explicit acknowledgment fingerprint.go's own doc comment
// describes. This predicate runs that real repo test as a subprocess and
// requires it to pass, so a diff that bumps decisionVersion but forgets the
// pin update is caught.
func TestC1707_006_DecisionSurfacePinTracksTheChange(t *testing.T) {
	const modelqueryPkg = "github.com/mickeyyaya/evolve-loop/go/internal/modelquery"
	stdout, stderr, code, err := acsassert.SubprocessOutput(
		"go", "test", "-count=1", "-run", "^TestDecisionVersion_PinnedToAlgorithmSurface$",
		"-v", modelqueryPkg)
	out := stdout + stderr
	// code < 0 is a genuine launch failure (binary missing / killed by
	// signal), not a test verdict; SubprocessOutput returns a non-nil err for
	// ANY non-zero exit, so a plain assertion failure (the RED signal) must
	// flow through as a normal predicate failure, not a launch-failure abort.
	if code < 0 {
		t.Fatalf("go test failed to launch for %s: code=%d err=%v\n%s", modelqueryPkg, code, err, out)
	}
	if code != 0 {
		t.Errorf("TestDecisionVersion_PinnedToAlgorithmSurface failed (decisionSurfacePin out of sync with the decision surface):\n%s", out)
	}
}

// promotionCase is one PromoteLatest scenario: the classifier's pick for the
// deep tier, the CLI-listed candidates in listing order, and the required pick.
type promotionCase struct {
	name       string
	sel        string
	candidates []string
	want       string
}

// runPromotionCases drives each case through PromoteLatest, the production
// entry point liveTiers calls after Classify, and reports every mismatch.
func runPromotionCases(t *testing.T, cases []promotionCase) {
	t.Helper()
	for _, tc := range cases {
		got := modelquery.PromoteLatest(map[string]string{"deep": tc.sel}, tc.candidates, modelquery.FreshnessPolicy{})
		if got["deep"] != tc.want {
			t.Errorf("%s: PromoteLatest(deep=%q, candidates=%q) = %q, want %q", tc.name, tc.sel, tc.candidates, got["deep"], tc.want)
		}
	}
}

// TestC1707_007_PromoteLatestNeverDowngradesAcrossDateStamps is audit H1: the
// widened LineageKey puts dated ids in the same bucket as undated or
// other-version siblings, so a date must never outrank a version. A selection
// must never be replaced by an id that is not provably newer — in EITHER
// candidate listing order, since the bucket order is the CLI's listing order.
func TestC1707_007_PromoteLatestNeverDowngradesAcrossDateStamps(t *testing.T) {
	runPromotionCases(t, []promotionCase{
		{"undated newer version vs dated older version", "gpt-5", []string{"gpt-5", "gpt-4-2024-04-09"}, "gpt-5"},
		{"undated newer version vs dated older version (dated listed first)", "gpt-5", []string{"gpt-4-2024-04-09", "gpt-5"}, "gpt-5"},
		{"both dated: higher version beats a later date", "gpt-5-2025-01-01", []string{"gpt-5-2025-01-01", "gpt-4-2025-06-01"}, "gpt-5-2025-01-01"},
		{"both dated: higher version beats a later date (older version listed first)", "gpt-5-2025-01-01", []string{"gpt-4-2025-06-01", "gpt-5-2025-01-01"}, "gpt-5-2025-01-01"},
		{"undated alias not frozen to a same-version dated snapshot", "gpt-4o", []string{"gpt-4o", "gpt-4o-2024-08-06"}, "gpt-4o"},
		{"undated alias not frozen to a same-version dated snapshot (dated listed first)", "gpt-4o", []string{"gpt-4o-2024-08-06", "gpt-4o"}, "gpt-4o"},
		{"a date is not a version number", "gpt-5", []string{"gpt-5", "gpt-2024-04-09"}, "gpt-5"},
		{"a date is not a version number (dated listed first)", "gpt-5", []string{"gpt-2024-04-09", "gpt-5"}, "gpt-5"},
	})
}

// TestC1707_008_NewestInLineageComparesVersionBeforeDate is audit H1 against
// NewestInLineage's own documented contract (numerically-newest wins; input
// order never changes the winner among distinct versions; a versioned id
// outranks an unversioned one). The version is read with the date run
// removed, and the date only breaks a tie between equal versions.
func TestC1707_008_NewestInLineageComparesVersionBeforeDate(t *testing.T) {
	cases := []struct {
		ids  []string
		want string
	}{
		{[]string{"gpt-5", "gpt-4-2024-04-09"}, "gpt-5"},
		{[]string{"gpt-4-2024-04-09", "gpt-5"}, "gpt-5"},
		{[]string{"gpt-5-2025-01-01", "gpt-4-2025-06-01"}, "gpt-5-2025-01-01"},
		{[]string{"gpt-4-2025-06-01", "gpt-5-2025-01-01"}, "gpt-5-2025-01-01"},
		{[]string{"gpt-5", "gpt-2024-04-09"}, "gpt-5"},
		{[]string{"gpt-2024-04-09", "gpt-5"}, "gpt-5"},
		// Equal versions, only one side dated: the undated id carries no date
		// to compare against, so the dated id must not displace the incumbent.
		{[]string{"gpt-4o", "gpt-4o-2024-08-06"}, "gpt-4o"},
	}
	for _, tc := range cases {
		if got := modelquery.NewestInLineage(tc.ids); got != tc.want {
			t.Errorf("NewestInLineage(%q) = %q, want %q (version must decide before the date)", tc.ids, got, tc.want)
		}
	}
}

// TestC1707_009_PromoteLatestStillPromotesAfterTheRepair guards the H1 repair
// against over-correction: same-version dated snapshots must still promote to
// the later date in either order (AC1), and the undated live lines of the
// enumerating CLIs must keep promoting exactly as before.
func TestC1707_009_PromoteLatestStillPromotesAfterTheRepair(t *testing.T) {
	runPromotionCases(t, []promotionCase{
		{"same-version dated snapshots promote to the later date", "gpt-4o-2024-08-06", []string{"gpt-4o-2024-08-06", "gpt-4o-2024-11-20"}, "gpt-4o-2024-11-20"},
		{"same-version dated snapshots promote to the later date (newer listed first)", "gpt-4o-2024-08-06", []string{"gpt-4o-2024-11-20", "gpt-4o-2024-08-06"}, "gpt-4o-2024-11-20"},
		{"undated codex line promotes within itself", "gpt-5.5", []string{"gpt-5.5", "gpt-5.6", "gpt-5.6-mini"}, "gpt-5.6"},
		{"undated ollama line promotes within its size class", "llama3.1:8b", []string{"llama3.1:8b", "llama3.1:70b", "llama3.3:8b"}, "llama3.3:8b"},
		{"component-wise numeric order, not lexicographic", "opus-4.9", []string{"opus-4.9", "opus-4.10"}, "opus-4.10"},
	})
}
