package acssuite

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// writePred writes an executable bash predicate under root/acs/<rel>/<name>.
func writePred(t *testing.T, root, rel, name, body string) {
	t.Helper()
	dir := filepath.Join(root, "acs", rel)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, name), []byte("#!/usr/bin/env bash\n"+body+"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
}

// TestRun_MixedSuite exercises all three roots with real bash execution and
// verifies the green/red accounting + per-source counts.
func TestRun_MixedSuite(t *testing.T) {
	root := t.TempDir()
	writePred(t, root, "cycle-9", "001-pass.sh", "exit 0")
	writePred(t, root, "regression-suite/cycle-7", "001-pass.sh", "exit 0")
	writePred(t, root, "regression-suite/cycle-8", "001-fail.sh", "echo boom; exit 1")
	writePred(t, root, "red-team", "rt-001-pass.sh", "exit 0")

	v, err := Run(Options{Root: root, Cycle: 9})
	if err != nil {
		t.Fatal(err)
	}
	if v.PredicateSuite.Total != 4 {
		t.Errorf("Total=%d, want 4", v.PredicateSuite.Total)
	}
	if v.PredicateSuite.ThisCycleCount != 1 || v.PredicateSuite.RegressionSuiteCount != 2 || v.PredicateSuite.RedTeamCount != 1 {
		t.Errorf("counts this=%d reg=%d rt=%d, want 1/2/1",
			v.PredicateSuite.ThisCycleCount, v.PredicateSuite.RegressionSuiteCount, v.PredicateSuite.RedTeamCount)
	}
	if v.RedCount != 1 || v.GreenCount != 3 {
		t.Errorf("red=%d green=%d, want 1/3", v.RedCount, v.GreenCount)
	}
	if v.SkipCount != 0 {
		t.Errorf("skip=%d, want 0 (no skips in this suite)", v.SkipCount)
	}
	if v.Verdict != "FAIL" || v.ShipEligible {
		t.Errorf("verdict=%q ship_eligible=%v, want FAIL/false", v.Verdict, v.ShipEligible)
	}
	if len(v.RedIDs) != 1 || v.RedIDs[0] != "regression-suite/cycle-8/001-fail" {
		t.Errorf("RedIDs=%v, want [regression-suite/cycle-8/001-fail]", v.RedIDs)
	}
	// The red result carries an evidence excerpt; greens do not.
	for _, r := range v.Results {
		if r.ResultStr == "red" && r.EvidenceExcerpt == "" {
			t.Errorf("red predicate %s missing evidence", r.ACID)
		}
		if r.ResultStr == "green" && r.EvidenceExcerpt != "" {
			t.Errorf("green predicate %s should have no evidence", r.ACID)
		}
	}
}

// Issue #12 (cycle-177): predicate FILES are discovered from the worktree (Root),
// but predicates read `.evolve/` runtime data (history, baselines, current build-
// report) which lives in MAIN. The suite must set EVOLVE_PROJECT_ROOT to the main
// project root so a predicate run from a worktree (post issue-#9 audit-cwd=worktree)
// still resolves `.evolve/` to main. Without it, regression predicates that read
// `.evolve/runs/...`/baselines false-RED (builder saw green from main, auditor saw
// red from worktree).
func TestRun_SetsEvolveProjectRootForPredicates(t *testing.T) {
	worktree := t.TempDir() // predicate files discovered here (Root)
	mainRoot := t.TempDir() // .evolve/ data lives here (ProjectRoot)
	writePred(t, worktree, "cycle-5", "001-projroot.sh",
		`[ "$EVOLVE_PROJECT_ROOT" = "`+mainRoot+`" ] || { echo "got=[$EVOLVE_PROJECT_ROOT] want=[`+mainRoot+`]"; exit 1; }`)

	v, err := Run(Options{Root: worktree, ProjectRoot: mainRoot, Cycle: 5})
	if err != nil {
		t.Fatal(err)
	}
	if v.RedCount != 0 || v.GreenCount != 1 {
		t.Fatalf("red=%d green=%d, want 0/1 — predicate must see EVOLVE_PROJECT_ROOT=%s", v.RedCount, v.GreenCount, mainRoot)
	}
}

// TestRun_SkipExit77_NeitherGreenNorRed — a predicate exiting 77 (TAP/automake
// SKIP) is classified "skip": it increments neither GreenCount nor RedCount, so
// a suite of only skips is still PASS + ship_eligible (the fresh-clone case).
func TestRun_SkipExit77_NeitherGreenNorRed(t *testing.T) {
	root := t.TempDir()
	writePred(t, root, "cycle-1", "001-skip.sh", "echo 'SKIP: evidence absent'; exit 77")
	v, err := Run(Options{Root: root, Cycle: 1})
	if err != nil {
		t.Fatal(err)
	}
	if len(v.Results) != 1 || v.Results[0].ResultStr != "skip" {
		t.Fatalf("result=%q, want skip", v.Results[0].ResultStr)
	}
	if v.SkipCount != 1 || v.GreenCount != 0 || v.RedCount != 0 {
		t.Errorf("skip=%d green=%d red=%d, want 1/0/0", v.SkipCount, v.GreenCount, v.RedCount)
	}
	if v.Verdict != "PASS" || !v.ShipEligible {
		t.Errorf("verdict=%q ship=%v, want PASS/true (skip-only suite ships)", v.Verdict, v.ShipEligible)
	}
	if len(v.SkipIDs) != 1 {
		t.Errorf("SkipIDs=%v, want one id", v.SkipIDs)
	}
	if v.Results[0].EvidenceExcerpt == "" {
		t.Errorf("skip predicate should capture an evidence excerpt")
	}
	if v.PredicateSuite.SkippedCount != 1 {
		t.Errorf("PredicateSuite.SkippedCount=%d, want 1", v.PredicateSuite.SkippedCount)
	}
}

// TestRun_MixedGreenRedSkip — green(0)+red(1)+skip(77) accounting: Total counts
// all three; the single red drives FAIL; SkipIDs is populated.
func TestRun_MixedGreenRedSkip(t *testing.T) {
	root := t.TempDir()
	writePred(t, root, "cycle-1", "001-green.sh", "exit 0")
	writePred(t, root, "cycle-1", "002-red.sh", "echo boom; exit 1")
	writePred(t, root, "cycle-1", "003-skip.sh", "echo skipme; exit 77")
	v, err := Run(Options{Root: root, Cycle: 1})
	if err != nil {
		t.Fatal(err)
	}
	if v.GreenCount != 1 || v.RedCount != 1 || v.SkipCount != 1 {
		t.Errorf("green=%d red=%d skip=%d, want 1/1/1", v.GreenCount, v.RedCount, v.SkipCount)
	}
	if v.PredicateSuite.Total != 3 {
		t.Errorf("Total=%d, want 3 (skips included)", v.PredicateSuite.Total)
	}
	if v.Verdict != "FAIL" || v.ShipEligible {
		t.Errorf("verdict=%q ship=%v, want FAIL/false (a red is present)", v.Verdict, v.ShipEligible)
	}
	if len(v.SkipIDs) != 1 {
		t.Errorf("SkipIDs=%v, want one id", v.SkipIDs)
	}
	if v.PredicateSuite.Total != v.GreenCount+v.RedCount+v.SkipCount {
		t.Errorf("Total(%d) != green+red+skip(%d)", v.PredicateSuite.Total, v.GreenCount+v.RedCount+v.SkipCount)
	}
}

// TestRun_GreenPlusSkipIsShipEligible — the fresh-clone proof: a suite of one
// green + one skip has red_count==0 and PASSES, so a fresh worktree (where the
// 4 runtime-only predicates SKIP) clears the EGPS gate.
func TestRun_GreenPlusSkipIsShipEligible(t *testing.T) {
	root := t.TempDir()
	writePred(t, root, "cycle-1", "001-green.sh", "exit 0")
	writePred(t, root, "regression-suite/cycle-9", "001-skip.sh", "exit 77")
	v, err := Run(Options{Root: root, Cycle: 1})
	if err != nil {
		t.Fatal(err)
	}
	if v.RedCount != 0 || v.Verdict != "PASS" || !v.ShipEligible {
		t.Errorf("red=%d verdict=%q ship=%v, want 0/PASS/true", v.RedCount, v.Verdict, v.ShipEligible)
	}
}

// TestRun_GreenPlusRealExit1Blocks — a genuine exit 1 (not 77) still counts RED,
// so the gate blocks real defects: SKIP must not weaken the gate.
func TestRun_GreenPlusRealExit1Blocks(t *testing.T) {
	root := t.TempDir()
	writePred(t, root, "cycle-1", "001-green.sh", "exit 0")
	writePred(t, root, "cycle-1", "002-real-red.sh", "echo defect; exit 1")
	v, err := Run(Options{Root: root, Cycle: 1})
	if err != nil {
		t.Fatal(err)
	}
	if v.RedCount != 1 || v.Verdict != "FAIL" || v.ShipEligible {
		t.Errorf("red=%d verdict=%q ship=%v, want 1/FAIL/false", v.RedCount, v.Verdict, v.ShipEligible)
	}
}

// TestRun_AllGreenIsShipEligiblePass — clean suite ⇒ PASS + ship_eligible.
func TestRun_AllGreenIsShipEligiblePass(t *testing.T) {
	root := t.TempDir()
	writePred(t, root, "cycle-1", "001.sh", "exit 0")
	writePred(t, root, "red-team", "rt-001.sh", "exit 0")
	v, err := Run(Options{Root: root, Cycle: 1})
	if err != nil {
		t.Fatal(err)
	}
	if v.Verdict != "PASS" || !v.ShipEligible || v.RedCount != 0 {
		t.Errorf("verdict=%q ship=%v red=%d, want PASS/true/0", v.Verdict, v.ShipEligible, v.RedCount)
	}
}

// TestRun_EmptySuiteIsPass — no predicates ⇒ PASS (bootstrap; nothing RED).
func TestRun_EmptySuiteIsPass(t *testing.T) {
	root := t.TempDir()
	v, err := Run(Options{Root: root, Cycle: 5})
	if err != nil {
		t.Fatal(err)
	}
	if v.Verdict != "PASS" || v.PredicateSuite.Total != 0 {
		t.Errorf("verdict=%q total=%d, want PASS/0", v.Verdict, v.PredicateSuite.Total)
	}
}

// TestRun_Validation covers the required-field guards.
func TestRun_Validation(t *testing.T) {
	if _, err := Run(Options{Cycle: 1}); err == nil {
		t.Error("want error for empty Root")
	}
	if _, err := Run(Options{Root: t.TempDir(), Cycle: 0}); err == nil {
		t.Error("want error for non-positive Cycle")
	}
}

// TestRun_ExecSeam_TimeoutIsRed — the exec seam lets a test force a non-zero
// (timeout-class) exit without spawning a real long-running process.
func TestRun_ExecSeam_TimeoutIsRed(t *testing.T) {
	root := t.TempDir()
	writePred(t, root, "cycle-3", "001-hang.sh", "sleep 100")
	v, err := Run(Options{
		Root:  root,
		Cycle: 3,
		Exec:  func(_ context.Context, _ string) (int, string) { return 126, "killed: timeout" },
	})
	if err != nil {
		t.Fatal(err)
	}
	if v.RedCount != 1 || v.Verdict != "FAIL" {
		t.Errorf("red=%d verdict=%q, want 1/FAIL", v.RedCount, v.Verdict)
	}
}

// TestWriteVerdict_RoundTrip — verdict is written atomically and re-parses to
// the schema the audit + ship gates read (red_count/green_count/verdict).
func TestWriteVerdict_RoundTrip(t *testing.T) {
	root := t.TempDir()
	writePred(t, root, "cycle-2", "001.sh", "exit 0")
	v, err := Run(Options{Root: root, Cycle: 2})
	if err != nil {
		t.Fatal(err)
	}
	evolveDir := filepath.Join(root, ".evolve")
	dst, err := WriteVerdict(evolveDir, v)
	if err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(dst)
	if err != nil {
		t.Fatal(err)
	}
	var got struct {
		RedCount       int    `json:"red_count"`
		GreenCount     int    `json:"green_count"`
		Verdict        string `json:"verdict"`
		PredicateSuite struct {
			Total int `json:"total"`
		} `json:"predicate_suite"`
	}
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatalf("re-parse acs-verdict.json: %v", err)
	}
	if got.Verdict != "PASS" || got.GreenCount != 1 || got.RedCount != 0 || got.PredicateSuite.Total != 1 {
		t.Errorf("round-trip mismatch: %+v", got)
	}
}

// TestRun_LongOutputIsTruncated — a red predicate emitting >evidenceMax bytes
// has its evidence excerpt truncated with an ellipsis.
func TestRun_LongOutputIsTruncated(t *testing.T) {
	root := t.TempDir()
	// Emit ~2000 bytes then fail.
	writePred(t, root, "cycle-1", "001-noisy.sh", "for i in $(seq 1 200); do echo 0123456789; done; exit 1")
	v, err := Run(Options{Root: root, Cycle: 1})
	if err != nil {
		t.Fatal(err)
	}
	if v.RedCount != 1 {
		t.Fatalf("red=%d, want 1", v.RedCount)
	}
	ex := v.Results[0].EvidenceExcerpt
	if len(ex) > evidenceMax+len("…")+1 {
		t.Errorf("excerpt len=%d, want ≤ %d", len(ex), evidenceMax+4)
	}
	if ex[len(ex)-len("…"):] != "…" {
		t.Errorf("long excerpt should end with ellipsis, got tail %q", ex[len(ex)-4:])
	}
}

// TestWriteVerdict_MkdirError — a non-directory evolveDir surfaces an error
// rather than silently dropping the verdict.
func TestWriteVerdict_MkdirError(t *testing.T) {
	root := t.TempDir()
	blocker := filepath.Join(root, "blocker")
	if err := os.WriteFile(blocker, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	// evolveDir under a regular file → MkdirAll fails.
	if _, err := WriteVerdict(filepath.Join(blocker, "evolve"), Verdict{Cycle: 1}); err == nil {
		t.Error("want error when evolveDir cannot be created")
	}
}

// TestRun_DiscoverGlobError — a Root containing an unterminated '[' makes the
// cycle-predicate glob pattern malformed (filepath.ErrBadPattern), so discover
// fails and Run propagates the error rather than returning an empty PASS verdict
// (acssuite.go:136-138, 203-205). Pins: a glob syntax error is surfaced, never
// silently treated as "no predicates".
func TestRun_DiscoverGlobError(t *testing.T) {
	// The literal '[' lands in the joined glob path before the '*.sh' segment,
	// leaving the bracket class unterminated.
	badRoot := filepath.Join(t.TempDir(), "x[")
	if _, err := Run(Options{Root: badRoot, Cycle: 1}); err == nil {
		t.Fatal("Run must surface a glob syntax error from discover, got nil")
	}
}

// TestDiscover_GlobErrorIsWrapped — direct white-box check that discover wraps a
// malformed cycle glob with its "glob cycle" context (acssuite.go:203-205).
func TestDiscover_GlobErrorIsWrapped(t *testing.T) {
	_, err := discover(filepath.Join(t.TempDir(), "x["), 1)
	if err == nil {
		t.Fatal("discover must return an error for a malformed glob pattern")
	}
	if got := err.Error(); !strings.Contains(got, "glob cycle") {
		t.Errorf("error must carry the 'glob cycle' context; got %q", got)
	}
}

// TestRunBash_ExecError — when the bash binary cannot be resolved at all (PATH
// emptied for this test), runBash cannot start the process: the error is NOT an
// *exec.ExitError, so runBash returns the 126 exec-error sentinel with an
// "exec error" diagnostic rather than masking the failure (acssuite.go:272).
func TestRunBash_ExecError(t *testing.T) {
	// t.Setenv forbids t.Parallel — this test mutates process PATH.
	t.Setenv("PATH", "")
	code, out := runBash(context.Background(), filepath.Join(t.TempDir(), "noexist.sh"), "")
	if code != 126 {
		t.Errorf("exit code=%d, want 126 (could-not-exec sentinel)", code)
	}
	if !strings.Contains(out, "exec error") {
		t.Errorf("output must carry the exec-error diagnostic; got %q", out)
	}
}

// TestRelPath_FallbackReturnsPath — when filepath.Rel cannot relativize (an
// absolute root vs a relative target), relPath returns the input path unchanged
// instead of an empty string (acssuite.go:285).
func TestRelPath_FallbackReturnsPath(t *testing.T) {
	const target = "relative/predicate.sh"
	if got := relPath("/abs/root", target); got != target {
		t.Errorf("relPath fallback = %q, want the input path %q", got, target)
	}
}

// TestWriteVerdict_RenameError — when the destination acs-verdict.json already
// exists as a DIRECTORY, the final os.Rename(tmp, dst) cannot complete, so
// WriteVerdict surfaces a "rename" error rather than reporting success
// (acssuite.go:321-323).
func TestWriteVerdict_RenameError(t *testing.T) {
	evolveDir := t.TempDir()
	// Pre-create <evolveDir>/runs/cycle-1/acs-verdict.json as a directory so the
	// rename of the temp file onto it fails.
	collide := filepath.Join(evolveDir, "runs", "cycle-1", "acs-verdict.json")
	if err := os.MkdirAll(collide, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := WriteVerdict(evolveDir, Verdict{Cycle: 1}); err == nil {
		t.Fatal("WriteVerdict must error when the destination cannot be renamed onto")
	} else if !strings.Contains(err.Error(), "rename") {
		t.Errorf("error must carry the 'rename' context; got %q", err.Error())
	}
}

// TestWriteVerdict_CreateTempError — the cycle dir already exists but is
// read-only: MkdirAll is a no-op (dir present), then os.CreateTemp cannot create
// the temp file, so WriteVerdict surfaces a "create tmp" error rather than
// silently dropping the verdict (acssuite.go:311-313).
func TestWriteVerdict_CreateTempError(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("read-only-dir permission denial does not hold for root")
	}
	evolveDir := t.TempDir()
	cycleDir := filepath.Join(evolveDir, "runs", "cycle-1")
	if err := os.MkdirAll(cycleDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(cycleDir, 0o555); err != nil {
		t.Fatal(err)
	}
	// Restore write perm so t.TempDir's recursive cleanup can remove it.
	t.Cleanup(func() { _ = os.Chmod(cycleDir, 0o755) })

	if _, err := WriteVerdict(evolveDir, Verdict{Cycle: 1}); err == nil {
		t.Fatal("WriteVerdict must error when the temp file cannot be created")
	} else if !strings.Contains(err.Error(), "create tmp") {
		t.Errorf("error must carry the 'create tmp' context; got %q", err.Error())
	}
}

// TestRun_RealTimeout — a genuinely hanging predicate is killed by the
// per-predicate timeout and counts RED (no seam; exercises runBash).
func TestRun_RealTimeout(t *testing.T) {
	if testing.Short() {
		t.Skip("skip real-timeout in -short")
	}
	root := t.TempDir()
	writePred(t, root, "cycle-4", "001-hang.sh", "sleep 30")
	start := time.Now()
	v, err := Run(Options{Root: root, Cycle: 4, Timeout: 200 * time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}
	if time.Since(start) > 5*time.Second {
		t.Error("timeout did not fire promptly")
	}
	if v.RedCount != 1 {
		t.Errorf("red=%d, want 1 (timed-out predicate is RED)", v.RedCount)
	}
}
