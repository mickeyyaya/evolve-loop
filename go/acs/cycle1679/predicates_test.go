//go:build acs

// Package cycle1679 materialises the cycle-1679 acceptance criteria for the one
// fleet-scoped task pinned to this lane: `crossartifact-invariant-stack`.
//
// WHAT THE CONTRACT ACTUALLY IS. The lane's inbox record
// (.evolve/inbox/processing/cycle-1679/2026-08-04T07-11-00Z-crossartifact-invariant-stack.json)
// carries TWO halves in its `fix` field, and `connects_to` names both homes:
//
//	"SWE-agent rule enforced: each invariant ships ADVISORY until its
//	 false-positive rate is evidenced ~0 (the 1054/1060 breaker lesson), then
//	 graduates to blocking. DOCS per 3.8 into
//	 docs/research/deliverable-alignment-2026-08/README.md."
//
//	connects_to: ["go/internal/coherence/",
//	              "docs/research/deliverable-alignment-2026-08/README.md"]
//
// The CODE half is already landed on this branch and green — verified this
// phase, not assumed: internal/coherence/crossartifact.go implements all four
// invariants, internal/core/crossartifact_invariants.go:36 records them, and
// cyclerun.go:241 is the production caller. Predicates 001-003 pin that behaviour
// so it cannot silently regress; they are expected PRE-EXISTING GREEN and are
// declared as such in test-report.md rather than deleted (a landed contract that
// stops being checked is how a shipped invariant rots).
//
// The DOCS half has never landed, and that is this cycle's real RED. §6 of the
// README ("Experience record for the new moves (to be extended per §3.8)") holds
// 6.1, 6.2 and 6.3 — there is no 6.x entry for item rank 5. Line 140's portfolio
// row still reads "**NEW — filed 0.85**" and line 124 still calls the stack
// "partial (`coherence`)", both stale against code that exists. Because the doc
// deliverable its own acceptance contract names was never written, the item's
// acceptance is unmet, so it is re-claimed every cycle: the identical record sits
// unconsumed in .evolve/inbox/processing/cycle-<N>/ for 1601 through 1679.
// Predicates 004-005 are the RED that ends that streak.
//
// ADVERSARIAL DIVERSITY (skills/adversarial-testing §6):
//   - NEGATIVE : 002 breaks each invariant class one at a time and demands that
//     class be named — an aggregate that returns "ok" for everything, or
//     "violated" for everything, fails it.
//   - EDGE/OOD : 003 drives the empty workspace and demands `indeterminate`,
//     never `violated` — the property that keeps an advisory's false-positive
//     rate at zero, which is the inbox record's own graduation condition.
//   - SEMANTIC : 004 does not merely look for a heading; it demands the §6.4
//     body carry the Issue/Gap/Solution/Measured shape 6.1-6.3 use, be
//     git-TRACKED (a gitignored doc is dropped at ship — the cycle-93 lesson),
//     and that every repo path it cites RESOLVE ON DISK. That last check is the
//     README being held to the same referenced-paths-exist invariant the feature
//     it documents enforces on everyone else.
package cycle1679

import (
	"bufio"
	"encoding/json"
	"fmt"
	"go/build/constraint"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/coherence"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

// alignmentREADME is the doc deliverable the inbox record's `fix` field names.
const alignmentREADME = "docs/research/deliverable-alignment-2026-08/README.md"

// allFour is the aggregate's declared, stable invariant order.
var allFour = []string{
	coherence.InvariantVerdictAgreement,
	coherence.InvariantTestCounts,
	coherence.InvariantReferencedPaths,
	coherence.InvariantPhaseOrder,
}

// writeAudit writes an audit-report.md carrying a REAL canonical evolve-verdict
// sentinel, built with encoding/json so the fixture is the shape phasecontract
// actually parses (never hand-spelled).
func writeAudit(t *testing.T, dir, verdict string, evidencePaths []string) {
	t.Helper()
	payload := map[string]any{"phase": "audit", "verdict": verdict, "schema_version": 1}
	if len(evidencePaths) > 0 {
		payload["schema_version"] = 2
		payload["failure"] = map[string]any{
			"class":          "code-audit-fail",
			"defects":        []string{"H1: cited artifact"},
			"evidence_paths": evidencePaths,
		}
	}
	b, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal sentinel: %v", err)
	}
	writeRaw(t, dir, "audit-report.md",
		"# Audit Report\n\n## Verdict\n**"+verdict+"**\n<!-- evolve-verdict: "+string(b)+" -->\n")
}

func writeRaw(t *testing.T, dir, name, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
}

// acsBody builds an acs-verdict.json in the live acssuite.Verdict shape: the
// claimed summary counts up top, the independently parsed runner results below,
// so a fixture can make the claim disagree with its own evidence.
func acsBody(t *testing.T, verdict string, total, green, red, skip int, results []string) string {
	t.Helper()
	rows := make([]map[string]any, 0, len(results))
	for _, r := range results {
		rows = append(rows, map[string]any{"name": "p", "result": r})
	}
	b, err := json.Marshal(map[string]any{
		"schema_version":  "1.0",
		"cycle":           1679,
		"predicate_suite": map[string]any{"this_cycle_count": len(results), "total": total},
		"results":         rows,
		"green_count":     green,
		"red_count":       red,
		"skip_count":      skip,
		"verdict":         verdict,
		"ship_eligible":   red == 0,
	})
	if err != nil {
		t.Fatalf("marshal acs-verdict: %v", err)
	}
	return string(b)
}

func writeTiming(t *testing.T, dir string, phases [][3]string) {
	t.Helper()
	rows := make([]map[string]any, 0, len(phases))
	for _, p := range phases {
		rows = append(rows, map[string]any{
			"phase": p[0], "duration_ms": 1000, "verdict": "PASS", "cost_usd": 0,
			"started_at": p[1], "ended_at": p[2], "attempt_count": 1,
		})
	}
	b, err := json.Marshal(rows)
	if err != nil {
		t.Fatalf("marshal phase-timing: %v", err)
	}
	writeRaw(t, dir, "phase-timing.json", string(b))
}

// coherentWorkspace is the all-green fixture: agreeing verdicts, counts that
// survive a recount, a cited path that exists, a forward-running phase chain.
func coherentWorkspace(t *testing.T) (workspace, worktree string) {
	t.Helper()
	workspace, worktree = t.TempDir(), t.TempDir()
	writeAudit(t, workspace, "PASS", []string{"acs-verdict.json"})
	writeRaw(t, workspace, "acs-verdict.json", acsBody(t, "PASS", 3, 2, 0, 1, []string{"green", "green", "skip"}))
	writeTiming(t, workspace, [][3]string{
		{"scout", "2026-09-14T03:41:45Z", "2026-09-14T03:45:45Z"},
		{"tdd", "2026-09-14T03:46:23Z", "2026-09-14T03:48:15Z"},
		{"build", "2026-09-14T03:48:15Z", "2026-09-14T03:59:00Z"},
		{"audit", "2026-09-14T04:00:00Z", "2026-09-14T04:10:00Z"},
	})
	return workspace, worktree
}

func find(t *testing.T, r coherence.InvariantReport, name string) coherence.Invariant {
	t.Helper()
	for _, inv := range r.Invariants {
		if inv.Name == name {
			return inv
		}
	}
	t.Fatalf("invariant %q absent from the report; got %+v", name, r.Invariants)
	return coherence.Invariant{}
}

// ---------------------------------------------------------------------------
// AC1 — one deterministic suite evaluates all four invariant classes from
// independent on-disk evidence and returns an aggregate with named violations.
// ---------------------------------------------------------------------------

// TestC1679_001_SuiteReportsAllFourInvariantsOnACoherentWorkspace is the
// positive pole. Without it, an implementation that answered "violated" to
// everything would satisfy every negative predicate below.
func TestC1679_001_SuiteReportsAllFourInvariantsOnACoherentWorkspace(t *testing.T) {
	ws, wt := coherentWorkspace(t)
	got := coherence.CheckCrossArtifactInvariants(ws, wt)
	if !got.Advisory {
		t.Errorf("RED: the aggregate must declare itself advisory (inbox `fix`: ADVISORY until FP rate ~0)")
	}
	for _, name := range allFour {
		if inv := find(t, got, name); inv.Status != coherence.InvariantOK {
			t.Errorf("RED: %s = %q on a coherent workspace, want ok (evidence: %s)", name, inv.Status, inv.Evidence)
		}
	}
	if v := got.Violations(); len(v) != 0 {
		t.Errorf("RED: a coherent workspace reported %d violation(s): %+v", len(v), v)
	}
}

// TestC1679_002_EachInvariantClassIsIndependentlyFalsifiable breaks ONE source
// of evidence at a time and demands the matching class be named. This is the
// anti-no-op predicate: a suite that only reads the embedded sentinel cannot
// pass the count, path and ordering cases.
func TestC1679_002_EachInvariantClassIsIndependentlyFalsifiable(t *testing.T) {
	cases := []struct {
		name    string
		want    string
		corrupt func(t *testing.T, ws, wt string)
	}{
		{
			name: "verdict disagreement",
			want: coherence.InvariantVerdictAgreement,
			corrupt: func(t *testing.T, ws, wt string) {
				writeRaw(t, ws, "acs-verdict.json", acsBody(t, "FAIL", 3, 2, 0, 1, []string{"green", "green", "skip"}))
			},
		},
		{
			name: "claimed counts contradict the parsed results",
			want: coherence.InvariantTestCounts,
			corrupt: func(t *testing.T, ws, wt string) {
				// Claims zero red while its own results carry one — the cycle-1673 M1 shape.
				writeRaw(t, ws, "acs-verdict.json", acsBody(t, "PASS", 3, 3, 0, 0, []string{"green", "green", "red"}))
			},
		},
		{
			name: "cited evidence path exists nowhere",
			want: coherence.InvariantReferencedPaths,
			corrupt: func(t *testing.T, ws, wt string) {
				writeAudit(t, ws, "PASS", []string{"no/such/artifact-that-never-existed.json"})
			},
		},
		{
			name: "phase chain runs backwards",
			want: coherence.InvariantPhaseOrder,
			corrupt: func(t *testing.T, ws, wt string) {
				writeTiming(t, ws, [][3]string{
					{"audit", "2026-09-14T03:41:45Z", "2026-09-14T03:45:45Z"},
					{"build", "2026-09-14T04:00:00Z", "2026-09-14T04:10:00Z"},
				})
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ws, wt := coherentWorkspace(t)
			tc.corrupt(t, ws, wt)
			got := coherence.CheckCrossArtifactInvariants(ws, wt)
			inv := find(t, got, tc.want)
			if inv.Status != coherence.InvariantViolated {
				t.Errorf("RED: %s = %q, want violated", tc.want, inv.Status)
			}
			if strings.TrimSpace(inv.Evidence) == "" {
				t.Errorf("RED: %s violated with no evidence — an unactionable finding", tc.want)
			}
			named := false
			for _, v := range got.Violations() {
				if v.Name == tc.want {
					named = true
				}
			}
			if !named {
				t.Errorf("RED: Violations() did not name %s; got %+v", tc.want, got.Violations())
			}
		})
	}
}

// TestC1679_003_AbsentArtifactsAreIndeterminateNeverViolated pins the inbox
// record's graduation condition: an advisory that fires on ABSENCE earns a
// false-positive rate and gets switched off (the 1054/1060 breaker lesson).
func TestC1679_003_AbsentArtifactsAreIndeterminateNeverViolated(t *testing.T) {
	empty, wt := t.TempDir(), t.TempDir()
	got := coherence.CheckCrossArtifactInvariants(empty, wt)
	if len(got.Invariants) != len(allFour) {
		t.Fatalf("RED: empty workspace reported %d invariants, want %d", len(got.Invariants), len(allFour))
	}
	for _, name := range allFour {
		if inv := find(t, got, name); inv.Status != coherence.InvariantIndeterminate {
			t.Errorf("RED: %s = %q on an empty workspace, want indeterminate", name, inv.Status)
		}
	}
	if v := got.Violations(); len(v) != 0 {
		t.Errorf("RED: absence produced %d violation(s) — a false-positive generator: %+v", len(v), v)
	}
}

// ---------------------------------------------------------------------------
// The DOCS half of the acceptance contract — this cycle's real RED.
// ---------------------------------------------------------------------------

// TestC1679_004_ExperienceRecordLandedAndItsCitationsResolve is the deliverable
// the inbox record's `fix` field names ("DOCS per 3.8 into
// docs/research/deliverable-alignment-2026-08/README.md").
//
// acs-predicate: config-check — the deliverable for this AC IS document content,
// so the document is the system under test. It is NOT a bare magic-string grep:
// the section must carry the Issue/Gap/Solution/Measured shape §6.1-6.3 use, the
// file must be git-TRACKED, and every repo path the section cites must RESOLVE
// ON DISK — the same referenced-paths-exist invariant the documented feature
// enforces on everyone else. A one-line heading satisfies none of that.
func TestC1679_004_ExperienceRecordLandedAndItsCitationsResolve(t *testing.T) {
	root := acsassert.RepoRoot(t)
	abs := filepath.Join(root, alignmentREADME)
	raw, err := os.ReadFile(abs)
	if err != nil {
		t.Fatalf("RED: cannot read %s: %v", alignmentREADME, err)
	}
	body := string(raw)

	// The file must be tracked — a gitignored doc is silently dropped at ship.
	if _, _, code, _ := acsassert.SubprocessOutput("git", "-C", root, "ls-files", "--error-unmatch", alignmentREADME); code != 0 {
		t.Errorf("RED: %s is untracked — it would be dropped at ship (cycle-93)", alignmentREADME)
	}

	// A §6.x experience record naming the cross-artifact stack must exist.
	head := regexp.MustCompile(`(?m)^### 6\.\d+ .*[Cc]ross-artifact.*$`)
	loc := head.FindStringIndex(body)
	if loc == nil {
		t.Fatalf("RED: no '### 6.x' experience-record subsection for the cross-artifact invariant stack "+
			"(item rank 5) in %s — §6 carries 6.1-6.3 only, and the inbox record's `fix` field requires "+
			"this entry per §3.8", alignmentREADME)
	}

	// Scope to that subsection: from its heading to the next '### ' or '## '.
	rest := body[loc[1]:]
	if next := regexp.MustCompile(`(?m)^#{2,3} `).FindStringIndex(rest); next != nil {
		rest = rest[:next[0]]
	}
	for _, want := range []string{"Issue", "Gap", "Solution", "Measured"} {
		if !strings.Contains(rest, want) {
			t.Errorf("RED: the §6.x cross-artifact entry has no **%s.** paragraph — §6.1-6.3 shape not followed", want)
		}
	}

	// Every repo path the entry cites must resolve on disk.
	cited := regexp.MustCompile(`go/(?:internal|cmd|acs|pkg)/[A-Za-z0-9_./-]+\.go`).FindAllString(rest, -1)
	if len(cited) == 0 {
		t.Errorf("RED: the §6.x entry cites no implementation file — an experience record with no Solution citation")
	}
	for _, rel := range cited {
		clean := strings.TrimRight(rel, ".,;:)")
		if _, err := os.Stat(filepath.Join(root, clean)); err != nil {
			t.Errorf("RED: the §6.x entry cites %s, which does not exist on disk — the doc fails the very "+
				"referenced-paths-exist invariant it documents", clean)
		}
	}
}

// TestC1679_005_PortfolioAndLayerRowsNoLongerMarkTheStackNewOrPartial closes the
// staleness half: two rows still describe the stack as unbuilt while the code is
// landed, wired and green.
//
// acs-predicate: config-check — document content is the deliverable (see 004).
// Each assertion is scoped to the ONE row that must change, so an unrelated edit
// elsewhere in the README can neither satisfy nor break it.
func TestC1679_005_PortfolioAndLayerRowsNoLongerMarkTheStackNewOrPartial(t *testing.T) {
	root := acsassert.RepoRoot(t)
	raw, err := os.ReadFile(filepath.Join(root, alignmentREADME))
	if err != nil {
		t.Fatalf("RED: cannot read %s: %v", alignmentREADME, err)
	}
	var portfolio, layer string
	for _, line := range strings.Split(string(raw), "\n") {
		switch {
		case strings.HasPrefix(line, "| 5 | **Cross-artifact metamorphic invariant stack**"):
			portfolio = line
		case strings.HasPrefix(line, "| **L3 Verification**"):
			layer = line
		}
	}
	if portfolio == "" {
		t.Errorf("RED: the ranked-portfolio row for item rank 5 is gone from %s", alignmentREADME)
	} else if strings.Contains(portfolio, "NEW — filed 0.85") {
		t.Errorf("RED: the rank-5 portfolio row still reads '**NEW — filed 0.85**' while the stack is landed, " +
			"wired at go/internal/core/cyclerun.go:241 and green")
	}
	if layer == "" {
		t.Errorf("RED: the L3 Verification row is gone from %s", alignmentREADME)
	} else if strings.Contains(layer, "cross-artifact invariant stack partial") {
		t.Errorf("RED: the L3 Verification row still calls the stack 'partial (`coherence`)' — all four " +
			"invariants now live in internal/coherence and are recorded from internal/core")
	}
}

// ---------------------------------------------------------------------------
// AUDIT REPAIR (round 2) — the two defects the auditor named, as RED tests.
//
// H1 -> 006. `TestC1676_006_MaterializedEvalIsDurableAndBehavioral` is RED in
// the shipped tree: it requires the eval it grades to carry >=4 `[code]`
// behavioural checks, and .evolve/evals/crossartifact-invariant-stack.md
// carries zero. BOTH sides are added paths in this diff, so this is the lane's
// own inconsistency. The frozen predicate is NOT the thing to change — 23 live
// evals already carry `score_cap` frontmatter AND `### ACn: … [code]` sections
// (docs/eval-grader-best-practices.md §"Recognized grader formats";
// .evolve/evals/retro-delivery-format-binding.md:49 is the canonical shape), so
// the eval is simply missing its code-graded half.
//
// M1 -> 007. The cycle claimed a verification it never ran, "and no gate before
// ship could have caught it". The remedy cannot be report prose — a predicate
// that greps build-report.md for an invocation string is exactly the gameable
// grep cycle-85 forbids, and the cycle-75 lesson is that Builder-authored
// verification prose must never be the load-bearing evidence. So 007 RUNS the
// ship-time added-test backstop DURING the cycle: it re-derives the same seed
// (go/**/*_test.go the tree ADDS), groups by declared build tags and executes
// each group, mirroring internal/phases/ship/repocontract.go:353
// addedTestPackageGroups. A claim cannot outrun evidence the gate itself
// produces.
//
// Flaky-shape contract (Gate D): no `/...` sweep and no known-slow suite named
// — the run set is the diff's own added packages, which after self-exclusion is
// `./acs/cycle1676` alone (4.5s measured); every git call is `-C` anchored and
// every go call is `go -C` anchored, so cwd never decides the answer; no
// wall-clock bounds, no literal PIDs, no un-reaped load generators.
// ---------------------------------------------------------------------------

const (
	// selfPkg is THIS package. 007 must never run it: a test that shells
	// `go test` on its own package recurses until the process dies.
	selfPkg = "./acs/cycle1679"

	// gradingPredicate is the frozen added predicate the auditor found RED,
	// and gradedEval is the added eval it grades. The pair must agree.
	gradingPredicate = "TestC1676_006_MaterializedEvalIsDurableAndBehavioral"
	gradedEvalPkg    = "./acs/cycle1676"
)

// goModRoot anchors every `go` invocation at the lane's module root, so a
// predicate resolves the same package patterns from the main tree, this
// worktree, or any fleet lane's cwd.
func goModRoot(t *testing.T) string {
	t.Helper()
	return filepath.Join(acsassert.RepoRoot(t), "go")
}

// TestC1679_006_AddedEvalAndItsGradingPredicateAgreeInTheShippedTree — audit H1.
//
// This does not restate 006's assertions (that would be a second copy to drift);
// it RUNS the real frozen predicate over the real eval and requires it to pass.
// Asserting on the `--- PASS:` line rather than exit 0 is load-bearing: a `-run`
// pattern that matches NO test exits 0 with "no tests to run", so a renamed or
// deleted predicate would otherwise false-GREEN this check.
func TestC1679_006_AddedEvalAndItsGradingPredicateAgreeInTheShippedTree(t *testing.T) {
	stdout, stderr, code, err := acsassert.SubprocessOutput(
		"go", "-C", goModRoot(t), "test", "-tags", "acs", "-count=1", "-v",
		"-run", "^"+gradingPredicate+"$", gradedEvalPkg)
	if code == -1 {
		t.Fatalf("go test failed to launch for %s: %v\nstderr:\n%s", gradedEvalPkg, err, stderr)
	}
	out := stdout + stderr
	if !strings.Contains(out, "--- PASS: "+gradingPredicate) {
		t.Errorf("RED (audit H1): %s does not pass over %s in the shipped tree, so the "+
			"repo-contract added-test backstop blocks the push. Both the predicate and the eval it "+
			"grades are ADDED paths in this diff — this is the lane's own inconsistency, not inherited "+
			"breakage.\n\nThe frozen predicate is not the thing to change (never weaken a test to clear "+
			"a rejection): it wants >=4 `[code]` behavioural checks in the eval, and the eval carries "+
			"zero. Add the code-graded half — `### ACn: <criterion> [code]` sections, each followed by a "+
			"```bash fence whose command RUNS something and exits 0 — alongside the existing score_cap "+
			"frontmatter. 23 live evals already carry both schemas; "+
			".evolve/evals/retro-delivery-format-binding.md:49 is the canonical shape.\n\n"+
			"exit=%d\ncombined go-test output:\n%s", gradingPredicate, gradedEvalPkg, code, out)
	}
}

// collectTags walks a parsed //go:build expression and records every tag it
// names, mirroring internal/phases/ship/repocontract.go collectBuildTags.
func collectTags(e constraint.Expr, set map[string]bool) {
	switch v := e.(type) {
	case *constraint.TagExpr:
		set[v.Tag] = true
	case *constraint.NotExpr:
		collectTags(v.X, set)
	case *constraint.AndExpr:
		collectTags(v.X, set)
		collectTags(v.Y, set)
	case *constraint.OrExpr:
		collectTags(v.X, set)
		collectTags(v.Y, set)
	}
}

// buildTagsOf returns the build tags a Go file's //go:build line declares, and
// whether the file is runnable here (a requires_tmux file is excluded by the
// production gate on this host, so it is excluded here too).
func buildTagsOf(t *testing.T, abs string) (tags []string, runnable bool) {
	t.Helper()
	f, err := os.Open(abs)
	if err != nil {
		t.Fatalf("open %s: %v", abs, err)
	}
	defer func() { _ = f.Close() }()

	var expr constraint.Expr
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if constraint.IsGoBuild(line) {
			expr, err = constraint.Parse(line)
			if err != nil {
				t.Fatalf("parse build constraint in %s: %v", abs, err)
			}
			break
		}
		// Stop at the first line that is neither blank nor a comment: the
		// build constraint must precede the package clause.
		if line != "" && !strings.HasPrefix(line, "//") {
			break
		}
	}
	if err := sc.Err(); err != nil {
		t.Fatalf("scan %s: %v", abs, err)
	}
	if expr == nil {
		return nil, true // untagged: runs in the default build context
	}
	set := map[string]bool{}
	collectTags(expr, set)
	if set["requires_tmux"] {
		return nil, false
	}
	for tag := range set {
		tags = append(tags, tag)
	}
	sort.Strings(tags)
	return tags, true
}

// addedGoTestFiles re-derives the ship gate's seed: the `go/**/*_test.go` files
// this tree ADDS. Both halves matter — the branch-point diff still reports the
// lane's tests as added once they are committed, and the porcelain half catches
// them while they are still staged or untracked (a lane's new test is untracked
// until the ship stages it, which is the case the production gate calls out).
func addedGoTestFiles(t *testing.T, root string) []string {
	t.Helper()
	seen := map[string]bool{}
	add := func(p string) {
		p = strings.TrimSpace(p)
		if strings.HasPrefix(p, "go/") && strings.HasSuffix(p, "_test.go") {
			seen[p] = true
		}
	}

	for _, base := range []string{"origin/main", "main"} {
		mb, _, code, _ := acsassert.SubprocessOutput("git", "-C", root, "merge-base", "HEAD", base)
		if code != 0 {
			continue
		}
		out, _, code, _ := acsassert.SubprocessOutput(
			"git", "-C", root, "diff", "--name-only", "--diff-filter=A", strings.TrimSpace(mb), "HEAD")
		if code == 0 {
			for _, line := range strings.Split(out, "\n") {
				add(line)
			}
			break
		}
	}

	if out, _, code, _ := acsassert.SubprocessOutput("git", "-C", root, "status", "--porcelain"); code == 0 {
		for _, line := range strings.Split(out, "\n") {
			if len(line) < 4 {
				continue
			}
			// XY<space>path; an added file is staged-A or untracked.
			if strings.HasPrefix(line, "A") || strings.HasPrefix(line, "??") {
				add(line[3:])
			}
		}
	}

	paths := make([]string, 0, len(seen))
	for p := range seen {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	return paths
}

// TestC1679_007_EveryAddedGoTestPackageIsGreenBeforeShip — audit M1.
//
// The ship-time added-test backstop is the gate that caught H1, and it fires at
// push, after the cycle's budget is spent. This runs that same gate DURING the
// cycle, from the same seed, so "the added packages are green" is a claim the
// cycle cannot make without the evidence having been produced.
//
// Scope is the TAG-GUARDED added packages, and that is the whole point rather
// than a shortcut. An untagged added test (internal/coherence,
// internal/core) already runs in the `go test -count=1 ./...` every cycle owes
// the Go conventions, so it was never the hole. A `//go:build acs` package is
// invisible to every ordinary run — which is precisely how
// go/acs/cycle1676/predicates_test.go reached ship red while a report claimed
// it verified. Narrowing here also keeps the flaky-shape contract: the run set
// is one small package, not the 89s ./internal/core suite the full seed pulls
// in (cycles 1173/1175/1178 FAILed on sound work for exactly that).
//
// The empty-seed case is RED, not a silent pass: this diff demonstrably adds
// two tag-guarded test packages, so a discovery that finds none means the
// discovery broke. A vacuous green here would reproduce the exact M1 shape it
// exists to close.
// recordedAddedTests is the lane's own added test files — the DURABLE seed.
// The live seed (addedGoTestFiles: the diff against the merge base plus the
// working tree) is what proves the claim while the lane is unmerged; once the
// lane's commit is on main that diff is empty by construction, and a durable
// predicate that fatals on it is red on every clean checkout (2026-09-15,
// research F21: RED on main c5883955 in every whole-module floor). On a
// merged tree the predicate verifies its recorded set instead — the same
// tag-guarded packages, executed the same way — so the proof M1 demanded
// never goes vacuous and never depends on the lane's tree state.
var recordedAddedTests = []string{
	"go/acs/cycle1676/predicates_test.go",
	"go/acs/cycle1679/predicates_test.go",
}

func TestC1679_007_EveryAddedGoTestPackageIsGreenBeforeShip(t *testing.T) {
	root := acsassert.RepoRoot(t)
	added := addedGoTestFiles(t, root)
	if len(added) == 0 {
		for _, rel := range recordedAddedTests {
			if _, err := os.Stat(filepath.Join(root, rel)); err != nil {
				t.Fatalf("RED (audit M1): the live added-test seed is empty (merged tree) and the recorded added file %s is missing (%v) — the lane's own tag-guarded package is gone", rel, err)
			}
		}
		added = recordedAddedTests
		t.Logf("live added-test seed empty (the lane's commit is on main) — verifying the recorded added set %v", added)
	}

	// Group added packages by the build tags their files declare, exactly as
	// internal/phases/ship/repocontract.go addedTestPackageGroups does.
	pkgsByTagKey := map[string]map[string]bool{}
	tagsByKey := map[string][]string{}
	var excluded, untagged []string
	for _, rel := range added {
		pkg := "./" + filepath.ToSlash(filepath.Dir(strings.TrimPrefix(rel, "go/")))
		if pkg == selfPkg {
			continue // never recurse into this predicate's own package
		}
		tags, runnable := buildTagsOf(t, filepath.Join(root, rel))
		if !runnable {
			excluded = append(excluded, rel)
			continue
		}
		if len(tags) == 0 {
			// Already exercised by the mandatory `go test -count=1 ./...`;
			// re-running it here buys nothing and drags in the slow suites.
			untagged = append(untagged, pkg)
			continue
		}
		key := strings.Join(tags, ",")
		if pkgsByTagKey[key] == nil {
			pkgsByTagKey[key] = map[string]bool{}
			tagsByKey[key] = tags
		}
		pkgsByTagKey[key][pkg] = true
	}
	for _, rel := range excluded {
		t.Logf("excluded %s (requires_tmux or another build constraint unavailable on this host)", rel)
	}
	sort.Strings(untagged)
	for _, pkg := range untagged {
		t.Logf("deferred %s to the mandatory `go test -count=1 ./...` (untagged: an ordinary run already executes it)", pkg)
	}
	if len(pkgsByTagKey) == 0 {
		t.Fatalf("RED (audit M1): no TAG-GUARDED added test package was found to verify, but this lane "+
			"adds go/acs/cycle1676/predicates_test.go (`//go:build acs`). A tag-guarded package that no "+
			"gate runs in-cycle is exactly the hole M1 named. Seed was: %v", added)
	}

	keys := make([]string, 0, len(pkgsByTagKey))
	for key := range pkgsByTagKey {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	for _, key := range keys {
		pkgs := make([]string, 0, len(pkgsByTagKey[key]))
		for pkg := range pkgsByTagKey[key] {
			pkgs = append(pkgs, pkg)
		}
		sort.Strings(pkgs)

		args := append([]string{"-C", goModRoot(t), "test", "-count=1",
			"-tags", strings.Join(tagsByKey[key], ",")}, pkgs...)

		stdout, stderr, code, err := acsassert.SubprocessOutput("go", args...)
		if code == -1 {
			t.Fatalf("go test failed to launch for %v (tags %q): %v\nstderr:\n%s", pkgs, key, err, stderr)
		}
		if code != 0 {
			t.Errorf("RED (audit M1): added test package(s) %v are NOT green under tags %q — the "+
				"repo-contract added-test backstop will block the push at ship. Running it here, in "+
				"cycle, is the point: the previous round CLAIMED these were re-verified and no "+
				"invocation ever ran.\nexit=%d\ncombined go-test output:\n%s",
				pkgs, key, code, stdout+stderr)
		}
	}
}

// numberWords maps the spelled-out forms an explanation document may use for a
// small count back to digits, so the claim is compared numerically rather than
// by matching one blessed spelling.
var numberWords = map[string]int{
	"zero": 0, "one": 1, "two": 2, "three": 3, "four": 4, "five": 5,
	"six": 6, "seven": 7, "eight": 8, "nine": 9, "ten": 10,
}

// parseCount reads a count written either as digits ("7") or as an English word
// ("seven"), returning false when the token is neither.
func parseCount(tok string) (int, bool) {
	tok = strings.ToLower(strings.Trim(tok, "*_`.,"))
	if n, ok := numberWords[tok]; ok {
		return n, true
	}
	var n int
	if _, err := fmt.Sscanf(tok, "%d", &n); err == nil {
		return n, true
	}
	return 0, false
}

// TestC1679_008_ExplanationDocumentCountsAgreeWithTheShippedTree — audit M1.
//
// The explanation document is cycle-OWNED narrative, and M1 caught it asserting
// "this cycle's five acceptance predicates are 5/5 PASS" over a tree carrying
// seven, two of them RED. That is the claim-discrepancy class the explanation
// contract exists to catch, and nothing graded it — the document was left at its
// round-1 text while TDD round 2 added 006/007.
//
// Both sides of the comparison are derived from reality: the actual count is
// parsed out of the predicate file's own declarations, and the claimed count is
// parsed out of the document's prose. A magic string cannot satisfy it — the
// only way to green is for the narrative's arithmetic to match the tree's. The
// document is located by GLOB rather than by a pinned ULID filename so a
// regenerated explanation is still graded instead of silently skipped.
func TestC1679_008_ExplanationDocumentCountsAgreeWithTheShippedTree(t *testing.T) {
	root := acsassert.RepoRoot(t)

	// Actual: count this cycle's predicate declarations in the shipped file.
	predFile := filepath.Join(root, "go", "acs", "cycle1679", "predicates_test.go")
	src, err := os.ReadFile(predFile)
	if err != nil {
		t.Fatalf("read %s: %v", predFile, err)
	}
	actual := len(regexp.MustCompile(`(?m)^func TestC1679_\d+_`).FindAllString(string(src), -1))
	if actual == 0 {
		t.Fatalf("RED: found 0 TestC1679_* declarations in %s — the discovery is broken, and a "+
			"vacuous pass here would reproduce the very unearned claim M1 named", predFile)
	}

	matches, err := filepath.Glob(filepath.Join(root, "docs", "explain", "builds", "cycle-1679-*.md"))
	if err != nil || len(matches) == 0 {
		t.Fatalf("RED: no explanation document matched docs/explain/builds/cycle-1679-*.md (err=%v) — "+
			"the cycle's explanation contract names one as required", err)
	}

	for _, abs := range matches {
		rel, relErr := filepath.Rel(root, abs)
		if relErr != nil {
			rel = abs
		}
		if _, _, code, _ := acsassert.SubprocessOutput("git", "-C", root, "ls-files", "--error-unmatch", rel); code != 0 {
			t.Errorf("RED: %s is untracked — it may be gitignored and dropped at ship (cycle-93)", rel)
		}

		raw, readErr := os.ReadFile(abs)
		if readErr != nil {
			t.Fatalf("read %s: %v", rel, readErr)
		}
		// Collapse whitespace so a claim wrapped across lines ("five acceptance\n
		// predicates") is still matched as one phrase.
		flat := strings.Join(strings.Fields(string(raw)), " ")

		// Claim form A: "<n> acceptance predicates".
		for _, m := range regexp.MustCompile(`(?i)(\S+) acceptance predicates`).FindAllStringSubmatch(flat, -1) {
			claimed, ok := parseCount(m[1])
			if !ok {
				continue // e.g. "the acceptance predicates" — no number claimed
			}
			if claimed != actual {
				t.Errorf("RED (audit M1): %s claims %q but the shipped tree carries %d "+
					"TestC1679_* predicates. A cycle-owned explanation whose arithmetic disagrees "+
					"with its own tree is the claim-discrepancy class the explanation contract exists "+
					"to catch. Update the narrative to the tree; never the reverse.", rel, m[0], actual)
			}
		}

		// Claim form B: "... predicates are N/M PASS".
		for _, m := range regexp.MustCompile(`(?i)predicates are (\d+)/(\d+) PASS`).FindAllStringSubmatch(flat, -1) {
			passed, total := 0, 0
			fmt.Sscanf(m[1], "%d", &passed)
			fmt.Sscanf(m[2], "%d", &total)
			if total != actual {
				t.Errorf("RED (audit M1): %s asserts %q over a tree carrying %d predicates — the "+
					"document reports a green suite whose size it gets wrong.", rel, m[0], actual)
			}
			if passed != total {
				t.Errorf("RED (audit M1): %s asserts %q — a cycle must not ship an explanation "+
					"claiming a partially-red acceptance suite is verified.", rel, m[0])
			}
		}
	}
}
