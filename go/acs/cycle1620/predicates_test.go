//go:build acs

// Package cycle1620 materializes the cycle-1620 acceptance criteria for the one
// task this fleet lane committed (lane-scope.json todo_ids; triage-report.md
// ## top_n): `multi-slug-lane-scope-reconciliation`. Per R9.3 nothing here binds
// to the deferred `per-slug-lane-splitting` or `out-of-lane-carryovers` items —
// they get ZERO predicates.
//
// The defect (cycle-1480 batch-20260815c wave-2, recurred cycle-1483). A lane
// bundling `minted-phase-verdict-contract-unsatisfiable` + `dead-api-sweep` had
// TDD mint a cycle-wide predicate suite covering BOTH members while the
// Builder's deliverable contract bound only the FIRST. Nothing reconciled the
// two scopes, so the lane ran the full ~12-phase spine and FAILed at audit with
// slug 2 entirely undelivered. Reproduced this cycle in
// .evolve/runs/cycle-1620/bug-reproduction-report.md (the enforce-stage reviewer
// returns Approve:true on the two-member/one-declared shape).
//
// AC map (1:1 with test-report.md ## AC-Materialization):
//
//	AC1 2-slug lane delivers both or blocks at TDD->Build  → C1620_001..004
//	AC2 declared set == committed set, deterministic check  → C1620_005, C1620_006
//	    with a red-first test pinning the cycle-1480 shape
//	AC3 single projection source, no second lane-scope parser → C1620_007
//	AC4 go vet + touched suites + -race green               → C1620_008
//	(durability, cycle-1501 lesson: the recurring slug's eval) → C1620_009
//
// Adversarial axes. NEGATIVE: C1620_002 and C1620_003 — the cheapest way to pass
// every block predicate is to reject all multi-member lanes (or every lane), which
// kills the fleet's only legitimate bundling path and every single-slug cycle;
// both must still proceed. EDGE/OOD: C1620_006 — a COMPLETE handoff shown inside
// an outer `~~~markdown` example fence is illustration, not declaration, and must
// not be mistaken for the real one (the 2026-09-09 recovery review's false-accept
// control). SEMANTIC: blocking, proceeding, production arming, projection
// single-sourcing, toolchain health and durable memory are six distinct
// behaviors, not one behavior restated.
//
// No grep-only predicates (the cycle-85 ban): C1620_001..003 and C1620_006 drive
// the REAL production reviewer constructor at the REAL enforce stage and assert
// on its verdict; C1620_004 and C1620_007 call real functions and assert on
// returned values (their file assertions are auxiliary); C1620_005 and C1620_008
// execute the toolchain and require named markers / exit codes; C1620_009 runs
// the eval's own graders.
package cycle1620

import (
	"context"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/cycleoutcome"
	"github.com/mickeyyaya/evolve-loop/go/internal/evalqualitycheck"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/topngate"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	topngatePkg = "github.com/mickeyyaya/evolve-loop/go/internal/topngate"

	// redFirstTest is the AC2-mandated red-first unit test pinning the
	// cycle-1480 shape, and redFirstFile is where it lives.
	redFirstTest = "TestTDDScopeGate_TwoSlugHandoffOmittingCommittedMemberBlocks"
	redFirstFile = "go/internal/topngate/scope_reconciliation_test.go"

	evalSlug = "multi-slug-lane-scope-reconciliation"

	// mdFence is the markdown code-fence delimiter (a Go raw string cannot
	// contain a backtick).
	mdFence = "```"

	slugA = "minted-phase-verdict-contract-unsatisfiable"
	slugB = "dead-api-sweep"
)

// writeTriage writes a triage-report.md committing exactly topN under ## top_n,
// in the shape agents/evolve-triage.md contracts.
func writeTriage(t *testing.T, workspace string, topN ...string) {
	t.Helper()
	var ids []string
	for _, s := range topN {
		ids = append(ids, `{"id":"`+s+`"}`)
	}
	writeFile(t, filepath.Join(workspace, "triage-decision.json"), `{"top_n":[`+strings.Join(ids, ",")+`],"deferred":[]}`)
	var b strings.Builder
	b.WriteString("<!-- ANCHOR:triage_decision -->\n# Triage Decision — Cycle 1480\n\n## top_n (commit to THIS cycle)\n")
	for _, s := range topN {
		b.WriteString("- " + s + ": committed for this cycle — priority=H, evidence=x, source=inbox\n")
	}
	b.WriteString("\n## deferred (carry to NEXT cycle's carryoverTodos)\n- none\n")
	writeFile(t, filepath.Join(workspace, "triage-report.md"), b.String())
}

// writeTDD writes a test-report.md declaring exactly `declared` as its member
// set, stated BOTH ways a TDD deliverable can state it — the "## Task:" header
// and the "## Handoff to Builder" JSON's slugs[] — so these predicates bind to
// the DECLARATION rather than to one syntax. preamble is rendered verbatim
// between the header and the RED output.
func writeTDD(t *testing.T, workspace string, declared []string, preamble string) {
	t.Helper()
	handoff, err := json.Marshal(map[string]any{
		"slugs":            declared,
		"testFiles":        []string{"go/acs/cycle1480/predicates_test.go"},
		"redRunConfirmed":  true,
		"doNotModifyTests": true,
	})
	if err != nil {
		t.Fatalf("marshal handoff: %v", err)
	}
	writeFile(t, filepath.Join(workspace, "test-report.md"), strings.Join([]string{
		"# TDD Report — Cycle 1480",
		"",
		"## Task: " + strings.Join(declared, ", "),
		"",
		preamble,
		"## RED Run Output",
		"",
		mdFence, "FAIL", mdFence,
		"",
		"## Handoff to Builder",
		"",
		mdFence + "json",
		string(handoff),
		mdFence,
		"",
	}, "\n"))
}

func writeFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

// reviewTDD drives the PRODUCTION reviewer at the TDD->Build boundary: the same
// constructor and stage cmd/evolve/cmd_cycle.go wires into the cycle's reviewer
// chain. Calling an internal helper directly would prove nothing about the
// shipped path.
func reviewTDD(workspace string) core.ReviewResult {
	return topngate.NewReviewer(config.StageEnforce).Review(context.Background(), core.ReviewInput{
		Phase:     string(core.PhaseTDD),
		Workspace: workspace,
	})
}

// AC1: the cycle-1480 shape. Two committed members, one declared — Build must
// not start, and the block must NAME the undelivered member so the operator
// does not have to re-derive the omission from a diff.
func TestC1620_001_two_slug_lane_blocks_at_tdd_build_boundary(t *testing.T) {
	ws := t.TempDir()
	writeTriage(t, ws, slugA, slugB)
	writeTDD(t, ws, []string{slugA}, "")

	got := reviewTDD(ws)
	if got.Approve || !strings.Contains(got.Reason, "scope-mismatch") {
		t.Fatalf("a two-member commitment with a one-member TDD declaration must block before Build with a "+
			"named scope-mismatch defect — it may no longer run the full spine and FAIL at audit; got %+v", got)
	}
	if !strings.Contains(got.Reason, slugB) {
		t.Errorf("the block must name the omitted member %q; reason=%q", slugB, got.Reason)
	}
}

// AC1 (NEGATIVE / anti-no-op): the cheapest way to pass C1620_001 is to reject
// every multi-member lane. That would delete the fleet's only legitimate
// bundling path, so a COMPLETE declaration must proceed — in either order,
// because the committed members are an unordered set.
func TestC1620_002_complete_two_slug_declaration_proceeds(t *testing.T) {
	for _, declared := range [][]string{{slugA, slugB}, {slugB, slugA}} {
		ws := t.TempDir()
		writeTriage(t, ws, slugA, slugB)
		writeTDD(t, ws, declared, "")

		if got := reviewTDD(ws); !got.Approve {
			t.Errorf("a two-member commitment whose TDD declaration covers BOTH members must proceed to Build "+
				"(declared order %v must not matter); got %+v", declared, got)
		}
	}
}

// AC1 (EDGE / no-regression): single-member lanes are the overwhelming majority.
// Their fail-open behavior must be untouched by the new equality rule — a gate
// that blocks them converts a fleet-efficiency fix into a fleet outage.
func TestC1620_003_single_slug_lane_unaffected(t *testing.T) {
	ws := t.TempDir()
	writeTriage(t, ws, evalSlug)
	writeTDD(t, ws, []string{evalSlug}, "")

	if got := reviewTDD(ws); !got.Approve {
		t.Fatalf("a single-member lane whose TDD declaration matches must keep proceeding; got %+v", got)
	}
}

// AC1 (WIRING PROOF): the reconciliation must be reachable from the PRODUCTION
// composition, not just from a test. cmd/evolve/cmd_cycle.go appends
// topngate.NewReviewer(cfg.TopNGate) to the cycle's reviewer chain, and the
// resolved gate stage must be enforce by default — a correct gate wired at
// StageOff (or unregistered) blocks nothing and the cycle-1480 burn recurs.
func TestC1620_004_gate_is_armed_in_the_production_composition(t *testing.T) {
	if stage := (policy.Policy{}).GatesConfig().TopNGate; stage != "enforce" {
		t.Errorf("the resolved default TopNGate stage is %q, not \"enforce\" — the reconciliation would never block a live lane", stage)
	}
	root := acsassert.RepoRoot(t)
	composition := filepath.Join(root, "go", "cmd", "evolve", "cmd_cycle.go")
	if !acsassert.FileContains(t, composition, "topngate.NewReviewer(cfg.TopNGate)") {
		t.Errorf("the composition root no longer registers the topngate reviewer — the TDD->Build reconciliation is unreachable from production")
	}
}

// AC2: the red-first test pinning the cycle-1480 shape must exist IN THE REPO,
// be git-tracked (an untracked reproducer is dropped at ship — the cycle-92
// class), and PASS. One named test in ONE small package, -run-narrowed; the
// named `--- PASS:` marker is required because `go test -run` exits 0 when it
// matches nothing.
func TestC1620_005_red_first_test_pins_the_cycle_1480_shape(t *testing.T) {
	root := acsassert.RepoRoot(t)
	if !acsassert.FileExists(t, filepath.Join(root, redFirstFile)) {
		t.Fatalf("RED: %s missing on disk", redFirstFile)
	}
	if _, _, code, _ := acsassert.SubprocessOutput("git", "-C", root, "ls-files", "--error-unmatch", redFirstFile); code != 0 {
		t.Errorf("RED: %s is untracked — it would be dropped at ship and the class would lose its regression pin", redFirstFile)
	}
	stdout, stderr, code, err := acsassert.SubprocessOutput(
		"go", "test", "-count=1", "-v", "-run", "^"+redFirstTest+"$", topngatePkg)
	if code != 0 || err != nil {
		t.Errorf("go test -run %s %s exited %d (err=%v)\nstdout:\n%s\nstderr:\n%s", redFirstTest, topngatePkg, code, err, stdout, stderr)
	}
	if !strings.Contains(stdout, "--- PASS: "+redFirstTest) {
		t.Errorf("%s did not report PASS (renamed, skipped, or still red)\nstdout:\n%s", redFirstTest, stdout)
	}
}

// AC2 (ADVERSARIAL / OOD): a report that DOCUMENTS a complete handoff inside an
// outer `~~~markdown` example fence while declaring one member must still block.
// Illustration is not declaration; the live gate's fence walker takes the FIRST
// DECLARING JSON fence inside the real handoff section (a fence carrying neither
// slugs nor testFiles never counts — cycle-1620 audit M1), which is exactly how
// a fake complete declaration inside an example fence must not evade the check
// (2026-09-09 recovery review).
func TestC1620_006_outer_fence_fake_complete_handoff_rejected(t *testing.T) {
	ws := t.TempDir()
	writeTriage(t, ws, slugA, slugB)
	fake := strings.Join([]string{
		"## Handoff schema (documentation only — NOT this cycle's declaration)",
		"",
		"~~~markdown",
		"## Handoff to Builder",
		"",
		mdFence + "json",
		`{"slugs": ["` + slugA + `", "` + slugB + `"], "testFiles": ["go/acs/cycleN/predicates_test.go"], "redRunConfirmed": true}`,
		mdFence,
		"~~~",
		"",
	}, "\n")
	writeTDD(t, ws, []string{slugA}, fake)

	got := reviewTDD(ws)
	if got.Approve || !strings.Contains(got.Reason, "scope-mismatch") {
		t.Fatalf("a complete handoff shown inside an OUTER example fence must not be read as the declaration — "+
			"the lane still declares one of two committed members and must block; got %+v", got)
	}
}

// AC3: ONE projection source yields the lane's slug set. The behavioral half
// calls the surviving exported consumer entry point and pins its contract
// (member ORDER preserved; fail-open to nil on an absent/malformed pin — a
// projection that aborts a lane on a broken pin recreates the cycle-760..762
// destruction class). The structural half is the AC's own literal requirement
// ("grep-proof: no second parser of lane-scope slugs"): exactly ONE non-test Go
// file may declare the lane-scope wire shape: go/internal/core/lanescope.go
// (cycleoutcome and the audit defect ledger read it through core.LaneScopeIDs).
func TestC1620_007_one_lane_scope_projection_for_both_consumers(t *testing.T) {
	ws := t.TempDir()
	writeFile(t, filepath.Join(ws, "lane-scope.json"), `{"todo_ids":["alpha","beta"],"goal_hash":"h"}`)
	if got := cycleoutcome.LaneScopeIDs(ws); !slices.Equal(got, []string{"alpha", "beta"}) {
		t.Errorf("the lane-scope projection must yield the pinned members IN ORDER; got %v", got)
	}
	malformed := t.TempDir()
	writeFile(t, filepath.Join(malformed, "lane-scope.json"), "{not json")
	if got := cycleoutcome.LaneScopeIDs(malformed); len(got) != 0 {
		t.Errorf("a malformed pin must fail OPEN to the empty projection, never a partial set; got %v", got)
	}
	if got := cycleoutcome.LaneScopeIDs(t.TempDir()); len(got) != 0 {
		t.Errorf("an absent pin must fail OPEN to the empty projection; got %v", got)
	}

	decls := filesDeclaringLaneScopeShape(t, filepath.Join(acsassert.RepoRoot(t), "go"))
	if len(decls) != 1 {
		t.Errorf("the lane-scope slug set must have exactly ONE parser; %d non-test files declare the wire shape: %v",
			len(decls), decls)
	}
}

// filesDeclaringLaneScopeShape returns the repo-relative non-test Go files that
// declare the lane-scope wire shape (a `json:"todo_ids"` struct tag) — one entry
// per file, sorted, so the failure message is deterministic.
func filesDeclaringLaneScopeShape(t *testing.T, goRoot string) []string {
	t.Helper()
	var out []string
	err := filepath.WalkDir(goRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if name := d.Name(); name == "testdata" || name == "acs" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		body, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		if strings.Contains(string(body), `json:"todo_ids"`) {
			rel, relErr := filepath.Rel(goRoot, path)
			if relErr != nil {
				rel = path
			}
			out = append(out, filepath.ToSlash(rel))
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", goRoot, err)
	}
	slices.Sort(out)
	return out
}

// AC4: the touched suite must be green under vet and the race detector. Scoped
// to the ONE package this cycle changes behavior in — a whole-repo sweep is a
// banned flaky-predicate shape, and the wider `./internal/core -race` run is
// carried by the eval's grader, the build floor and CI (see test-report.md).
func TestC1620_008_touched_suite_is_vet_and_race_green(t *testing.T) {
	if stdout, stderr, code, err := acsassert.SubprocessOutput("go", "vet", topngatePkg); code != 0 || err != nil {
		t.Errorf("go vet %s exited %d (err=%v)\nstdout:\n%s\nstderr:\n%s", topngatePkg, code, err, stdout, stderr)
	}
	stdout, stderr, code, err := acsassert.SubprocessOutput("go", "test", "-race", "-count=1", topngatePkg)
	if code != 0 || err != nil {
		t.Errorf("go test -race %s exited %d (err=%v)\nstdout:\n%s\nstderr:\n%s", topngatePkg, code, err, stdout, stderr)
	}
}

// The recurring slug's durable memory (cycle-1501 lesson: an eval re-authored
// per attempt inside a discarded worktree lets attempt N+1 drop the criterion
// attempt N was graded CRITICAL on). The eval must be tracked, must clear the
// SSOT rigor checker, must carry one grader per behavioral criterion, and its
// FIRST grader is EXECUTED here so a decorative evidence string cannot stand in
// for a runnable one.
func TestC1620_009_eval_pins_the_class_durably(t *testing.T) {
	root := acsassert.RepoRoot(t)
	rel := filepath.Join(".evolve", "evals", evalSlug+".md")
	path := filepath.Join(root, rel)
	if !acsassert.FileExists(t, path) {
		t.Fatalf("RED: %s missing — the recurring slug would carry no machine-checked memory into its next attempt", rel)
	}
	if _, _, code, _ := acsassert.SubprocessOutput("git", "-C", root, "ls-files", "--error-unmatch", rel); code != 0 {
		t.Errorf("RED: %s is untracked — it would be dropped at ship", rel)
	}
	res, err := evalqualitycheck.Check(evalqualitycheck.Options{Path: path})
	if err != nil {
		t.Fatalf("eval quality-check %s: %v", path, err)
	}
	if res.Overall != evalqualitycheck.LevelPass {
		for _, c := range res.Commands {
			if c.Level != evalqualitycheck.LevelPass {
				t.Errorf("eval command %q classified level %d: %s", c.Line, c.Level, c.Reason)
			}
		}
		t.Fatalf("eval %s overall level %d, want PASS(0)", path, res.Overall)
	}
	caps := parseScoreCaps(t, path)
	if len(caps) < 4 {
		t.Fatalf("eval %s declares %d score_cap row(s); the inbox item carries 4 acceptance criteria and needs a grader per behavioral one", rel, len(caps))
	}
	for i, c := range caps {
		if c.criterion == "" || c.evidence == "" {
			t.Errorf("eval score_cap[%d] incomplete: criterion=%q evidence=%q — a grader with no criterion or no command caps nothing", i, c.criterion, c.evidence)
		}
	}
	if t.Failed() {
		return
	}
	// Behavioral half: the eval's own first grader must actually run green.
	stdout, stderr, code, err := acsassert.SubprocessOutput("sh", "-c", "cd "+root+" && "+caps[0].evidence)
	if code != 0 || err != nil {
		t.Errorf("eval grader %q exited %d (err=%v)\nstdout:\n%s\nstderr:\n%s", caps[0].evidence, code, err, stdout, stderr)
	}
}

// scoreCap is one row of the eval's YAML frontmatter score_cap block.
type scoreCap struct{ criterion, evidence string }

// parseScoreCaps reads the eval's score_cap rows. The eval is a deliverable of
// THIS cycle, so an unreadable or malformed one is a failure, never a skip.
func parseScoreCaps(t *testing.T, path string) []scoreCap {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read eval %s: %v", path, err)
	}
	var caps []scoreCap
	for _, line := range strings.Split(string(raw), "\n") {
		trimmed := strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(trimmed, "- criterion:"):
			caps = append(caps, scoreCap{criterion: unquote(strings.TrimPrefix(trimmed, "- criterion:"))})
		case strings.HasPrefix(trimmed, "evidence:") && len(caps) > 0:
			caps[len(caps)-1].evidence = unquote(strings.TrimPrefix(trimmed, "evidence:"))
		case trimmed == "---" && len(caps) > 0:
			return caps
		}
	}
	return caps
}

func unquote(s string) string { return strings.Trim(strings.TrimSpace(s), `"`) }
