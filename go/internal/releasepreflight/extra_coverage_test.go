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

func TestDefaultGitClean_NonRepo(t *testing.T) {
	t.Parallel()
	clean, err := defaultGitClean(t.TempDir())
	if err == nil {
		t.Errorf("expected error for non-repo dir, got clean=%v err=nil", clean)
	}
}

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

func TestDefaultGateTestRunner_Error(t *testing.T) {
	t.Parallel()
	if err := defaultGateTestRunner(filepath.Join(t.TempDir(), "no-such-repo"), "./bogus"); err == nil {
		t.Error("expected error when the go module dir is absent")
	}
}

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
	opts.SimulationRunner = nil
	res, err := Run(opts)
	if err != nil {
		t.Fatalf("advisory failure must not abort Run, got %v", err)
	}
	if res.SimulationAdvisoryOK == nil || *res.SimulationAdvisoryOK {
		t.Errorf("SimulationAdvisoryOK = %v, want &false (advisory failure)", res.SimulationAdvisoryOK)
	}
}

func TestDefaultSimulationRunner_GoBinDefault(t *testing.T) {
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go not on PATH")
	}
	if err := defaultSimulationRunner(filepath.Join(t.TempDir(), "no-such-repo")); err == nil {
		t.Error("expected error when the go module dir is absent")
	}
}

func auditEntry(artifactPath, ts string) string {
	line := `{"role":"auditor","kind":"agent_subprocess"`
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

func TestCheckRecentAudit_AllPhantom(t *testing.T) {
	t.Parallel()
	ledger := writeLedger(t,
		auditEntry("", "2026-05-27T00:00:00Z"),
		auditEntry("/nonexistent/audit-report.md", "2026-05-27T00:00:00Z"),
	)
	got, err := checkRecentAudit(ledger, "", false, time.Now())
	if err != nil {
		t.Errorf("all-phantom must be advisory (no error), got: %v", err)
	}
	if got.verdict != auditVerdictNone {
		t.Errorf("verdict = %q, want %q (advisory)", got.verdict, auditVerdictNone)
	}
}

func TestCheckRecentAudit_UnreadableArtifact(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	artDir := filepath.Join(dir, "audit-report.md")
	if err := os.MkdirAll(artDir, 0o755); err != nil {
		t.Fatal(err)
	}
	ledger := writeLedger(t, auditEntry(artDir, "2026-05-27T00:00:00Z"))
	_, err := checkRecentAudit(ledger, "", false, time.Now())
	if err == nil {
		t.Error("expected read error when artifact is a directory")
	}
}

func TestCheckRecentAudit_MissingTS(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	art := filepath.Join(dir, "audit-report.md")
	if err := os.WriteFile(art, []byte("Verdict: PASS\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	ledger := writeLedger(t, auditEntry(art, ""))
	_, err := checkRecentAudit(ledger, "", false, time.Now())
	if err == nil {
		t.Error("expected 'ledger entry missing ts' error")
	}
}

func TestRun_DryRunWithNilSeams(t *testing.T) {
	t.Parallel()
	r := makeRepo(t, "1.0.0")
	res, err := Run(Options{
		Target:   "1.0.1",
		RepoRoot: r,
		DryRun:   true,
	})
	if err != nil {
		t.Fatalf("dry-run with nil seams: %v", err)
	}
	if res.StepsPassed != 5 {
		t.Errorf("StepsPassed = %d, want 5", res.StepsPassed)
	}
}

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

func TestCheckRecentAudit_NoAuditorEntries(t *testing.T) {
	t.Parallel()
	ledger := writeLedger(t, `{"role":"builder","ts":"2026-05-27T00:00:00Z"}`+"\n")
	got, err := checkRecentAudit(ledger, "", false, time.Now())
	if err != nil {
		t.Errorf("no-auditor-entry must be advisory (no error), got: %v", err)
	}
	if got.verdict != auditVerdictNone {
		t.Errorf("verdict = %q, want %q (advisory)", got.verdict, auditVerdictNone)
	}
}

func TestCheckRecentAudit_AbsentLedger(t *testing.T) {
	t.Parallel()
	absent := filepath.Join(t.TempDir(), "nonexistent", "ledger.jsonl")
	got, err := checkRecentAudit(absent, "", false, time.Now())
	if err != nil {
		t.Errorf("absent ledger must be advisory (no error), got: %v", err)
	}
	if got.verdict != auditVerdictNone {
		t.Errorf("verdict = %q, want %q (advisory)", got.verdict, auditVerdictNone)
	}
}

func TestCheckRecentAudit_NoVerdictNonStrict(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	art := filepath.Join(dir, "audit-report.md")
	if err := os.WriteFile(art, []byte("# Audit\n\nno verdict here\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	ledger := writeLedger(t, auditEntry(art, time.Now().UTC().Format(time.RFC3339)))
	_, err := checkRecentAudit(ledger, "", false, time.Now())
	fixtures.RequireErrContains(t, err, "does not declare 'Verdict: PASS' or 'Verdict: WARN'")
}

func TestCheckRecentAudit_NoVerdictStrict(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	art := filepath.Join(dir, "audit-report.md")
	if err := os.WriteFile(art, []byte("# Audit\n\nno verdict here\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	ledger := writeLedger(t, auditEntry(art, time.Now().UTC().Format(time.RFC3339)))
	_, err := checkRecentAudit(ledger, "", true, time.Now())
	fixtures.RequireErrContains(t, err, "STRICT_PASS")
}

func TestCheckRecentAudit_UnparseableTS(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	art := filepath.Join(dir, "audit-report.md")
	if err := os.WriteFile(art, []byte("Verdict: PASS\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	ledger := writeLedger(t, auditEntry(art, "not-a-date"))
	res, err := checkRecentAudit(ledger, "", false, time.Now())
	if err != nil {
		t.Errorf("unparseable ts should skip age check (nil err), got %v", err)
	}
	if res.verdict != "PASS" {
		t.Errorf("verdict = %q, want PASS", res.verdict)
	}
}
