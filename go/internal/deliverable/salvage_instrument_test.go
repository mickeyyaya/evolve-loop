package deliverable

// salvage_instrument_test.go — names AND exercises every exported symbol added
// by the salvage-instrumentation layer (export-naming floor, ADR-0069):
//
//	type  SalvagePattern, BadVerdictClassification
//	const SalvagePatternNone / SalvagePatternFencedJSON /
//	      SalvagePatternTrailingComma / SalvagePatternDisplaced
//	func  ClassifyBadVerdict
//
// The ACS predicates (go/acs/cycle1389) drive the same symbols through the real
// VerifyWithStage/Reviewer path; this suite covers the classifier's PRECEDENCE
// and negative axes, which the acceptance criteria do not reach.

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasespec"
)

func TestClassifyBadVerdict_Patterns(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name        string
		content     string
		recoverable bool
		want        SalvagePattern
	}{
		{
			name:        "fenced json",
			content:     "## Verdict\n```json\n{\"phase\":\"audit\",\"verdict\":\"PASS\"}\n```\n",
			recoverable: true,
			want:        SalvagePatternFencedJSON,
		},
		{
			name:        "trailing comma in sentinel payload",
			content:     "## Verdict\n<!-- evolve-verdict: {\"phase\":\"audit\",\"verdict\":\"PASS\",} -->\n",
			recoverable: true,
			want:        SalvagePatternTrailingComma,
		},
		{
			name:        "bare displaced object",
			content:     "## Verdict\nit then states:\n{\"phase\":\"audit\",\"verdict\":\"PASS\"}\nand stops.\n",
			recoverable: true,
			want:        SalvagePatternDisplaced,
		},
		{
			name:        "prose only, genuinely absent",
			content:     "## Verdict\ninconclusive musings, no token, no structure at all\n",
			recoverable: false,
			want:        SalvagePatternNone,
		},
		{
			name:        "empty deliverable",
			content:     "",
			recoverable: false,
			want:        SalvagePatternNone,
		},
		{
			// A fence with no verdict key must NOT be mistaken for a displaced
			// object by the fence-stripping step, and must not claim fenced-json.
			name:        "fenced block without a verdict key",
			content:     "## Notes\n```go\nfoo := bar{baz: 1}\n```\nnothing else\n",
			recoverable: false,
			want:        SalvagePatternNone,
		},
		{
			// Precedence: a well-formed-but-out-of-vocabulary sentinel is a real
			// bad_verdict, but it is NOT one of the three shapes — claiming it
			// recoverable would inflate the baseline this layer exists to measure.
			name:        "sentinel parses but verdict is out of vocabulary",
			content:     "## Verdict\n<!-- evolve-verdict: {\"phase\":\"audit\",\"verdict\":\"MAYBE\"} -->\n",
			recoverable: false,
			want:        SalvagePatternNone,
		},
		{
			// Precedence: the sentinel branch wins over a fence appearing later.
			name:        "sentinel takes precedence over a trailing fence",
			content:     "<!-- evolve-verdict: {\"verdict\":\"PASS\",} -->\n```json\n{\"verdict\":\"FAIL\"}\n```\n",
			recoverable: true,
			want:        SalvagePatternTrailingComma,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var got BadVerdictClassification = ClassifyBadVerdict(tc.content)
			if got.Recoverable != tc.recoverable {
				t.Errorf("Recoverable = %v, want %v (got %+v)", got.Recoverable, tc.recoverable, got)
			}
			if got.Pattern != tc.want {
				t.Errorf("Pattern = %q, want %q", got.Pattern, tc.want)
			}
			if got.Reason == "" {
				t.Error("Reason must never be empty — a silent classification is not observability")
			}
		})
	}
}

// TestClassifyBadVerdict_IsPure guards the layer's defining property: the
// classifier is measurement, not extraction. It must never be a path by which a
// verdict is invented — so it returns only a description, and the Result it was
// derived from is not an input it can mutate.
func TestClassifyBadVerdict_IsPure(t *testing.T) {
	t.Parallel()
	const content = "```json\n{\"verdict\":\"PASS\"}\n```\n"
	first := ClassifyBadVerdict(content)
	second := ClassifyBadVerdict(content)
	if first != second {
		t.Errorf("ClassifyBadVerdict must be deterministic; %+v != %+v", first, second)
	}
}

// TestRecordBadVerdictBaseline_OnlyOnBadVerdict is the negative axis of the
// wiring: a non-verdict violation (a missing section) must write NO baseline
// record, or the measured rate is a count of every contract failure instead of
// the salvage layer's addressable population.
func TestRecordBadVerdictBaseline_OnlyOnBadVerdict(t *testing.T) {
	t.Parallel()
	ws, pr := t.TempDir(), t.TempDir()
	// Well-formed sentinel, but the build contract's required "## Changes"
	// section is absent ⇒ a block with no bad_verdict.
	writeFile(t, ws, "build-report.md", "## Something Else\n<!-- evolve-verdict: {\"phase\":\"build\",\"verdict\":\"PASS\"} -->\n")

	r := NewReviewerWithCatalogStageReportSize(config.StageEnforce, phasespec.Catalog{}, config.StageEnforce, config.StageOff, 0)
	got := r.Review(context.Background(), core.ReviewInput{Phase: "build", Workspace: ws, ProjectRoot: pr})
	if got.Approve {
		t.Fatalf("precondition: a missing required section must block; got Approve=true")
	}

	if _, err := os.Stat(filepath.Join(pr, ".evolve", baselineFile)); !os.IsNotExist(err) {
		t.Errorf("no bad_verdict violation occurred — the baseline sidecar must not exist; stat err=%v", err)
	}
}

// TestRecordBadVerdictBaseline_RecordShape asserts the JSONL record carries the
// fields §7's counts are tallied from, so the doc's aggregation can never drift
// from what the writer emits.
func TestRecordBadVerdictBaseline_RecordShape(t *testing.T) {
	t.Parallel()
	ws, pr := t.TempDir(), t.TempDir()
	writeFile(t, ws, "audit-report.md", "## Verdict\n```json\n{\"phase\":\"audit\",\"verdict\":\"PASS\"}\n```\n")

	r := NewReviewerWithCatalogStageReportSize(config.StageEnforce, phasespec.Catalog{}, config.StageEnforce, config.StageOff, 0)
	r.Review(context.Background(), core.ReviewInput{Phase: "audit", Workspace: ws, ProjectRoot: pr})

	data, err := os.ReadFile(filepath.Join(pr, ".evolve", baselineFile))
	if err != nil {
		t.Fatalf("read baseline sidecar: %v", err)
	}
	var rec map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(string(data))), &rec); err != nil {
		t.Fatalf("baseline record is not valid JSON: %v", err)
	}
	for k, want := range map[string]any{
		"event_type":  "bad_verdict_classified",
		"phase":       "audit",
		"recoverable": true,
		"pattern":     string(SalvagePatternFencedJSON),
	} {
		if rec[k] != want {
			t.Errorf("record[%q] = %v, want %v", k, rec[k], want)
		}
	}
	if rec["artifact_path"] == "" || rec["reason"] == "" {
		t.Errorf("artifact_path and reason must be populated; got %+v", rec)
	}
}
