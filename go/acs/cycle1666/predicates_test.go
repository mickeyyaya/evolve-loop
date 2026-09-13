//go:build acs

// Package cycle1666 materializes the acceptance criteria of the ONE inbox item
// this fleet lane committed (lane-scope.json todo_ids ∩ triage-report.md
// ## top_n) — and nothing else (R9.3):
//
//	dossier-corpus-carries-retro-mislabel  (medium, weight 0.84, defect)
//
// The lane's second scoped id, lost-ship-dossier-evidence, was triage-DROPPED
// (stale-consumed: its only record is .evolve/inbox/consumed/…, shipped
// 2026-09-13) and gets ZERO predicates here.
//
// The defect. PR #389 made the retro-skip mislabel fix FORWARD-ONLY: a
// pre-fix dossier whose `skipped_phases:[{phase:retro,reason:FAIL}]` means
// "retro RAN and its verdict was declined" is byte-for-byte the same shape as
// a post-fix dossier whose identical entry means "retro did not run". 134
// committed records (cycles 823-1217) carry that mislabel; the ledger holds a
// {role:retro, kind:agent_subprocess} receipt for every one of them, while
// their run dirs are gone. The remedy chosen is the inbox record's option
// (b): a `schema_version` discriminator on every new record, and a corpus
// seam that refuses to read a legacy entry as a skip — never the backfill.
//
// AC map (1:1 with test-report.md ## AC-Materialization):
//
//	AC1 a count of affected dossiers, derived not estimated, with the
//	    artifact cross-check that proves each one's retro really ran     → 001
//	AC2 the discriminator (not the backfill), choice justified in the
//	    commit body — the corpus is left untouched                        → 002
//	AC3 TestSchema_NoDrift stays green: the field on BOTH sides           → 003
//	AC4 a consumer-side test proving a pre-fix record is not treated as
//	    evidence retro was skipped                                       → 004
//
// Adversarial axes (skills/adversarial-testing §6). NEGATIVE: 002's
// no-rewrite pin over the real corpus (a backfill greens 001 and 004 and
// fails this), the absent-corpus exit in 001's binding, the legacy
// never-trusted rows in 004's binding. EDGE/OOD: wrong-cycle receipt, corrupt
// ledger line, empty run dir, nil record, empty phase name. SEMANTIC: derived
// count (001), forward-only stamp + untouched history (002), schema lockstep
// (003), consumer degrade (004) — four distinct behaviours.
//
// Flaky-shape contract: ONE named package per invocation, always -run
// narrowed (cmd/evolve is a known-slow suite), no wall-clock bounds, no
// literal PIDs, every git call is -C anchored.
//
// Reachability probe (cycle-644 rule): this package imports only
// pkg/acsassert and the standard library — a leaf. The frozen bindings are
// in-package (internal/dossier) or in package main (cmd/evolve, which already
// imports internal/dossier via cmd_dossier.go) — no new import edge is pinned.
package cycle1666

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	corePkg    = "github.com/mickeyyaya/evolve-loop/go/internal/core"
	dossierPkg = "github.com/mickeyyaya/evolve-loop/go/internal/dossier"
	cmdPkg     = "github.com/mickeyyaya/evolve-loop/go/cmd/evolve"

	// legacyRetroSkipFloor is the derived (not estimated) size of the affected
	// legacy set at RED time: 134 unversioned records carrying a retro
	// skipped_phases entry, cycles 823-1217, every one with a ledger receipt.
	// Legacy history is frozen — the count can only grow (an unversioned record
	// written by a pre-fix binary during this batch) — so fewer means the
	// corpus was rewritten.
	legacyRetroSkipFloor = 134
)

// ---------------------------------------------------------------------------
// Harness: the real CLI, built once (the cycle-1648/1659 TestMain shape).
// ---------------------------------------------------------------------------

var (
	evolveBin      string
	evolveBuildErr error
)

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "acs-cycle1666-")
	if err != nil {
		fmt.Fprintln(os.Stderr, "acs/cycle1666: tempdir:", err)
		os.Exit(1)
	}
	evolveBin = filepath.Join(dir, "evolve")
	root, err := repoRootFromCwd()
	if err != nil {
		evolveBuildErr = err
	} else {
		out, err := exec.Command("go", "-C", filepath.Join(root, "go"), "build", "-o", evolveBin, "./cmd/evolve").CombinedOutput()
		if err != nil {
			evolveBuildErr = fmt.Errorf("go build ./cmd/evolve: %v\n%s", err, out)
		}
	}
	code := m.Run()
	_ = os.RemoveAll(dir)
	os.Exit(code)
}

// repoRootFromCwd mirrors acsassert.RepoRoot for TestMain (no *testing.T yet).
func repoRootFromCwd() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	out, err := exec.Command("git", "-C", cwd, "rev-parse", "--show-toplevel").Output()
	if err != nil {
		return "", fmt.Errorf("git rev-parse --show-toplevel: %v", err)
	}
	return strings.TrimSpace(string(out)), nil
}

func evolveBinary(t *testing.T) string {
	t.Helper()
	if evolveBuildErr != nil {
		t.Fatalf("RED (harness): %v", evolveBuildErr)
	}
	return evolveBin
}

// assertSuiteTestsPass shells `go test [-tags T] -run '^(names)$' -count=1 -v
// pkg` against ONE named package and requires EVERY name to print a
// `--- PASS: <name>` line. Asserting on the PASS line, never exit 0, is
// load-bearing: a pattern matching NO test exits 0 with "no tests to run",
// so a still-missing binding would false-GREEN. The two argv shapes are
// spelled out as literal calls so the flaky-shape lint sees the -run
// narrowing one hop into this helper.
func assertSuiteTestsPass(t *testing.T, tags, pkg string, names ...string) {
	t.Helper()
	pattern := "^(" + strings.Join(names, "|") + ")$"
	var (
		stdout, stderr string
		code           int
		err            error
	)
	if tags == "" {
		stdout, stderr, code, err = acsassert.SubprocessOutput("go", "test", "-run", pattern, "-count=1", "-v", pkg)
	} else {
		stdout, stderr, code, err = acsassert.SubprocessOutput("go", "test", "-tags", tags, "-run", pattern, "-count=1", "-v", pkg)
	}
	if code == -1 {
		t.Fatalf("go test failed to launch for %s: %v\nstderr:\n%s", pkg, err, stderr)
	}
	out := stdout + stderr
	for _, name := range names {
		if !strings.Contains(out, "--- PASS: "+name) {
			t.Errorf("binding test %s did NOT pass in %s "+
				"(missing, failing, or hidden behind a build tag). exit=%d\ncombined go-test output:\n%s",
				name, pkg, code, out)
		}
	}
}

// ---------------------------------------------------------------------------
// Independent derivation over the REAL corpus — the oracle 001 compares the
// CLI against. Written from the wire (map[string]any), never from the
// package under test, so the two derivations share no code.
// ---------------------------------------------------------------------------

// legacyRetroSkipCycles returns, ascending, every cycle whose committed
// dossier has NO schema_version and names retro in skipped_phases.
func legacyRetroSkipCycles(t *testing.T, root string) []int {
	t.Helper()
	dir := filepath.Join(root, "knowledge-base", "cycles")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read corpus %s: %v", dir, err)
	}
	var out []int
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasPrefix(name, "cycle-") || filepath.Ext(name) != ".json" {
			continue
		}
		n, cerr := strconv.Atoi(strings.TrimSuffix(strings.TrimPrefix(name, "cycle-"), ".json"))
		if cerr != nil {
			continue
		}
		raw, rerr := os.ReadFile(filepath.Join(dir, name))
		if rerr != nil {
			continue
		}
		var m map[string]any
		if json.Unmarshal(raw, &m) != nil {
			continue
		}
		if _, stamped := m["schema_version"]; stamped {
			continue
		}
		if skipped, ok := m["skipped_phases"].([]any); ok {
			for _, s := range skipped {
				if entry, ok := s.(map[string]any); ok && entry["phase"] == "retro" {
					out = append(out, n)
					break
				}
			}
		}
	}
	sort.Ints(out)
	return out
}

// retroRanReceipts returns the set of cycles for which an execution receipt
// proves retro ran: a retro report in the cycle's run dir, or a ledger
// {role:retro, kind:agent_subprocess} entry. An absent ledger or runs dir
// simply contributes nothing.
func retroRanReceipts(t *testing.T, root string, cycles []int) map[int]bool {
	t.Helper()
	got := map[int]bool{}
	for _, c := range cycles {
		runDir := filepath.Join(root, ".evolve", "runs", fmt.Sprintf("cycle-%d", c))
		for _, name := range []string{"retrospective-report.md", "retro-report.md"} {
			if _, err := os.Stat(filepath.Join(runDir, name)); err == nil {
				got[c] = true
			}
		}
	}
	raw, err := os.ReadFile(filepath.Join(root, ".evolve", "ledger.jsonl"))
	if err != nil {
		t.Logf("no readable ledger at %s (%v) — ledger receipts contribute nothing to the oracle", filepath.Join(root, ".evolve", "ledger.jsonl"), err)
		return got
	}
	want := map[int]bool{}
	for _, c := range cycles {
		want[c] = true
	}
	for _, line := range bytes.Split(raw, []byte("\n")) {
		if !bytes.Contains(line, []byte(`"role":"retro"`)) || !bytes.Contains(line, []byte(`"kind":"agent_subprocess"`)) {
			continue
		}
		var e struct {
			Cycle int    `json:"cycle"`
			Role  string `json:"role"`
			Kind  string `json:"kind"`
		}
		if json.Unmarshal(line, &e) != nil {
			continue
		}
		if e.Role == "retro" && e.Kind == "agent_subprocess" && want[e.Cycle] {
			got[e.Cycle] = true
		}
	}
	return got
}

type mislabelReport struct {
	Candidates     int   `json:"candidates"`
	Mislabeled     []int `json:"mislabeled"`
	Uncorroborated []int `json:"uncorroborated"`
}

// TestC1666_001_DerivedAffectedCountCrossChecksArtifacts — AC1. The binding
// tests drive `evolve dossier retro-mislabel` through the real dispatcher over
// a six-record fixture corpus (both receipt sources, wrong-cycle receipt, a
// versioned skip, a verdict-not-adopted retro, a read-only guarantee, and the
// absent-corpus loud failure). Then the SAME command runs over the real
// corpus and must agree with an independent wire-level derivation: every
// candidate classified exactly once, mislabeled == candidates with a receipt.
// The number the Builder reports in build-report.md is this command's output,
// not a string-match estimate.
func TestC1666_001_DerivedAffectedCountCrossChecksArtifacts(t *testing.T) {
	assertSuiteTestsPass(t, "", cmdPkg,
		"TestDossierRetroMislabel_DerivedCountCrossChecksArtifacts",
		"TestDossierRetroMislabel_AbsentCorpusFailsLoudly",
	)

	root := acsassert.RepoRoot(t)
	stdout, stderr, code, err := acsassert.SubprocessOutput(evolveBinary(t), "dossier", "retro-mislabel", "--project-root", root, "--json")
	if err != nil || code != 0 {
		t.Fatalf("RED: evolve dossier retro-mislabel over the real corpus exited %d: %v\nstderr:\n%s", code, err, stderr)
	}
	var rep mislabelReport
	if uerr := json.Unmarshal([]byte(stdout), &rep); uerr != nil {
		t.Fatalf("--json output is not a JSON object: %v\n%s", uerr, stdout)
	}

	candidates := legacyRetroSkipCycles(t, root)
	receipts := retroRanReceipts(t, root, candidates)
	var wantMislabeled, wantUncorroborated []int
	for _, c := range candidates {
		if receipts[c] {
			wantMislabeled = append(wantMislabeled, c)
		} else {
			wantUncorroborated = append(wantUncorroborated, c)
		}
	}
	t.Logf("oracle: candidates=%d mislabeled=%d uncorroborated=%d; cli: candidates=%d mislabeled=%d uncorroborated=%d",
		len(candidates), len(wantMislabeled), len(wantUncorroborated), rep.Candidates, len(rep.Mislabeled), len(rep.Uncorroborated))
	if rep.Candidates != len(candidates) {
		t.Errorf("candidates = %d, independent derivation says %d", rep.Candidates, len(candidates))
	}
	if rep.Candidates != len(rep.Mislabeled)+len(rep.Uncorroborated) {
		t.Errorf("candidates (%d) != mislabeled (%d) + uncorroborated (%d)", rep.Candidates, len(rep.Mislabeled), len(rep.Uncorroborated))
	}
	if !reflect.DeepEqual(nonNil(rep.Mislabeled), nonNil(wantMislabeled)) {
		t.Errorf("mislabeled set disagrees with the independent receipt cross-check\n cli:    %v\n oracle: %v", rep.Mislabeled, wantMislabeled)
	}
	if !reflect.DeepEqual(nonNil(rep.Uncorroborated), nonNil(wantUncorroborated)) {
		t.Errorf("uncorroborated set disagrees with the independent receipt cross-check\n cli:    %v\n oracle: %v", rep.Uncorroborated, wantUncorroborated)
	}
	if len(candidates) < legacyRetroSkipFloor {
		t.Errorf("only %d legacy retro-skip records remain in the corpus (RED-time derivation: %d) — history was rewritten", len(candidates), legacyRetroSkipFloor)
	}
}

func nonNil(s []int) []int {
	if s == nil {
		return []int{}
	}
	return s
}

// gitC runs git anchored to dir and returns trimmed stdout.
func gitC(t *testing.T, dir string, args ...string) (string, error) {
	t.Helper()
	out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).Output()
	return strings.TrimSpace(string(out)), err
}

// laneBase is the commit the lane's worktree forked from main at (falls back
// to HEAD when main is unreachable, which makes the diff checks trivially
// green — logged, never silent).
func laneBase(t *testing.T, root string) string {
	t.Helper()
	base, err := gitC(t, root, "merge-base", "main", "HEAD")
	if err != nil || base == "" {
		t.Logf("merge-base main HEAD unavailable (%v); comparing against HEAD", err)
		return "HEAD"
	}
	return base
}

// TestC1666_002_DiscriminatorStampedForwardOnlyAndHistoryUntouched — AC2.
// The bindings pin the wire: Build stamps `schema_version` == CurrentSchemaVersion
// (>= 2) on every new record; a legacy record parses as version 0 and is NOT
// stamped when re-rendered. The corpus half is the NEGATIVE: the inbox record
// forbids the backfill alongside the discriminator, so no tracked dossier may
// be MODIFIED on this lane (added ones are sibling lanes' closeouts) and the
// legacy retro-skip set must be at least its RED-time size. The commit-body
// justification is the Auditor's checklist item (manual+checklist).
func TestC1666_002_DiscriminatorStampedForwardOnlyAndHistoryUntouched(t *testing.T) {
	assertSuiteTestsPass(t, "", dossierPkg,
		"TestSchemaVersion_BuildStampsTheDiscriminator",
		"TestSchemaVersion_LegacyRecordStaysUnstamped",
	)
	root := acsassert.RepoRoot(t)
	base := laneBase(t, root)
	modified, err := gitC(t, root, "diff", "--diff-filter=M", "--name-only", base, "--", "knowledge-base/cycles/")
	if err != nil {
		t.Fatalf("git diff against %s: %v", base, err)
	}
	if modified != "" {
		t.Errorf("committed dossiers were MODIFIED on this lane (a backfill — the remedy this cycle must NOT also do):\n%s", modified)
	}
	if n := len(legacyRetroSkipCycles(t, root)); n < legacyRetroSkipFloor {
		t.Errorf("legacy retro-skip records = %d, RED-time floor %d — the corpus was rewritten", n, legacyRetroSkipFloor)
	}
}

// goldenMinusVersion parses a golden JSON record and drops schema_version so
// two goldens can be compared on everything else.
func goldenMinusVersion(t *testing.T, raw []byte, label string) (map[string]any, any, bool) {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("%s: not JSON: %v", label, err)
	}
	v, stamped := m["schema_version"]
	delete(m, "schema_version")
	return m, v, stamped
}

// TestC1666_003_SchemaLockstepAndGoldensChangeOnlyByTheStamp — AC3. The
// schema drift guard is bidirectional, so the field must land on BOTH sides
// (TestSchema_NoDrift + the by-name integer/not-required pin). The stamp
// changes every produced record's bytes, so the two producer goldens the
// cycle-1663 floor froze MUST be regenerated — legitimately, and ONLY by the
// added `schema_version` line: each golden minus that key must equal the
// lane-base golden minus that key, and the core byte-pins must print PASS.
func TestC1666_003_SchemaLockstepAndGoldensChangeOnlyByTheStamp(t *testing.T) {
	assertSuiteTestsPass(t, "", dossierPkg,
		"TestSchema_NoDrift",
		"TestSchemaVersion_SchemaDeclaresTheField",
	)
	assertSuiteTestsPass(t, "", corePkg,
		"TestWriteCycleDossier_ParamsStructPreservesFixedInputBytes",
		"TestDossierSystemFailure_OrdinaryPassStaysByteClean",
		"TestWriteCycleDossier_WritesValidArtifact",
	)
	root := acsassert.RepoRoot(t)
	base := laneBase(t, root)
	for _, rel := range []string{
		"go/internal/core/testdata/dossierparams/cycle-4242.golden.json",
		"go/internal/core/testdata/dossierparams/cycle-4243.golden.json",
	} {
		now, err := os.ReadFile(filepath.Join(root, rel))
		if err != nil {
			t.Fatalf("read %s: %v", rel, err)
		}
		then, err := gitC(t, root, "show", base+":"+rel)
		if err != nil {
			t.Fatalf("git show %s:%s: %v", base, rel, err)
		}
		nowRest, nowVersion, stamped := goldenMinusVersion(t, now, rel)
		thenRest, _, _ := goldenMinusVersion(t, []byte(then), rel+"@"+base)
		if !stamped {
			t.Errorf("RED: %s carries no schema_version — the producer golden was not regenerated from the stamping Build", rel)
		} else if n, ok := nowVersion.(float64); !ok || n < 2 || n != float64(int(n)) {
			t.Errorf("%s schema_version = %#v, want an integer >= 2", rel, nowVersion)
		}
		if !reflect.DeepEqual(nowRest, thenRest) {
			t.Errorf("%s changed beyond the schema_version stamp relative to %s — regenerate goldens only for the discriminator, nothing else", rel, base)
		}
	}
}

// TestC1666_004_ConsumerRefusesLegacyRetroSkipAsEvidence — AC4. The binding
// tests drive dossier.PhaseSkipEvidence, the ONE sanctioned corpus read of a
// skipped_phases entry: a legacy record's retro entry is Contradicted when a
// receipt proves retro ran (run-dir report, pre-rename report, or the ledger
// receipt) and Unverified when nothing survives — never Trusted; a versioned
// record's entry is Trusted; no entry / nil record / empty phase is None.
// Caller proof (house rule 2): the seam must be reached from a PRODUCTION
// file (the retro-mislabel audit is its first caller) — a call site that is
// only ever a test is dead code.
func TestC1666_004_ConsumerRefusesLegacyRetroSkipAsEvidence(t *testing.T) {
	assertSuiteTestsPass(t, "", dossierPkg,
		"TestPhaseSkipEvidence_LegacyRetroSkipIsNeverTrusted",
		"TestPhaseSkipEvidence_VersionedAndAbsentEntries",
	)
	root := acsassert.RepoRoot(t)
	callers := 0
	for _, sub := range []string{"go/internal", "go/cmd"} {
		_ = filepath.WalkDir(filepath.Join(root, sub), func(path string, d os.DirEntry, err error) error {
			if err != nil || d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			raw, rerr := os.ReadFile(path)
			if rerr != nil {
				return nil
			}
			calls := strings.Count(string(raw), "PhaseSkipEvidence(") - strings.Count(string(raw), "func PhaseSkipEvidence(")
			if calls > 0 {
				callers += calls
				t.Logf("production caller of PhaseSkipEvidence: %s", strings.TrimPrefix(path, root+"/"))
			}
			return nil
		})
	}
	if callers == 0 {
		t.Errorf("RED: no production (non-test) file calls PhaseSkipEvidence( — the consumer seam is dead code until the audit command reaches it")
	}
}
