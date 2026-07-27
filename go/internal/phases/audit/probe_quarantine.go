package audit

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/gitexec"
	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

// probe_quarantine.go — the observer must not perturb the observed system.
//
// The adversarial auditor legitimately authors probe tests to refute a build,
// but writing them INTO the package under test poisons the EGPS run: any
// predicate that shells `go test` over that package inherits the probe's
// engineered failure as a builder regression (cycles 1115/1117 — auditor
// verdict PASS/"Not FAIL" beside a red gate). The probes were then discarded
// with the worktree, so the auditor's own finding was unrecoverable.
//
// New-since-dispatch *_test.go files (the audit-prompt artifact's mtime is
// the dispatch anchor — engine.go writes it at dispatch; builder files
// predate it) are PRESERVED under <workspace>/audit-probes/ and removed from
// the tree, one loud log line each. Tracked files are never touched, and
// go/acs/ is exempt (deleting the cycle's predicate package would nuke the
// gate itself — an auditor edit there is a different violation with a
// different guard). The sanctioned probe idiom remains `go test -overlay`
// (cycle-1106), which never writes the tree.
//
// Known limitation (adversarial review H1): the anchor is AUDIT dispatch, so
// a probe the adversarial-review phase left behind earlier is classified as
// builder work. That phase deletes its probes per its own persona; the
// residual risk is documented there rather than guessed at here — a wrong
// guess would quarantine bug-reproduction's legitimate repro tests.

// auditProbesDir is the workspace subdirectory that preserves quarantined
// probes — the durable record of what the auditor tried.
const auditProbesDir = "audit-probes"

// auditPromptArtifact is the dispatch anchor (engine.go: <agent>-prompt.txt).
const auditPromptArtifact = "audit-prompt.txt"

// quarantineGitTimeout bounds the status probe — a wedged git must degrade,
// not hang the audit phase (same bound class as ciparity's subprocess calls).
const quarantineGitTimeout = 30 * time.Second

// anchorFileName persists the FIRST audit dispatch time across retries:
// bridge.Engine.Launch rewrites audit-prompt.txt on EVERY dispatch, so
// anchoring on the latest prompt mtime would classify a dead first attempt's
// leftover probe as pre-dispatch builder work (diff-review HIGH — the
// 1115/1117 shape reopened on every audit retry).
const anchorFileName = ".dispatch-anchor"

// quarantineProbesForRequest is the Classify call site. No worktree → nothing
// to scan, logged loudly (the suite then runs against the main tree, so a
// silent skip here would hide the one case where exposure is HIGHEST). No
// dispatch anchor → skip loudly (no cutoff exists to discriminate with).
func quarantineProbesForRequest(req core.PhaseRequest) error {
	if req.Worktree == "" {
		fmt.Fprintf(os.Stderr, "[audit] probe quarantine skipped: no worktree on cycle %d — the EGPS suite runs against the main tree unscanned\n", req.Cycle)
		return nil
	}
	fi, err := os.Stat(filepath.Join(req.Workspace, auditPromptArtifact))
	if err != nil {
		fmt.Fprintf(os.Stderr, "[audit] probe quarantine skipped: no %s dispatch anchor in %s (%v)\n", auditPromptArtifact, req.Workspace, err)
		return nil
	}
	_, qerr := quarantineAuditProbes(req.Worktree, req.Workspace, firstDispatchAnchor(req.Workspace, fi.ModTime()), os.Stderr)
	return qerr
}

// firstDispatchAnchor returns the FIRST audit dispatch time for this cycle,
// persisting it under audit-probes/ on first sight. Later attempts read the
// persisted stamp instead of the rewritten prompt's fresh mtime. Best-effort:
// if the stamp cannot be written, the current prompt mtime is used (one
// attempt's protection rather than none).
func firstDispatchAnchor(workspace string, promptMtime time.Time) time.Time {
	stamp := filepath.Join(workspace, auditProbesDir, anchorFileName)
	if fi, err := os.Stat(stamp); err == nil {
		return fi.ModTime()
	}
	if err := os.MkdirAll(filepath.Dir(stamp), 0o755); err != nil {
		return promptMtime
	}
	if err := os.WriteFile(stamp, nil, 0o644); err != nil {
		return promptMtime
	}
	_ = os.Chtimes(stamp, promptMtime, promptMtime)
	return promptMtime
}

// quarantineAuditProbes preserves-then-removes audit-authored probe tests from
// worktree, returning the repo-relative paths it moved. Failure split
// (adversarial review H3): a git-status failure degrades OPEN with a loud log
// — a .git/index.lock race must not hard-fail the cycle this change exists to
// protect; but once a probe IS detected, any preserve/remove error fails
// loudly — silently leaving it poisons the gate, silently dropping it
// destroys auditor evidence.
func quarantineAuditProbes(worktree, workspace string, dispatchedAt time.Time, log io.Writer) ([]string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), quarantineGitTimeout)
	defer cancel()
	run := runCmd // capture once at entry (package convention, ciparity.go)
	out, err := sysexec.Output(ctx, run, worktree, "git", "status", "--porcelain", "-uall")
	if err != nil {
		fmt.Fprintf(log, "[audit] probe quarantine degraded OPEN: git status failed (%v) — tree not scanned for audit-authored probes\n", err)
		return nil, nil
	}

	var moved []string
	for _, line := range strings.Split(out, "\n") {
		rel, ok := newTestFilePath(line)
		if !ok {
			continue
		}
		abs := filepath.Join(worktree, rel)
		fi, statErr := os.Stat(abs)
		if statErr != nil || fi.ModTime().Before(dispatchedAt) {
			continue // pre-dispatch = builder deliverable; leave it alone
		}
		if err := preserveThenRemove(abs, filepath.Join(workspace, auditProbesDir, rel)); err != nil {
			return moved, fmt.Errorf("probe quarantine: %s: %w", rel, err)
		}
		moved = append(moved, rel)
		fmt.Fprintf(log, "[audit] quarantined audit-authored probe test %s → %s/%s (preserved, excluded from the EGPS tree — use `go test -overlay` for probes)\n",
			rel, auditProbesDir, rel)
	}
	return moved, nil
}

// preserveThenRemove copies src to dst, removing src only after the copy is on
// disk — losing auditor evidence is as bad as leaving the probe to poison the
// gate.
func preserveThenRemove(src, dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	raw, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	if err := os.WriteFile(dst, raw, 0o644); err != nil {
		return err
	}
	return os.Remove(src)
}

// newTestFilePath parses one `git status --porcelain -uall` line, returning
// the path when it is a NEW *_test.go outside go/acs/: untracked (`??`) or
// added-not-yet-committed (`A`-status — a preserved continuation worktree's
// pre-loop `git add -A` stages a prior attempt's leftovers, review M9). Path
// extraction goes through gitexec.PorcelainPath, the documented SSOT (a
// hand-rolled parse dropped git-quoted paths — silently missing exactly the
// files this exists to catch).
func newTestFilePath(line string) (string, bool) {
	if len(line) < 4 {
		return "", false
	}
	code := line[:2]
	if code != "??" && !strings.Contains(code, "A") {
		return "", false
	}
	p := gitexec.PorcelainPath(line)
	if p == "" || strings.HasSuffix(p, "/") || !strings.HasSuffix(p, "_test.go") {
		return "", false
	}
	if strings.HasPrefix(p, "go/acs/") {
		return "", false // the gate's own predicate tree — never quarantined
	}
	return p, true
}
