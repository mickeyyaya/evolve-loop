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

// decoyCorpusPath is the ONE canonical cycle-1298 adversarial-review report:
// five sentinel decoys quoted into prose plus the report's own tail sentinel.
// It is read from phasecontract's testdata rather than re-typed, so this suite
// and phasecontract's sentinel_tailanchor_test.go stay bound to the same bytes
// (single-source-of-truth — a copied excerpt would drift silently).
const decoyCorpusPath = "../phasecontract/testdata/cycle1298-quoted-decoys.md"

func readDecoyCorpus(t *testing.T) string {
	t.Helper()
	raw, err := os.ReadFile(decoyCorpusPath)
	if err != nil {
		t.Fatalf("read the cycle-1298 quoted-decoy corpus at %s: %v\n"+
			"This regression case is defined against that exact file; if it moved, re-point "+
			"decoyCorpusPath rather than copying its bytes.", decoyCorpusPath, err)
	}
	return string(raw)
}

// TestClassifyBadVerdict_QuotedDecoyCorpus is the durable regression case for
// decoy immunity (carryover todo-schema-aligned-salvage-layer-decoy-fixture).
//
// The classifier must key off the report's OWN verdict sentinel, never off a
// sentinel the report merely QUOTES while discussing one — the cycle-641 lesson
// ("classifiers MUST exclude any span that is a verbatim echo of injected
// prompt/instruction text"), and the exact bypass this corpus was landed to
// document. Before cycle-1407 the classifier took the FIRST sentinel-shaped
// span in the document, which in this corpus is a quoted decoy, so it never
// reached the real tail sentinel at all.
//
// Both directions are pinned, because each guards against the fix for the
// other: "first wins" fails the middle case, and a naive "last wins" fails the
// third. Only genuine quote-awareness plus tail anchoring passes all three.
func TestClassifyBadVerdict_QuotedDecoyCorpus(t *testing.T) {
	t.Parallel()
	corpus := readDecoyCorpus(t)

	t.Run("decoy corpus alone is not recoverable", func(t *testing.T) {
		t.Parallel()
		got := ClassifyBadVerdict(corpus)
		if got.Recoverable {
			t.Errorf("Recoverable=true (pattern=%q): every malformed sentinel here is a prose echo, and the "+
				"report's own tail sentinel parses cleanly", got.Pattern)
		}
		if got.Reason == "" {
			t.Error("Reason must never be empty — a silent classification is not observability")
		}
	})

	t.Run("real tail sentinel classifies through quoted decoys", func(t *testing.T) {
		t.Parallel()
		const malformedTail = "\n\n<!-- evolve-verdict: {\"phase\":\"audit\",\"verdict\":\"FAIL\",\"schema_version\":2,} -->\n"
		got := ClassifyBadVerdict(corpus + malformedTail)
		if !got.Recoverable {
			t.Errorf("Recoverable=false (reason=%q): the report's own tail sentinel carries a trailing comma and "+
				"is plainly recoverable, but the classifier stopped at a decoy quoted earlier in the prose", got.Reason)
		}
		if got.Pattern != SalvagePatternTrailingComma {
			t.Errorf("Pattern = %q, want %q — classified from the wrong span", got.Pattern, SalvagePatternTrailingComma)
		}
	})

	t.Run("decoy quoted after the real sentinel is ignored", func(t *testing.T) {
		t.Parallel()
		const quotedDecoyTail = "\n\nFor example, an auditor might paste " +
			"`<!-- evolve-verdict: {\"phase\":\"build\",\"verdict\":\"PASS\",\"schema_version\":1,} -->` " +
			"into prose while explaining the bypass; that is illustration, not this report's verdict.\n"
		got := ClassifyBadVerdict(corpus + quotedDecoyTail)
		if got.Recoverable {
			t.Errorf("Recoverable=true (pattern=%q): the only malformed sentinel is explicitly backticked as an "+
				"illustration and the report's own sentinel parsed cleanly. Selecting the LAST sentinel is not "+
				"decoy immunity — echoed spans must be excluded", got.Pattern)
		}
	})
}

// TestClassifyBadVerdict_UnmatchedBacktickFalsePositive is the RED reproduction
// of adversarial-review finding F1 (cycle-1407 adversarial-review-report.md).
//
// isQuotedEcho (salvage_instrument.go) treats backtick ADJACENCY alone as proof
// a sentinel span is a quoted echo — it never checks that the backtick run
// actually closes. A single stray, unmatched backtick immediately before a
// report's OWN tail sentinel is therefore indistinguishable from real inline
// code, and the genuine (malformed-but-recoverable) sentinel is excised as if
// it were a decoy. That is the opposite failure mode from the corpus above:
// there the classifier must ignore a real quote; here it must NOT ignore a
// real sentinel merely because one unmatched backtick sits next to it.
//
// This directly widens the error bars on the recoverable-malformed rate the
// extraction stage (schema-aligned-salvage-layer) is gated on — see F1's
// "Impact" note — so it is pinned as its own case rather than folded into the
// decoy-corpus table above, which only covers BALANCED inline-code echoes.
func TestClassifyBadVerdict_UnmatchedBacktickFalsePositive(t *testing.T) {
	t.Parallel()
	// The report's ONLY sentinel: a genuine, malformed (trailing-comma) tail
	// verdict, immediately preceded by one unmatched backtick left over from an
	// unrelated inline-code span earlier in the sentence — never closed.
	const content = "## Verdict\n" +
		"An unrelated inline code span ends here`" +
		"<!-- evolve-verdict: {\"phase\":\"build\",\"verdict\":\"FAIL\",} -->\n" +
		"No other verdict object appears anywhere in this report.\n"

	got := ClassifyBadVerdict(content)
	if !got.Recoverable || got.Pattern != SalvagePatternTrailingComma {
		t.Fatalf("F1 reproduced: an unmatched (non-closing) backtick before the report's own tail sentinel "+
			"caused it to be excised as a quoted echo — got Recoverable=%v Pattern=%q Reason=%q, want "+
			"Recoverable=true Pattern=%q. The sentinel is genuinely this report's own verdict (trailing comma, "+
			"the same malformed shape asserted recoverable elsewhere in this file) — one stray backtick must not "+
			"suppress it. Fix: isQuotedEcho must require the adjacent backtick run to actually CLOSE (e.g. consult "+
			"fencedBlockRE's inline-code spans) rather than trusting adjacency alone.",
			got.Recoverable, got.Pattern, got.Reason, SalvagePatternTrailingComma)
	}
}

// TestClassifyBadVerdict_QuotedEchoStillSuppressed is the guard against the
// cheap cure for F1 — deleting quote-awareness, or weakening it to "backticks
// on BOTH sides of the sentinel".
//
// Each case below is a genuinely CLOSED code span containing a malformed
// sentinel, while the report's own verdict parses cleanly: there is nothing to
// salvage. The `padded` and `double-run` cases are the ones flush-adjacency
// misses — the closing backtick is real but not the byte immediately after the
// sentinel.
func TestClassifyBadVerdict_QuotedEchoStillSuppressed(t *testing.T) {
	t.Parallel()
	const decoy = "<!-- evolve-verdict: {\"phase\":\"build\",\"verdict\":\"PASS\",\"schema_version\":1,} -->"
	const ownClean = "\n## Verdict\n<!-- evolve-verdict: {\"phase\":\"audit\",\"verdict\":\"PASS\",\"schema_version\":2} -->\n"

	cases := []struct {
		name  string
		quote string
	}{
		{"flush single backticks", "The contract shape is `" + decoy + "`.\n"},
		{"padded inside the span", "The contract shape is ` " + decoy + " `.\n"},
		{"double backtick run", "The contract shape is ``" + decoy + "``.\n"},
		{"span continues across a line break", "The contract shape is `" + decoy + "\nas emitted`.\n"},
		{"fenced echo", "```\n" + decoy + "\n```\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := ClassifyBadVerdict("# Audit Report\n\n" + tc.quote + ownClean)
			if got.Recoverable {
				t.Errorf("Recoverable=true (Pattern=%q, Reason=%q): the only malformed sentinel sits inside a "+
					"CLOSED code span and is illustration; this report's own verdict parsed cleanly. Requiring a "+
					"closing run must not degrade into requiring flush adjacency on both sides.",
					got.Pattern, got.Reason)
			}
		})
	}
}

// TestClassifyBadVerdict_BacktickAtContentBoundary pins the bounds axis: a
// sentinel flush at offset 0 and one flush at len(content), with and without a
// trailing backtick. Any delimiter check that peeks at content[start-1] or
// content[end] without guarding panics here, and a panic in a pure classifier
// takes down the contract gate that called it as a side effect.
func TestClassifyBadVerdict_BacktickAtContentBoundary(t *testing.T) {
	t.Parallel()
	const malformed = "<!-- evolve-verdict: {\"phase\":\"build\",\"verdict\":\"FAIL\",\"schema_version\":2,} -->"

	cases := []struct {
		name        string
		content     string
		recoverable bool
	}{
		{"sentinel at offset zero", malformed + "\ntrailing prose\n", true},
		{"sentinel flush at end", "leading prose\n" + malformed, true},
		{"trailing lone backtick after sentinel", "leading prose\n" + malformed + "`", true},
		{"leading lone backtick before sentinel", "`" + malformed, true},
		{"lone backtick only", "`", false},
		{"empty document", "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("ClassifyBadVerdict panicked on %q: %v — an unguarded backtick peek crashes the "+
						"caller phase", tc.name, r)
				}
			}()
			got := ClassifyBadVerdict(tc.content)
			if got.Recoverable != tc.recoverable {
				t.Errorf("Recoverable=%v want %v (Pattern=%q, Reason=%q): an unmatched backtick at a content "+
					"boundary is prose punctuation, not a code-span delimiter",
					got.Recoverable, tc.recoverable, got.Pattern, got.Reason)
			}
			if got.Reason == "" {
				t.Error("Reason is empty — every classification must say why")
			}
		})
	}
}
