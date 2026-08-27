package releasepreflight

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// recent_audit_scope_test.go — cycle-1571 H4, a live consequence of PR #503.
//
// checkRecentAudit takes the NEWEST auditor entry with an on-disk artifact —
// no run, cycle, or git_head filter — and a non-acceptable verdict aborts the
// release. Before #503 a FAILed cycle wrote no auditor entry at all, so that
// branch was unreachable; #503 made FAIL entries exist, so the newest entry is
// now routinely a FAILed LANE cycle that has nothing to do with the commit
// being released. Verified live: the runtime ledger's newest auditor entry was
// cycle-1574, exit 1, artifact on disk, marker verdict FAIL — i.e. `evolve
// release` was blocked by an unrelated lane failure.
//
// The gate was already inconsistent about this: NO audit is advisory-skipped
// because "CI-green on the release commit is the authoritative gate", while a
// FAILED audit of unrelated work hard-blocked. Scope the veto to the commit
// actually being released: an audit that bound a different HEAD gets no vote;
// an audit that bound THIS HEAD and rejected it still blocks.

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

// TestCheckRecentAudit_ForeignCommitFail_IsAdvisory is the pin for the live
// block: a FAILed audit that bound a DIFFERENT commit must not veto the
// release.
func TestCheckRecentAudit_ForeignCommitFail_IsAdvisory(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 8, 27, 12, 0, 0, 0, time.UTC)
	ledgerPath := seedLedgerAndArtifact(t, "laneheadsha0000000000000000000000000000", "FAIL", now.Add(-time.Hour), "")

	res, err := checkRecentAudit(ledgerPath, "releasehead111111111111111111111111111", false, now)
	if err != nil {
		t.Fatalf("a FAILed audit of a DIFFERENT commit must not block the release, got: %v", err)
	}
	// SCOPED_OUT, deliberately NOT auditVerdictNone: the audit exists and did
	// not pass, it simply did not examine this release commit. Collapsing the
	// two would make the operator log claim "no on-disk audit" about an audit
	// that is present and failed — misdirection precisely when someone is
	// debugging a blocked release.
	if res.verdict != auditVerdictScopedOut {
		t.Errorf("verdict = %q, want %q — an audit of a different commit is scoped out, not absent",
			res.verdict, auditVerdictScopedOut)
	}
	if res.auditedHead == "" {
		t.Error("a scoped-out result must retain the audited head so the operator log can name it")
	}
}

// TestCheckRecentAudit_SameCommitFail_StillBlocks: the true block must survive.
// This is the assertion that stops the fix from degenerating into "ignore all
// FAILs".
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

// TestCheckRecentAudit_PassIsUnaffectedByScoping: a PASS still satisfies the
// step regardless of which commit it bound — scoping the VETO must not
// accidentally scope the blessing and turn every release into an advisory skip.
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

// TestCheckRecentAudit_UnknownReleaseHeadKeepsBlocking: if the release HEAD
// cannot be resolved we cannot prove the failing audit is unrelated, so the
// conservative branch is taken. Scoping must never become a bypass.
func TestCheckRecentAudit_UnknownReleaseHeadKeepsBlocking(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 8, 27, 12, 0, 0, 0, time.UTC)
	ledgerPath := seedLedgerAndArtifact(t, "laneheadsha0000000000000000000000000000", "FAIL", now.Add(-time.Hour), "")

	if _, err := checkRecentAudit(ledgerPath, "", false, now); err == nil {
		t.Fatal("an unresolvable release HEAD must keep the conservative block — scoping is not a bypass")
	}
}

// TestCheckRecentAudit_LaneAuditOnSameHeadIsAdvisory is the REAL live shape, and
// it falsifies this fix's first premise. A cycle audit's git_head is the PROJECT
// ROOT's HEAD (phase_bindings.go resolves it with `rev-parse HEAD` against
// projectRoot), i.e. main's tip — NOT a lane worktree head. Verified in the
// runtime ledger: cycles 1572, 1573 and 1574 all carry git_head 31ae6518, each
// with a DISTINCT worktree_tree_sha.
//
// So head equality alone cannot separate "audited the release commit" from
// "audited a lane's uncommitted work that happened to be based on it", and
// scoping on head alone would still have vetoed the release that motivated this
// change. The second discriminator is worktree_tree_sha — a WRITER marker, not
// a delta marker: the orchestrator's recorder runs `git add -A; git write-tree`,
// which yields a tree even for a clean worktree, so every cycle audit records
// one, while the manual `evolve subagent run auditor` writer never emits it.
// That is the cut wanted: a manual audit of the released commit keeps its veto.
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

// TestCheckRecentAudit_ReleaseCommitAuditStillBlocks: an audit with NO worktree
// delta examined the committed tree itself, so its rejection is about the thing
// being released and must still block.
func TestCheckRecentAudit_ReleaseCommitAuditStillBlocks(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 8, 27, 12, 0, 0, 0, time.UTC)
	const head = "31ae6518ec5ae2139e466210b35d729d255b0467"
	ledgerPath := seedLedgerAndArtifact(t, head, "FAIL", now.Add(-time.Hour), "")

	if _, err := checkRecentAudit(ledgerPath, head, false, now); err == nil {
		t.Fatal("an audit of the release commit itself that rejected it must still block")
	}
}
