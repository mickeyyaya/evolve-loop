//go:build integration

// audit_gaps_test.go — covers verifyAuditBinding branches not exercised by
// the parity matrix: auditor-exit-code>1, dual PASS+FAIL verdict,
// WARN+STRICT_AUDIT=1, no-verdict, stale audit, GitHEAD mismatch.
package ship

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

// TestVerifyAuditBinding_AuditorExitCode2_IntegrityError: an auditor that
// exited 2+ (error state, not the unix-findings-convention exit 1) must
// block ship with IntegrityError. Exit codes 0 and 1 are the only allowed
// "findings signal" values.
func TestVerifyAuditBinding_AuditorExitCode2_IntegrityError(t *testing.T) {
	repo := makeRepo(t)
	seedAudit(t, repo, "PASS", map[string]string{"exit_code": "2"})
	opts := auditOpts(t, repo)
	err := verifyAuditBinding(context.Background(), opts, &RunResult{}) //nolint:staticcheck
	wantShipErr(t, err, core.CodeAuditBindingAuditorExit, core.ShipClassPrecondition, "exited 2")
}

// TestVerifyAuditBinding_DualVerdict_PASS_and_FAIL_IntegrityError: an
// audit report that declares BOTH "Verdict: PASS" and "Verdict: FAIL"
// is an inconsistent artifact. Ship must refuse it with IntegrityError
// (v8.30.0 dual-verdict detection).
func TestVerifyAuditBinding_DualVerdict_PASS_and_FAIL_IntegrityError(t *testing.T) {
	repo := makeRepo(t)
	seedCustomAudit(t, repo,
		"<!-- challenge-token: testtoken123 -->\n# Audit Report — Cycle 1\n\nVerdict: PASS\nVerdict: FAIL\n\nAll criteria met (test fixture).\n",
		0,
	)
	opts := auditOpts(t, repo)
	err := verifyAuditBinding(context.Background(), opts, &RunResult{}) //nolint:staticcheck
	wantShipErr(t, err, core.CodeAuditBindingDualVerdict, core.ShipClassPrecondition, "BOTH")
}

// TestVerifyAuditBinding_Fail_Verdict_IntegrityError: "Verdict: FAIL" must
// block ship — the most common auditor rejection path.
func TestVerifyAuditBinding_FailVerdict_IntegrityError(t *testing.T) {
	repo := makeRepo(t)
	seedAudit(t, repo, "FAIL")
	opts := auditOpts(t, repo)
	err := verifyAuditBinding(context.Background(), opts, &RunResult{}) //nolint:staticcheck
	wantShipErr(t, err, core.CodeAuditBindingVerdictFail, core.ShipClassPrecondition, "FAIL")
}

// TestVerifyAuditBinding_WarnWithStrictAudit_IntegrityError: a WARN verdict
// with policy.json workflow.strict_audit must block ship. Without it, WARN ships.
func TestVerifyAuditBinding_WarnWithStrictAudit_IntegrityError(t *testing.T) {
	repo := makeRepo(t)
	// No git changes needed — just need a WARN verdict + matching HEAD/tree.
	seedAudit(t, repo, "WARN")
	writeStrictAuditPolicy(t, repo)
	opts := auditOpts(t, repo)
	err := verifyAuditBinding(context.Background(), opts, &RunResult{}) //nolint:staticcheck
	wantShipErr(t, err, core.CodeAuditBindingVerdictWarn, core.ShipClassPrecondition, "WARN")
}

// TestVerifyAuditBinding_NoVerdict_IntegrityError: an audit report with no
// recognizable verdict token (PASS/WARN/FAIL) is malformed and must block.
func TestVerifyAuditBinding_NoVerdict_IntegrityError(t *testing.T) {
	repo := makeRepo(t)
	seedCustomAudit(t, repo,
		"<!-- challenge-token: testtoken123 -->\n# Audit Report — Cycle 1\n\nConclusion: Everything looks fine.\n",
		0,
	)
	opts := auditOpts(t, repo)
	err := verifyAuditBinding(context.Background(), opts, &RunResult{}) //nolint:staticcheck
	wantShipErr(t, err, core.CodeAuditBindingMalformed, core.ShipClassPrecondition, "no recognizable verdict")
}

// TestVerifyAuditBinding_GitHEADMismatch_IntegrityError: the audit was
// recorded against a different HEAD. Current HEAD has moved — ship must refuse.
func TestVerifyAuditBinding_GitHEADMismatch_IntegrityError(t *testing.T) {
	repo := makeRepo(t)
	// Seed audit against main's current HEAD.
	seedAudit(t, repo, "PASS")
	// Add a new commit so HEAD moves.
	mustWrite(t, filepath.Join(repo, "new.txt"), "post-audit change\n")
	runGit(t, repo, "add", "-A")
	runGit(t, repo, "-c", "commit.gpgsign=false", "commit", "-m", "post-audit commit")

	opts := auditOpts(t, repo)
	err := verifyAuditBinding(context.Background(), opts, &RunResult{}) //nolint:staticcheck
	wantShipErr(t, err, core.CodeAuditBindingHeadMoved, core.ShipClassPrecondition, "git HEAD has moved")
}

// TestVerifyAuditBinding_StaleAudit_IntegrityError: an audit report older
// than 7 days must be rejected. We fake the file mod time.
func TestVerifyAuditBinding_StaleAudit_IntegrityError(t *testing.T) {
	repo := makeRepo(t)
	seedAudit(t, repo, "PASS")

	// Back-date the audit-report.md by 8 days.
	auditPath := filepath.Join(repo, ".evolve", "runs", "cycle-1", "audit-report.md")
	old := time.Now().Add(-8 * 24 * time.Hour)
	if err := os.Chtimes(auditPath, old, old); err != nil {
		t.Fatalf("chtimes: %v", err)
	}

	opts := auditOpts(t, repo)
	// Freeze NowFn at current time so the age exceeds 7 days.
	opts.NowFn = func() Now {
		unix := time.Now().Unix()
		return Now{Unix: unix, RFC3339: time.Unix(unix, 0).UTC().Format(time.RFC3339)}
	}
	err := verifyAuditBinding(context.Background(), opts, &RunResult{}) //nolint:staticcheck
	wantShipErr(t, err, core.CodeAuditBindingStale, core.ShipClassPrecondition, "old")
}

// TestVerifyAuditBinding_ArtifactMissing_IntegrityError: ledger points to
// an audit-report.md path that doesn't exist on disk.
func TestVerifyAuditBinding_ArtifactMissing_IntegrityError(t *testing.T) {
	repo := makeRepo(t)
	seedAudit(t, repo, "PASS")
	// Delete the artifact after seeding.
	auditPath := filepath.Join(repo, ".evolve", "runs", "cycle-1", "audit-report.md")
	if err := os.Remove(auditPath); err != nil {
		t.Fatalf("remove: %v", err)
	}
	opts := auditOpts(t, repo)
	err := verifyAuditBinding(context.Background(), opts, &RunResult{}) //nolint:staticcheck
	wantShipErr(t, err, core.CodeAuditBindingArtifactMissing, core.ShipClassPrecondition, "missing on disk")
}

// TestVerifyAuditBinding_ArtifactSHAMismatch_IntegrityError: artifact exists
// but its content was changed after the ledger entry was written.
func TestVerifyAuditBinding_ArtifactSHAMismatch_IntegrityError(t *testing.T) {
	repo := makeRepo(t)
	seedAudit(t, repo, "PASS")
	// Corrupt the artifact after seeding (changes SHA).
	auditPath := filepath.Join(repo, ".evolve", "runs", "cycle-1", "audit-report.md")
	if err := os.WriteFile(auditPath, []byte("tampered content\n"), 0o644); err != nil {
		t.Fatalf("tamper: %v", err)
	}
	opts := auditOpts(t, repo)
	err := verifyAuditBinding(context.Background(), opts, &RunResult{}) //nolint:staticcheck
	wantShipErr(t, err, core.CodeAuditBindingArtifactSHA, core.ShipClassPrecondition, "SHA mismatch")
}

// TestVerifyAuditBinding_LegacyEntryNoGitHead_IntegrityError: an auditor
// ledger entry without git_head/tree_state_sha (pre-v8.13.0) must block ship
// with a "predates v8.13.0 cycle-binding" message.
func TestVerifyAuditBinding_LegacyEntryNoGitHead_IntegrityError(t *testing.T) {
	repo := makeRepo(t)
	// Build a ledger entry that looks like a PASS audit but lacks git_head.
	auditPath := filepath.Join(repo, ".evolve", "runs", "cycle-1", "audit-report.md")
	body := "<!-- challenge-token: testtoken123 -->\n# Audit Report — Cycle 1\n\nVerdict: PASS\n\nAll criteria met (test fixture).\n"
	mustWrite(t, auditPath, body)
	sha := mustHashFile(t, auditPath)
	// Ledger entry with no git_head / tree_state_sha.
	entry := fmt.Sprintf(`{"role":"auditor","kind":"agent_subprocess","exit_code":0,"artifact_path":%q,"artifact_sha256":%q}`+"\n",
		auditPath, sha)
	mustWrite(t, filepath.Join(repo, ".evolve", "ledger.jsonl"), entry)

	opts := auditOpts(t, repo)
	err := verifyAuditBinding(context.Background(), opts, &RunResult{}) //nolint:staticcheck
	wantShipErr(t, err, core.CodeAuditBindingNoLedger, core.ShipClassPrecondition, "predates v8.13.0")
}

// TestCheckEGPSGate_SkipCountWithRedZero_Passes: a verdict carrying skip_count
// + a result:"skip" row + red_count==0 must clear the EGPS gate (the
// fresh-clone case). The gate keys solely off red_count; unknown/new fields are
// tolerated by the anonymous-struct unmarshal.
func TestCheckEGPSGate_SkipCountWithRedZero_Passes(t *testing.T) {
	repo := t.TempDir()
	verdict := `{
		"red_count": 0,
		"green_count": 1,
		"skip_count": 4,
		"verdict": "PASS",
		"red_ids": [],
		"skip_ids": ["regression-suite/cycle-57/030-build-report-verdict-count-match"],
		"predicate_suite": {"total": 5, "skipped_count": 4},
		"results": [
			{"ac_id": "cycle-1/001", "result": "green", "exit_code": 0},
			{"ac_id": "regression-suite/cycle-57/030-build-report-verdict-count-match", "result": "skip", "exit_code": 77}
		]
	}`
	path := filepath.Join(repo, "acs-verdict.json")
	mustWrite(t, path, verdict)
	res := &RunResult{}
	if err := checkEGPSGate(path, res); err != nil {
		t.Fatalf("checkEGPSGate returned %v, want nil (red_count==0 with skips must pass)", err)
	}
}

// TestCheckEGPSGate_RedCountWithSkipsPresent_Blocks: skips present alongside a
// genuine red must still block ship — SKIP cannot mask a real RED.
func TestCheckEGPSGate_RedCountWithSkipsPresent_Blocks(t *testing.T) {
	repo := t.TempDir()
	verdict := `{
		"red_count": 1,
		"green_count": 1,
		"skip_count": 2,
		"verdict": "FAIL",
		"red_ids": ["cycle-1/002"],
		"predicate_suite": {"total": 4, "skipped_count": 2}
	}`
	path := filepath.Join(repo, "acs-verdict.json")
	mustWrite(t, path, verdict)
	res := &RunResult{}
	err := checkEGPSGate(path, res)
	wantShipErr(t, err, core.CodeEGPSRedCount, core.ShipClassPrecondition, "RED predicate")
}

// --- helpers ----------------------------------------------------------------

// auditOpts returns an Options wired for verifyAuditBinding unit tests: real
// exec runner, ship-binary-fixture for TOFU, project root = repo.
func auditOpts(t *testing.T, repo string) *Options {
	t.Helper()
	bin := filepath.Join(repo, "ship-binary-fixture")
	// Pin the TOFU state upfront so verifyAuditBinding doesn't fail on TOFU.
	preSeedTOFU(t, repo, bin)
	return &Options{
		ProjectRoot:    repo,
		PluginRoot:     repo,
		ShipBinaryPath: bin,
		Runner:         execRunner,
		NowFn:          defaultNow,
	}
}

// preSeedTOFU writes the current ship binary SHA into state.json:expected_ship_sha
// so verifySelfSHA passes without re-pinning during test.
func preSeedTOFU(t *testing.T, repo, binPath string) {
	t.Helper()
	sha, err := sha256File(binPath)
	if err != nil {
		t.Fatalf("sha256File(%s): %v", binPath, err)
	}
	stPath := filepath.Join(repo, ".evolve", "state.json")
	m, _ := readStateMap(stPath)
	m["expected_ship_sha"] = sha
	m["expected_ship_version"] = ""
	if err := writeStateMap(stPath, m); err != nil {
		t.Fatalf("preSeedTOFU writeStateMap: %v", err)
	}
}

// seedCustomAudit writes a custom body as audit-report.md and an auditor
// ledger entry with the given exit code, using HEAD/tree of repo at call time.
func seedCustomAudit(t *testing.T, repo, body string, exitCode int) {
	t.Helper()
	auditPath := filepath.Join(repo, ".evolve", "runs", "cycle-1", "audit-report.md")
	mustWrite(t, auditPath, body)
	sha := mustHashFile(t, auditPath)
	headSHA := strings.TrimSpace(runGitOut(t, repo, "rev-parse", "HEAD"))
	treeSHA := treeStateSHA(t, repo)
	entry := fmt.Sprintf(`{"role":"auditor","kind":"agent_subprocess","exit_code":%d,"artifact_path":%q,"artifact_sha256":%q,"git_head":%q,"tree_state_sha":%q}`+"\n",
		exitCode, auditPath, sha, headSHA, treeSHA)
	mustWrite(t, filepath.Join(repo, ".evolve", "ledger.jsonl"), entry)
}

// TestParseVerdicts_BareHeadingLine: the heading form must also accept a BARE
// verdict line (`## Verdict` + `PASS` without bold) — the cycle-249 shape that
// blocked the v16.8.0 release preflight and would equally have produced a
// false AUDIT_BINDING_MALFORMED_VERDICT here. The bare line must be exactly
// the verdict word (a sentence containing PASS must NOT match).
func TestParseVerdicts_BareHeadingLine(t *testing.T) {
	cases := []struct {
		name             string
		body             string
		pass, warn, fail bool
	}{
		{"bare PASS", "## Verdict\nPASS\n\n**Confidence:** 0.97\n", true, false, false},
		{"bare WARN", "## Verdict\nWARN\n", false, true, false},
		{"bare FAIL", "## Verdict\nFAIL\n", false, false, true},
		{"bare PASS with blank line", "## Verdict\n\nPASS\n", true, false, false},
		{"sentence containing PASS not matched", "## Verdict\nAll tests PASS here\n", false, false, false},
		{"bare PASS outside 5-line window", "## Verdict\n\n\n\n\n\nPASS\n", false, false, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			pass, warn, fail := parseVerdicts(tc.body, config.StageOff)
			if pass != tc.pass || warn != tc.warn || fail != tc.fail {
				t.Errorf("parseVerdicts = (pass=%v warn=%v fail=%v), want (pass=%v warn=%v fail=%v)",
					pass, warn, fail, tc.pass, tc.warn, tc.fail)
			}
		})
	}
}
