package deliverable

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
			name:        "fenced block without a verdict key",
			content:     "## Notes\n```go\nfoo := bar{baz: 1}\n```\nnothing else\n",
			recoverable: false,
			want:        SalvagePatternNone,
		},
		{
			name:        "sentinel parses but verdict is out of vocabulary",
			content:     "## Verdict\n<!-- evolve-verdict: {\"phase\":\"audit\",\"verdict\":\"MAYBE\"} -->\n",
			recoverable: false,
			want:        SalvagePatternNone,
		},
		{
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

// Purity is checked as determinism: the classifier takes no Result it could mutate.
func TestClassifyBadVerdict_IsPure(t *testing.T) {
	t.Parallel()
	const content = "```json\n{\"verdict\":\"PASS\"}\n```\n"
	first := ClassifyBadVerdict(content)
	second := ClassifyBadVerdict(content)
	if first != second {
		t.Errorf("ClassifyBadVerdict must be deterministic; %+v != %+v", first, second)
	}
}

func TestRecordBadVerdictBaseline_OnlyOnBadVerdict(t *testing.T) {
	t.Parallel()
	ws, pr := t.TempDir(), t.TempDir()
	// A well-formed sentinel with the build contract's "## Changes" section absent: a block without bad_verdict.
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

// strayBacktickPreamble puts a lone, never-closed backtick in prose ahead of the report's own verdict.
const strayBacktickPreamble = "The auditor noted a stray ` tick in the transcript and moved on.\n\n"

// Each shape is classified as written and behind the preamble; the two results must match.
func TestClassifyBadVerdict_UnmatchedBacktickDoesNotMisclassify(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name        string
		content     string
		recoverable bool
		want        SalvagePattern
	}{
		{
			name:        "sentinel with trailing comma",
			content:     "# Audit Report\n\n<!-- evolve-verdict: {\"phase\":\"audit\",\"verdict\":\"FAIL\",} -->\n",
			recoverable: true,
			want:        SalvagePatternTrailingComma,
		},
		{
			name:        "fenced json",
			content:     "# Audit Report\n\n```json\n{\"verdict\":\"PASS\"}\n```\n",
			recoverable: true,
			want:        SalvagePatternFencedJSON,
		},
		{
			name:        "displaced line",
			content:     "# Audit Report\n\nThe verdict object is {\"verdict\":\"PASS\"} inline in prose.\n",
			recoverable: true,
			want:        SalvagePatternDisplaced,
		},
		{
			name:        "genuinely absent",
			content:     "# Audit Report\n\nProse only. No verdict payload of any kind was emitted.\n",
			recoverable: false,
			want:        SalvagePatternNone,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			control := ClassifyBadVerdict(tc.content)
			if control.Recoverable != tc.recoverable || control.Pattern != tc.want {
				t.Fatalf("control classification drifted: got{recoverable=%v pattern=%q}, want{recoverable=%v pattern=%q}",
					control.Recoverable, control.Pattern, tc.recoverable, tc.want)
			}
			poisoned := ClassifyBadVerdict(strayBacktickPreamble + tc.content)
			if poisoned.Recoverable != control.Recoverable || poisoned.Pattern != control.Pattern {
				t.Errorf("a stray unmatched backtick perturbed classification — control{recoverable=%v pattern=%q} vs poisoned{recoverable=%v pattern=%q}",
					control.Recoverable, control.Pattern, poisoned.Recoverable, poisoned.Pattern)
			}
			if poisoned.Reason == "" {
				t.Error("Reason must never be empty — a silent classification is not observability")
			}
		})
	}

	for _, content := range []string{"", "`", "```", strayBacktickPreamble} {
		if got := ClassifyBadVerdict(content); got.Recoverable || got.Pattern != SalvagePatternNone {
			t.Errorf("content %q classified {recoverable=%v pattern=%q} — a verdict-free deliverable is not recoverable",
				content, got.Recoverable, got.Pattern)
		}
	}
}

// The symbol scan skips test files, which name the banned helpers.
func TestNoQuotedEchoRegression(t *testing.T) {
	t.Parallel()

	const poisoned = strayBacktickPreamble + "<!-- evolve-verdict: {\"phase\":\"audit\",\"verdict\":\"FAIL\",} -->\n"
	got := ClassifyBadVerdict(poisoned)
	if !got.Recoverable || got.Pattern != SalvagePatternTrailingComma {
		t.Errorf("the cycle-1406/1407 defect is back: a recoverable trailing-comma sentinel behind a stray backtick "+
			"classified {recoverable=%v pattern=%q}, want {true %q}", got.Recoverable, got.Pattern, SalvagePatternTrailingComma)
	}

	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("read package dir: %v", err)
	}
	scanned := 0
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		src, err := os.ReadFile(name)
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		scanned++
		for _, sym := range []string{"isQuotedEcho", "insideStringLiteral"} {
			if strings.Contains(string(src), sym) {
				t.Errorf("%s: %s reintroduced — that is the cycle-1406/1407 backtick-adjacency defect returning", name, sym)
			}
		}
	}
	if scanned == 0 {
		t.Fatal("scanned no production sources — the tripwire would pass vacuously")
	}
}

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

// decoyCorpusPath is read rather than re-typed, so this suite and phasecontract's tests share one set of bytes.
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

// First-match selection fails the second subtest and last-match alone fails the third.
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

func TestClassifyBadVerdict_UnmatchedBacktickFalsePositive(t *testing.T) {
	t.Parallel()
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

// The padded and double-run cases are the ones a flush-adjacency check misses.
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
