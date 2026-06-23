package releasepreflight

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	osexec "os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/test/fixtures"
)

var osExec = osexec.Command

// makeRepo sets up a minimal preflight-compatible repo fixture matching
// the bash test helper. Returns the repo dir; cleanup is t.TempDir-managed.
//
// The isolated project root + .evolve/ scaffolding come from
// fixtures.NewWorkspace (replacing the hand-rolled t.TempDir + MkdirAll
// dance); the preflight-specific files (plugin.json, audit-report.md, ledger)
// are seeded through the workspace's relative-path writers.
func makeRepo(t *testing.T, version string) string {
	t.Helper()
	auditRel := filepath.Join(".evolve", "runs", "cycle-99", "audit-report.md")
	ws := fixtures.NewWorkspace(t).
		WithFiles(map[string]string{
			filepath.Join(".claude-plugin", "plugin.json"): fmt.Sprintf(`{"name":"x","version":"%s"}`, version),
			auditRel: "# Audit\n\nVerdict: PASS\n\nConfidence: 1.0\n",
		}).
		Build()
	auditPath := ws.Path(auditRel)
	now := time.Now().UTC().Format(time.RFC3339)
	ledger := fmt.Sprintf(
		`{"ts":"%s","cycle":99,"role":"auditor","kind":"agent_subprocess","model":"opus","exit_code":0,"artifact_path":"%s","artifact_sha256":"deadbeef","git_head":"none","tree_state_sha":"none"}`+"\n",
		now, auditPath)
	ws.Write(filepath.Join(".evolve", "ledger.jsonl"), ledger)
	return ws.Root
}

// stubOpts returns Options pre-wired with passing seams for a happy-path repo.
func stubOpts(repo, target string) Options {
	return Options{
		Target:         target,
		RepoRoot:       repo,
		SkipTests:      true,
		Now:            func() time.Time { return time.Now() },
		GitClean:       func(string) (bool, error) { return true, nil },
		CurrentBranch:  func(string) (string, error) { return "main", nil },
		GateTestRunner: func(string, string) error { return nil },
	}
}

// === Test 1: happy path → no error ==========================================
func TestRun_HappyPath(t *testing.T) {
	r := makeRepo(t, "1.0.0")
	var buf bytes.Buffer
	opts := stubOpts(r, "1.0.1")
	opts.Stderr = &buf

	res, err := Run(opts)
	if err != nil {
		t.Fatalf("Run err = %v\nlog=%s", err, buf.String())
	}
	if res.StepsPassed != 5 {
		t.Errorf("StepsPassed = %d, want 5", res.StepsPassed)
	}
	if res.AuditVerdict != "PASS" {
		t.Errorf("AuditVerdict = %q, want PASS", res.AuditVerdict)
	}
	if res.CurrentVersion != "1.0.0" {
		t.Errorf("CurrentVersion = %q, want 1.0.0", res.CurrentVersion)
	}
}

// === Test 2: dirty tree → ErrCheckFailed ====================================
func TestRun_DirtyTree(t *testing.T) {
	r := makeRepo(t, "1.0.0")
	opts := stubOpts(r, "1.0.1")
	opts.GitClean = func(string) (bool, error) { return false, nil }
	_, err := Run(opts)
	if !errors.Is(err, ErrCheckFailed) {
		t.Fatalf("err = %v, want ErrCheckFailed", err)
	}
	if !strings.Contains(err.Error(), "uncommitted") {
		t.Errorf("err = %v, want contains 'uncommitted'", err)
	}
}

// === Test 3: detached HEAD → ErrCheckFailed =================================
func TestRun_DetachedHEAD(t *testing.T) {
	r := makeRepo(t, "1.0.0")
	opts := stubOpts(r, "1.0.1")
	opts.CurrentBranch = func(string) (string, error) { return "", nil }
	_, err := Run(opts)
	if !errors.Is(err, ErrCheckFailed) {
		t.Fatalf("err = %v, want ErrCheckFailed", err)
	}
	if !strings.Contains(err.Error(), "detached") {
		t.Errorf("err = %v, want contains 'detached'", err)
	}
}

// === Test 4: invalid semver target → ErrCheckFailed =========================
func TestRun_InvalidSemverTarget(t *testing.T) {
	r := makeRepo(t, "1.0.0")
	opts := stubOpts(r, "not-a-version")
	_, err := Run(opts)
	if !errors.Is(err, ErrCheckFailed) {
		t.Fatalf("err = %v, want ErrCheckFailed", err)
	}
}

// === Test 5: target equals current → ErrCheckFailed =========================
func TestRun_NoOpBump(t *testing.T) {
	r := makeRepo(t, "1.0.0")
	opts := stubOpts(r, "1.0.0")
	_, err := Run(opts)
	if !errors.Is(err, ErrCheckFailed) {
		t.Fatalf("err = %v, want ErrCheckFailed", err)
	}
	if !strings.Contains(err.Error(), "nothing to bump") {
		t.Errorf("err = %v, want contains 'nothing to bump'", err)
	}
}

// === Test 6: downgrade → ErrCheckFailed =====================================
func TestRun_Downgrade(t *testing.T) {
	r := makeRepo(t, "2.0.0")
	opts := stubOpts(r, "1.5.0")
	_, err := Run(opts)
	if !errors.Is(err, ErrCheckFailed) {
		t.Fatalf("err = %v, want ErrCheckFailed", err)
	}
	if !strings.Contains(err.Error(), "not greater than") {
		t.Errorf("err = %v, want contains 'not greater than'", err)
	}
}

// === Test 7: missing ledger → ADVISORY (deterministic release) ==============
// Determinism fix: a release from a clean checkout / CI / fresh worktree (no
// ledger) must NOT be blocked — the audit signal is unavailable, not failed, and
// CI-green on the release commit is the authoritative gate (/publish). Preflight
// passes with step 4 advisory.
func TestRun_MissingLedger(t *testing.T) {
	r := makeRepo(t, "1.0.0")
	os.Remove(filepath.Join(r, ".evolve", "ledger.jsonl"))
	opts := stubOpts(r, "1.0.1")
	res, err := Run(opts)
	if err != nil {
		t.Fatalf("missing ledger must be advisory (no error), got: %v", err)
	}
	if res.AuditVerdict != auditVerdictNone {
		t.Errorf("AuditVerdict = %q, want %q (advisory)", res.AuditVerdict, auditVerdictNone)
	}
	if res.StepsPassed != res.StepsTotal {
		t.Errorf("StepsPassed=%d, want all %d (preflight passes with advisory audit)", res.StepsPassed, res.StepsTotal)
	}
}

// === Test 8: audit verdict WARN → blocked in strict mode ====================
func TestRun_WarnVerdict_NonStrict(t *testing.T) {
	r := makeRepo(t, "1.0.0")
	// Rewrite audit-report to WARN.
	auditPath := filepath.Join(r, ".evolve", "runs", "cycle-99", "audit-report.md")
	if err := os.WriteFile(auditPath, []byte("# Audit\n\nVerdict: WARN\n"), 0o644); err != nil {
		t.Fatalf("rewrite: %v", err)
	}
	// Non-strict (default): WARN should pass.
	opts := stubOpts(r, "1.0.1")
	res, err := Run(opts)
	if err != nil {
		t.Fatalf("non-strict WARN should pass, got err = %v", err)
	}
	if res.AuditVerdict != "WARN" {
		t.Errorf("AuditVerdict = %q, want WARN", res.AuditVerdict)
	}
}

func TestRun_WarnVerdict_Strict(t *testing.T) {
	r := makeRepo(t, "1.0.0")
	auditPath := filepath.Join(r, ".evolve", "runs", "cycle-99", "audit-report.md")
	if err := os.WriteFile(auditPath, []byte("# Audit\n\nVerdict: WARN\n"), 0o644); err != nil {
		t.Fatalf("rewrite: %v", err)
	}
	opts := stubOpts(r, "1.0.1")
	opts.StrictPass = true
	_, err := Run(opts)
	if !errors.Is(err, ErrCheckFailed) {
		t.Fatalf("strict WARN should fail, got err = %v", err)
	}
	if !strings.Contains(err.Error(), "STRICT_PASS") {
		t.Errorf("err = %v, want contains 'STRICT_PASS'", err)
	}
}

// === Test 9: --dry-run honors no-execute ===================================
func TestRun_DryRun(t *testing.T) {
	r := makeRepo(t, "1.0.0")
	gitCalls := 0
	branchCalls := 0
	gateCalls := 0
	opts := stubOpts(r, "1.0.1")
	opts.DryRun = true
	opts.SkipTests = false
	opts.GitClean = func(string) (bool, error) { gitCalls++; return true, nil }
	opts.CurrentBranch = func(string) (string, error) { branchCalls++; return "main", nil }
	opts.GateTestRunner = func(string, string) error { gateCalls++; return nil }
	var buf bytes.Buffer
	opts.Stderr = &buf

	_, err := Run(opts)
	if err != nil {
		t.Fatalf("dry-run err = %v\nlog=%s", err, buf.String())
	}
	if gitCalls != 0 || branchCalls != 0 || gateCalls != 0 {
		t.Errorf("seam calls: git=%d branch=%d gate=%d, want all 0",
			gitCalls, branchCalls, gateCalls)
	}
	if !strings.Contains(buf.String(), "DRY-RUN") {
		t.Errorf("log missing DRY-RUN: %s", buf.String())
	}
}

// === Test 10: --skip-tests bypasses step 5 only =============================
func TestRun_SkipTests(t *testing.T) {
	r := makeRepo(t, "1.0.0")
	gateCalls := 0
	opts := stubOpts(r, "1.0.1")
	opts.SkipTests = true
	opts.GateTestRunner = func(string, string) error {
		gateCalls++
		return errors.New("should-not-run")
	}
	res, err := Run(opts)
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if gateCalls != 0 {
		t.Errorf("gate calls = %d, want 0 with SkipTests", gateCalls)
	}
	if res.StepsPassed != 5 {
		t.Errorf("StepsPassed = %d, want 5", res.StepsPassed)
	}
	if res.GateTestsPassed != 0 {
		t.Errorf("GateTestsPassed = %d, want 0", res.GateTestsPassed)
	}
}

// === Advisory simulation step (v12.1.5) =====================================
// Table-driven: covers skip, dry-run, pass, fail-but-advisory. Asserts that
// no path returns ErrCheckFailed (advisory) and that SimulationAdvisoryOK
// is nil/true/false as appropriate. Verifies StepsPassed stays 5.
func TestRun_SimulationAdvisory(t *testing.T) {
	cases := []struct {
		name       string
		skipTests  bool
		dryRun     bool
		simErr     error
		wantSimOK  *bool  // nil = skipped; true = pass; false = warn
		wantLogHas string // substring expected on stderr
	}{
		{name: "skip-tests skips advisory", skipTests: true, wantSimOK: nil, wantLogHas: "skipped (--skip-tests)"},
		{name: "dry-run skips advisory", dryRun: true, wantSimOK: nil, wantLogHas: "skipped (dry-run)"},
		{name: "happy path → true", simErr: nil, wantSimOK: ptrBool(true), wantLogHas: "auto-respond simulation suite passed"},
		{name: "bats failure → advisory warn, no error", simErr: errors.New("bats failed"), wantSimOK: ptrBool(false), wantLogHas: "advisory in v12.1.5"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := makeRepo(t, "1.0.0")
			var buf bytes.Buffer
			opts := stubOpts(r, "1.0.1")
			opts.SkipTests = tc.skipTests
			opts.DryRun = tc.dryRun
			opts.Stderr = &buf
			opts.SimulationRunner = func(string) error { return tc.simErr }

			res, err := Run(opts)
			if err != nil {
				t.Fatalf("Run err = %v\nlog=%s", err, buf.String())
			}
			if res.StepsPassed != 5 {
				t.Errorf("StepsPassed = %d, want 5 (advisory must not count)", res.StepsPassed)
			}
			gotPtr := res.SimulationAdvisoryOK
			switch {
			case tc.wantSimOK == nil && gotPtr != nil:
				t.Errorf("SimulationAdvisoryOK = %v, want nil", *gotPtr)
			case tc.wantSimOK != nil && gotPtr == nil:
				t.Errorf("SimulationAdvisoryOK = nil, want %v", *tc.wantSimOK)
			case tc.wantSimOK != nil && gotPtr != nil && *gotPtr != *tc.wantSimOK:
				t.Errorf("SimulationAdvisoryOK = %v, want %v", *gotPtr, *tc.wantSimOK)
			}
			if tc.wantLogHas != "" && !strings.Contains(buf.String(), tc.wantLogHas) {
				t.Errorf("log missing %q\nlog=%s", tc.wantLogHas, buf.String())
			}
		})
	}
}

func ptrBool(b bool) *bool { return &b }

// === Phantom-entry handling: walks past entries with missing artifacts ======
func TestRun_PhantomEntries(t *testing.T) {
	r := makeRepo(t, "1.0.0")
	auditPath := filepath.Join(r, ".evolve", "runs", "cycle-99", "audit-report.md")
	now := time.Now().UTC().Format(time.RFC3339)
	// Two phantom entries (missing artifact) followed by one valid (older).
	// Reverse-order traversal must skip the phantoms and accept the valid.
	ledger := strings.Join([]string{
		fmt.Sprintf(`{"ts":"%s","role":"auditor","artifact_path":"%s"}`, now, auditPath),
		fmt.Sprintf(`{"ts":"%s","role":"auditor","artifact_path":"/tmp/doesnotexist-1.md"}`, now),
		fmt.Sprintf(`{"ts":"%s","role":"auditor","artifact_path":"/tmp/doesnotexist-2.md"}`, now),
	}, "\n") + "\n"
	if err := os.WriteFile(filepath.Join(r, ".evolve", "ledger.jsonl"),
		[]byte(ledger), 0o644); err != nil {
		t.Fatalf("ledger: %v", err)
	}
	opts := stubOpts(r, "1.0.1")
	res, err := Run(opts)
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if res.PhantomEntries != 2 {
		t.Errorf("PhantomEntries = %d, want 2", res.PhantomEntries)
	}
	if res.AuditArtifact != auditPath {
		t.Errorf("AuditArtifact = %q, want %q", res.AuditArtifact, auditPath)
	}
}

// === Old audit (>7 days) → ErrCheckFailed ==================================
func TestRun_StaleAudit(t *testing.T) {
	r := makeRepo(t, "1.0.0")
	auditPath := filepath.Join(r, ".evolve", "runs", "cycle-99", "audit-report.md")
	// 8 days ago.
	staleTs := time.Now().Add(-8 * 24 * time.Hour).UTC().Format(time.RFC3339)
	ledger := fmt.Sprintf(
		`{"ts":"%s","role":"auditor","artifact_path":"%s"}`+"\n",
		staleTs, auditPath)
	if err := os.WriteFile(filepath.Join(r, ".evolve", "ledger.jsonl"),
		[]byte(ledger), 0o644); err != nil {
		t.Fatalf("ledger: %v", err)
	}
	opts := stubOpts(r, "1.0.1")
	_, err := Run(opts)
	if !errors.Is(err, ErrCheckFailed) {
		t.Fatalf("err = %v, want ErrCheckFailed", err)
	}
	if !strings.Contains(err.Error(), "old") {
		t.Errorf("err = %v, want contains 'old'", err)
	}
}

// === Gate-test failure propagates ===========================================
func TestRun_GateTestFailure(t *testing.T) {
	r := makeRepo(t, "1.0.0")
	opts := stubOpts(r, "1.0.1")
	opts.SkipTests = false
	opts.GateTestRunner = func(_, suite string) error {
		if strings.Contains(suite, "phases/ship") {
			return errors.New("simulated failure")
		}
		return nil
	}
	_, err := Run(opts)
	if !errors.Is(err, ErrCheckFailed) {
		t.Fatalf("err = %v, want ErrCheckFailed", err)
	}
	if !strings.Contains(err.Error(), "phases/ship") {
		t.Errorf("err = %v, want contains 'phases/ship'", err)
	}
}

// TestDefaultGateTestSuites_AreGoPackages guards against regressing to the
// deleted legacy/scripts/tests/*.sh paths: v12 removed the bash suites, so the
// preflight gate must run Go test packages (./internal/...), not dead shells.
func TestDefaultGateTestSuites_AreGoPackages(t *testing.T) {
	if len(DefaultGateTestSuites) == 0 {
		t.Fatal("no gate-test suites configured")
	}
	for _, s := range DefaultGateTestSuites {
		if strings.HasSuffix(s, ".sh") {
			t.Errorf("gate-test suite %q is a (deleted) bash path — must be a Go package pattern", s)
		}
		if !strings.HasPrefix(s, "./") {
			t.Errorf("gate-test suite %q is not a relative Go package pattern", s)
		}
	}
}

// TestStripBypassEnv ensures the gate-test runner drops the ship/role-gate
// bypass vars so the guard DENY-tests stay hermetic regardless of the
// operator's session env (e.g. a dev's settings.local.json sets them).
func TestStripBypassEnv(t *testing.T) {
	in := []string{"PATH=/bin", "EVOLVE_BYPASS_SHIP_GATE=1", "HOME=/h", "EVOLVE_BYPASS_ROLE_GATE=1", "FOO=bar"}
	got := stripBypassEnv(in)
	if len(got) != 3 {
		t.Fatalf("len=%d want 3: %v", len(got), got)
	}
	keep := map[string]bool{"PATH=/bin": true, "HOME=/h": true, "FOO=bar": true}
	for _, kv := range got {
		if strings.HasPrefix(kv, "EVOLVE_BYPASS_") {
			t.Errorf("bypass var survived: %s", kv)
		}
		if !keep[kv] {
			t.Errorf("unexpected entry: %s", kv)
		}
	}
}

// === Verdict heading form (## Verdict\n**PASS**) is accepted ===============
func TestRun_HeadingVerdictForm(t *testing.T) {
	r := makeRepo(t, "1.0.0")
	auditPath := filepath.Join(r, ".evolve", "runs", "cycle-99", "audit-report.md")
	body := "# Audit\n\n## Verdict\n\n**PASS**\n\nMore content.\n"
	if err := os.WriteFile(auditPath, []byte(body), 0o644); err != nil {
		t.Fatalf("rewrite: %v", err)
	}
	opts := stubOpts(r, "1.0.1")
	res, err := Run(opts)
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if res.AuditVerdict != "PASS" {
		t.Errorf("AuditVerdict = %q, want PASS", res.AuditVerdict)
	}
}

// === No RepoRoot → ErrCheckFailed (programmer error) =======================
func TestRun_NoRepoRoot(t *testing.T) {
	_, err := Run(Options{Target: "1.0.0"})
	if !errors.Is(err, ErrCheckFailed) {
		t.Fatalf("err = %v, want ErrCheckFailed", err)
	}
}

// === Unit tests for helpers =================================================

func TestParseSemver(t *testing.T) {
	cases := []struct {
		in    string
		major int
		minor int
		patch int
		want  bool
	}{
		{"1.2.3", 1, 2, 3, true},
		{"11.7.2", 11, 7, 2, true},
		{"0.0.0", 0, 0, 0, true},
		{"1.2.3-alpha", 1, 2, 3, true}, // tail allowed
		{"v1.2.3", 0, 0, 0, false},
		{"1.2", 0, 0, 0, false},
		{"garbage", 0, 0, 0, false},
		{"", 0, 0, 0, false},
	}
	for _, tc := range cases {
		maj, mn, pt, ok := ParseSemver(tc.in)
		if ok != tc.want || (ok && (maj != tc.major || mn != tc.minor || pt != tc.patch)) {
			t.Errorf("ParseSemver(%q) = (%d,%d,%d,%v), want (%d,%d,%d,%v)",
				tc.in, maj, mn, pt, ok, tc.major, tc.minor, tc.patch, tc.want)
		}
	}
}

func TestSemverGT(t *testing.T) {
	cases := []struct {
		a, b string
		want bool
	}{
		{"1.0.1", "1.0.0", true},
		{"1.1.0", "1.0.99", true},
		{"2.0.0", "1.99.99", true},
		{"1.0.0", "1.0.0", false},
		{"1.0.0", "1.0.1", false},
		{"garbage", "1.0.0", false},
		{"1.0.0", "garbage", false},
	}
	for _, tc := range cases {
		if got := SemverGT(tc.a, tc.b); got != tc.want {
			t.Errorf("SemverGT(%q, %q) = %v, want %v", tc.a, tc.b, got, tc.want)
		}
	}
}

func TestExtractJSONVersion(t *testing.T) {
	d := t.TempDir()
	p := filepath.Join(d, "plugin.json")
	if err := os.WriteFile(p, []byte(`{"version":"3.4.5"}`), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	v, err := ExtractJSONVersion(p)
	if err != nil || v != "3.4.5" {
		t.Errorf("ExtractJSONVersion = (%q, %v), want (3.4.5, nil)", v, err)
	}
}

func TestExtractVerdict(t *testing.T) {
	cases := []struct {
		name   string
		body   string
		strict bool
		want   string
		ok     bool
	}{
		{"inline PASS plain", "Verdict: PASS\n", false, "PASS", true},
		{"inline PASS bold", "**Verdict: PASS**\n", false, "PASS", true},
		{"inline PASS inner-bold", "Verdict: **PASS**\n", false, "PASS", true},
		{"inline WARN non-strict", "Verdict: WARN\n", false, "WARN", true},
		{"inline WARN strict-rejected", "Verdict: WARN\n", true, "", false},
		{"heading PASS", "## Verdict\n\n**PASS**\n", false, "PASS", true},
		{"heading WARN non-strict", "## Verdict\n\n**WARN**\n", false, "WARN", true},
		{"heading WARN strict-rejected", "## Verdict\n\n**WARN**\n", true, "", false},
		{"no verdict", "lorem ipsum\n", false, "", false},
		{"heading PASS too far", "## Verdict\n\n\n\n\n\n**PASS**\n", false, "", false},
		// Bare-line heading form — the cycle-249 release-blocker shape
		// (auditor wrote `## Verdict\nPASS` without bold).
		{"heading bare PASS", "## Verdict\nPASS\n\n**Confidence:** 0.97\n", false, "PASS", true},
		{"heading bare WARN non-strict", "## Verdict\nWARN\n", false, "WARN", true},
		{"heading bare WARN strict-rejected", "## Verdict\nWARN\n", true, "", false},
		{"heading bare FAIL not accepted", "## Verdict\nFAIL\n", false, "", false},
		{"bare PASS inside sentence not accepted", "## Verdict\nAll tests PASS here\n", false, "", false},
		{"heading bare PASS too far", "## Verdict\n\n\n\n\n\nPASS\n", false, "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := extractVerdict(tc.body, tc.strict)
			if ok != tc.ok || got != tc.want {
				t.Errorf("extractVerdict = (%q, %v), want (%q, %v)", got, ok, tc.want, tc.ok)
			}
		})
	}
}

// defaultGitClean / defaultCurrentBranch on a real fixture (integration-light).
func TestDefaultGitClean_OnRealRepo(t *testing.T) {
	d := t.TempDir()
	mustRunBash(t, "git -C "+d+" init -q -b main")
	mustRunBash(t, "git -C "+d+" config user.email t@t.t && git -C "+d+" config user.name t")
	if err := os.WriteFile(filepath.Join(d, "x.txt"), []byte("hi"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	mustRunBash(t, "git -C "+d+" add x.txt && git -C "+d+" commit -q -m init")
	// Clean state.
	ok, err := defaultGitClean(d)
	if err != nil || !ok {
		t.Errorf("clean tree = (%v, %v), want (true, nil)", ok, err)
	}
	// Dirty state.
	if err := os.WriteFile(filepath.Join(d, "x.txt"), []byte("changed"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	ok, err = defaultGitClean(d)
	if err != nil || ok {
		t.Errorf("dirty tree = (%v, %v), want (false, nil)", ok, err)
	}
}

func TestDefaultCurrentBranch_OnRealRepo(t *testing.T) {
	d := t.TempDir()
	mustRunBash(t, "git -C "+d+" init -q -b main")
	mustRunBash(t, "git -C "+d+" config user.email t@t.t && git -C "+d+" config user.name t")
	if err := os.WriteFile(filepath.Join(d, "x.txt"), []byte("hi"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	mustRunBash(t, "git -C "+d+" add x.txt && git -C "+d+" commit -q -m init")
	branch, err := defaultCurrentBranch(d)
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if branch != "main" {
		t.Errorf("branch = %q, want main", branch)
	}
}

// mustRunBash is a tiny helper for integration-light tests that need real git.
func mustRunBash(t *testing.T, cmdline string) {
	t.Helper()
	out, err := osExec("bash", "-c", cmdline).CombinedOutput()
	if err != nil {
		t.Fatalf("shell %q failed: %v\noutput: %s", cmdline, err, out)
	}
}
