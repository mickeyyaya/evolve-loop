package core

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/continuation"
	"github.com/mickeyyaya/evolve-loop/go/internal/gitexec"
)

var snapshotIdentity = []string{"-c", "user.name=evolve-loop", "-c", "user.email=evolve@localhost"}

func snapshotPreservedWorktree(ctx context.Context, projectRoot, worktree string) (string, error) {
	if inPlaceWorktree(worktree, projectRoot) {
		return "", errors.New("the active worktree is the project root — no salvage snapshot (never commit the operator's tree)")
	}
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
			"commit", "-m", salvageSnapshotSubject, "--no-verify")
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

func (o *Orchestrator) stampContinuationManifest(ctx context.Context, cs CycleState, cycle int, projectRoot string) {
	if cs.ActiveWorktree == "" {
		return
	}
	sha, err := snapshotPreservedWorktree(ctx, projectRoot, cs.ActiveWorktree)
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
		FindingsPath: filepath.Join(cs.WorkspacePath, "audit-fail-reason.json"),
		Cycle:        cycle,
	}
	if err := continuation.WriteManifest(cs.WorkspacePath, m); err != nil {
		fmt.Fprintf(os.Stderr, "[orchestrator] WARN cycle %d continuation: %v\n", cycle, err)
		return
	}
	registerLaneScopeBinding(cs.WorkspacePath, projectRoot, m, cycle)
	fmt.Fprintf(os.Stderr, "[orchestrator] cycle %d continuation: preserved work snapshot %s (branch %s) stamped for resumption\n", cycle, sha[:12], m.Branch)
}

func registerLaneScopeBinding(workspace, projectRoot string, m continuation.Continuation, cycle int) {
	ls := loadLaneScope(workspace)
	if ls == nil {
		return
	}
	for _, id := range ls.TodoIDs {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		if err := continuation.WriteRegistryEntry(projectRoot, id, m); err != nil {
			fmt.Fprintf(os.Stderr, "[orchestrator] WARN cycle %d continuation: lane-scope binding for %q not registered: %v\n", cycle, id, err)
			continue
		}
		fmt.Fprintf(os.Stderr, "[orchestrator] cycle %d continuation: registered lane-scope binding %q → %s\n", cycle, id, m.SnapshotSHA[:12])
	}
}

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

const maxFindingsBytes = 8 * 1024

func readContinuationFindings(path string, warn io.Writer) string {
	if path == "" {
		return ""
	}
	body, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintf(warn, "[orchestrator] WARN continuation: findings artifact %s unreadable (%v) — builder gets no prior-attempt findings\n", path, err)
		return ""
	}
	var a auditFailReason
	if json.Unmarshal(body, &a) == nil && len(a.Reasons) > 0 {
		return renderFailReasons(a.Phase, a.Reasons)
	}
	return truncateFindings(strings.TrimSpace(string(body)))
}

const CtxKeyContinuationRepairFirst = "continuation_repair_first"

func continuationRepairFirst(c *continuation.Continuation, worktree string, warn io.Writer) string {
	if c.FindingsPath == "" {
		return ""
	}
	directive, err := continuation.RepairFirst(filepath.Dir(c.FindingsPath), worktree)
	if err != nil {
		fmt.Fprintf(warn, "[orchestrator] WARN continuation: no repair-first directive from %s (%v) — builder gets the descriptive findings only\n", filepath.Dir(c.FindingsPath), err)
	}
	return directive
}

func renderFailReasons(phase string, reasons []string) string {
	return truncateFindings("failed phase: " + phase + "\n- " + strings.Join(reasons, "\n- "))
}

func truncateFindings(s string) string {
	if len(s) <= maxFindingsBytes {
		return s
	}
	cut := lastLineBreakWithin(s, maxFindingsBytes)
	return s[:cut] + fmt.Sprintf("\n…[truncated %d bytes]", len(s)-cut)
}

func lastLineBreakWithin(s string, budget int) int {
	if i := strings.LastIndexByte(s[:budget], '\n'); i > 0 {
		return i
	}
	return budget
}

func (cr *cycleRun) adoptContinuationAfterTriage() error {
	if cr.o.continuationFor == nil {
		return nil
	}
	var scopeIDs []string
	if ls := loadLaneScope(cr.cs.WorkspacePath); ls != nil {
		scopeIDs = ls.TodoIDs
	}
	c := cr.o.continuationFor(cr.req.ProjectRoot, cr.cycle, scopeIDs)
	if c == nil {
		return nil
	}
	if err := validateContinuation(cr.ctx, cr.req.ProjectRoot, c); err != nil {
		fmt.Fprintf(os.Stderr, "[orchestrator] WARN cycle %d continuation from cycle %d rejected (%v) — keeping fresh worktree\n", cr.cycle, c.Cycle, err)
		releaseDeclinedBinding(cr.req.ProjectRoot, scopeIDs, c)
		cr.o.shedDeclinedPointer(cr.req.ProjectRoot, cr.cycle, *c)
		return nil
	}
	seeder, ok := cr.o.worktree.(interface {
		CreateFrom(projectRoot string, cycle int, startRef string) (string, error)
	})
	if !ok {
		return nil
	}
	wt, err := seeder.CreateFrom(cr.req.ProjectRoot, cr.cycle, c.SnapshotSHA)
	if err != nil {
		return fmt.Errorf("continuation seeding from cycle %d: %w", c.Cycle, err)
	}

	cr.cs.ActiveWorktree = wt
	cr.cs.WorktreeBaseSHA = c.BaseSHA
	base, err := advanceContinuationBase(cr.ctx, wt, cr.cycle)
	if err != nil {
		return fmt.Errorf("continuation from cycle %d: %w", c.Cycle, err)
	}
	cr.cs.WorktreeBaseSHA = base
	if err := cr.prepareAdoptedTree(wt, base); err != nil {
		return err
	}
	if err := cr.o.storage.WriteCycleState(cr.ctx, cr.cs); err != nil {
		return fmt.Errorf("continuation cycle-state persist after adoption: %w", err)
	}
	if err := continuation.WriteManifest(cr.cs.WorkspacePath, *c); err != nil {
		return fmt.Errorf("continuation workspace manifest copy: %w", err)
	}
	SeedDispositionSkeleton(cr.cs.WorkspacePath, cr.req.ProjectRoot, c.Cycle)
	if findings := readContinuationFindings(c.FindingsPath, cr.o.errOut()); findings != "" {
		cr.ctxSnap["continuation_findings"] = findings
	}
	if first := continuationRepairFirst(c, wt, cr.o.errOut()); first != "" {
		cr.ctxSnap[CtxKeyContinuationRepairFirst] = first
	}
	fmt.Fprintf(os.Stderr, "[orchestrator] cycle %d ADOPTED continuation: worktree re-seeded from cycle-%d snapshot %s (base %s)\n", cr.cycle, c.Cycle, c.SnapshotSHA[:12], base)
	return nil
}

func (o *Orchestrator) shedDeclinedPointer(projectRoot string, cycle int, declined continuation.Continuation) {
	if o.continuationShed == nil {
		return
	}
	shed, err := o.continuationShed(projectRoot, cycle, declined)
	if err != nil {
		fmt.Fprintf(o.errOut(), "[orchestrator] WARN cycle %d continuation: the declined pointer of cycle %d was not shed: %v\n", cycle, declined.Cycle, err)
	}
	for _, id := range shed {
		fmt.Fprintf(o.errOut(), "[orchestrator] cycle %d continuation: SHED the declined continuation pointer of %q (snapshot %s, cycle %d)\n", cycle, id, declined.SnapshotSHA, declined.Cycle)
	}
}

func releaseDeclinedBinding(projectRoot string, scopeIDs []string, declined *continuation.Continuation) {
	if declined == nil {
		return
	}
	for _, id := range scopeIDs {
		if id == "" {
			continue
		}
		released, err := continuation.DeleteRegistryEntryIfCycle(projectRoot, id, declined.Cycle)
		if err != nil {
			fmt.Fprintf(os.Stderr, "[orchestrator] WARN continuation: releasing declined binding %q (cycle-%d) failed: %v\n", id, declined.Cycle, err)
			continue
		}
		if released {
			fmt.Fprintf(os.Stderr, "[orchestrator] continuation: RELEASED stale binding %q -> cycle-%d (adoption declined; scope restarts fresh instead of auto-FAILing the ledger gate)\n", id, declined.Cycle)
		}
	}
}
