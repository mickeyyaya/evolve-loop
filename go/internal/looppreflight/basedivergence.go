// basedivergence.go — boot-time guard against cutting lanes from a stale base.
//
// A fleet lane's worktree is branched from whatever the project root's HEAD is
// at boot. When that local base has fallen behind `origin/<base>`, every lane in
// the batch is built on stale history and the ship at the end fails with
// GIT_PUSH_REJECTED — after the whole batch's work is already spent (cycle-969).
// The reconcile is a single operator command (`evolve sync-main`), so the cheap
// remedy is to fetch origin at boot and HALT loudly, naming that command, BEFORE
// any lane spawns.
//
// The check fetches origin ITSELF rather than reading a possibly-stale local
// `origin/<base>` ref: an operator-prepared ref is exactly the thing that is out
// of date in this failure mode. A fetch that cannot complete degrades to Warn —
// unverified is surfaced, never silently passed — and a base that is merely
// AHEAD of origin (normal unpushed work) is a pass, not a halt.
package looppreflight

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/gitexec"
)

// reconcileCommand is the operator command the halt must name so the stop comes
// with a next step rather than just a wall.
const reconcileCommand = "evolve sync-main"

// baseDivergenceTimeout bounds the whole probe (fetch included) so a wedged
// remote cannot hang boot.
const baseDivergenceTimeout = 60 * time.Second

// baseState is the probe's verdict. Skipped marks a topology the check has no
// opinion on (not a git work tree, detached HEAD, no origin remote); Reason
// carries the why for the operator-visible detail.
type baseState struct {
	Skipped bool
	Reason  string
	Branch  string
	Ahead   int
	Behind  int
}

// baseDivergenceProbe is the injectable seam. Production uses the real git
// probe; in-package tests substitute a deterministic verdict so the fast tier
// never shells out to git or touches a network remote.
var baseDivergenceProbe = defaultBaseDivergenceProbe

// newGit builds the git runner the probe drives. Split out so the probe's own
// branch logic is testable against scripted git replies instead of a real repo
// with a real remote.
var newGit = gitexec.Default

// defaultBaseDivergenceProbe compares projectRoot's current branch against the
// freshly fetched origin counterpart. An error means UNVERIFIED (caller warns);
// a Skipped result means there is nothing to compare.
func defaultBaseDivergenceProbe(ctx context.Context, projectRoot string) (baseState, error) {
	g := newGit(projectRoot)

	if _, err := g.Output(ctx, "rev-parse", "--is-inside-work-tree"); err != nil {
		return baseState{Skipped: true, Reason: "not a git work tree"}, nil
	}
	branch, err := g.Output(ctx, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return baseState{}, fmt.Errorf("resolve current branch: %w", err)
	}
	if branch == "" || branch == "HEAD" {
		return baseState{Skipped: true, Reason: "detached HEAD — no base branch to compare"}, nil
	}
	remotes, err := g.Output(ctx, "remote")
	if err != nil {
		return baseState{}, fmt.Errorf("list remotes: %w", err)
	}
	if !hasRemoteOrigin(remotes) {
		return baseState{Skipped: true, Reason: "no `origin` remote"}, nil
	}

	// The fetch is the point of the check: FETCH_HEAD is written by THIS
	// invocation, so the comparison below can never read a stale ref.
	if err := g.Run(ctx, "fetch", "origin", branch); err != nil {
		return baseState{}, fmt.Errorf("fetch origin %s: %w", branch, err)
	}
	counts, err := g.Output(ctx, "rev-list", "--left-right", "--count", "HEAD...FETCH_HEAD")
	if err != nil {
		return baseState{}, fmt.Errorf("count divergence against origin/%s: %w", branch, err)
	}
	ahead, behind, err := parseLeftRightCount(counts)
	if err != nil {
		return baseState{}, err
	}
	return baseState{Branch: branch, Ahead: ahead, Behind: behind}, nil
}

// hasRemoteOrigin reports whether `git remote` output lists origin.
func hasRemoteOrigin(remotes string) bool {
	for _, r := range strings.Fields(remotes) {
		if r == "origin" {
			return true
		}
	}
	return false
}

// parseLeftRightCount parses `rev-list --left-right --count A...B` output
// ("<left>\t<right>") into ahead (left, local-only) and behind (right,
// remote-only) commit counts.
func parseLeftRightCount(out string) (ahead, behind int, err error) {
	fields := strings.Fields(out)
	if len(fields) != 2 {
		return 0, 0, fmt.Errorf("unexpected rev-list --count output %q", out)
	}
	if ahead, err = strconv.Atoi(fields[0]); err != nil {
		return 0, 0, fmt.Errorf("parse ahead count %q: %w", fields[0], err)
	}
	if behind, err = strconv.Atoi(fields[1]); err != nil {
		return 0, 0, fmt.Errorf("parse behind count %q: %w", fields[1], err)
	}
	return ahead, behind, nil
}

// checkBaseDivergence (Halt) refuses to start a batch whose base is behind the
// fetched origin base. Ahead-only is healthy (unpushed local work); an
// unverifiable comparison is a Warn, so a transient network fault degrades the
// signal without benching an otherwise-ready boot.
func checkBaseDivergence(o resolved) CheckResult {
	const name = "base-divergence"

	ctx, cancel := context.WithTimeout(context.Background(), baseDivergenceTimeout)
	defer cancel()

	st, err := baseDivergenceProbe(ctx, o.projectRoot)
	if err != nil {
		return CheckResult{
			Name:    name,
			Level:   LevelWarn,
			Message: "base vs origin UNVERIFIED",
			Detail: fmt.Sprintf("could not compare %s against origin: %v\n"+
				"lanes may be cut from a stale base; run `%s` if the base is behind",
				o.projectRoot, err, reconcileCommand),
		}
	}
	if st.Skipped {
		return CheckResult{
			Name:    name,
			Level:   LevelPass,
			Message: "base divergence not applicable",
			Detail:  st.Reason,
		}
	}
	if st.Behind > 0 {
		return CheckResult{
			Name:    name,
			Level:   LevelHalt,
			Message: fmt.Sprintf("local %s is %d commit(s) behind origin/%s", st.Branch, st.Behind, st.Branch),
			Detail: fmt.Sprintf(
				"every lane in this batch would be cut from a stale base and its ship would be rejected at push (GIT_PUSH_REJECTED).\n"+
					"local %s: %d ahead / %d behind origin/%s (freshly fetched).\n"+
					"reconcile first: `%s`",
				st.Branch, st.Ahead, st.Behind, st.Branch, reconcileCommand),
		}
	}
	return CheckResult{
		Name:    name,
		Level:   LevelPass,
		Message: fmt.Sprintf("base %s is up to date with origin", st.Branch),
		Detail:  fmt.Sprintf("%d ahead / 0 behind origin/%s", st.Ahead, st.Branch),
	}
}
