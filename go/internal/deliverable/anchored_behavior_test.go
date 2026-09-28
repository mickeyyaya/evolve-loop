package deliverable

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
)

func TestReview_UnsetBreakerPathIsRootedInEvolveDir(t *testing.T) {
	pr := t.TempDir()
	if err := os.MkdirAll(filepath.Join(pr, ".evolve"), 0o755); err != nil {
		t.Fatal(err)
	}
	r := newTestReviewer(config.StageEnforce, "", 5)
	got := r.Review(context.Background(), reviewInput("build", t.TempDir(), pr))
	if got.Approve || got.Blocks != 1 {
		t.Fatalf("review = %+v", got)
	}
	if n := readBreaker(filepath.Join(pr, ".evolve", breakerFile)); n != 1 {
		t.Fatalf("breaker = %d", n)
	}
}

func salvageSidecar(t *testing.T, lines ...string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, salvageAppliedFile), []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func salvageRecord(pattern, run string) string {
	return `{"event_type":"salvage_applied","pattern":"` + pattern + `","run":"` + run + `"}`
}

func TestSalvageSummaryLine_ExactBreakdown(t *testing.T) {
	dir := salvageSidecar(t, salvageRecord("fenced-json", salvageRunID), salvageRecord("trailing-comma", salvageRunID), salvageRecord("fenced-json", salvageRunID), salvageRecord("displaced-line", "other-run"))
	if got, want := SalvageSummaryLine(dir), "Salvaged verdicts: 3 (fenced-json=2, trailing-comma=1)"; got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestSalvageSummaryLine_TornLegacyRecordSkipped(t *testing.T) {
	dir := salvageSidecar(t, `{"pattern":"fenced-json"}`, `{torn`)
	if got, want := SalvageSummaryLine(dir), "Salvaged verdicts: 1 (fenced-json=1)"; got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestSalvageSummaryLine_OmittedOnPartialRead(t *testing.T) {
	dir := salvageSidecar(t, salvageRecord("fenced-json", salvageRunID), strings.Repeat("x", 5*1024*1024))
	if got := SalvageSummaryLine(dir); got != "" {
		t.Fatalf("got %q", got)
	}
}

func TestSalvageSummaryLine_EmptyAtZero(t *testing.T) {
	dir := salvageSidecar(t, "")
	if got := SalvageSummaryLine(dir); got != "" {
		t.Fatalf("got %q", got)
	}
}

func TestSalvageSummaryLine_ForgedPatternIsUnknown(t *testing.T) {
	dir := salvageSidecar(t, salvageRecord("gate-open", salvageRunID))
	if got, want := SalvageSummaryLine(dir), "Salvaged verdicts: 1 (unknown=1)"; got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestVerdictCandidates_SpansPerMode(t *testing.T) {
	cases := []struct {
		content     string
		stringAware bool
		want        []verdictSpan
	}{
		{`{"verdict":"}"}`, false, []verdictSpan{{0, 13}}},
		{`{"verdict":"}"}`, true, []verdictSpan{{0, 15}}},
		{`{"verdict":"a\"}"}`, true, []verdictSpan{{0, 18}}},
		{`{"verdict":"a\\n"} {"verdict":"b"}`, true, []verdictSpan{{0, 18}, {19, 34}}},
		{`{"a":1}`, true, nil},
		{`{"a":1}`, false, nil},
		{`{"verdict":"x"`, true, []verdictSpan{{0, -1}}},
		{`{"verdict":"x"`, false, []verdictSpan{{0, -1}}},
	}
	for _, c := range cases {
		if got := verdictCandidates(c.content, c.stringAware); !reflect.DeepEqual(got, c.want) {
			t.Errorf("verdictCandidates(%q, %v) = %v, want %v", c.content, c.stringAware, got, c.want)
		}
	}
}
