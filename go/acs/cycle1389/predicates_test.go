//go:build acs

// Package cycle1389 materializes the acceptance criteria of this lane's sole
// fleet-scoped inbox item `schema-aligned-salvage-layer` (weight 0.9,
// scout-report.md Task 1 "instrument-bad-verdict-classification" + Task 2
// "wire-bad-verdict-baseline-docs").
//
// Scope, per the inbox item's own text ("FIRST deliverable = instrumentation
// only"): a log-only classifier that inspects the exact bytes a CodeBadVerdict
// Result was computed from (deliverable.Result.Content — the single-read seam,
// deliverable.go:44-64) and recognizes three SAP-cited (docs/research/
// deliverable-alignment-2026-08/README.md §3.3) recoverable-malformed shapes —
// a fenced-JSON-wrapped sentinel, a trailing-comma sentinel payload, and a
// bare/displaced (unwrapped) verdict object — vs. a genuinely absent verdict.
// NO extraction/coercion logic ships this cycle; Result.OK/Violations must be
// byte-identical before/after (predicate 005).
//
// New production surface this cycle's Builder implements (contract fixed by
// this test file, cycle-644 lesson honored — no package-qualified pin without
// a compiler-probed shape; every symbol below is a NEW exported name inside
// the ALREADY-enrolled go/internal/deliverable package, not a new package, so
// no go/.apicover-enforce edit is required — house rule 1 n/a):
//
//	// go/internal/deliverable/salvage_instrument.go
//	type SalvagePattern string
//	const (
//		SalvagePatternNone          SalvagePattern = ""
//		SalvagePatternFencedJSON    SalvagePattern = "fenced-json"
//		SalvagePatternTrailingComma SalvagePattern = "trailing-comma"
//		SalvagePatternDisplaced     SalvagePattern = "displaced-line"
//	)
//	type BadVerdictClassification struct {
//		Recoverable bool
//		Pattern     SalvagePattern
//		Reason      string
//	}
//	func ClassifyBadVerdict(content string) BadVerdictClassification
//
// Predicates 001-004 drive that function directly (a real behavioral call,
// never a source-grep — the cycle-85 degenerate-predicate ban). Predicate 005
// is the zero-mutation regression guard (scout's Acceptance Criteria Summary:
// "full regression: go test ./go/internal/deliverable/..."). Predicate 006 is
// the REACHABILITY / wiring proof (house rule 2): it drives the real
// production caller — deliverable.NewReviewer(...).Review, the host contract
// gate reviewer.go:102 wires behind core.DeliverableReviewer — not
// ClassifyBadVerdict directly, so a classifier with no production caller stays
// RED. It expects Task 2's Builder to append one JSONL record per bad_verdict
// to <ProjectRoot>/.evolve/bad-verdict-baseline.jsonl (reusing the existing
// log.SidecarWriter/EmitAbnormal pattern, log/events.go) from inside
// Reviewer.Review, strictly AFTER the res.OK branch (so the block/approve
// decision itself is untouched — instrumentation only). Predicates 007/008
// materialize Task 2's doc AC (README.md gains a real, non-fabricated §7
// baseline section + a wiring-proof excerpt) — 007 is the structural half
// (predicate-testable: the heading and a fenced code excerpt naming
// ClassifyBadVerdict exist); the "numbers are real, not fabricated" half is
// NOT mechanically verifiable and is dispositioned manual+checklist in
// test-report.md instead of faked here as a predicate.
package cycle1389

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/deliverable"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasespec"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

// writeFile is the same tiny helper deliverable_test.go and every ACS
// predicate package re-declares locally (no shared test-only package exists).
func writeFile(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
}

// hasBadVerdict reports whether a deliverable.Result carries CodeBadVerdict —
// a local equivalent of the unexported hasCode helper in deliverable_test.go
// (this package cannot reach unexported package symbols).
func hasBadVerdict(res deliverable.Result) bool {
	for _, v := range res.Violations {
		if v.Code == deliverable.CodeBadVerdict {
			return true
		}
	}
	return false
}

// --- 001/002/003: recoverable shapes -> Recoverable=true + the named pattern ---

// fencedJSONBadVerdictContent is a report whose only verdict-bearing text is a
// markdown-fenced JSON object carrying a "verdict" key — no `<!-- evolve-
// verdict: ... -->` sentinel comment anywhere, so the real sentinel parser
// (phasecontract.ParseVerdictSentinelFull) finds nothing and, at
// config.StageEnforce (prose fallback gated off), deliverable.Verify
// confirms CodeBadVerdict. A lenient reader COULD recover "PASS" from the
// fenced block; that is exactly the "fenced/mislabeled JSON" pattern the
// inbox item names.
const fencedJSONBadVerdictContent = "## Verdict\n" +
	"```json\n" +
	`{"phase":"audit","verdict":"PASS"}` + "\n" +
	"```\n"

// TestC1389_001_ClassifyBadVerdict_FencedJSON_Recoverable is Task 1 AC(a): the
// classifier recognizes a fenced-JSON-wrapped verdict as recoverable.
func TestC1389_001_ClassifyBadVerdict_FencedJSON_Recoverable(t *testing.T) {
	// Anchor: prove this content is a REAL bad_verdict via the production
	// entry point first, so the fixture cannot silently drift from what
	// Verify actually rejects.
	ws := t.TempDir()
	writeFile(t, ws, "audit-report.md", fencedJSONBadVerdictContent)
	res, err := deliverable.VerifyWithStage("audit", phasecontract.Roots{Workspace: ws}, phasecontract.BuiltinResolver{}, config.StageEnforce)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !hasBadVerdict(res) {
		t.Fatalf("fixture must be a genuine bad_verdict at StageEnforce; got %+v", res.Violations)
	}

	got := deliverable.ClassifyBadVerdict(res.Content)
	if !got.Recoverable {
		t.Errorf("want Recoverable=true for fenced-JSON content, got %+v", got)
	}
	if got.Pattern != deliverable.SalvagePatternFencedJSON {
		t.Errorf("want Pattern=%q, got %q (classification=%+v)", deliverable.SalvagePatternFencedJSON, got.Pattern, got)
	}
	if got.Reason == "" {
		t.Errorf("want a non-empty Reason (audit-visible log entry per coercion, README §3.3) — silent classification is not observability")
	}
}

// trailingCommaBadVerdictContent carries a well-formed `<!-- evolve-verdict:
// ... -->` COMMENT whose JSON payload has a trailing comma before the closing
// brace — the comment/regex shape matches, but json.Unmarshal (invoked by
// phasecontract.parseSentinelPayload) rejects the trailing comma, so the real
// sentinel parser fails and, at StageEnforce, deliverable.Verify confirms
// CodeBadVerdict.
const trailingCommaBadVerdictContent = "## Verdict\n" +
	`<!-- evolve-verdict: {"phase":"audit","verdict":"PASS",} -->` + "\n"

// TestC1389_002_ClassifyBadVerdict_TrailingComma_Recoverable is Task 1 AC(b).
func TestC1389_002_ClassifyBadVerdict_TrailingComma_Recoverable(t *testing.T) {
	ws := t.TempDir()
	writeFile(t, ws, "audit-report.md", trailingCommaBadVerdictContent)
	res, err := deliverable.VerifyWithStage("audit", phasecontract.Roots{Workspace: ws}, phasecontract.BuiltinResolver{}, config.StageEnforce)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !hasBadVerdict(res) {
		t.Fatalf("fixture must be a genuine bad_verdict at StageEnforce; got %+v", res.Violations)
	}

	got := deliverable.ClassifyBadVerdict(res.Content)
	if !got.Recoverable {
		t.Errorf("want Recoverable=true for trailing-comma content, got %+v", got)
	}
	if got.Pattern != deliverable.SalvagePatternTrailingComma {
		t.Errorf("want Pattern=%q, got %q (classification=%+v)", deliverable.SalvagePatternTrailingComma, got.Pattern, got)
	}
}

// displacedBadVerdictContent carries a bare (unwrapped, uncommented) JSON
// verdict object sitting on its own prose line — no `<!-- evolve-verdict -->`
// markers, no code fence. The sentinel regex requires the comment markers, so
// this never matches; at StageEnforce the prose fallback is gated off too, so
// deliverable.Verify confirms CodeBadVerdict even though a lenient reader
// could plainly recover "verdict":"PASS" — the "displaced sentinel" pattern.
const displacedBadVerdictContent = "## Verdict\n" +
	"The agent's own reasoning trails off here, then states:\n" +
	`{"phase":"audit","verdict":"PASS"}` + "\n" +
	"...and nothing else follows.\n"

// TestC1389_003_ClassifyBadVerdict_Displaced_Recoverable is Task 1 AC(c).
func TestC1389_003_ClassifyBadVerdict_Displaced_Recoverable(t *testing.T) {
	ws := t.TempDir()
	writeFile(t, ws, "audit-report.md", displacedBadVerdictContent)
	res, err := deliverable.VerifyWithStage("audit", phasecontract.Roots{Workspace: ws}, phasecontract.BuiltinResolver{}, config.StageEnforce)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !hasBadVerdict(res) {
		t.Fatalf("fixture must be a genuine bad_verdict at StageEnforce; got %+v", res.Violations)
	}

	got := deliverable.ClassifyBadVerdict(res.Content)
	if !got.Recoverable {
		t.Errorf("want Recoverable=true for displaced bare-JSON content, got %+v", got)
	}
	if got.Pattern != deliverable.SalvagePatternDisplaced {
		t.Errorf("want Pattern=%q, got %q (classification=%+v)", deliverable.SalvagePatternDisplaced, got.Pattern, got)
	}
}

// absentBadVerdictContent is a genuinely absent verdict: prose musings, no
// JSON shape of any kind, fenced or bare. This is the negative control (SKILL
// §6 negative axis) — the highest-leverage anti-no-op signal, since a
// classifier that marks EVERYTHING recoverable would pass 001-003 vacuously.
const absentBadVerdictContent = "## Verdict\n" +
	"inconclusive musings, no token, no structure of any kind here at all\n"

// TestC1389_004_ClassifyBadVerdict_GenuinelyAbsent_NotRecoverable is Task 1
// AC(d) — the negative test.
func TestC1389_004_ClassifyBadVerdict_GenuinelyAbsent_NotRecoverable(t *testing.T) {
	ws := t.TempDir()
	writeFile(t, ws, "audit-report.md", absentBadVerdictContent)
	res, err := deliverable.VerifyWithStage("audit", phasecontract.Roots{Workspace: ws}, phasecontract.BuiltinResolver{}, config.StageEnforce)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !hasBadVerdict(res) {
		t.Fatalf("fixture must be a genuine bad_verdict at StageEnforce; got %+v", res.Violations)
	}

	got := deliverable.ClassifyBadVerdict(res.Content)
	if got.Recoverable {
		t.Errorf("want Recoverable=false for a genuinely absent verdict, got %+v — a classifier that always says recoverable is a no-op (SKILL §6 negative axis)", got)
	}
	if got.Pattern != deliverable.SalvagePatternNone {
		t.Errorf("want Pattern=%q (none) for not-recoverable, got %q", deliverable.SalvagePatternNone, got.Pattern)
	}
}

// TestC1389_005_ExistingDeliverableSuite_ZeroMutation is the zero-mutation
// regression guard scout's Acceptance Criteria Summary requires ("full
// regression: go test ./go/internal/deliverable/..."). The classifier is
// observability-only (README §3.3: "never invents values"); this predicate
// shells ONE named package (flaky-predicate-shape: no /... sweep, no
// known-slow suite) so any Result.OK/Violations mutation in the existing
// table tests fails loudly here rather than being caught only by CI.
func TestC1389_005_ExistingDeliverableSuite_ZeroMutation(t *testing.T) {
	root := acsassert.RepoRoot(t)
	cmd := exec.Command("go", "test", "./internal/deliverable")
	cmd.Dir = filepath.Join(root, "go")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Errorf("go test ./internal/deliverable/... must stay green (zero Result.OK/Violations mutation from the new classifier); err=%v\n%s", err, out)
	}
}

// TestC1389_006_ReviewerWiring_LogsClassificationOnBadVerdict is the
// REACHABILITY / wiring proof (house rule 2, Task 2). It drives the real
// production caller — deliverable.NewReviewer(config.StageEnforce).Review,
// the exact seam reviewer.go:102 wires behind core.DeliverableReviewer for
// the host contract gate — never ClassifyBadVerdict directly, so a
// classifier the Reviewer never calls stays RED here even if 001-004 are
// green. Expects one JSONL record appended to
// <ProjectRoot>/.evolve/bad-verdict-baseline.jsonl carrying the phase and the
// classification, added strictly as an observability side effect: the
// Reviewer's block/approve decision (asserted below) must be UNCHANGED from
// pre-instrumentation behavior (StageEnforce + bad_verdict ⇒ still a real
// contract block, not silently waived by the new logging).
func TestC1389_006_ReviewerWiring_LogsClassificationOnBadVerdict(t *testing.T) {
	ws, pr := t.TempDir(), t.TempDir()
	writeFile(t, ws, "audit-report.md", trailingCommaBadVerdictContent)

	// Builder correction to the RED fixture (evidence in build-report.md
	// "Predicate-006 constructor correction"): the original drove
	// deliverable.NewReviewer(StageEnforce), which pins phaseIO=StageOff — and
	// at StageOff deliverable.go's LEGACY PROSE FALLBACK (verdictPresent,
	// deliverable.go:342-348) rescues any report whose text merely CONTAINS
	// "PASS". The trailing-comma fixture does, so that constructor returned
	// Approve=true with zero violations: the predicate's own first assertion was
	// unsatisfiable for a reason unrelated to the classifier, and no
	// implementation could have turned it green. Swapped to the EXACT production
	// constructor cmd/evolve/cmd_cycle.go:620 wires behind
	// core.DeliverableReviewer, threaded with the phaseIO stage at which the
	// prose fallback is gated off and a malformed sentinel genuinely resolves to
	// CodeBadVerdict — which is the only regime where a salvage layer has
	// anything to salvage. Every assertion below is unchanged and now strictly
	// stronger (it reaches a real bad_verdict block).
	r := deliverable.NewReviewerWithCatalogStageReportSize(
		config.StageEnforce, phasespec.Catalog{}, config.StageEnforce, config.StageOff, 0)
	got := r.Review(context.Background(), core.ReviewInput{Phase: "audit", Workspace: ws, ProjectRoot: pr})

	// Instrumentation must not change the gate's own decision.
	if got.Approve {
		t.Fatalf("bad_verdict at StageEnforce must still BLOCK (instrumentation is observability-only, never a waiver); got Approve=true")
	}

	baselinePath := filepath.Join(pr, ".evolve", "bad-verdict-baseline.jsonl")
	data, err := os.ReadFile(baselinePath)
	if err != nil {
		t.Fatalf("want a baseline JSONL record appended at %s by Reviewer.Review on bad_verdict (Task 2 wiring), got read error: %v", baselinePath, err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	last := lines[len(lines)-1]

	var rec map[string]any
	if err := json.Unmarshal([]byte(last), &rec); err != nil {
		t.Fatalf("baseline record is not valid JSON: %v\nline=%q", err, last)
	}
	if rec["phase"] != "audit" {
		t.Errorf("want phase=%q in the baseline record, got %+v", "audit", rec)
	}
	if rec["recoverable"] != true {
		t.Errorf("this fixture (trailing-comma) is classifier-recoverable; want recoverable=true in the baseline record, got %+v", rec)
	}
	if rec["pattern"] != string(deliverable.SalvagePatternTrailingComma) {
		t.Errorf("want pattern=%q in the baseline record, got %+v", deliverable.SalvagePatternTrailingComma, rec)
	}
}

// readmeRelPath is the SSOT doc the inbox item names for the baseline write-up
// (operating-policy 3.8 issue/gap/solution format).
const readmeRelPath = "docs/research/deliverable-alignment-2026-08/README.md"

// section7RE matches a top-level "## 7." heading — the next number after the
// existing "## 6. Experience record ..." section (README currently tops out
// at "## 6", "## Sources"). Anchored to line start so it cannot match inside a
// fenced code excerpt quoting the heading as an example.
var section7RE = regexp.MustCompile(`(?m)^## 7\.`)

// TestC1389_007_ReadmeGainsBaselineSectionWithWiringExcerpt is Task 2's
// structural doc AC: README.md gains a real §7 baseline section that (a)
// exists as a genuine top-level heading and (b) carries a wiring-proof code
// excerpt naming the real production call site — the observable, mechanically
// verifiable half of Task 2's AC. Whether the reported COUNTS are real
// (pulled from a live run) vs. fabricated is NOT mechanically verifiable and
// is dispositioned manual+checklist in test-report.md instead (no predicate
// can distinguish a real "12" from an invented one).
func TestC1389_007_ReadmeGainsBaselineSectionWithWiringExcerpt(t *testing.T) {
	root := acsassert.RepoRoot(t)
	path := filepath.Join(root, readmeRelPath)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", readmeRelPath, err)
	}
	content := string(data)

	if !section7RE.MatchString(content) {
		t.Errorf("%s has no top-level \"## 7.\" heading — append the baseline section (operating-policy 3.8 issue/gap/solution format)", readmeRelPath)
	}
	if !strings.Contains(content, "ClassifyBadVerdict") {
		t.Errorf("%s §7 must carry a wiring-proof excerpt naming ClassifyBadVerdict (the classifier actually invoked from Reviewer.Review, not a dead helper)", readmeRelPath)
	}
	if !strings.Contains(content, "bad-verdict-baseline.jsonl") {
		t.Errorf("%s §7 must name the baseline sidecar file the counts were pulled from", readmeRelPath)
	}
}
