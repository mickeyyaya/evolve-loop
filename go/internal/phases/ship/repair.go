package ship

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

// repairOutcome is the dispatcher's verdict on a single repair attempt.
type repairOutcome int

const (
	// repairNone: no repair applies, or the repair declined/failed — the
	// caller surfaces the ORIGINAL error unchanged.
	repairNone repairOutcome = iota
	// repairRetryStage: the invariant was re-established — the caller re-runs
	// the failed stage exactly once.
	repairRetryStage
	// repairCompleted: the repair completed the ship itself (push-only resume
	// closure) — the caller skips the remaining mutate stages and proceeds to
	// the success path.
	repairCompleted
)

// repairFn is one typed repair. It must be conservative: any doubt → repairNone.
type repairFn func(ctx context.Context, opts *Options, res *RunResult, se *core.ShipError) repairOutcome

// repairFns maps each repairable ShipError code to its single typed repair.
// GIT_PUSH_REJECTED is deliberately absent: its retry runs inline at the push
// site (pushWithRepair in gitops_landing.go) so the post-push tree
// verification and ship-binding sidecar still execute on the healed path.
var repairFns = map[core.ShipErrorCode]repairFn{
	core.CodeSelfSHATampered:       repairSelfSHAPin,
	core.CodeGitFFMergeDiverged:    repairColliders,
	core.CodeAuditBindingHeadMoved: repairResumeUnpushed,
}

// ensureRepairMap lazily initializes opts.repairAttempted.
func ensureRepairMap(opts *Options) {
	if opts.repairAttempted == nil {
		opts.repairAttempted = map[core.ShipErrorCode]bool{}
	}
}

// attemptRepair is the single dispatch point of the ladder. It enforces the
// once-per-code guard, records observability fields, and marks declined
// attempts on the original error's Debug map.
func attemptRepair(ctx context.Context, opts *Options, res *RunResult, err error) repairOutcome {
	if opts.DryRun {
		return repairNone // repairs mutate; dry-run reports the raw failure
	}
	se, ok := core.AsShipError(err)
	if !ok {
		return repairNone
	}
	fn := repairFns[se.Code]
	if fn == nil {
		return repairNone
	}
	ensureRepairMap(opts)
	if opts.repairAttempted[se.Code] {
		return repairNone // once per code per Run — no in-process loop
	}
	opts.repairAttempted[se.Code] = true
	res.RepairAttempted = string(se.Code)
	res.Logs = append(res.Logs, fmt.Sprintf("[ship] REPAIR: attempting typed repair for %s", se.Code))

	out := fn(ctx, opts, res, se)
	if out == repairNone {
		res.RepairOutcome = "declined"
		se.Debug["repair_attempted"] = string(se.Code)
		se.Debug["repair_outcome"] = "declined"
		res.Logs = append(res.Logs, fmt.Sprintf("[ship] REPAIR: %s declined — original error stands", se.Code))
	}
	return out
}

// runStageWithRepair runs a ship stage; on failure it consults the ladder and
// either re-runs the stage once (invariant re-established), reports the ship
// as completed (push-only closure), or returns the original error.
func runStageWithRepair(ctx context.Context, opts *Options, res *RunResult, stage func() error) (completed bool, err error) {
	err = stage()
	if err == nil {
		return false, nil
	}
	switch attemptRepair(ctx, opts, res, err) {
	case repairRetryStage:
		return false, stage()
	case repairCompleted:
		return true, nil
	default:
		return false, err
	}
}

// --- mode #1: SELF_SHA_TAMPERED from a stale TOFU pin ----------------------

// repairSelfSHAPin heals a stale TOFU pin when the running binary's SHA
// still matches the blob committed at HEAD; any divergence keeps the
// integrity block (the same trust boundary repinPostCycle uses).
func repairSelfSHAPin(ctx context.Context, opts *Options, res *RunResult, _ *core.ShipError) repairOutcome {
	binPath := opts.ShipBinaryPath
	if binPath == "" {
		var err error
		if binPath, err = os.Executable(); err != nil {
			return repairNone
		}
	}
	actualSHA, err := sha256File(binPath)
	if err != nil {
		return repairNone
	}
	relBin, err := filepath.Rel(opts.ProjectRoot, binPath)
	if err != nil || strings.HasPrefix(relBin, "..") {
		return repairNone
	}
	committedSHA := committedBinSHA(ctx, opts, filepath.ToSlash(relBin))
	if committedSHA == "" || committedSHA != actualSHA {
		return repairNone // binary diverges from committed source — real tampering posture
	}

	statePath := filepath.Join(opts.ProjectRoot, ".evolve", "state.json")
	pluginVer := pluginVersion(opts.PluginRoot)
	// See ADR-0049.
	// Serializes the read-modify-write under the shared state.json lock;
	// any lock/read/write error declines the repair.
	if err := withStateLock(statePath, func() error {
		stateMap, err := readStateMap(statePath)
		if err != nil {
			return err
		}
		stateMap["expected_ship_sha"] = actualSHA
		stateMap["expected_ship_version"] = pluginVer
		return writeStateMap(statePath, stateMap)
	}); err != nil {
		return repairNone
	}
	res.RepairOutcome = "repinned-verified-rebuild"
	res.Logs = append(res.Logs, fmt.Sprintf(
		"[ship] REPAIR: stale TOFU pin healed — running binary matches HEAD:%s (verified rebuild of committed source); re-pinned", filepath.ToSlash(relBin)))
	return repairRetryStage
}

// committedBinSHA returns sha256 of the blob at HEAD:<relBin>, or "" when the
// path is not a blob at HEAD (or git fails). Shared with repinPostCycle.
func committedBinSHA(ctx context.Context, opts *Options, relBin string) string {
	var buf strings.Builder
	exitCode, err := opts.run(ctx, "git", []string{"show", "HEAD:" + relBin}, &buf, io.Discard)
	if err != nil || exitCode != 0 {
		return ""
	}
	h := sha256.New()
	_, _ = h.Write([]byte(buf.String()))
	return hex.EncodeToString(h.Sum(nil))
}

// --- mode #3: GIT_FF_MERGE_DIVERGED untracked colliders ---------------------

// repairColliders heals untracked main-side files blocking the worktree
// ff-merge: byte-identical copies are removed, differing copies are
// quarantine-moved (content is never deleted), and the atomic-ship stage is
// re-run so it re-detects colliders from scratch rather than trusting this
// repair's own plan.
func repairColliders(ctx context.Context, opts *Options, res *RunResult, se *core.ShipError) repairOutcome {
	if se.Debug["colliders"] == "" {
		return repairNone // the real-divergence variant of GIT_FF_MERGE_DIVERGED — not repairable here
	}
	worktree := readActiveWorktree(opts)
	if worktree == "" {
		return repairNone
	}
	branch, err := currentBranch(ctx, opts)
	if err != nil || branch == "" {
		return repairNone
	}
	cycleBranch, err := captureGitOutputAtDir(ctx, opts, worktree, "symbolic-ref", "--short", "HEAD")
	if err != nil {
		return repairNone
	}
	colliders, err := detectColliders(ctx, opts, worktree, branch, strings.TrimSpace(cycleBranch))
	if err != nil || len(colliders) == 0 {
		return repairNone
	}

	csMap, err := readStateMap(opts.cycleStateFile()) // See ADR-0049: run-scoped (cycle_id)
	if err != nil {
		return repairNone
	}
	cid, ok := stateInt(csMap, "cycle_id")
	if !ok {
		return repairNone
	}
	qDir := filepath.Join(opts.ProjectRoot, ".evolve", "quarantine", fmt.Sprintf("cycle-%d", cid))

	// Plan-then-execute: every collider is verified readable/comparable before
	// any mutation, and a partial heal never re-runs the stage — the original
	// error stands so no merge can land over a half-healed tree.
	type colliderAction struct {
		path      string
		identical bool
	}
	plan := make([]colliderAction, 0, len(colliders))
	for _, p := range colliders {
		identical, cmpErr := filesIdentical(filepath.Join(opts.ProjectRoot, p), filepath.Join(worktree, p))
		if cmpErr != nil {
			return repairNone // can't prove safety for this collider — decline with zero mutations
		}
		plan = append(plan, colliderAction{path: p, identical: identical})
	}

	var removed, quarantined []string
	execFail := func(p string, opErr error) repairOutcome {
		res.Logs = append(res.Logs, fmt.Sprintf(
			"[ship] WARN: collider repair aborted at %s (%v) AFTER completing: removed-identical=[%s] quarantined=[%s] (quarantine dir %s) — original error stands, no merge attempted",
			p, opErr, strings.Join(removed, ","), strings.Join(quarantined, ","), qDir))
		return repairNone
	}
	for _, a := range plan {
		mainPath := filepath.Join(opts.ProjectRoot, a.path)
		if a.identical {
			if rmErr := os.Remove(mainPath); rmErr != nil {
				return execFail(a.path, rmErr)
			}
			removed = append(removed, a.path)
			continue
		}
		qPath := filepath.Join(qDir, a.path)
		if mkErr := os.MkdirAll(filepath.Dir(qPath), 0o755); mkErr != nil {
			return execFail(a.path, mkErr)
		}
		if mvErr := os.Rename(mainPath, qPath); mvErr != nil {
			return execFail(a.path, mvErr)
		}
		quarantined = append(quarantined, a.path)
	}
	if len(quarantined) > 0 {
		if mErr := appendQuarantineManifest(opts, qDir, cid, quarantined); mErr != nil {
			res.Logs = append(res.Logs, "[ship] WARN: quarantine manifest write failed: "+mErr.Error())
		}
	}
	res.RepairOutcome = fmt.Sprintf("colliders-healed:%d-identical-removed,%d-quarantined", len(removed), len(quarantined))
	res.Logs = append(res.Logs, fmt.Sprintf(
		"[ship] REPAIR: collider heal — %d byte-identical removed (%s), %d differing quarantined to %s (%s)",
		len(removed), strings.Join(removed, ","), len(quarantined), qDir, strings.Join(quarantined, ",")))
	return repairRetryStage
}

// filesIdentical reports byte equality of two files.
func filesIdentical(a, b string) (bool, error) {
	ba, err := os.ReadFile(a)
	if err != nil {
		return false, err
	}
	bb, err := os.ReadFile(b)
	if err != nil {
		return false, err
	}
	return string(ba) == string(bb), nil
}

// appendQuarantineManifest records quarantined paths in
// <qDir>/manifest.json (append-merge, atomic write).
func appendQuarantineManifest(opts *Options, qDir string, cycle int, paths []string) error {
	type entry struct {
		Path   string `json:"path"`
		Reason string `json:"reason"`
		Cycle  int    `json:"cycle"`
		TS     string `json:"ts"`
	}
	manifestPath := filepath.Join(qDir, "manifest.json")
	var entries []entry
	if raw, err := os.ReadFile(manifestPath); err == nil {
		_ = json.Unmarshal(raw, &entries) // malformed existing manifest → start fresh
	}
	nowFn := opts.NowFn
	if nowFn == nil {
		nowFn = defaultNow // Run() defaults this, but direct callers may not
	}
	ts := nowFn().RFC3339
	for _, p := range paths {
		entries = append(entries, entry{Path: p, Reason: "differing-untracked-collider", Cycle: cycle, TS: ts})
	}
	buf, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return err
	}
	tmp := manifestPath + ".tmp"
	if err := os.WriteFile(tmp, append(buf, '\n'), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, manifestPath)
}

// --- mode #2: AUDIT_BINDING_HEAD_MOVED resume-unpushed closure --------------

// repairResumeUnpushed heals a ship that died after its own commit/merge
// moved HEAD but before the push. When (a) HEAD's tree equals the
// audit-bound tree, (b) the audited base is an ancestor of HEAD, and
// (c) origin/<branch> is strictly behind HEAD on the same history, the
// audited work is already committed and merely unpushed — complete with a
// push-only closure. Anything else declines to the re-audit route.
func repairResumeUnpushed(ctx context.Context, opts *Options, res *RunResult, se *core.ShipError) repairOutcome {
	if opts.Class != ClassCycle {
		return repairNone
	}
	bound := opts.internalAuditBoundTreeSHA
	if bound == "" {
		return repairNone
	}
	headTree, err := captureGitOutput(ctx, opts, "rev-parse", "HEAD^{tree}")
	if err != nil {
		return repairNone
	}
	headTree = strings.TrimSpace(headTree)
	if headTree != bound {
		// A consumed-item PASS ship's HEAD tree legitimately differs from the
		// bound tree by the sanctioned consumption delta, but this rung runs in
		// a fresh process where the explained-by-consumption check cannot run —
		// decline to the slower re-audit path, deliberately fail-safe.
		return repairNone // HEAD is not the audited work
	}
	auditedHead := se.Debug["audited"]
	if auditedHead == "" || !isAncestor(ctx, opts, auditedHead, "HEAD") {
		return repairNone
	}
	branch, err := currentBranch(ctx, opts)
	if err != nil || branch == "" {
		return repairNone
	}
	// Refresh the remote ref; a fetch failure falls back to the local ref.
	_, _ = opts.run(ctx, "git", []string{"fetch", "origin", branch}, io.Discard, io.Discard)
	originRef, err := captureGitOutput(ctx, opts, "rev-parse", "origin/"+branch)
	if err != nil {
		return repairNone
	}
	originRef = strings.TrimSpace(originRef)
	head, err := captureGitOutput(ctx, opts, "rev-parse", "HEAD")
	if err != nil {
		return repairNone
	}
	head = strings.TrimSpace(head)

	if originRef != head {
		if !isAncestor(ctx, opts, originRef, "HEAD") {
			return repairNone // origin diverged — never rebase, never force-push
		}
		exit, pushErr := opts.run(ctx, "git", []string{"push", "origin", branch}, opts.Stdout, opts.Stderr)
		if pushErr != nil || exit != 0 {
			return repairNone
		}
	}
	res.CommitSHA = head
	if bindErr := writeShipBinding(opts, headTree, head); bindErr != nil {
		// Without the sidecar a re-dispatch cannot recognize the idempotent
		// state (the once-guard blocks a second resume); the push itself
		// still succeeded.
		res.Logs = append(res.Logs, "[ship] WARN: could not write ship-binding.json on resume ("+bindErr.Error()+
			") — a re-dispatch will NOT be idempotent; run `evolve cycle reset` if this cycle is re-dispatched")
	}
	res.RepairOutcome = "resume-pushed"
	res.Logs = append(res.Logs, fmt.Sprintf(
		"[ship] REPAIR: resume-unpushed — HEAD %s carries the audit-bound tree %s; completed with push-only closure", head, bound))
	return repairCompleted
}
