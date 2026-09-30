package releasepreflight

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func seedLedgerAndArtifact(t *testing.T, gitHead, verdict string, ts time.Time, worktreeTree string) string {
	t.Helper()
	dir := t.TempDir()
	artifact := filepath.Join(dir, "audit-report.md")
	body := "# Audit Report\n\n<!-- evolve-verdict: {\"phase\":\"audit\",\"verdict\":\"" + verdict +
		"\",\"schema_version\":2} -->\n\n## Verdict\n\n**" + verdict + "**\n"
	if err := os.WriteFile(artifact, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	ledgerPath := filepath.Join(dir, "ledger.jsonl")
	line := `{"ts":"` + ts.UTC().Format(time.RFC3339) + `","cycle":1574,"role":"auditor","kind":"agent_subprocess",` +
		`"exit_code":1,"artifact_path":"` + artifact + `","git_head":"` + gitHead + `"` + worktreeTree + `}` + "\n"
	if err := os.WriteFile(ledgerPath, []byte(line), 0o644); err != nil {
		t.Fatal(err)
	}
	return ledgerPath
}

func TestCheckRecentAudit_ForeignCommitFail_IsAdvisory(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 8, 27, 12, 0, 0, 0, time.UTC)
	ledgerPath := seedLedgerAndArtifact(t, "laneheadsha0000000000000000000000000000", "FAIL", now.Add(-time.Hour), "")

	res, err := checkRecentAudit(ledgerPath, "releasehead111111111111111111111111111", false, now)
	if err != nil {
		t.Fatalf("a FAILed audit of a DIFFERENT commit must not block the release, got: %v", err)
	}
	if res.verdict != auditVerdictScopedOut {
		t.Errorf("verdict = %q, want %q — an audit of a different commit is scoped out, not absent",
			res.verdict, auditVerdictScopedOut)
	}
	if res.auditedHead == "" {
		t.Error("a scoped-out result must retain the audited head so the operator log can name it")
	}
}

func TestCheckRecentAudit_SameCommitFail_StillBlocks(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 8, 27, 12, 0, 0, 0, time.UTC)
	const head = "releasehead111111111111111111111111111"
	ledgerPath := seedLedgerAndArtifact(t, head, "FAIL", now.Add(-time.Hour), "")

	_, err := checkRecentAudit(ledgerPath, head, false, now)
	if err == nil {
		t.Fatal("an audit that bound THIS release commit and rejected it must still block the release")
	}
	if !strings.Contains(err.Error(), "PASS") && !strings.Contains(err.Error(), "FAIL") {
		t.Errorf("the refusal should name the verdict problem, got: %v", err)
	}
}

func TestCheckRecentAudit_PassIsUnaffectedByScoping(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 8, 27, 12, 0, 0, 0, time.UTC)
	ledgerPath := seedLedgerAndArtifact(t, "someotherhead00000000000000000000000000", "PASS", now.Add(-time.Hour), "")

	res, err := checkRecentAudit(ledgerPath, "releasehead111111111111111111111111111", false, now)
	if err != nil {
		t.Fatalf("a recent PASS must still satisfy step 4: %v", err)
	}
	if res.verdict != "PASS" {
		t.Errorf("verdict = %q, want PASS", res.verdict)
	}
}

func TestCheckRecentAudit_UnknownReleaseHeadKeepsBlocking(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 8, 27, 12, 0, 0, 0, time.UTC)
	ledgerPath := seedLedgerAndArtifact(t, "laneheadsha0000000000000000000000000000", "FAIL", now.Add(-time.Hour), "")

	if _, err := checkRecentAudit(ledgerPath, "", false, now); err == nil {
		t.Fatal("an unresolvable release HEAD must keep the conservative block — scoping is not a bypass")
	}
}

func TestCheckRecentAudit_LaneAuditOnSameHeadIsAdvisory(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 8, 27, 12, 0, 0, 0, time.UTC)
	const head = "31ae6518ec5ae2139e466210b35d729d255b0467"
	ledgerPath := seedLedgerAndArtifact(t, head, "FAIL", now.Add(-time.Hour),
		`,"worktree_tree_sha":"c77b7ccf9476aa11223344556677889900aabbcc"`)

	res, err := checkRecentAudit(ledgerPath, head, false, now)
	if err != nil {
		t.Fatalf("a FAILed LANE audit (uncommitted work based on the release commit) must not veto the release, got: %v", err)
	}
	if res.verdict != auditVerdictScopedOut {
		t.Errorf("verdict = %q, want %q", res.verdict, auditVerdictScopedOut)
	}
}

func TestCheckRecentAudit_ReleaseCommitAuditStillBlocks(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 8, 27, 12, 0, 0, 0, time.UTC)
	const head = "31ae6518ec5ae2139e466210b35d729d255b0467"
	ledgerPath := seedLedgerAndArtifact(t, head, "FAIL", now.Add(-time.Hour), "")

	if _, err := checkRecentAudit(ledgerPath, head, false, now); err == nil {
		t.Fatal("an audit of the release commit itself that rejected it must still block")
	}
}
