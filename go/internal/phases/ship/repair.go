// repair.go — the self-healing ship repair ladder (ADR-0039 §8).
//
// Ship is a pure executor: after audit PASS its job is to find the LEGITIMATE
// way to land the audited tree. Historically every verification failure was
// binary — it killed the cycle even when audited-PASS work sat ready to push
// (cycles 230, 246-248). The repair ladder gives each ShipError code at most
// ONE typed, provably-safe repair attempt per Run, then escalates through the
// existing orchestrator/router recovery machinery.
//
// Invariants (operator-approved 2026-06-07):
//   - Bounded: a given code is repaired at most once per Run (opts.repairAttempted);
//     the orchestrator's maxRecoveryDepth bounds the outer loop independently.
//   - Provably safe: every repair re-verifies the original invariant afterwards
//     (the failed stage is re-run, or the closure re-checks the tree binding).
//   - Policy-compliant: never rebase, never force-push, never set bypass env
//     vars, never delete content (differing colliders are quarantine-moved).
//   - Observable: every attempt is logged, recorded on the RunResult
//     (RepairAttempted/RepairOutcome), and stamped into the ShipError Debug
//     map when declined — flowing into ship-error.json and the failure floor.
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
// GIT_PUSH_REJECTED is deliberately absent: the push retry runs inline at the
// push site (repairPushRace) so the post-push tree verification and
// ship-binding sidecar still execute on the healed path.
// Read-only after package init — never mutated at runtime.
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

// repairSelfSHAPin heals the cycles-246-248 stale-pin signature: the running
// ship binary's SHA equals the SHA of the binary blob COMMITTED AT GIT HEAD —
// a legitimate rebuild/manual-ship of audited, committed source whose pin was
// never refreshed. Re-pin and let verifySelfSHA re-run. Any divergence from
// the committed blob keeps the integrity BLOCK (this is the same trust
// boundary repinPostCycle already uses: HEAD is the audited reference).
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
	stateMap, err := readStateMap(statePath)
	if err != nil {
		return repairNone
	}
	pluginVer := pluginVersion(opts.PluginRoot)
	stateMap["expected_ship_sha"] = actualSHA
	stateMap["expected_ship_version"] = pluginVer
	if err := writeStateMap(statePath, stateMap); err != nil {
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
	runner := opts.Runner
	if runner == nil {
		runner = execRunner
	}
	var buf strings.Builder
	exitCode, err := runner(ctx, "git", []string{"show", "HEAD:" + relBin}, os.Environ(), opts.ProjectRoot, nil, &buf, io.Discard)
	if err != nil || exitCode != 0 {
		return ""
	}
	h := sha256.New()
	_, _ = h.Write([]byte(buf.String()))
	return hex.EncodeToString(h.Sum(nil))
}

// --- mode #3: GIT_FF_MERGE_DIVERGED untracked colliders ---------------------

// repairColliders heals the cycle-230 signature: untracked main-side files
// blocking the worktree ff-merge. Byte-identical copies are removed (the same
// bytes arrive via the merge); differing copies are quarantine-moved to
// .evolve/quarantine/cycle-<N>/ with a manifest record — content is never
// deleted. The atomic-ship stage is then re-run, which re-detects colliders
// from scratch (the repair never bypasses the pre-flight).
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

	csMap, err := readStateMap(filepath.Join(opts.ProjectRoot, ".evolve", "cycle-state.json"))
	if err != nil {
		return repairNone
	}
	cid, ok := stateInt(csMap, "cycle_id")
	if !ok {
		return repairNone
	}
	qDir := filepath.Join(opts.ProjectRoot, ".evolve", "quarantine", fmt.Sprintf("cycle-%d", cid))

	// Plan-then-execute: verify EVERY collider is readable/comparable BEFORE
	// mutating anything, so an unreadable file declines the repair with the
	// main tree untouched. An execution failure after that point is loudly
	// logged with the already-completed actions (audit trail for recovery);
	// the stage is NOT re-run on a partial heal (repairNone → the original
	// error stands), so no merge can land over a half-healed tree.
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

// repairResumeUnpushed heals the cycle-246 signature: ship died after its own
// commit/merge moved HEAD but before the push. When (a) HEAD's tree equals
// the audit-bound tree, (b) the audited base is an ancestor of HEAD, and
// (c) origin/<branch> is strictly behind HEAD on the same history, the
// audited work is already committed and merely unpushed — complete with a
// push-only closure. Anything else declines (→ the re-audit route).
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
	_, _ = opts.Runner(ctx, "git", []string{"fetch", "origin", branch}, os.Environ(), opts.ProjectRoot, nil, io.Discard, io.Discard)
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
		exit, pushErr := opts.Runner(ctx, "git", []string{"push", "origin", branch}, os.Environ(), opts.ProjectRoot, nil, opts.Stdout, opts.Stderr)
		if pushErr != nil || exit != 0 {
			return repairNone
		}
	}
	res.CommitSHA = head
	if bindErr := writeShipBinding(opts, headTree, head); bindErr != nil {
		// Without the sidecar a further re-dispatch cannot recognize the
		// idempotent state (the once-guard blocks a second resume) — the
		// operator must `evolve cycle reset`. The push itself succeeded.
		res.Logs = append(res.Logs, "[ship] WARN: could not write ship-binding.json on resume ("+bindErr.Error()+
			") — a re-dispatch will NOT be idempotent; run `evolve cycle reset` if this cycle is re-dispatched")
	}
	res.RepairOutcome = "resume-pushed"
	res.Logs = append(res.Logs, fmt.Sprintf(
		"[ship] REPAIR: resume-unpushed — HEAD %s carries the audit-bound tree %s; completed with push-only closure", head, bound))
	return repairCompleted
}

// isAncestor reports whether anc is an ancestor of desc (git merge-base).
func isAncestor(ctx context.Context, opts *Options, anc, desc string) bool {
	exit, err := opts.Runner(ctx, "git", []string{"merge-base", "--is-ancestor", anc, desc}, os.Environ(), opts.ProjectRoot, nil, io.Discard, io.Discard)
	return err == nil && exit == 0
}

// --- mode #4: GIT_PUSH_REJECTED inline fetch + ff-retry ---------------------

// repairPushRace runs INLINE at the push sites (shipDirect/shipFromWorktree)
// so the healed path still flows through post-push verification and the
// ship-binding sidecar. Returns nil when the push landed (retried or already
// present on origin); otherwise the error to surface — the original transient
// rejection, or a Precondition reclassification (repair_outcome=needs-reaudit)
// when origin diverged and the only legitimate route is a re-audit on the new
// base. Never rebases, never force-pushes.
func repairPushRace(ctx context.Context, opts *Options, res *RunResult, branch string, origErr error) error {
	se, ok := core.AsShipError(origErr)
	if !ok || opts.DryRun {
		return origErr
	}
	ensureRepairMap(opts)
	if opts.repairAttempted[core.CodeGitPushRejected] {
		return origErr
	}
	opts.repairAttempted[core.CodeGitPushRejected] = true
	res.RepairAttempted = string(core.CodeGitPushRejected)
	res.Logs = append(res.Logs, "[ship] REPAIR: push rejected — fetching origin and probing for a fast-forward retry")

	declined := func() error {
		res.RepairOutcome = "declined"
		se.Debug["repair_attempted"] = string(core.CodeGitPushRejected)
		se.Debug["repair_outcome"] = "declined"
		return origErr
	}

	if exit, err := opts.Runner(ctx, "git", []string{"fetch", "origin", branch}, os.Environ(), opts.ProjectRoot, nil, io.Discard, io.Discard); err != nil || exit != 0 {
		return declined()
	}
	originRef, err := captureGitOutput(ctx, opts, "rev-parse", "origin/"+branch)
	if err != nil {
		return declined()
	}
	originRef = strings.TrimSpace(originRef)
	head, err := captureGitOutput(ctx, opts, "rev-parse", "HEAD")
	if err != nil {
		return declined()
	}
	head = strings.TrimSpace(head)

	if originRef == head {
		res.RepairOutcome = "already-pushed"
		res.Logs = append(res.Logs, "[ship] REPAIR: origin already at HEAD — push race resolved itself")
		return nil
	}
	if isAncestor(ctx, opts, originRef, "HEAD") {
		exit, pushErr := opts.Runner(ctx, "git", []string{"push", "origin", branch}, os.Environ(), opts.ProjectRoot, nil, opts.Stdout, opts.Stderr)
		if pushErr == nil && exit == 0 {
			res.RepairOutcome = "push-retried"
			res.Logs = append(res.Logs, "[ship] REPAIR: push retry after fetch succeeded (origin was an ancestor — fast-forward)")
			return nil
		}
		return declined()
	}
	// Origin diverged: a push would need a rebase/merge, which mutates the
	// audited tree. Reclassify so the recovery chain re-audits on the new
	// base — the local commit is preserved for a cheap re-land.
	res.RepairOutcome = "needs-reaudit"
	return shipErr(core.CodeGitPushRejected, core.ShipClassPrecondition, core.StageAtomicShip,
		fmt.Sprintf("ship: push rejected and origin/%s diverged — audited tree must be re-audited on the new base (no auto-rebase; local commit preserved)", branch),
		"branch", branch, "origin_ref", originRef, "head", head,
		"repair_attempted", string(core.CodeGitPushRejected), "repair_outcome", "needs-reaudit")
}
