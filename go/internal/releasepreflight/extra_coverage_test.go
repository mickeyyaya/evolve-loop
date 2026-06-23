package releasepreflight

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/test/fixtures"
)

// TestExtractJSONVersion_Errors covers the two error branches: unreadable file
// and a file with no "version" field.
func TestExtractJSONVersion_Errors(t *testing.T) {
	t.Parallel()
	if _, err := ExtractJSONVersion(filepath.Join(t.TempDir(), "nope.json")); err == nil {
		t.Error("expected read error for missing file")
	}
	d := t.TempDir()
	p := filepath.Join(d, "plugin.json")
	if err := os.WriteFile(p, []byte(`{"name":"x"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := ExtractJSONVersion(p); err == nil {
		t.Error("expected error for JSON with no version field")
	}
}

// TestDefaultGitClean_NonRepo covers the git-error branch: a non-repo dir makes
// `git diff --quiet HEAD` exit 128 (not the dirty-tree exit 1).
func TestDefaultGitClean_NonRepo(t *testing.T) {
	t.Parallel()
	clean, err := defaultGitClean(t.TempDir())
	if err == nil {
		t.Errorf("expected error for non-repo dir, got clean=%v err=nil", clean)
	}
}

// TestDefaultCurrentBranch_NonRepo covers the symbolic-ref error branch: a
// non-repo dir returns ("", nil) to mirror the bash detached-HEAD semantics.
func TestDefaultCurrentBranch_NonRepo(t *testing.T) {
	t.Parallel()
	branch, err := defaultCurrentBranch(t.TempDir())
	if err != nil {
		t.Errorf("non-repo dir should return nil error, got %v", err)
	}
	if branch != "" {
		t.Errorf("non-repo dir should return empty branch, got %q", branch)
	}
}

// TestDefaultGateTestRunner_Error covers the error branch without a nested
// `go test`: pointing at a non-existent module dir makes exec fail to start.
func TestDefaultGateTestRunner_Error(t *testing.T) {
	t.Parallel()
	// repoRoot/go does not exist → cmd.Dir chdir fails → CombinedOutput errors
	// before any `go test` subprocess is spawned.
	if err := defaultGateTestRunner(filepath.Join(t.TempDir(), "no-such-repo"), "./bogus"); err == nil {
		t.Error("expected error when the go module dir is absent")
	}
}

// TestDefaultSimulationRunner covers both branches via the defaultGoBinFn
// shim seam — a fake `go` that exits 0 (success) then 1 (failure), avoiding a
// real nested test run. Not parallel: mutates package-level var defaultGoBinFn.
func TestDefaultSimulationRunner(t *testing.T) {
	if testing.Short() {
		t.Skip("skips real subprocess (go/gh/git) invocation under -short; full `go test` + CI still run it")
	}
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "go"), 0o755); err != nil {
		t.Fatal(err)
	}
	shim := filepath.Join(dir, "fake-go")
	old := defaultGoBinFn
	t.Cleanup(func() { defaultGoBinFn = old })
	defaultGoBinFn = func() string { return shim }

	if err := os.WriteFile(shim, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := defaultSimulationRunner(dir); err != nil {
		t.Errorf("shim exit 0 should succeed, got %v", err)
	}

	if err := os.WriteFile(shim, []byte("#!/bin/sh\necho boom; exit 1\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := defaultSimulationRunner(dir); err == nil {
		t.Error("shim exit 1 should return an error")
	}
}

// TestRun_AdvisorySimulationDefaultRunner covers the SimulationRunner==nil
// branch in Run: with no seam supplied (and tests not skipped), Run wires the
// default runner. A fake `go` shim (exit 1) makes that default fail fast — the
// failure is advisory, so Run still returns nil with SimulationAdvisoryOK=false.
// Not parallel: mutates process env via t.Setenv.
func TestRun_AdvisorySimulationDefaultRunner(t *testing.T) {
	if testing.Short() {
		t.Skip("skips real subprocess (go/gh/git) invocation under -short; full `go test` + CI still run it")
	}
	r := makeRepo(t, "1.0.0")
	if err := os.MkdirAll(filepath.Join(r, "go"), 0o755); err != nil {
		t.Fatal(err)
	}
	shim := filepath.Join(t.TempDir(), "fake-go")
	if err := os.WriteFile(shim, []byte("#!/bin/sh\necho boom; exit 1\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	old := defaultGoBinFn
	t.Cleanup(func() { defaultGoBinFn = old })
	defaultGoBinFn = func() string { return shim }

	opts := stubOpts(r, "1.0.1")
	opts.SkipTests = false
	opts.GateTestRunner = func(string, string) error { return nil }
	opts.SimulationRunner = nil // force the default-runner branch
	res, err := Run(opts)
	if err != nil {
		t.Fatalf("advisory failure must not abort Run, got %v", err)
	}
	if res.SimulationAdvisoryOK == nil || *res.SimulationAdvisoryOK {
		t.Errorf("SimulationAdvisoryOK = %v, want &false (advisory failure)", res.SimulationAdvisoryOK)
	}
}

// TestDefaultSimulationRunner_GoBinDefault covers the defaultGoBinFn→"go" default
// in defaultSimulationRunner: with a repo whose go/ module dir is absent, the
// real `go test` fails fast (chdir error) before any package compiles —
// exercising the default-binary path without a hermetic nested test run.
// Skips if `go` is not on PATH.
func TestDefaultSimulationRunner_GoBinDefault(t *testing.T) {
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go not on PATH")
	}
	// repoRoot/go does not exist → cmd.Dir chdir fails → error returned.
	if err := defaultSimulationRunner(filepath.Join(t.TempDir(), "no-such-repo")); err == nil {
		t.Error("expected error when the go module dir is absent")
	}
}

// auditEntry builds a single ledger JSONL line for an auditor entry.
func auditEntry(artifactPath, ts string) string {
	line := `{"role":"auditor"`
	if artifactPath != "" {
		line += `,"artifact_path":"` + artifactPath + `"`
	}
	if ts != "" {
		line += `,"ts":"` + ts + `"`
	}
	return line + "}\n"
}

func writeLedger(t *testing.T, lines ...string) string {
	t.Helper()
	return fixtures.MustWrite(t, filepath.Join(t.TempDir(), "ledger.jsonl"), strings.Join(lines, ""))
}

// TestCheckRecentAudit_AllPhantom covers the all-entries-phantom branch: every
// auditor entry points at a missing artifact (artifacts GC'd). The audit signal
// is UNAVAILABLE, not failed — so this is ADVISORY (verdict NONE, no error), per
// the deterministic-release fix: CI-green is the authoritative gate, a missing
// on-disk audit must not block a clean/GC'd worktree's release.
func TestCheckRecentAudit_AllPhantom(t *testing.T) {
	t.Parallel()
	ledger := writeLedger(t,
		auditEntry("", "2026-05-27T00:00:00Z"),                             // empty path → phantom
		auditEntry("/nonexistent/audit-report.md", "2026-05-27T00:00:00Z"), // missing → phantom
	)
	got, err := checkRecentAudit(ledger, false, time.Now())
	if err != nil {
		t.Errorf("all-phantom must be advisory (no error), got: %v", err)
	}
	if got.verdict != auditVerdictNone {
		t.Errorf("verdict = %q, want %q (advisory)", got.verdict, auditVerdictNone)
	}
}

// TestCheckRecentAudit_UnreadableArtifact covers the read-artifact error
// branch: the artifact path resolves (Stat ok) but is a directory, so ReadFile
// fails.
func TestCheckRecentAudit_UnreadableArtifact(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	artDir := filepath.Join(dir, "audit-report.md") // a directory, not a file
	if err := os.MkdirAll(artDir, 0o755); err != nil {
		t.Fatal(err)
	}
	ledger := writeLedger(t, auditEntry(artDir, "2026-05-27T00:00:00Z"))
	_, err := checkRecentAudit(ledger, false, time.Now())
	if err == nil {
		t.Error("expected read error when artifact is a directory")
	}
}

// TestCheckRecentAudit_MissingTS covers the ts-missing branch: a valid PASS
// artifact but the ledger entry has no ts field.
func TestCheckRecentAudit_MissingTS(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	art := filepath.Join(dir, "audit-report.md")
	if err := os.WriteFile(art, []byte("Verdict: PASS\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	ledger := writeLedger(t, auditEntry(art, "")) // no ts
	_, err := checkRecentAudit(ledger, false, time.Now())
	if err == nil {
		t.Error("expected 'ledger entry missing ts' error")
	}
}

// TestRun_DryRunWithNilSeams covers the seam-default assignment block in Run
// (Now/GitClean/CurrentBranch/GateTestRunner all nil → real defaults wired).
// DryRun short-circuits steps 1/2/4/5 so the real git/test defaults are
// assigned but never invoked — exercising the nil-default branches
// deterministically without shelling out.
func TestRun_DryRunWithNilSeams(t *testing.T) {
	t.Parallel()
	r := makeRepo(t, "1.0.0")
	res, err := Run(Options{
		Target:   "1.0.1",
		RepoRoot: r,
		DryRun:   true,
		// All seams nil → defaults assigned inside Run.
	})
	if err != nil {
		t.Fatalf("dry-run with nil seams: %v", err)
	}
	if res.StepsPassed != 5 {
		t.Errorf("StepsPassed = %d, want 5", res.StepsPassed)
	}
}

// TestRun_Step1GitError covers the step-1 error branch: GitClean returning a
// non-nil error aborts with ErrCheckFailed referencing the git failure.
func TestRun_Step1GitError(t *testing.T) {
	t.Parallel()
	r := makeRepo(t, "1.0.0")
	opts := stubOpts(r, "1.0.1")
	opts.GitClean = func(string) (bool, error) { return false, errors.New("git boom") }
	_, err := Run(opts)
	if !errors.Is(err, ErrCheckFailed) {
		t.Fatalf("err = %v, want ErrCheckFailed", err)
	}
	if !strings.Contains(err.Error(), "step 1 git error") {
		t.Errorf("err = %v, want contains 'step 1 git error'", err)
	}
}

// TestRun_Step2BranchError covers the step-2 error branch: CurrentBranch
// returning a non-nil error aborts with ErrCheckFailed.
func TestRun_Step2BranchError(t *testing.T) {
	t.Parallel()
	r := makeRepo(t, "1.0.0")
	opts := stubOpts(r, "1.0.1")
	opts.CurrentBranch = func(string) (string, error) { return "", errors.New("symbolic-ref boom") }
	_, err := Run(opts)
	if !errors.Is(err, ErrCheckFailed) {
		t.Fatalf("err = %v, want ErrCheckFailed", err)
	}
	if !strings.Contains(err.Error(), "step 2 git error") {
		t.Errorf("err = %v, want contains 'step 2 git error'", err)
	}
}

// TestRun_PluginJSONMissing covers the os.Stat-on-plugin.json error branch:
// a repo without .claude-plugin/plugin.json fails step 3.
func TestRun_PluginJSONMissing(t *testing.T) {
	t.Parallel()
	r := makeRepo(t, "1.0.0")
	if err := os.Remove(filepath.Join(r, ".claude-plugin", "plugin.json")); err != nil {
		t.Fatal(err)
	}
	opts := stubOpts(r, "1.0.1")
	_, err := Run(opts)
	if !errors.Is(err, ErrCheckFailed) {
		t.Fatalf("err = %v, want ErrCheckFailed", err)
	}
	if !strings.Contains(err.Error(), "plugin.json missing") {
		t.Errorf("err = %v, want contains 'plugin.json missing'", err)
	}
}

// TestRun_ExtractVersionError covers the ExtractJSONVersion error branch: a
// plugin.json with no "version" field fails step 3 before the semver check.
func TestRun_ExtractVersionError(t *testing.T) {
	t.Parallel()
	r := makeRepo(t, "1.0.0")
	if err := os.WriteFile(filepath.Join(r, ".claude-plugin", "plugin.json"),
		[]byte(`{"name":"x"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	opts := stubOpts(r, "1.0.1")
	_, err := Run(opts)
	if !errors.Is(err, ErrCheckFailed) {
		t.Fatalf("err = %v, want ErrCheckFailed", err)
	}
	if !strings.Contains(err.Error(), "no version field") {
		t.Errorf("err = %v, want contains 'no version field'", err)
	}
}

// TestRun_CurrentVersionNotSemver covers the branch where the on-disk
// plugin.json version is present but not a valid semver — step 3 rejects it.
func TestRun_CurrentVersionNotSemver(t *testing.T) {
	t.Parallel()
	r := makeRepo(t, "not.a.semver")
	opts := stubOpts(r, "1.0.1")
	_, err := Run(opts)
	if !errors.Is(err, ErrCheckFailed) {
		t.Fatalf("err = %v, want ErrCheckFailed", err)
	}
	if !strings.Contains(err.Error(), "current plugin.json version not semver") {
		t.Errorf("err = %v, want contains 'current plugin.json version not semver'", err)
	}
}

// TestRun_GateTestsRunWithStubSeam covers the step-5 success loop body
// (GateTestsPassed increments) with a passing seam — distinct from the
// SkipTests path which never enters the loop.
func TestRun_GateTestsRunWithStubSeam(t *testing.T) {
	t.Parallel()
	r := makeRepo(t, "1.0.0")
	opts := stubOpts(r, "1.0.1")
	opts.SkipTests = false
	calls := 0
	opts.GateTestRunner = func(string, string) error { calls++; return nil }
	opts.SimulationRunner = func(string) error { return nil }
	res, err := Run(opts)
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if calls != len(DefaultGateTestSuites) {
		t.Errorf("gate runner calls = %d, want %d", calls, len(DefaultGateTestSuites))
	}
	if res.GateTestsPassed != len(DefaultGateTestSuites) {
		t.Errorf("GateTestsPassed = %d, want %d", res.GateTestsPassed, len(DefaultGateTestSuites))
	}
}

// TestCheckRecentAudit_NoAuditorEntries covers the no-auditor-entry branch: a
// ledger that exists but holds no auditor entry. The audit signal is
// UNAVAILABLE (not failed) → ADVISORY (verdict NONE, no error). CI-green is the
// authoritative gate.
func TestCheckRecentAudit_NoAuditorEntries(t *testing.T) {
	t.Parallel()
	ledger := writeLedger(t, `{"role":"builder","ts":"2026-05-27T00:00:00Z"}`+"\n")
	got, err := checkRecentAudit(ledger, false, time.Now())
	if err != nil {
		t.Errorf("no-auditor-entry must be advisory (no error), got: %v", err)
	}
	if got.verdict != auditVerdictNone {
		t.Errorf("verdict = %q, want %q (advisory)", got.verdict, auditVerdictNone)
	}
}

// TestCheckRecentAudit_AbsentLedger is the core determinism case: a clean
// checkout / CI / fresh worktree has no ledger at all. This MUST be advisory
// (verdict NONE, no error) so a reproducible release from a clean tree is not
// blocked by transient runtime state — the prior behavior ("no Auditor has ever
// run") made releases worktree-dependent.
func TestCheckRecentAudit_AbsentLedger(t *testing.T) {
	t.Parallel()
	absent := filepath.Join(t.TempDir(), "nonexistent", "ledger.jsonl")
	got, err := checkRecentAudit(absent, false, time.Now())
	if err != nil {
		t.Errorf("absent ledger must be advisory (no error), got: %v", err)
	}
	if got.verdict != auditVerdictNone {
		t.Errorf("verdict = %q, want %q (advisory)", got.verdict, auditVerdictNone)
	}
}

// TestCheckRecentAudit_NoVerdictNonStrict covers the verdict-not-found branch
// in non-strict mode: a valid, recent artifact whose body declares no verdict
// fails with the non-strict message.
func TestCheckRecentAudit_NoVerdictNonStrict(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	art := filepath.Join(dir, "audit-report.md")
	if err := os.WriteFile(art, []byte("# Audit\n\nno verdict here\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	ledger := writeLedger(t, auditEntry(art, time.Now().UTC().Format(time.RFC3339)))
	_, err := checkRecentAudit(ledger, false, time.Now())
	fixtures.RequireErrContains(t, err, "does not declare 'Verdict: PASS' or 'Verdict: WARN'")
}

// TestCheckRecentAudit_NoVerdictStrict covers the verdict-not-found branch in
// strict mode: the STRICT_PASS message is emitted instead.
func TestCheckRecentAudit_NoVerdictStrict(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	art := filepath.Join(dir, "audit-report.md")
	if err := os.WriteFile(art, []byte("# Audit\n\nno verdict here\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	ledger := writeLedger(t, auditEntry(art, time.Now().UTC().Format(time.RFC3339)))
	_, err := checkRecentAudit(ledger, true, time.Now())
	fixtures.RequireErrContains(t, err, "STRICT_PASS")
}

// TestCheckRecentAudit_UnparseableTS covers the ts-parse-fallback branch: an
// unparseable ts skips the age check and returns ok (nil error).
func TestCheckRecentAudit_UnparseableTS(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	art := filepath.Join(dir, "audit-report.md")
	if err := os.WriteFile(art, []byte("Verdict: PASS\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	ledger := writeLedger(t, auditEntry(art, "not-a-date"))
	res, err := checkRecentAudit(ledger, false, time.Now())
	if err != nil {
		t.Errorf("unparseable ts should skip age check (nil err), got %v", err)
	}
	if res.verdict != "PASS" {
		t.Errorf("verdict = %q, want PASS", res.verdict)
	}
}
