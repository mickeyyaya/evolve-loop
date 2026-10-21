//go:build acs

package cycle1707

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/modelquery"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

func TestC1707_001_DatedSnapshotsShareLineageKey(t *testing.T) {
	a, b := modelquery.LineageKey("gpt-4o-2024-08-06"), modelquery.LineageKey("gpt-4o-2024-11-20")
	if a != b {
		t.Errorf("LineageKey(gpt-4o-2024-08-06)=%q != LineageKey(gpt-4o-2024-11-20)=%q — dated snapshots of the same line must share a key", a, b)
	}
}

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

func TestC1707_003_NewestInLineageOrdersDatedSnapshots(t *testing.T) {
	got := modelquery.NewestInLineage([]string{"gpt-4o-2024-08-06", "gpt-4o-2024-11-20"})
	if got != "gpt-4o-2024-11-20" {
		t.Errorf("NewestInLineage([gpt-4o-2024-08-06, gpt-4o-2024-11-20]) = %q, want gpt-4o-2024-11-20 (the later date)", got)
	}
	got2 := modelquery.NewestInLineage([]string{"gpt-4o-2024-11-20", "gpt-4o-2024-08-06"})
	if got2 != "gpt-4o-2024-11-20" {
		t.Errorf("NewestInLineage([gpt-4o-2024-11-20, gpt-4o-2024-08-06]) = %q, want gpt-4o-2024-11-20 (order must not flip the winner)", got2)
	}
}

func TestC1707_004_KnownLimitationPinMigratedToEquality(t *testing.T) {
	a, b := modelquery.LineageKey("gpt-4o-2024-08-06"), modelquery.LineageKey("gpt-4o-2024-11-20")
	if a != b {
		t.Errorf("known-limitation pin not yet migrated: LineageKey(gpt-4o-2024-08-06)=%q still differs from LineageKey(gpt-4o-2024-11-20)=%q", a, b)
	}
}

const oldFingerprintV1 = "sha256:441aa02fd68539bda9f2826334d8165598de47251384bf564244f616b8722a3b"

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

func TestC1707_006_DecisionSurfacePinTracksTheChange(t *testing.T) {
	const modelqueryPkg = "github.com/mickeyyaya/evolve-loop/go/internal/modelquery"
	stdout, stderr, code, err := acsassert.SubprocessOutput(
		"go", "test", "-count=1", "-run", "^TestDecisionVersion_PinnedToAlgorithmSurface$",
		"-v", modelqueryPkg)
	out := stdout + stderr
	if code < 0 {
		t.Fatalf("go test failed to launch for %s: code=%d err=%v\n%s", modelqueryPkg, code, err, out)
	}
	if code != 0 {
		t.Errorf("TestDecisionVersion_PinnedToAlgorithmSurface failed (decisionSurfacePin out of sync with the decision surface):\n%s", out)
	}
}

type promotionCase struct {
	name       string
	sel        string
	candidates []string
	want       string
}

func runPromotionCases(t *testing.T, cases []promotionCase) {
	t.Helper()
	for _, tc := range cases {
		got := modelquery.PromoteLatest(map[string]string{"deep": tc.sel}, tc.candidates, modelquery.FreshnessPolicy{})
		if got["deep"] != tc.want {
			t.Errorf("%s: PromoteLatest(deep=%q, candidates=%q) = %q, want %q", tc.name, tc.sel, tc.candidates, got["deep"], tc.want)
		}
	}
}

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
		{[]string{"gpt-4o", "gpt-4o-2024-08-06"}, "gpt-4o"},
	}
	for _, tc := range cases {
		if got := modelquery.NewestInLineage(tc.ids); got != tc.want {
			t.Errorf("NewestInLineage(%q) = %q, want %q (version must decide before the date)", tc.ids, got, tc.want)
		}
	}
}

func TestC1707_009_PromoteLatestStillPromotesAfterTheRepair(t *testing.T) {
	runPromotionCases(t, []promotionCase{
		{"same-version dated snapshots promote to the later date", "gpt-4o-2024-08-06", []string{"gpt-4o-2024-08-06", "gpt-4o-2024-11-20"}, "gpt-4o-2024-11-20"},
		{"same-version dated snapshots promote to the later date (newer listed first)", "gpt-4o-2024-08-06", []string{"gpt-4o-2024-11-20", "gpt-4o-2024-08-06"}, "gpt-4o-2024-11-20"},
		{"undated codex line promotes within itself", "gpt-5.5", []string{"gpt-5.5", "gpt-5.6", "gpt-5.6-mini"}, "gpt-5.6"},
		{"undated ollama line promotes within its size class", "llama3.1:8b", []string{"llama3.1:8b", "llama3.1:70b", "llama3.3:8b"}, "llama3.3:8b"},
		{"component-wise numeric order, not lexicographic", "opus-4.9", []string{"opus-4.9", "opus-4.10"}, "opus-4.10"},
	})
}
