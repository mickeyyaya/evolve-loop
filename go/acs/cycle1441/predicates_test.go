//go:build acs

// Package cycle1441 materialises the acceptance criteria for this lane's two
// fleet-scoped tasks (triage-report.md ## top_n):
//
//	salvage-extraction-stage-port   — land the stranded extraction/coercion stage
//	salvage-report-cli-and-docs     — a `saved` counter distinct from `recoverable`
//
// What this cycle is. Not new design: a LANDING. The instrumentation half of
// `schema-aligned-salvage-layer` (ClassifyBadVerdict, SummarizeBadVerdictBaseline,
// `evolve salvage report`) is already on main. The EXTRACTION half — the pass
// that repairs a sole, unambiguous, recoverable bad_verdict and re-verifies the
// repaired bytes before approving — is built, green, and stranded in ten+
// continuation worktrees, most advanced being .evolve/worktrees/cycle-42824668-1434
// (snapshot a2d65920). Nothing on main calls it; no PR was ever opened for it.
//
// Predicate strategy — wiring proof, not unit proof. 001-003 drive the REAL
// production entry point, `Reviewer.Review` (the contract gate), through the
// exported constructor, and assert on the gate's own decision plus the sidecar
// it wrote. They deliberately do NOT call SalvageVerdict directly: a salvage
// stage whose only caller is a test is dead code, and the entire defect this
// cycle closes is that the stage exists but no production path reaches it.
// 004-005 exercise the exported seams directly for the fail-closed and operator
// -surfacing contracts; 006 runs the ported package's own named tests; 007-008
// build and drive the real CLI binary. No predicate here is load-bearing on a
// source grep — the cycle-85 degenerate-predicate ban. The only greps present
// are auxiliary git-tracking checks (cycle-93: on-disk-but-untracked files are
// silently dropped at ship).
//
// RED baseline (this worktree, main-based):
//   - 001 fails: reviewer.go has no salvage call at all, so a sole recoverable
//     bad_verdict blocks and no salvage-applied.jsonl is ever written.
//   - 004/005 fail to COMPILE: deliverable.SalvageVerdict and
//     deliverable.SalvageSummaryLine do not exist on main. A predicate package
//     that fails to compile is a hard RED for the whole package (acs/README.md),
//     which is the correct signal here — the ported symbols are the deliverable.
//   - 006 fails: the ported test files are absent/untracked.
//   - 007/008 fail: `evolve salvage report -json` emits no `saved` key.
//   - 002/003 are the REGRESSION guards. They are pre-existing GREEN on main for
//     the trivial reason that main salvages nothing at all — so they are only
//     load-bearing AFTER the port, where they pin the two refusals a careless
//     port would drop: multi-violation salvage is a report-forgery bypass
//     (cycle-1392 CRITICAL-1) and multi-candidate salvage silently picks a
//     verdict the report never gave (cycle-1406 CRITICAL-1). They must stay
//     green THROUGH the landing; the port is not done if either flips.
package cycle1441

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/deliverable"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasespec"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

// salvageAppliedFile is the sidecar the extraction stage appends to when (and
// only when) it actually recovered a verdict. Named here rather than imported
// because it is package-private to deliverable; the predicates assert on the
// FILE an operator would find, which is the observable contract.
const salvageAppliedFile = "salvage-applied.jsonl"

// soleFencedPass is a report whose ONLY contract violation is bad_verdict: the
// required "## Verdict" section is present, and the verdict itself is carried in
// a displayable fenced JSON block instead of the required evolve-verdict
// sentinel comment. Measured in the research memo (§7.1) as the dominant
// recoverable shape (13/15). Exactly one candidate span, repairs to a payload
// that re-verifies clean — the one case salvage is allowed to act on.
const soleFencedPass = "## Verdict\n" +
	"```json\n" + `{"phase":"audit","verdict":"PASS"}` + "\n```\n"

// multiViolationFenced is the SAME recoverable verdict shape with the required
// "## Verdict" section removed, so bad_verdict co-occurs with missing_section.
// Salvage repairs the VERDICT and nothing else; acting here would erase the
// co-occurring violation wholesale (cycle-1392 audit CRITICAL-1).
const multiViolationFenced = "## Summary\n" +
	"```json\n" + `{"phase":"audit","verdict":"PASS"}` + "\n```\n"

// ambiguousFenced carries TWO candidate verdict spans disagreeing on the
// outcome. The stage must refuse rather than pick one — approving here means the
// gate reports a verdict the report never unambiguously gave (cycle-1406
// audit CRITICAL-1).
const ambiguousFenced = "## Verdict\n" +
	"```json\n" + `{"phase":"audit","verdict":"PASS"}` + "\n```\n" +
	"An earlier draft said:\n" +
	"```json\n" + `{"phase":"audit","verdict":"FAIL"}` + "\n```\n"

// --- helpers -----------------------------------------------------------------

// reviewFixture materialises a workspace holding audit-report.md with content,
// plus a separate project root with an empty .evolve/. Returns (workspace,
// projectRoot). Separate temp dirs so a stray-artifact check cannot see the
// report twice.
func reviewFixture(t *testing.T, content string) (string, string) {
	t.Helper()
	ws := t.TempDir()
	if err := os.WriteFile(filepath.Join(ws, "audit-report.md"), []byte(content), 0o644); err != nil {
		t.Fatalf("write audit-report.md: %v", err)
	}
	proj := t.TempDir()
	if err := os.MkdirAll(filepath.Join(proj, ".evolve"), 0o755); err != nil {
		t.Fatalf("mkdir .evolve: %v", err)
	}
	return ws, proj
}

// productionGate builds the contract gate through an EXPORTED constructor — the
// same seam core wires in production — at ContractGate=enforce and
// PhaseIO=enforce. PhaseIO=enforce is what makes the sentinel comment strictly
// required, so a displayable fenced payload lands as a bad_verdict instead of
// parsing clean.
func productionGate() core.DeliverableReviewer {
	return deliverable.NewReviewerWithCatalogStage(config.StageEnforce, phasespec.Catalog{}, config.StageEnforce)
}

// appliedRecords returns the parsed salvage_applied records under proj/.evolve.
// An absent sidecar is zero records, never a failure: "salvage never fired" is
// the state 002/003 assert.
func appliedRecords(t *testing.T, proj string) []map[string]any {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(proj, ".evolve", salvageAppliedFile))
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		t.Fatalf("read %s: %v", salvageAppliedFile, err)
	}
	var out []map[string]any
	for _, line := range strings.Split(string(data), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var m map[string]any
		if err := json.Unmarshal([]byte(line), &m); err != nil {
			t.Fatalf("%s carries an unparseable record %q: %v", salvageAppliedFile, line, err)
		}
		if m["event_type"] == "salvage_applied" {
			out = append(out, m)
		}
	}
	return out
}

// --- 001-003: the gate's decision (production entry point) -------------------

// TestC1441_001_ReviewSalvagesSoleRecoverableBadVerdict — the landing's whole
// point, asserted at the seam an operator actually runs. Not "SalvageVerdict
// returns true" (that passes on dead code) but "the contract gate, constructed
// the way production constructs it, approved a deliverable it blocks today, and
// left the operator an audit record saying it coerced one".
func TestC1441_001_ReviewSalvagesSoleRecoverableBadVerdict(t *testing.T) {
	ws, proj := reviewFixture(t, soleFencedPass)

	got := productionGate().Review(context.Background(), core.ReviewInput{
		Phase: "audit", Workspace: ws, ProjectRoot: proj,
	})
	if !got.Approve {
		t.Fatalf("contract gate BLOCKED a sole, unambiguous, recoverable bad_verdict (reason=%q) — the extraction "+
			"stage is not reached from Reviewer.Review. A salvage seam whose only caller is a test is dead code.", got.Reason)
	}
	if got.Demoted {
		t.Errorf("approval came from the breaker demotion path (Demoted=true, blocks=%d), not from salvage — a "+
			"demoted approval is the gate giving up, not the gate recovering a verdict", got.Blocks)
	}

	recs := appliedRecords(t, proj)
	if len(recs) != 1 {
		t.Fatalf("salvage-applied.jsonl holds %d salvage_applied record(s), want exactly 1 — every coercion must be "+
			"recorded for the operator (README §8), and exactly once", len(recs))
	}
	if got, want := recs[0]["pattern"], "fenced-json"; got != want {
		t.Errorf("recorded pattern = %v, want %q — the record must name the shape that was actually coerced", got, want)
	}
	if got, want := recs[0]["phase"], "audit"; got != want {
		t.Errorf("recorded phase = %v, want %q", got, want)
	}
}

// TestC1441_002_ReviewNeverSalvagesMultiViolation — the anti-forgery regression
// guard (cycle-1392 audit CRITICAL-1). Salvage acts on the SOLE-violation case
// or not at all: a bad_verdict co-occurring with any other violation must fall
// through to block, because approving via the salvaged Result erases ALL
// violations, including the anti-forgery proof-of-read check.
func TestC1441_002_ReviewNeverSalvagesMultiViolation(t *testing.T) {
	ws, proj := reviewFixture(t, multiViolationFenced)

	got := productionGate().Review(context.Background(), core.ReviewInput{
		Phase: "audit", Workspace: ws, ProjectRoot: proj,
	})
	if got.Approve {
		t.Errorf("contract gate APPROVED a deliverable whose bad_verdict co-occurs with a missing required section — "+
			"salvage repairs the verdict and nothing else; approving here erases every other violation wholesale "+
			"(report-forgery bypass, cycle-1392 CRITICAL-1). reason=%q demoted=%v", got.Reason, got.Demoted)
	}
	if n := len(appliedRecords(t, proj)); n != 0 {
		t.Errorf("%d salvage_applied record(s) written for a multi-violation deliverable, want 0 — a refusal must "+
			"leave no coercion record", n)
	}
}

// TestC1441_003_ReviewRefusesAmbiguousCandidates — the ambiguity regression
// guard (cycle-1406 audit CRITICAL-1). Two candidate spans disagreeing PASS vs
// FAIL: silently picking one manufactures a verdict the report never gave.
func TestC1441_003_ReviewRefusesAmbiguousCandidates(t *testing.T) {
	ws, proj := reviewFixture(t, ambiguousFenced)

	got := productionGate().Review(context.Background(), core.ReviewInput{
		Phase: "audit", Workspace: ws, ProjectRoot: proj,
	})
	if got.Approve {
		t.Errorf("contract gate APPROVED a deliverable carrying two disagreeing verdict candidates (PASS and FAIL) — "+
			"genuine ambiguity must be REFUSED, never resolved by picking a candidate. reason=%q demoted=%v",
			got.Reason, got.Demoted)
	}
	if n := len(appliedRecords(t, proj)); n != 0 {
		t.Errorf("%d salvage_applied record(s) written for an ambiguous deliverable, want 0", n)
	}
}

// --- 004-005: exported seam contracts ----------------------------------------

// TestC1441_004_SalvageVerdictFailsClosedOnUnresolvablePhase — the edge/OOD
// case. A phase whose contract cannot be resolved cannot be re-verified, and
// salvage must therefore refuse: it may never flip OK from the classification
// alone, because that skips every content check the strict parse never reached
// (cycle-1392 MEDIUM-3). The refused Result must come back byte-identical.
func TestC1441_004_SalvageVerdictFailsClosedOnUnresolvablePhase(t *testing.T) {
	in := deliverable.Result{
		Phase:        "no-such-phase-cycle1441",
		ArtifactPath: "/nonexistent/no-such-phase-report.md",
		Content:      soleFencedPass,
		Violations:   []deliverable.Violation{{Code: deliverable.CodeBadVerdict, Message: "seeded"}},
	}
	out, applied := deliverable.SalvageVerdict(in)
	if applied {
		t.Fatalf("SalvageVerdict claimed a salvage for a phase whose contract cannot be resolved — with no contract "+
			"there is nothing to re-verify the repaired bytes against, so the only safe answer is refusal (got OK=%v)", out.OK)
	}
	if out.OK {
		t.Errorf("refused salvage returned OK=true — a refusal must never approve")
	}
	if out.Content != in.Content || out.Phase != in.Phase || out.ArtifactPath != in.ArtifactPath ||
		len(out.Violations) != len(in.Violations) {
		t.Errorf("refused salvage mutated the Result\n got: %+v\nwant (byte-identical): %+v", out, in)
	}
}

// TestC1441_005_SalvageSummaryLineSurfacesRealSalvage — README §8 promises every
// coercion is "logged + surfaced". The renderer must read the SAME sidecar the
// gate just wrote (single-sourced, never a second counter that can drift), so
// this drives a real Review salvage first and then asserts the rendered line
// reflects it. Empty at zero records — no zero-noise.
func TestC1441_005_SalvageSummaryLineSurfacesRealSalvage(t *testing.T) {
	quiet := t.TempDir()
	if err := os.MkdirAll(filepath.Join(quiet, ".evolve"), 0o755); err != nil {
		t.Fatal(err)
	}
	if line := deliverable.SalvageSummaryLine(filepath.Join(quiet, ".evolve")); line != "" {
		t.Errorf("SalvageSummaryLine on an empty .evolve returned %q, want \"\" — zero salvages must render nothing", line)
	}

	ws, proj := reviewFixture(t, soleFencedPass)
	got := productionGate().Review(context.Background(), core.ReviewInput{
		Phase: "audit", Workspace: ws, ProjectRoot: proj,
	})
	if !got.Approve {
		t.Fatalf("precondition: a sole recoverable bad_verdict must salvage to Approve=true; got block (%s)", got.Reason)
	}

	line := deliverable.SalvageSummaryLine(filepath.Join(proj, ".evolve"))
	if want := "Salvaged verdicts: 1 (fenced-json=1)"; !strings.Contains(line, want) {
		t.Errorf("summary line must render the sidecar's real count and pattern breakdown\n want substring: %q\n got: %q",
			want, line)
	}
}

// --- 006: the ported package's own suite + tracking --------------------------

// TestC1441_006_PortedSalvageSuiteGreenAndTracked runs the ported package's own
// salvage tests against ONE named package (never a ./... sweep — flaky-predicate
// shape rules) and pins that every ported file is git-TRACKED, not merely on
// disk: an untracked file is silently dropped at ship (cycle-93).
func TestC1441_006_PortedSalvageSuiteGreenAndTracked(t *testing.T) {
	root := acsassert.RepoRoot(t)

	stdout, stderr, code, err := acsassert.SubprocessOutput(
		"go", "-C", filepath.Join(root, "go"), "test", "-count=1", "-run", "Salvage", "./internal/deliverable")
	if err != nil || code != 0 {
		t.Fatalf("go test -run Salvage ./internal/deliverable: exit=%d err=%v\nstdout:\n%s\nstderr:\n%s",
			code, err, stdout, stderr)
	}

	for _, rel := range []string{
		"go/internal/deliverable/salvage_extract.go",
		"go/internal/deliverable/salvage_extract_test.go",
		"go/internal/deliverable/reviewer_salvage_surface_test.go",
		"go/internal/deliverable/salvage_keycase_test.go",
	} {
		if !acsassert.FileExists(t, filepath.Join(root, rel)) {
			t.Errorf("RED: %s missing — the ported extraction stage is incomplete", rel)
			continue
		}
		if _, _, c, _ := acsassert.SubprocessOutput("git", "-C", root, "ls-files", "--error-unmatch", rel); c != 0 {
			t.Errorf("RED: %s is untracked — it will be dropped at ship", rel)
		}
	}
}

// --- 007-008: `saved` counter on the operator CLI ----------------------------

// buildEvolve compiles the REAL CLI entry point into a temp dir and returns its
// path. `go -C` so the module resolves from the worktree rather than whatever
// cwd the fleet lane happens to have.
func buildEvolve(t *testing.T) string {
	t.Helper()
	root := acsassert.RepoRoot(t)
	bin := filepath.Join(t.TempDir(), "evolve")
	_, stderr, code, err := acsassert.SubprocessOutput(
		"go", "-C", filepath.Join(root, "go"), "build", "-o", bin, "./cmd/evolve")
	if err != nil || code != 0 {
		t.Fatalf("go build ./cmd/evolve: exit=%d err=%v\n%s", code, err, stderr)
	}
	return bin
}

// savedReport is the CLI's JSON envelope, with the counter this cycle adds.
type savedReport struct {
	Total       int  `json:"total"`
	Recoverable int  `json:"recoverable"`
	Saved       int  `json:"saved"`
	savedSeen   bool // set by decode below
}

// decodeSavedReport decodes stdout and separately records whether the `saved`
// key was PRESENT — an absent key decodes to the zero value, which would make a
// "saved == 0" assertion pass on a CLI that never learned the field.
func decodeSavedReport(t *testing.T, stdout string) savedReport {
	t.Helper()
	var raw map[string]json.RawMessage
	if err := json.Unmarshal([]byte(stdout), &raw); err != nil {
		t.Fatalf("stdout is not the JSON envelope: %v\n%s", err, stdout)
	}
	var out savedReport
	if err := json.Unmarshal([]byte(stdout), &out); err != nil {
		t.Fatalf("envelope does not decode: %v\n%s", err, stdout)
	}
	_, out.savedSeen = raw["saved"]
	return out
}

// TestC1441_007_SalvageReportExposesSavedCounter — Task 2's contract, driven
// through the real binary. `saved` counts ACTUAL coercions (salvage-applied.jsonl,
// the sidecar the gate writes when it fires) and is deliberately a DIFFERENT
// number from `recoverable` (bad-verdict-baseline.jsonl, measured potential).
// The fixture makes them differ — 3 recoverable, 2 saved — so a CLI that simply
// aliases one to the other cannot pass. Foreign event types and blank lines must
// not enter either count.
func TestC1441_007_SalvageReportExposesSavedCounter(t *testing.T) {
	bin := buildEvolve(t)

	proj := t.TempDir()
	evolveDir := filepath.Join(proj, ".evolve")
	if err := os.MkdirAll(evolveDir, 0o755); err != nil {
		t.Fatal(err)
	}
	baseline := strings.Join([]string{
		`{"event_type":"bad_verdict_classified","recoverable":true,"pattern":"fenced-json"}`,
		`{"event_type":"bad_verdict_classified","recoverable":true,"pattern":"fenced-json"}`,
		`{"event_type":"bad_verdict_classified","recoverable":true,"pattern":"trailing-comma"}`,
		`{"event_type":"bad_verdict_classified","recoverable":false,"pattern":""}`,
		``,
	}, "\n")
	if err := os.WriteFile(filepath.Join(evolveDir, "bad-verdict-baseline.jsonl"), []byte(baseline), 0o644); err != nil {
		t.Fatal(err)
	}
	applied := strings.Join([]string{
		`{"event_type":"salvage_applied","phase":"audit","pattern":"fenced-json","run":"111"}`,
		`{"event_type":"some_other_emitter","phase":"build","pattern":"fenced-json","run":"111"}`,
		``,
		`{"event_type":"salvage_applied","phase":"build","pattern":"trailing-comma","run":"222"}`,
		``,
	}, "\n")
	if err := os.WriteFile(filepath.Join(evolveDir, salvageAppliedFile), []byte(applied), 0o644); err != nil {
		t.Fatal(err)
	}

	stdout, stderr, code, err := acsassert.SubprocessOutput(bin, "salvage", "report", "-json", "-project-root", proj)
	if err != nil || code != 0 {
		t.Fatalf("evolve salvage report -json: exit=%d err=%v\nstdout:\n%s\nstderr:\n%s", code, err, stdout, stderr)
	}
	got := decodeSavedReport(t, stdout)
	if !got.savedSeen {
		t.Fatalf("JSON envelope has no `saved` key — operators cannot tell measured POTENTIAL (recoverable) from "+
			"ACTUAL coercions. envelope:\n%s", stdout)
	}
	if got.Saved != 2 {
		t.Errorf("saved = %d, want 2 — count salvage_applied records only; the foreign emitter's line and the blank "+
			"lines must not enter the count", got.Saved)
	}
	if got.Recoverable != 3 {
		t.Errorf("recoverable = %d, want 3 — the existing baseline fold must not change", got.Recoverable)
	}
	if got.Saved == got.Recoverable {
		t.Errorf("saved == recoverable == %d on a fixture built to make them differ — the new counter must be read "+
			"from salvage-applied.jsonl, not aliased to the baseline's recoverable count", got.Saved)
	}
}

// TestC1441_008_SalvageReportSavedZeroWithoutAppliedSidecar — the edge case a
// fresh project root is always in: the gate has never salvaged, so no
// salvage-applied.jsonl exists. That is the normal un-populated state, not a
// failure: the command must still exit 0 and report saved=0 through the same
// envelope, so a consumer never special-cases "no file".
func TestC1441_008_SalvageReportSavedZeroWithoutAppliedSidecar(t *testing.T) {
	bin := buildEvolve(t)

	proj := t.TempDir()
	evolveDir := filepath.Join(proj, ".evolve")
	if err := os.MkdirAll(evolveDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(evolveDir, "bad-verdict-baseline.jsonl"),
		[]byte(`{"event_type":"bad_verdict_classified","recoverable":true,"pattern":"fenced-json"}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	stdout, stderr, code, err := acsassert.SubprocessOutput(bin, "salvage", "report", "-json", "-project-root", proj)
	if err != nil || code != 0 {
		t.Fatalf("an absent salvage-applied.jsonl must not be an error: exit=%d err=%v\nstdout:\n%s\nstderr:\n%s",
			code, err, stdout, stderr)
	}
	got := decodeSavedReport(t, stdout)
	if !got.savedSeen {
		t.Fatalf("JSON envelope has no `saved` key even on the empty path — the key must always be present so "+
			"consumers need no special case. envelope:\n%s", stdout)
	}
	if got.Saved != 0 {
		t.Errorf("saved = %d with no salvage-applied.jsonl, want 0", got.Saved)
	}
	if got.Recoverable != 1 {
		t.Errorf("recoverable = %d, want 1 — the baseline fold must still run when the applied sidecar is absent",
			got.Recoverable)
	}
}
