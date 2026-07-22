package core

// continuation_stamp.go — ADR-0076 slice C, produce side. At the preserve
// decision (finalizeCycle, FAIL verdict) the cycle's dirty worktree is
// snapshot-committed onto its cycle branch and a continuation manifest is
// written into the workspace — IF the carry-forward screen classifies the
// snapshot Clean against main. The inbox mover later stamps released items
// from this manifest, transactionally with the release; the next claim adopts
// the snapshot ref. Everything here is best-effort and LOUD: a salvage failure
// must never fail cycle finalization, and must never be silent.

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/continuation"
	"github.com/mickeyyaya/evolve-loop/go/internal/gitexec"
)

// snapshotIdentity pins a deterministic committer for salvage snapshots so
// preservation never depends on host-level git identity config.
var snapshotIdentity = []string{"-c", "user.name=evolve-loop", "-c", "user.email=evolve@localhost"}

// snapshotPreservedWorktree commits ALL dirty state in worktree (tracked edits
// + untracked files) as one salvage snapshot on the current cycle branch and
// returns its SHA. A clean worktree is idempotent: returns HEAD unchanged.
// The scoped `add -A` is intentional here — a salvage snapshot's job is to
// preserve EVERYTHING the attempt produced in its isolated worktree; the
// declared-manifest staging discipline applies to SHIP binding, not salvage.
func snapshotPreservedWorktree(ctx context.Context, worktree string) (string, error) {
	g := gitexec.Git{Dir: worktree, Exec: gitRunner}
	porcelain, stderr, code, err := g.Capture(ctx, "status", "--porcelain", "-uall")
	if err != nil || code != 0 {
		return "", fmt.Errorf("snapshot: status: rc=%d err=%v: %s", code, err, stderr)
	}
	if strings.TrimSpace(porcelain) != "" {
		if _, stderr, code, err := g.Capture(ctx, "add", "-A"); err != nil || code != 0 {
			return "", fmt.Errorf("snapshot: add: rc=%d err=%v: %s", code, err, stderr)
		}
		args := append(append([]string{}, snapshotIdentity...),
			"commit", "-m", "salvage snapshot (ADR-0076 continuation-on-fail)", "--no-verify")
		if _, stderr, code, err := g.Capture(ctx, args...); err != nil || code != 0 {
			return "", fmt.Errorf("snapshot: commit: rc=%d err=%v: %s", code, err, stderr)
		}
	}
	sha, stderr, code, err := g.Capture(ctx, "rev-parse", "HEAD")
	if err != nil || code != 0 {
		return "", fmt.Errorf("snapshot: rev-parse: rc=%d err=%v: %s", code, err, stderr)
	}
	return strings.TrimSpace(sha), nil
}

// stampContinuationManifest snapshots a preserved worktree and, when the
// carry-forward screen classifies the snapshot Clean against main, writes the
// continuation manifest into the workspace. AlreadyLanded means there is
// nothing to resume; Conflict belongs to the debugger path — neither stamps.
func (o *Orchestrator) stampContinuationManifest(ctx context.Context, cs CycleState, cycle int, projectRoot string) {
	if cs.ActiveWorktree == "" {
		return
	}
	sha, err := snapshotPreservedWorktree(ctx, cs.ActiveWorktree)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[orchestrator] WARN cycle %d continuation: snapshot failed (%v) — preserved work stays dirty-only, no continuation stamped\n", cycle, err)
		return
	}
	verdict, err := ClassifyFleetRebaseCandidate(ctx, cs.ActiveWorktree, sha, "main")
	if err != nil {
		fmt.Fprintf(os.Stderr, "[orchestrator] WARN cycle %d continuation: carry-forward classify failed (%v) — no continuation stamped\n", cycle, err)
		return
	}
	if verdict != FleetRebaseClean {
		fmt.Fprintf(os.Stderr, "[orchestrator] cycle %d continuation: snapshot %s classified verdict=%d — not stamped (only Clean work resumes; conflicts route to the debugger, landed work has nothing to resume)\n", cycle, sha[:12], int(verdict))
		return
	}
	branch, stderr, code, berr := gitexec.Git{Dir: cs.ActiveWorktree, Exec: gitRunner}.Capture(ctx, "symbolic-ref", "--short", "HEAD")
	if berr != nil || code != 0 {
		fmt.Fprintf(os.Stderr, "[orchestrator] WARN cycle %d continuation: branch resolve failed rc=%d (%v: %s) — no continuation stamped\n", cycle, code, berr, stderr)
		return
	}
	m := continuation.Continuation{
		Worktree:     cs.ActiveWorktree,
		Branch:       strings.TrimSpace(branch),
		SnapshotSHA:  sha,
		BaseSHA:      cs.WorktreeBaseSHA,
		FindingsPath: filepath.Join(cs.WorkspacePath, "failure-digest.json"),
		Cycle:        cycle,
	}
	if err := continuation.WriteManifest(cs.WorkspacePath, m); err != nil {
		fmt.Fprintf(os.Stderr, "[orchestrator] WARN cycle %d continuation: %v\n", cycle, err)
		return
	}
	fmt.Fprintf(os.Stderr, "[orchestrator] cycle %d continuation: preserved work snapshot %s (branch %s) stamped for resumption\n", cycle, sha[:12], m.Branch)
}

// validateContinuation re-screens a stamped continuation at adopt time: the
// snapshot must still exist, sit on top of its recorded base, and still merge
// Clean against main (main may have moved since the stamp — landed or
// conflicting work must fall back to a fresh start, not resume).
func validateContinuation(ctx context.Context, projectRoot string, c *continuation.Continuation) error {
	if c == nil || c.SnapshotSHA == "" {
		return fmt.Errorf("continuation: empty binding")
	}
	g := gitexec.Git{Dir: projectRoot, Exec: gitRunner}
	if _, stderr, code, err := g.Capture(ctx, "cat-file", "-e", c.SnapshotSHA+"^{commit}"); err != nil || code != 0 {
		return fmt.Errorf("continuation: snapshot %s not in repo (rc=%d %v %s)", c.SnapshotSHA, code, err, strings.TrimSpace(stderr))
	}
	if c.BaseSHA != "" {
		if _, _, code, err := g.Capture(ctx, "merge-base", "--is-ancestor", c.BaseSHA, c.SnapshotSHA); err != nil || code != 0 {
			return fmt.Errorf("continuation: base %s is not an ancestor of snapshot %s", c.BaseSHA, c.SnapshotSHA)
		}
	}
	verdict, err := ClassifyFleetRebaseCandidate(ctx, projectRoot, c.SnapshotSHA, "main")
	if err != nil {
		return fmt.Errorf("continuation: adopt-time re-screen: %w", err)
	}
	if verdict != FleetRebaseClean {
		return fmt.Errorf("continuation: snapshot %s no longer Clean against main (verdict=%d)", c.SnapshotSHA[:12], int(verdict))
	}
	return nil
}

// maxFindingsBytes caps the prior-attempt findings injected into the build
// prompt — enough for any failure digest, small enough to never flood it.
const maxFindingsBytes = 8 * 1024

// readContinuationFindings loads the prior attempt's findings artifact,
// tolerantly: missing or unreadable is simply "no findings" (the continuation
// still adopts — the work matters more than the notes).
func readContinuationFindings(path string) string {
	if path == "" {
		return ""
	}
	body, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	if len(body) > maxFindingsBytes {
		body = body[:maxFindingsBytes]
	}
	return strings.TrimSpace(string(body))
}

// adoptContinuationAfterTriage is the ADR-0076 slice C adoption seam, invoked
// right after the triage phase completes — the moment this cycle's claims
// exist on disk, so the resolver reads REAL scope (architect finding #1: any
// earlier and processing/cycle-N does not exist yet). On a valid stamped
// claim it RE-SEEDS the cycle worktree from the salvage snapshot (CreateFrom
// force-recreates the same lane-namespaced path, so downstream phases pick it
// up unchanged), moves the review base to the ORIGINAL attempt's base so the
// cumulative work is reviewed and shipped whole, copies the continuation
// manifest into THIS cycle's workspace (ship's manifest reconciliation unions
// the prior attempt's declared paths from it; a re-FAIL overwrites it at the
// next preserve), and serves the prior findings to the build prompt. Every
// failure is a loud fall-back to the already-provisioned fresh worktree.
func (cr *cycleRun) adoptContinuationAfterTriage() {
	if cr.o.continuationFor == nil {
		return
	}
	c := cr.o.continuationFor(cr.req.ProjectRoot, cr.cycle)
	if c == nil {
		return
	}
	if err := validateContinuation(cr.ctx, cr.req.ProjectRoot, c); err != nil {
		fmt.Fprintf(os.Stderr, "[orchestrator] WARN cycle %d continuation from cycle %d rejected (%v) — keeping fresh worktree\n", cr.cycle, c.Cycle, err)
		return
	}
	seeder, ok := cr.o.worktree.(interface {
		CreateFrom(projectRoot string, cycle int, startRef string) (string, error)
	})
	if !ok {
		return
	}
	wt, err := seeder.CreateFrom(cr.req.ProjectRoot, cr.cycle, c.SnapshotSHA)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[orchestrator] WARN cycle %d continuation seeding failed (%v) — keeping fresh worktree\n", cr.cycle, err)
		return
	}
	cr.cs.ActiveWorktree = wt
	if c.BaseSHA != "" {
		cr.cs.WorktreeBaseSHA = c.BaseSHA
	}
	if err := cr.o.storage.WriteCycleState(cr.ctx, cr.cs); err != nil {
		fmt.Fprintf(os.Stderr, "[orchestrator] WARN cycle %d continuation: cycle-state persist after adoption: %v\n", cr.cycle, err)
	}
	if err := continuation.WriteManifest(cr.cs.WorkspacePath, *c); err != nil {
		fmt.Fprintf(os.Stderr, "[orchestrator] WARN cycle %d continuation: workspace manifest copy: %v\n", cr.cycle, err)
	}
	if findings := readContinuationFindings(c.FindingsPath); findings != "" {
		cr.ctxSnap["continuation_findings"] = findings
	}
	fmt.Fprintf(os.Stderr, "[orchestrator] cycle %d ADOPTED continuation: worktree re-seeded from cycle-%d snapshot %s (base %s)\n", cr.cycle, c.Cycle, c.SnapshotSHA[:12], c.BaseSHA)
}
