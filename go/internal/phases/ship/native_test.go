// native_test.go — 23-case parity matrix vs ship-integration-test.sh.
//
// Each test case mirrors one of A, B, C, C2, D, E, F, G, H, I, J, K, L, M, N,
// O, P, Q, R, S, T, U, V in legacy/scripts/tests/ship-integration-test.sh.
//
// Tests create ephemeral git repos via makeRepo() and seed audit
// ledger entries via seedAudit(). The native Run() is invoked directly
// (no shell-out). Each assertion mirrors the corresponding bash check.
//
// Requires `git` on PATH. Most tests also create a bare remote for push.

package ship

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// makeRepo creates a fresh git repo with:
//   - fixture.txt tracked
//   - .gitignore (.evolve/)
//   - empty .evolve/{ledger.jsonl,state.json}
//   - a stub ship-binary-fixture file (TOFU pins its SHA)
//   - initial commit "initial test repo"
//
// Returns the absolute repo path. Cleanup is via t.Cleanup (TempDir).
func makeRepo(t *testing.T) string {
	t.Helper()
	repo := t.TempDir()
	mustWrite(t, filepath.Join(repo, ".gitignore"), ".evolve/\n")
	mustMkdir(t, filepath.Join(repo, ".evolve", "runs", "cycle-1"))
	mustWrite(t, filepath.Join(repo, ".evolve", "ledger.jsonl"), "")
	mustWrite(t, filepath.Join(repo, ".evolve", "state.json"), "{}\n")
	mustWrite(t, filepath.Join(repo, "fixture.txt"), "fixture line 1\n")
	// Ship binary fixture: stable initial content. Tests that need to
	// "tamper" with the ship binary modify this file directly.
	mustWrite(t, filepath.Join(repo, "ship-binary-fixture"), "ship-binary-v1\n")

	runGit(t, repo, "init", "-q")
	runGit(t, repo, "config", "user.email", "test@evolve-loop.test")
	runGit(t, repo, "config", "user.name", "Test User")
	runGit(t, repo, "config", "core.hooksPath", "/dev/null")
	runGit(t, repo, "add", "-A")
	runGit(t, repo, "-c", "commit.gpgsign=false", "commit", "-q", "-m", "initial test repo")
	return repo
}

// addRemote creates a bare repo to serve as origin and registers it.
// Also forces branch to main and pushes initial commit so `git push`
// is fast-forward later.
func addRemote(t *testing.T, repo string) string {
	t.Helper()
	bare := filepath.Join(t.TempDir(), "remote.git")
	cmd := exec.Command("git", "init", "-q", "--bare", bare)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git init --bare: %v\n%s", err, out)
	}
	runGit(t, repo, "remote", "add", "origin", bare)
	runGit(t, repo, "branch", "-M", "main")
	return bare
}

// seedAudit writes audit-report.md + ledger.jsonl entry mirroring the
// bash seed_audit helper. exitCode defaults to 0; pass -1 to use the
// override. verdict text is embedded into the report body.
//
// optOverrides: optional map with keys "head", "tree", "exit_code" to
// override the ledger entry fields. Use to test mismatch cases.
func seedAudit(t *testing.T, repo, verdict string, optOverrides ...map[string]string) {
	t.Helper()
	auditPath := filepath.Join(repo, ".evolve", "runs", "cycle-1", "audit-report.md")
	body := fmt.Sprintf("<!-- challenge-token: testtoken123 -->\n# Audit Report — Cycle 1\n\nVerdict: %s\n\nAll criteria met (test fixture).\n", verdict)
	mustWrite(t, auditPath, body)
	sha := mustHashFile(t, auditPath)

	overrides := map[string]string{}
	if len(optOverrides) > 0 {
		overrides = optOverrides[0]
	}

	headSHA := overrides["head"]
	if headSHA == "" {
		headSHA = strings.TrimSpace(runGitOut(t, repo, "rev-parse", "HEAD"))
	}
	treeSHA := overrides["tree"]
	if treeSHA == "" {
		treeSHA = treeStateSHA(t, repo)
	}
	exitCode := 0
	if v := overrides["exit_code"]; v != "" {
		fmt.Sscanf(v, "%d", &exitCode)
	}

	entry := map[string]any{
		"ts":              "2026-04-27T00:00:00Z",
		"cycle":           1,
		"role":            "auditor",
		"kind":            "agent_subprocess",
		"model":           "sonnet",
		"exit_code":       exitCode,
		"duration_s":      "30",
		"artifact_path":   auditPath,
		"artifact_sha256": sha,
		"challenge_token": "testtoken123",
		"git_head":        headSHA,
		"tree_state_sha":  treeSHA,
	}
	line, _ := json.Marshal(entry)
	mustWrite(t, filepath.Join(repo, ".evolve", "ledger.jsonl"), string(line)+"\n")
}

// treeStateSHA computes sha256(git diff HEAD) — the same fingerprint
// the audit-binding model uses. Wraps the git invocation for test setup.
func treeStateSHA(t *testing.T, repo string) string {
	t.Helper()
	out := runGitOut(t, repo, "diff", "HEAD")
	h := sha256.New()
	_, _ = h.Write([]byte(out))
	return hex.EncodeToString(h.Sum(nil))
}

func mustHashFile(t *testing.T, path string) string {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open %s: %v", path, err)
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		t.Fatalf("hash %s: %v", path, err)
	}
	return hex.EncodeToString(h.Sum(nil))
}

func mustWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", path, err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func mustMkdir(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", path, err)
	}
}

func runGit(t *testing.T, repo string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", repo}, args...)...)
	cmd.Env = filteredEnv()
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

func runGitOut(t *testing.T, repo string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", repo}, args...)...)
	cmd.Env = filteredEnv()
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git %v: %v", args, err)
	}
	return string(out)
}

// filteredEnv strips evolve-loop env vars from the parent test process
// so we don't pick up the operator's actual state.
func filteredEnv() []string {
	var out []string
	for _, kv := range os.Environ() {
		if strings.HasPrefix(kv, "EVOLVE_") {
			continue
		}
		out = append(out, kv)
	}
	return out
}

// runShip invokes the native Run() with sensible defaults for testing.
// Returns the result; tests assert on .ExitCode and .Logs.
func runShip(t *testing.T, repo string, opts Options) (RunResult, error) {
	t.Helper()
	if opts.ProjectRoot == "" {
		opts.ProjectRoot = repo
	}
	if opts.PluginRoot == "" {
		opts.PluginRoot = repo
	}
	if opts.ShipBinaryPath == "" {
		opts.ShipBinaryPath = filepath.Join(repo, "ship-binary-fixture")
	}
	// Use real exec runner for tests — they assert real git semantics.
	if opts.Runner == nil {
		opts.Runner = execRunner
	}
	// Default to bytes.Buffer for stderr capture so tests can assert log lines.
	var stderr bytes.Buffer
	if opts.Stderr == nil {
		opts.Stderr = &stderr
	}
	if opts.Stdin == nil {
		opts.Stdin = bytes.NewReader(nil)
	}
	if opts.Stdout == nil {
		opts.Stdout = io.Discard
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	return Run(ctx, opts)
}

// containsLog reports whether any entry in res.Logs contains substr.
func containsLog(res RunResult, substr string) bool {
	for _, l := range res.Logs {
		if strings.Contains(l, substr) {
			return true
		}
	}
	return false
}

// --- Test A: no auditor ledger entry → refuses (rc=2) ---------------

func TestNative_A_NoAuditor_Refuses(t *testing.T) {
	repo := makeRepo(t)
	res, err := runShip(t, repo, Options{Class: ClassCycle, CommitMessage: "test commit"})
	if res.ExitCode != ExitIntegrity {
		t.Fatalf("want ExitIntegrity, got %d (err=%v, logs=%v)", res.ExitCode, err, res.Logs)
	}
	if !containsLog(res, "no Auditor") {
		t.Errorf("missing 'no Auditor' log in: %v", res.Logs)
	}
}

// --- Test B: PASS audit + matching state → ships (rc=0) -------------

func TestNative_B_PASS_AuditMatching_Ships(t *testing.T) {
	repo := makeRepo(t)
	mustWrite(t, filepath.Join(repo, "fixture.txt"), "fixture line 1\nmodified content\n")
	seedAudit(t, repo, "PASS")
	addRemote(t, repo)
	res, err := runShip(t, repo, Options{Class: ClassCycle, CommitMessage: "feat: test"})
	if res.ExitCode != ExitOK {
		t.Fatalf("want ExitOK, got %d (err=%v, logs=%v)", res.ExitCode, err, res.Logs)
	}
	if res.CommitSHA == "" {
		t.Errorf("expected non-empty CommitSHA")
	}
}

// --- Test C: WARN audit ships by default (fluent) -------------------

func TestNative_C_WARN_ShipsFluent(t *testing.T) {
	repo := makeRepo(t)
	mustWrite(t, filepath.Join(repo, "fixture.txt"), "fixture line 1\nwarn change\n")
	seedAudit(t, repo, "WARN")
	addRemote(t, repo)
	res, _ := runShip(t, repo, Options{Class: ClassCycle, CommitMessage: "feat: shipping with WARN"})
	if res.ExitCode != ExitOK {
		t.Fatalf("want ExitOK, got %d (logs=%v)", res.ExitCode, res.Logs)
	}
	if !containsLog(res, "audit verdict: WARN — shipping") {
		t.Errorf("missing WARN-shipping log in: %v", res.Logs)
	}
}

// --- Test C2: EVOLVE_STRICT_AUDIT=1 → WARN refused ------------------

func TestNative_C2_StrictAudit_WARN_Refused(t *testing.T) {
	repo := makeRepo(t)
	mustWrite(t, filepath.Join(repo, "fixture.txt"), "fixture line 1\nwarn change strict\n")
	seedAudit(t, repo, "WARN")
	res, _ := runShip(t, repo, Options{
		Class:         ClassCycle,
		CommitMessage: "should not ship",
		Env:           map[string]string{"EVOLVE_STRICT_AUDIT": "1"},
	})
	if res.ExitCode != ExitIntegrity {
		t.Fatalf("want ExitIntegrity, got %d", res.ExitCode)
	}
	if !containsLog(res, "EVOLVE_STRICT_AUDIT=1") {
		t.Errorf("missing strict-audit message in: %v", res.Logs)
	}
}

// --- Test D: tree-state mismatch (modified after audit) → refuses ---

func TestNative_D_TreeStateMismatch_Refuses(t *testing.T) {
	repo := makeRepo(t)
	mustWrite(t, filepath.Join(repo, "fixture.txt"), "fixture line 1\nversion 1 of audited content\n")
	seedAudit(t, repo, "PASS")
	// Mutate after audit:
	mustWrite(t, filepath.Join(repo, "fixture.txt"), "fixture line 1\nversion 1 of audited content\nversion 2 — added after audit\n")
	res, _ := runShip(t, repo, Options{Class: ClassCycle, CommitMessage: "should refuse"})
	if res.ExitCode != ExitIntegrity {
		t.Fatalf("want ExitIntegrity, got %d (logs=%v)", res.ExitCode, res.Logs)
	}
	if !containsLog(res, "tree-state mismatch") {
		t.Errorf("missing tree-state-mismatch in: %v", res.Logs)
	}
}

// --- Test E: HEAD moved since audit → refuses -----------------------

func TestNative_E_HEADMismatch_Refuses(t *testing.T) {
	repo := makeRepo(t)
	seedAudit(t, repo, "PASS", map[string]string{
		"head": "0000000000000000000000000000000000000000",
	})
	res, _ := runShip(t, repo, Options{Class: ClassCycle, CommitMessage: "should refuse"})
	if res.ExitCode != ExitIntegrity {
		t.Fatalf("want ExitIntegrity, got %d", res.ExitCode)
	}
	if !containsLog(res, "HEAD has moved") {
		t.Errorf("missing 'HEAD has moved' in: %v", res.Logs)
	}
}

// --- Test F: ship binary modified within same plugin version → refuses ---

func TestNative_F_SelfSHATamperedWithinVersion_Refuses(t *testing.T) {
	repo := makeRepo(t)
	mustMkdir(t, filepath.Join(repo, ".claude-plugin"))
	mustWrite(t, filepath.Join(repo, ".claude-plugin", "plugin.json"), `{"version":"1.0.0"}`)
	addRemote(t, repo)

	// First ship: pins SHA + version=1.0.0
	mustWrite(t, filepath.Join(repo, "audited.txt"), "audited\n")
	seedAudit(t, repo, "PASS")
	res1, _ := runShip(t, repo, Options{Class: ClassCycle, CommitMessage: "first ship"})
	if res1.ExitCode != ExitOK {
		t.Fatalf("first ship: want ExitOK got %d (logs=%v)", res1.ExitCode, res1.Logs)
	}

	// Tamper: modify the ship binary fixture (simulates an attacker
	// editing ship.sh while plugin.json:version is unchanged).
	mustWrite(t, filepath.Join(repo, "ship-binary-fixture"), "ship-binary-v1\n# malicious comment\n")

	// Second ship: same version, different SHA → INTEGRITY-FAIL.
	mustWrite(t, filepath.Join(repo, "another.txt"), "another change\n")
	seedAudit(t, repo, "PASS")
	res2, _ := runShip(t, repo, Options{Class: ClassCycle, CommitMessage: "second ship"})
	if res2.ExitCode != ExitIntegrity {
		t.Fatalf("second ship: want ExitIntegrity got %d (logs=%v)", res2.ExitCode, res2.Logs)
	}
	if !containsLog(res2, "WITHIN plugin version") {
		t.Errorf("missing 'WITHIN plugin version' in: %v", res2.Logs)
	}
}

// --- Test G: EVOLVE_BYPASS_SHIP_VERIFY=1 → ships ---------------------

func TestNative_G_BypassEnv_AllowsShip(t *testing.T) {
	repo := makeRepo(t)
	mustWrite(t, filepath.Join(repo, "emergency.txt"), "emergency change\n")
	addRemote(t, repo)
	res, _ := runShip(t, repo, Options{
		Class:         ClassCycle,
		CommitMessage: "emergency",
		Env:           map[string]string{"EVOLVE_SHIP_AUTO_CONFIRM": "1", "EVOLVE_BYPASS_SHIP_VERIFY": "1"},
	})
	if res.ExitCode != ExitOK {
		t.Fatalf("want ExitOK, got %d (logs=%v)", res.ExitCode, res.Logs)
	}
}

// --- Test H: --class release → ships without audit ------------------

func TestNative_H_ClassRelease_ShipsNoAudit(t *testing.T) {
	repo := makeRepo(t)
	mustWrite(t, filepath.Join(repo, "release.txt"), "release bump\n")
	addRemote(t, repo)
	res, _ := runShip(t, repo, Options{Class: ClassRelease, CommitMessage: "release: v9.99.99"})
	if res.ExitCode != ExitOK {
		t.Fatalf("want ExitOK got %d (logs=%v)", res.ExitCode, res.Logs)
	}
	if !containsLog(res, "class: release") {
		t.Errorf("missing 'class: release' log in: %v", res.Logs)
	}
}

// --- Test I: --class manual without tty → refuses --------------------

func TestNative_I_ManualNoTTY_Refuses(t *testing.T) {
	repo := makeRepo(t)
	mustWrite(t, filepath.Join(repo, "manual.txt"), "manual change\n")
	addRemote(t, repo)
	// Stdin is bytes.Buffer (non-tty), no EVOLVE_SHIP_AUTO_CONFIRM.
	res, _ := runShip(t, repo, Options{
		Class:         ClassManual,
		CommitMessage: "manual change",
	})
	if res.ExitCode != ExitIntegrity {
		t.Fatalf("want ExitIntegrity got %d (logs=%v)", res.ExitCode, res.Logs)
	}
	if !containsLog(res, "requires interactive stdin") {
		t.Errorf("missing tty-required message in: %v", res.Logs)
	}
}

// --- Test J: --class manual + AUTO_CONFIRM=1 → ships -----------------

func TestNative_J_ManualAutoConfirm_Ships(t *testing.T) {
	repo := makeRepo(t)
	mustWrite(t, filepath.Join(repo, "ci.txt"), "ci change\n")
	addRemote(t, repo)
	res, _ := runShip(t, repo, Options{
		Class:         ClassManual,
		CommitMessage: "ci change",
		// Bypass commit-gate: this test exercises the manual auto-confirm ship
		// mechanics, not the review-attestation gate (covered in commitgate_test.go).
		Env: map[string]string{"EVOLVE_SHIP_AUTO_CONFIRM": "1", "EVOLVE_BYPASS_COMMIT_GATE": "1"},
	})
	if res.ExitCode != ExitOK {
		t.Fatalf("want ExitOK got %d (logs=%v)", res.ExitCode, res.Logs)
	}
	if !containsLog(res, "auto-confirmed") {
		t.Errorf("missing 'auto-confirmed' in: %v", res.Logs)
	}
}

// --- Test K: legacy EVOLVE_BYPASS_SHIP_VERIFY → deprecation log ------

func TestNative_K_BypassEnvDeprecationLogged(t *testing.T) {
	repo := makeRepo(t)
	mustWrite(t, filepath.Join(repo, "bridge.txt"), "bridge change\n")
	addRemote(t, repo)
	res, _ := runShip(t, repo, Options{
		Class:         ClassCycle,
		CommitMessage: "legacy bypass",
		Env:           map[string]string{"EVOLVE_BYPASS_SHIP_VERIFY": "1"},
	})
	if res.ExitCode != ExitOK {
		t.Fatalf("want ExitOK got %d (logs=%v)", res.ExitCode, res.Logs)
	}
	if !containsLog(res, "DEPRECATION: EVOLVE_BYPASS_SHIP_VERIFY=1") {
		t.Errorf("missing deprecation log in: %v", res.Logs)
	}
}

// --- Test L: invalid class → rejected with rc=1 ----------------------

func TestNative_L_InvalidClass_Rejected(t *testing.T) {
	repo := makeRepo(t)
	res, err := runShip(t, repo, Options{Class: Class("garbage"), CommitMessage: "msg"})
	if err == nil {
		t.Fatalf("want error, got nil (res=%+v)", res)
	}
	if !strings.Contains(err.Error(), "invalid --class") {
		t.Errorf("missing 'invalid --class' in error: %v", err)
	}
}

// --- Test M: exit_code=1 + Verdict:PASS → ships ----------------------

func TestNative_M_AuditorExit1_PASS_Ships(t *testing.T) {
	repo := makeRepo(t)
	mustWrite(t, filepath.Join(repo, "fixture.txt"), "fixture line 1\nmodified for exit-1 test\n")
	seedAudit(t, repo, "PASS", map[string]string{"exit_code": "1"})
	addRemote(t, repo)
	res, _ := runShip(t, repo, Options{Class: ClassCycle, CommitMessage: "feat: ship with exit-1"})
	if res.ExitCode != ExitOK {
		t.Fatalf("want ExitOK got %d (logs=%v)", res.ExitCode, res.Logs)
	}
}

// --- Test N: exit_code=2 → refuses (anti-gaming) ---------------------

func TestNative_N_AuditorExit2_Refuses(t *testing.T) {
	repo := makeRepo(t)
	mustWrite(t, filepath.Join(repo, "fixture.txt"), "fixture line 1\nmodified for exit-2 test\n")
	seedAudit(t, repo, "PASS", map[string]string{"exit_code": "2"})
	res, _ := runShip(t, repo, Options{Class: ClassCycle, CommitMessage: "feat: ship with exit-2"})
	if res.ExitCode != ExitIntegrity {
		t.Fatalf("want ExitIntegrity got %d (logs=%v)", res.ExitCode, res.Logs)
	}
	if !containsLog(res, "Auditor exited 2") {
		t.Errorf("missing 'Auditor exited 2' in: %v", res.Logs)
	}
}

// --- Test O: exit_code=0 + Verdict:FAIL → refuses --------------------

func TestNative_O_VerdictFAIL_Refuses(t *testing.T) {
	repo := makeRepo(t)
	mustWrite(t, filepath.Join(repo, "fixture.txt"), "fixture line 1\nmodified for verdict-fail test\n")
	seedAudit(t, repo, "FAIL")
	res, _ := runShip(t, repo, Options{Class: ClassCycle, CommitMessage: "feat: ship with verdict fail"})
	if res.ExitCode != ExitIntegrity {
		t.Fatalf("want ExitIntegrity got %d (logs=%v)", res.ExitCode, res.Logs)
	}
	if !containsLog(res, "Verdict: FAIL") && !containsLog(res, "auditor explicitly rejected") {
		t.Errorf("missing FAIL verdict diagnostic in: %v", res.Logs)
	}
}

// --- Test P: dual-verdict (PASS + FAIL) → refuses --------------------

func TestNative_P_DualVerdict_Refuses(t *testing.T) {
	repo := makeRepo(t)
	mustWrite(t, filepath.Join(repo, "fixture.txt"), "fixture line 1\ndual-verdict change\n")
	auditPath := filepath.Join(repo, ".evolve", "runs", "cycle-1", "audit-report.md")
	body := `<!-- challenge-token: testtoken123 -->
# Audit Report — Cycle 1

## Verdict
**FAIL**

But also somewhere in this report:
Verdict: PASS

(simulating cycle-25's actual audit-report.md inconsistency)
`
	mustWrite(t, auditPath, body)
	sha := mustHashFile(t, auditPath)
	headSHA := strings.TrimSpace(runGitOut(t, repo, "rev-parse", "HEAD"))
	treeSHA := treeStateSHA(t, repo)
	entry := map[string]any{
		"ts": "2026-04-27T00:00:00Z", "cycle": 1, "role": "auditor",
		"kind": "agent_subprocess", "model": "sonnet", "exit_code": 0,
		"duration_s": "30", "artifact_path": auditPath, "artifact_sha256": sha,
		"challenge_token": "testtoken123", "git_head": headSHA, "tree_state_sha": treeSHA,
	}
	line, _ := json.Marshal(entry)
	mustWrite(t, filepath.Join(repo, ".evolve", "ledger.jsonl"), string(line)+"\n")

	res, _ := runShip(t, repo, Options{Class: ClassCycle, CommitMessage: "ship dual-verdict"})
	if res.ExitCode != ExitIntegrity {
		t.Fatalf("want ExitIntegrity got %d (logs=%v)", res.ExitCode, res.Logs)
	}
	if !containsLog(res, "BOTH 'Verdict: FAIL' AND 'Verdict: PASS'") {
		t.Errorf("missing dual-verdict message in: %v", res.Logs)
	}
}

// --- Test Q: plugin version bump → re-pins, ships --------------------

func TestNative_Q_PluginVersionBump_RePins(t *testing.T) {
	repo := makeRepo(t)
	mustMkdir(t, filepath.Join(repo, ".claude-plugin"))
	mustWrite(t, filepath.Join(repo, ".claude-plugin", "plugin.json"), `{"version":"1.0.0"}`)
	addRemote(t, repo)

	// First ship at v1.0.0
	mustWrite(t, filepath.Join(repo, "q1.txt"), "first audited\n")
	seedAudit(t, repo, "PASS")
	res1, _ := runShip(t, repo, Options{Class: ClassCycle, CommitMessage: "first ship at v1.0.0"})
	if res1.ExitCode != ExitOK {
		t.Fatalf("first ship: want ExitOK got %d (logs=%v)", res1.ExitCode, res1.Logs)
	}

	// Bump version + modify the ship binary to simulate plugin update.
	mustWrite(t, filepath.Join(repo, ".claude-plugin", "plugin.json"), `{"version":"1.1.0"}`)
	mustWrite(t, filepath.Join(repo, "ship-binary-fixture"), "ship-binary-v2\n# v1.1.0 tweak\n")
	mustWrite(t, filepath.Join(repo, "q2.txt"), "second\n")
	seedAudit(t, repo, "PASS")
	res2, _ := runShip(t, repo, Options{Class: ClassCycle, CommitMessage: "ship at v1.1.0"})
	if res2.ExitCode != ExitOK {
		t.Fatalf("second ship: want ExitOK got %d (logs=%v)", res2.ExitCode, res2.Logs)
	}
	if !containsLog(res2, "plugin version changed: '1.0.0' → '1.1.0'") {
		t.Errorf("missing version-change log in: %v", res2.Logs)
	}
}

// --- Test R: legacy SHA-only pin → migrates ------------------------

func TestNative_R_LegacySHAOnlyPin_Migrates(t *testing.T) {
	repo := makeRepo(t)
	mustMkdir(t, filepath.Join(repo, ".claude-plugin"))
	mustWrite(t, filepath.Join(repo, ".claude-plugin", "plugin.json"), `{"version":"2.0.0"}`)
	// Pre-populate state.json with expected_ship_sha matching current binary
	// but NO expected_ship_version (legacy schema).
	binPath := filepath.Join(repo, "ship-binary-fixture")
	currentSHA := mustHashFile(t, binPath)
	mustWrite(t, filepath.Join(repo, ".evolve", "state.json"),
		fmt.Sprintf(`{"expected_ship_sha":"%s"}`, currentSHA))
	mustWrite(t, filepath.Join(repo, "r.txt"), "audited\n")
	seedAudit(t, repo, "PASS")
	addRemote(t, repo)

	res, _ := runShip(t, repo, Options{Class: ClassCycle, CommitMessage: "ship after migration"})
	if res.ExitCode != ExitOK {
		t.Fatalf("want ExitOK got %d (logs=%v)", res.ExitCode, res.Logs)
	}
	// Verify state.json now has expected_ship_version=2.0.0
	stMap, _ := readStateMap(filepath.Join(repo, ".evolve", "state.json"))
	if v := stateString(stMap, "expected_ship_version"); v != "2.0.0" {
		t.Errorf("want expected_ship_version=2.0.0, got %q", v)
	}
	if !containsLog(res, "migrating legacy SHA-only pin") && !containsLog(res, "schema migration") {
		t.Errorf("missing migration log in: %v", res.Logs)
	}
}

// --- Test S: cycle ship advances lastCycleNumber ---------------------

func TestNative_S_CycleAdvancesLastCycleNumber(t *testing.T) {
	repo := makeRepo(t)
	mustWrite(t, filepath.Join(repo, "fixture.txt"), "fixture line 1\nv8.34.0 cycle ship test\n")
	seedAudit(t, repo, "PASS")
	mustWrite(t, filepath.Join(repo, ".evolve", "cycle-state.json"), `{"cycle_id":1,"phase":"ship"}`)
	mustWrite(t, filepath.Join(repo, ".evolve", "state.json"), `{"lastCycleNumber":0}`)
	addRemote(t, repo)

	res, _ := runShip(t, repo, Options{Class: ClassCycle, CommitMessage: "feat: cycle 1 work"})
	if res.ExitCode != ExitOK {
		t.Fatalf("want ExitOK got %d (logs=%v)", res.ExitCode, res.Logs)
	}
	stMap, _ := readStateMap(filepath.Join(repo, ".evolve", "state.json"))
	got, _ := stateInt(stMap, "lastCycleNumber")
	if got != 1 {
		t.Errorf("want lastCycleNumber=1, got %d", got)
	}
	if !containsLog(res, "advanced state.json:lastCycleNumber to 1") {
		t.Errorf("missing advance log in: %v", res.Logs)
	}
}

// --- Test T: manual ship leaves lastCycleNumber unchanged ----------

func TestNative_T_ManualPreservesLastCycleNumber(t *testing.T) {
	repo := makeRepo(t)
	mustWrite(t, filepath.Join(repo, "fixture.txt"), "fixture line 1\nmanual change v8.34\n")
	mustWrite(t, filepath.Join(repo, ".evolve", "cycle-state.json"), `{"cycle_id":99,"phase":"ship"}`)
	mustWrite(t, filepath.Join(repo, ".evolve", "state.json"), `{"lastCycleNumber":5}`)
	addRemote(t, repo)

	res, _ := runShip(t, repo, Options{
		Class:         ClassManual,
		CommitMessage: "manual: ad-hoc fix",
		// Bypass commit-gate: this test asserts lastCycleNumber semantics, not
		// the review-attestation gate (covered in commitgate_test.go).
		Env: map[string]string{"EVOLVE_SHIP_AUTO_CONFIRM": "1", "EVOLVE_BYPASS_COMMIT_GATE": "1"},
	})
	if res.ExitCode != ExitOK {
		t.Fatalf("want ExitOK got %d (logs=%v)", res.ExitCode, res.Logs)
	}
	stMap, _ := readStateMap(filepath.Join(repo, ".evolve", "state.json"))
	got, _ := stateInt(stMap, "lastCycleNumber")
	if got != 5 {
		t.Errorf("want lastCycleNumber=5 (unchanged), got %d", got)
	}
}

// --- Test U: actual-diff footer appended for cycle commit -----------

func TestNative_U_ActualDiffFooter_CycleCommit(t *testing.T) {
	repo := makeRepo(t)
	mustWrite(t, filepath.Join(repo, "fixture.txt"), "fixture line 1\ndiff transparency test\n")
	mustWrite(t, filepath.Join(repo, "newfile.txt"), "new file content\n")
	seedAudit(t, repo, "PASS")
	mustWrite(t, filepath.Join(repo, ".evolve", "cycle-state.json"), `{"cycle_id":2,"phase":"ship"}`)
	addRemote(t, repo)

	res, _ := runShip(t, repo, Options{Class: ClassCycle, CommitMessage: "feat: claims do not match"})
	if res.ExitCode != ExitOK {
		t.Fatalf("want ExitOK got %d (logs=%v)", res.ExitCode, res.Logs)
	}
	lastMsg := runGitOut(t, repo, "log", "-1", "--format=%B")
	if !strings.Contains(lastMsg, "## Actual diff (v8.34.0+)") {
		t.Errorf("missing actual-diff header in: %s", lastMsg)
	}
	if !strings.Contains(lastMsg, "fixture.txt") || !strings.Contains(lastMsg, "newfile.txt") {
		t.Errorf("missing file entries in: %s", lastMsg)
	}
}

// --- Test V: --class release skips actual-diff footer ---------------

func TestNative_V_ReleaseSkipsFooter(t *testing.T) {
	repo := makeRepo(t)
	mustWrite(t, filepath.Join(repo, "fixture.txt"), "fixture line 1\nrelease content\n")
	addRemote(t, repo)
	res, _ := runShip(t, repo, Options{Class: ClassRelease, CommitMessage: "release: v9.0.0"})
	if res.ExitCode != ExitOK {
		t.Fatalf("want ExitOK got %d (logs=%v)", res.ExitCode, res.Logs)
	}
	lastMsg := runGitOut(t, repo, "log", "-1", "--format=%B")
	if strings.Contains(lastMsg, "## Actual diff") {
		t.Errorf("release commit should NOT have actual-diff footer, got: %s", lastMsg)
	}
}
