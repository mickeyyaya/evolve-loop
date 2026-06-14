//go:build acs

// Package cycle336 materializes the cycle-336 acceptance criteria for the single
// behavior-preserving DRY task committed to triage `## top_n`:
//
//	aggregator-dedup-helpers — three behavior-preserving simplifications inside
//	    go/internal/aggregator/aggregator.go:
//	      1. remove the dead write-only `anyWarn` variable from writeVerdict
//	         (suppressed today by `_ = anyWarn`; the verdict depends only on
//	         anyFail/allPass, so removal is a pure no-op);
//	      2. extract `scanFirstCapture(path string, re *regexp.Regexp) string`
//	         and rewrite extractVerdict + extractScore to delegate;
//	      3. extract `appendWorkerSections(b *strings.Builder, heading string,
//	         workers []string, readFile func(string)([]byte,error)) error` and
//	         rewrite the four write* functions (writeConcat / writeVerdict /
//	         writePlanReview / writeCrossCLIVote) to call it.
//
// Floor binding (R9.3 / cycle-280 lesson). Triage `## top_n` for cycle 336 holds
// exactly this one task; it is gated here. The nine carryoverTodos are
// triage-DROPPED historical infra/phase-delivery failures and get ZERO
// predicates (deferred-floor starvation, cycle-280). No coverage floor is
// committed, so no floor predicate.
//
// Predicate design (cycle-85 lesson — every load-bearing gate EXERCISES the
// system under test; structural source assertions are config-check WAIVED):
//
//   - C336_001 is BEHAVIORAL: it imports the real aggregator package and drives
//     the exported Aggregate() entry point end-to-end across the merge modes the
//     refactor touches. The three changes are behavior-PRESERVING, so this gate
//     is pre-existing GREEN today and acts as the anti-regression lock — a
//     builder edit that changed any observable consequence (a verdict, an exit
//     code, a worker-heading level) would flip it RED. It pins precisely the
//     surfaces the refactor risks: the WARN/default verdict paths (the anyWarn
//     removal must not change them) and both worker-heading levels ("## Worker:"
//     and "### Worker:" — appendWorkerSections must reproduce both).
//
//   - C336_002 / C336_003 / C336_004 / C336_005 are the structural
//     dedup-completion gates (config-check waived — see inline waivers). They
//     assert the SOURCE-STRUCTURE outcome of the code-reduction goal: the dead
//     variable is gone, the two helpers exist AND are delegated to (call sites,
//     not just a definition), and the file is strictly shorter than the 466-line
//     baseline. These carry the RED today (anyWarn present ×4, helpers absent,
//     466 lines). The behavioral weight is carried by C336_001.
//
// AC map (1:1 with scout-report "Acceptance Criteria Summary"):
//
//	AC1 anyWarn absent from aggregator.go                         → C336_002
//	AC2 scanFirstCapture defined; extractVerdict/extractScore delegate → C336_003 (def + ≥2 call sites)
//	AC3 appendWorkerSections defined and called (≥4 occurrences)  → C336_004
//	AC4 aggregator test suite green (behavior preserved)          → C336_001 (drives Aggregate end-to-end)
//	AC5 aggregator.go strictly < 466 lines                        → C336_005
//	AC6 cycle-336 ACS predicates pass                             → this package (meta)
package cycle336

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/aggregator"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const aggregatorSrcRel = "go/internal/aggregator/aggregator.go"

// aggregatorBaselineLines is the pre-refactor line count of aggregator.go.
// AC5 requires a strict net reduction below this.
const aggregatorBaselineLines = 466

// fixedNow pins the timestamp so Aggregate output is deterministic.
func fixedNow() func() time.Time {
	return func() time.Time { return time.Date(2026, 6, 14, 12, 0, 0, 0, time.UTC) }
}

// writeWorker creates a worker artifact under dir and returns its path.
func writeWorker(t *testing.T, dir, name, body string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatalf("write worker %s: %v", name, err)
	}
	return p
}

// runAggregate drives the real Aggregate entry point and returns (rc, output).
func runAggregate(t *testing.T, phase string, workers ...string) (int, string) {
	t.Helper()
	out := filepath.Join(t.TempDir(), "agg.md")
	rc := aggregator.Aggregate(aggregator.Inputs{
		Phase:   phase,
		Output:  out,
		Workers: workers,
		Now:     fixedNow(),
	}, io.Discard)
	body, err := os.ReadFile(out)
	if err != nil {
		// Usage-error paths legitimately leave no output; return empty body.
		return rc, ""
	}
	return rc, string(body)
}

// ---------------------------------------------------------------------------
// C336_001 — behavior preserved across the merge modes the refactor touches.
//
// Exercises the real exported Aggregate() (no source grep). Pins the exact
// surfaces the three changes risk:
//   - verdict aggregation incl. the WARN and default(missing) paths — the dead
//     anyWarn removal MUST NOT alter any verdict or exit code;
//   - the concat ("## Worker:") and cross-cli-vote ("### Worker:") heading
//     levels — appendWorkerSections must reproduce BOTH (the only inter-call
//     variation), so a hard-coded heading regresses one of them.
// Pre-existing GREEN today (refactor is behavior-preserving); this is the
// anti-regression lock for Builder.
// ---------------------------------------------------------------------------
func TestC336_001_AggregateBehaviourPreserved(t *testing.T) {
	dir := t.TempDir()

	// --- audit verdict paths (anyWarn-removal sensitive) ---
	pass1 := writeWorker(t, dir, "p1.md", "Verdict: PASS\n# a\nok")
	pass2 := writeWorker(t, dir, "p2.md", "verdict: pass\n# b\nfine")
	fail := writeWorker(t, dir, "f.md", "Verdict: FAIL\nbroken")
	warn := writeWorker(t, dir, "w.md", "Verdict: WARN\nminor")
	noverdict := writeWorker(t, dir, "nv.md", "no verdict line, just prose")

	verdictCases := []struct {
		name     string
		workers  []string
		wantRC   int
		wantHead string
	}{
		{"all-pass", []string{pass1, pass2}, aggregator.ExitOK, "Verdict: PASS"},
		{"any-fail-vetoes", []string{pass1, fail}, aggregator.ExitVerdictBad, "Verdict: FAIL"},
		{"pass-plus-warn", []string{pass1, warn}, aggregator.ExitOK, "Verdict: WARN"},
		{"missing-verdict-is-warn", []string{noverdict}, aggregator.ExitOK, "Verdict: WARN"},
	}
	for _, tc := range verdictCases {
		rc, body := runAggregate(t, "audit", tc.workers...)
		if rc != tc.wantRC {
			t.Errorf("audit %s: rc=%d, want %d", tc.name, rc, tc.wantRC)
		}
		if !strings.HasPrefix(body, tc.wantHead) {
			t.Errorf("audit %s: body must start with %q, got:\n%s", tc.name, tc.wantHead, body)
		}
	}

	// --- concat mode: "## Worker:" headings + bodies present ---
	c1 := writeWorker(t, dir, "alpha.md", "findings-alpha")
	c2 := writeWorker(t, dir, "beta.md", "findings-beta")
	rc, body := runAggregate(t, "scout", c1, c2)
	if rc != aggregator.ExitOK {
		t.Fatalf("scout concat: rc=%d, want %d", rc, aggregator.ExitOK)
	}
	for _, want := range []string{"## Worker: alpha", "## Worker: beta", "findings-alpha", "findings-beta"} {
		if !strings.Contains(body, want) {
			t.Errorf("scout concat: missing %q in:\n%s", want, body)
		}
	}

	// --- cross-cli-vote mode: "### Worker:" heading (different level) ---
	v1 := writeWorker(t, dir, "claude.md", "Verdict: PASS\n")
	v2 := writeWorker(t, dir, "gemini.md", "Verdict: PASS\n")
	rc, body = runAggregate(t, "cross-cli-vote", v1, v2)
	if rc != aggregator.ExitOK {
		t.Fatalf("cross-cli-vote: rc=%d, want %d", rc, aggregator.ExitOK)
	}
	if !strings.Contains(body, "### Worker: claude") {
		t.Errorf("cross-cli-vote: missing '### Worker: claude' (heading level must stay ###) in:\n%s", body)
	}
	if strings.Contains(body, "## Worker: claude") && !strings.Contains(body, "### Worker: claude") {
		t.Errorf("cross-cli-vote: heading downgraded from ### to ##:\n%s", body)
	}
}

// ---------------------------------------------------------------------------
// C336_002 — dead `anyWarn` removed (AC1).
//
// acs-predicate: config-check — asserts the SOURCE-STRUCTURE outcome of the
// code-reduction goal: the write-only `anyWarn` variable (and its `_ = anyWarn`
// suppressor) no longer appear anywhere in aggregator.go. Behavioral proof that
// the verdict paths are unchanged lives in C336_001. Waiver per the Predicate
// Quality table (Waived grep / config-check).
// RED today: anyWarn appears 4× (decl + 2 assignments + the `_ = anyWarn` line).
// ---------------------------------------------------------------------------
func TestC336_002_DeadAnyWarnRemoved(t *testing.T) {
	root := acsassert.RepoRoot(t)
	src := filepath.Join(root, aggregatorSrcRel)
	if n := acsassert.CountOccurrencesAny(src, "anyWarn"); n != 0 {
		t.Errorf("RED: aggregator.go still references anyWarn %d time(s) — remove the dead write-only var (want 0)", n)
	}
}

// ---------------------------------------------------------------------------
// C336_003 — scanFirstCapture helper exists AND is delegated to (AC2).
//
// acs-predicate: config-check — asserts the dedup is real: the helper is defined
// AND both extractVerdict and extractScore call it (def + ≥2 call sites ⇒ ≥3
// occurrences of the identifier). A lone definition with no callers would not
// dedup the boilerplate. Behavioral proof that extract*/Aggregate still work
// lives in C336_001. Waiver per the Predicate Quality table (config-check).
// RED today: scanFirstCapture appears 0×.
// ---------------------------------------------------------------------------
func TestC336_003_ScanFirstCaptureExtractedAndDelegated(t *testing.T) {
	root := acsassert.RepoRoot(t)
	src := filepath.Join(root, aggregatorSrcRel)
	if !acsassert.FileContains(t, src, "func scanFirstCapture") {
		t.Errorf("RED: aggregator.go missing `func scanFirstCapture` helper")
	}
	if n := acsassert.CountOccurrencesAny(src, "scanFirstCapture"); n < 3 {
		t.Errorf("RED: scanFirstCapture appears %d× — want ≥3 (1 def + ≥2 call sites in extractVerdict/extractScore)", n)
	}
}

// ---------------------------------------------------------------------------
// C336_004 — appendWorkerSections helper exists AND is delegated to (AC3).
//
// acs-predicate: config-check — asserts the dedup is real: the helper is defined
// AND called from at least three of the four write* functions (def + ≥3 call
// sites ⇒ ≥4 occurrences, matching scout's AC3 grep ≥4). Behavioral proof that
// both heading levels still render lives in C336_001. Waiver per the Predicate
// Quality table (config-check).
// RED today: appendWorkerSections appears 0×.
// ---------------------------------------------------------------------------
func TestC336_004_AppendWorkerSectionsExtractedAndDelegated(t *testing.T) {
	root := acsassert.RepoRoot(t)
	src := filepath.Join(root, aggregatorSrcRel)
	if !acsassert.FileContains(t, src, "func appendWorkerSections") {
		t.Errorf("RED: aggregator.go missing `func appendWorkerSections` helper")
	}
	if n := acsassert.CountOccurrencesAny(src, "appendWorkerSections"); n < 4 {
		t.Errorf("RED: appendWorkerSections appears %d× — want ≥4 (1 def + ≥3 call sites across write* funcs)", n)
	}
}

// ---------------------------------------------------------------------------
// C336_005 — net line reduction below the pre-refactor baseline (AC5).
//
// acs-predicate: config-check — the code-reduction goal requires aggregator.go
// to be strictly shorter than its 466-line baseline after the three
// dedup/removal changes. Waiver per the Predicate Quality table (config-check).
// RED today: the file is exactly 466 lines.
// ---------------------------------------------------------------------------
func TestC336_005_NetLineReduction(t *testing.T) {
	root := acsassert.RepoRoot(t)
	src := filepath.Join(root, aggregatorSrcRel)
	data, err := os.ReadFile(src)
	if err != nil {
		t.Fatalf("RED: cannot read %s: %v", aggregatorSrcRel, err)
	}
	lines := bytes.Count(data, []byte("\n"))
	if lines >= aggregatorBaselineLines {
		t.Errorf("RED: aggregator.go has %d lines — want < %d (net reduction not yet realized)", lines, aggregatorBaselineLines)
	}
}
