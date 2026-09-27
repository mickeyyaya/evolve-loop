//go:build integration

package ship

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/acssuite"
	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

func TestVerifyAuditBinding_AuditorExitCode2_IntegrityError(t *testing.T) {
	repo := makeRepo(t)
	seedAudit(t, repo, "PASS", map[string]string{"exit_code": "2"})
	opts := auditOpts(t, repo)
	err := verifyAuditBinding(context.Background(), opts, &RunResult{}) //nolint:staticcheck
	wantShipErr(t, err, core.CodeAuditBindingAuditorExit, core.ShipClassPrecondition, "exited 2")
}

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

func TestVerifyAuditBinding_FailVerdict_IntegrityError(t *testing.T) {
	repo := makeRepo(t)
	seedAudit(t, repo, "FAIL")
	opts := auditOpts(t, repo)
	err := verifyAuditBinding(context.Background(), opts, &RunResult{}) //nolint:staticcheck
	wantShipErr(t, err, core.CodeAuditBindingVerdictFail, core.ShipClassPrecondition, "FAIL")
}

func TestVerifyAuditBinding_WarnWithStrictAudit_IntegrityError(t *testing.T) {
	repo := makeRepo(t)
	// No git changes needed — just need a WARN verdict + matching HEAD/tree.
	seedAudit(t, repo, "WARN")
	writeStrictAuditPolicy(t, repo)
	opts := auditOpts(t, repo)
	err := verifyAuditBinding(context.Background(), opts, &RunResult{}) //nolint:staticcheck
	wantShipErr(t, err, core.CodeAuditBindingVerdictWarn, core.ShipClassPrecondition, "WARN")
}

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

func TestVerifyAuditBinding_GitHEADMismatch_IntegrityError(t *testing.T) {
	repo := makeRepo(t)
	seedAudit(t, repo, "PASS")
	mustWrite(t, filepath.Join(repo, "new.txt"), "post-audit change\n")
	runGit(t, repo, "add", "-A")
	runGit(t, repo, "-c", "commit.gpgsign=false", "commit", "-m", "post-audit commit")

	opts := auditOpts(t, repo)
	err := verifyAuditBinding(context.Background(), opts, &RunResult{}) //nolint:staticcheck
	wantShipErr(t, err, core.CodeAuditBindingHeadMoved, core.ShipClassPrecondition, "git HEAD has moved")
}

func TestVerifyAuditBinding_StaleAudit_IntegrityError(t *testing.T) {
	repo := makeRepo(t)
	seedAudit(t, repo, "PASS")

	auditPath := filepath.Join(repo, ".evolve", "runs", "cycle-1", "audit-report.md")
	old := time.Now().Add(-8 * 24 * time.Hour)
	if err := os.Chtimes(auditPath, old, old); err != nil {
		t.Fatalf("chtimes: %v", err)
	}

	opts := auditOpts(t, repo)
	opts.NowFn = func() Now {
		unix := time.Now().Unix()
		return Now{Unix: unix, RFC3339: time.Unix(unix, 0).UTC().Format(time.RFC3339)}
	}
	err := verifyAuditBinding(context.Background(), opts, &RunResult{}) //nolint:staticcheck
	wantShipErr(t, err, core.CodeAuditBindingStale, core.ShipClassPrecondition, "old")
}

func TestVerifyAuditBinding_ArtifactMissing_IntegrityError(t *testing.T) {
	repo := makeRepo(t)
	seedAudit(t, repo, "PASS")
	auditPath := filepath.Join(repo, ".evolve", "runs", "cycle-1", "audit-report.md")
	if err := os.Remove(auditPath); err != nil {
		t.Fatalf("remove: %v", err)
	}
	opts := auditOpts(t, repo)
	err := verifyAuditBinding(context.Background(), opts, &RunResult{}) //nolint:staticcheck
	wantShipErr(t, err, core.CodeAuditBindingArtifactMissing, core.ShipClassPrecondition, "missing on disk")
}

func TestVerifyAuditBinding_ArtifactSHAMismatch_IntegrityError(t *testing.T) {
	repo := makeRepo(t)
	seedAudit(t, repo, "PASS")
	auditPath := filepath.Join(repo, ".evolve", "runs", "cycle-1", "audit-report.md")
	if err := os.WriteFile(auditPath, []byte("tampered content\n"), 0o644); err != nil {
		t.Fatalf("tamper: %v", err)
	}
	opts := auditOpts(t, repo)
	err := verifyAuditBinding(context.Background(), opts, &RunResult{}) //nolint:staticcheck
	wantShipErr(t, err, core.CodeAuditBindingArtifactSHA, core.ShipClassPrecondition, "SHA mismatch")
}

func TestVerifyAuditBinding_LegacyEntryNoGitHead_IntegrityError(t *testing.T) {
	repo := makeRepo(t)
	auditPath := filepath.Join(repo, ".evolve", "runs", "cycle-1", "audit-report.md")
	body := "<!-- challenge-token: testtoken123 -->\n# Audit Report — Cycle 1\n\nVerdict: PASS\n\nAll criteria met (test fixture).\n"
	mustWrite(t, auditPath, body)
	sha := mustHashFile(t, auditPath)
	entry := fmt.Sprintf(`{"role":"auditor","kind":"agent_subprocess","exit_code":0,"artifact_path":%q,"artifact_sha256":%q}`+"\n",
		auditPath, sha)
	mustWrite(t, filepath.Join(repo, ".evolve", "ledger.jsonl"), entry)

	opts := auditOpts(t, repo)
	opts.RunID = ""                                                     // Exercise legacy binding diagnostics without claiming a modern host run.
	err := verifyAuditBinding(context.Background(), opts, &RunResult{}) //nolint:staticcheck
	wantShipErr(t, err, core.CodeAuditBindingNoLedger, core.ShipClassPrecondition, "predates v8.13.0")
}

func TestCheckEGPSGate_SkipCountWithRedZero_Passes(t *testing.T) {
	repo := t.TempDir()
	verdict, err := json.Marshal(predicateVerdictFixture(1, 1, 0, 4))
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(repo, "acs-verdict.json")
	mustWrite(t, path, string(verdict))
	res := &RunResult{}
	if _, err = checkEGPSGate(path, res); err != nil {
		t.Fatalf("checkEGPSGate returned %v, want nil (red_count==0 with skips must pass)", err)
	}
}

func TestCheckEGPSGate_RedCountWithSkipsPresent_Blocks(t *testing.T) {
	repo := t.TempDir()
	verdict, err := json.Marshal(predicateVerdictFixture(1, 1, 1, 4))
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(repo, "acs-verdict.json")
	mustWrite(t, path, string(verdict))
	res := &RunResult{}
	_, err = checkEGPSGate(path, res)
	wantShipErr(t, err, core.CodeEGPSRedCount, core.ShipClassPrecondition, "RED predicate")
}

// --- helpers ----------------------------------------------------------------

func seedCustomAudit(t *testing.T, repo, body string, exitCode int) {
	t.Helper()
	seedAudit(t, repo, "PASS", map[string]string{"exit_code": fmt.Sprint(exitCode)})
	ledgerPath := filepath.Join(repo, ".evolve", "ledger.jsonl")
	raw, err := os.ReadFile(ledgerPath)
	if err != nil {
		t.Fatal(err)
	}
	var entry map[string]any
	if err := json.Unmarshal(raw, &entry); err != nil {
		t.Fatal(err)
	}
	path := entry["artifact_path"].(string)
	mustWrite(t, path, body)
	sealTestPredicateEvidence(t, acssuite.EvidenceIdentity{Cycle: 1, RunID: "test-run", Round: 1, TreeSHA: entry["worktree_tree_sha"].(string)}, path)
	entry["artifact_sha256"] = mustHashFile(t, path)
	line, _ := json.Marshal(entry)
	mustWrite(t, ledgerPath, string(line)+"\n")
}

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
