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
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/ship/landing"
)

var marshalIndentJSON = json.MarshalIndent

type repairOutcome int

const (
	repairNone repairOutcome = iota
	repairRetryStage
	repairCompleted
)

type repairFn func(ctx context.Context, opts *Options, res *RunResult, se *core.ShipError) repairOutcome

var repairFns = map[core.ShipErrorCode]repairFn{
	core.CodeSelfSHATampered:       repairSelfSHAPin,
	core.CodeGitFFMergeDiverged:    repairColliders,
	core.CodeAuditBindingHeadMoved: repairResumeUnpushed,
}

func ensureRepairMap(opts *Options) {
	if opts.repairAttempted == nil {
		opts.repairAttempted = map[core.ShipErrorCode]bool{}
	}
}

func attemptRepair(ctx context.Context, opts *Options, res *RunResult, err error) repairOutcome {
	if opts.DryRun {
		return repairNone
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
		return repairNone
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

func repairSelfSHAPin(ctx context.Context, opts *Options, res *RunResult, _ *core.ShipError) repairOutcome {
	binPath := opts.ShipBinaryPath
	if binPath == "" {
		var err error
		if binPath, err = osExecutable(); err != nil {
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
		return repairNone
	}

	statePath := filepath.Join(opts.ProjectRoot, ".evolve", "state.json")
	pluginVer := pluginVersion(opts.PluginRoot)
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

func repairColliders(ctx context.Context, opts *Options, res *RunResult, se *core.ShipError) repairOutcome {
	if se.Debug["colliders"] == "" {
		return repairNone
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

	csMap, err := readStateMap(opts.cycleStateFile())
	if err != nil {
		return repairNone
	}
	cid, ok := stateInt(csMap, "cycle_id")
	if !ok {
		return repairNone
	}
	qDir := filepath.Join(opts.ProjectRoot, ".evolve", "quarantine", fmt.Sprintf("cycle-%d", cid))

	type colliderAction struct {
		path      string
		identical bool
	}
	plan := make([]colliderAction, 0, len(colliders))
	for _, p := range colliders {
		identical, cmpErr := filesIdentical(filepath.Join(opts.ProjectRoot, p), filepath.Join(worktree, p))
		if cmpErr != nil {
			return repairNone
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
		_ = json.Unmarshal(raw, &entries)
	}
	nowFn := opts.NowFn
	if nowFn == nil {
		nowFn = defaultNow
	}
	ts := nowFn().RFC3339
	for _, p := range paths {
		entries = append(entries, entry{Path: p, Reason: "differing-untracked-collider", Cycle: cycle, TS: ts})
	}
	buf, err := marshalIndentJSON(entries, "", "  ")
	if err != nil {
		return err
	}
	tmp := manifestPath + ".tmp"
	if err := os.WriteFile(tmp, append(buf, '\n'), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, manifestPath)
}

func repairResumeUnpushed(ctx context.Context, opts *Options, res *RunResult, se *core.ShipError) repairOutcome {
	in, ok := strandedLanding(ctx, opts, se)
	if !ok {
		return repairNone
	}
	verdict := opts.landing().Admit(ctx, in)
	if verdict.Declined != "" {
		res.Logs = append(res.Logs, "[ship] REPAIR: resume-unpushed declined: "+verdict.Declined)
		return repairNone
	}
	if !verdict.OnOrigin {
		if err := pushLanding(ctx, opts, res, in); err != nil {
			return repairNone
		}
	}
	if err := settleLanding(ctx, opts, res, in); err != nil {
		res.Logs = append(res.Logs, "[ship] WARN: resume-unpushed landed "+in.CommitSHA+" but cannot settle it: "+err.Error())
		return repairNone
	}
	res.RepairOutcome = "resume-pushed"
	res.Logs = append(res.Logs, fmt.Sprintf(
		"[ship] REPAIR: resume-unpushed — HEAD %s carries the audit-bound tree %s; the landing's ancestry checks admitted it", in.CommitSHA, in.AuditedTree))
	return repairCompleted
}

func strandedLanding(ctx context.Context, opts *Options, se *core.ShipError) (landing.Intent, bool) {
	if opts.Class != ClassCycle || opts.internalAuditBoundTreeSHA == "" {
		return landing.Intent{}, false
	}
	head, err := captureGitOutput(ctx, opts, "rev-parse", "HEAD")
	if err != nil {
		return landing.Intent{}, false
	}
	headTree, err := captureGitOutput(ctx, opts, "rev-parse", "HEAD^{tree}")
	if err != nil {
		return landing.Intent{}, false
	}
	head, headTree = strings.TrimSpace(head), strings.TrimSpace(headTree)
	auditedHead := se.Debug["audited"]
	if ok, _ := auditBindingSatisfied(ctx, opts, "", headTree); !ok || auditedHead == "" || !isAncestor(ctx, opts, auditedHead, "HEAD") {
		return landing.Intent{}, false
	}
	branch, err := currentBranch(ctx, opts)
	cid, hasCycle, cerr := cycleIDForShip(opts)
	if err != nil || branch == "" || cerr != nil || !hasCycle {
		return landing.Intent{}, false
	}
	return landing.Intent{Cycle: cid, RunID: opts.RunID, AuditArtifactSHA256: opts.internalAuditArtifactSHA,
		AuditedTree: opts.internalAuditBoundTreeSHA, WorktreeBaseSHA: auditedHead, CommitSHA: head, CommitTree: headTree,
		PreMain: head, Branch: branch, Status: landing.IntentPrepared}, true
}
