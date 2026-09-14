package loopchain

import (
	"fmt"
	"io"
	"path/filepath"
	"strconv"

	"github.com/mickeyyaya/evolve-loop/go/internal/phaseintegrity"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

// RefreshDeps are the Refresher's collaborators — every process seam the
// boundary refresh touches, explicit at construction. Rebuild (`make -C go
// build`) and ReExec (syscall.Exec) have no leaf default: the host owns the
// two process adapters. Flush drains the host's Signal Center before the
// terminal exec (a successful exec never returns, so nothing after it runs).
// A nil dep is a programming error and panics at first use.
type RefreshDeps struct {
	RunningCommit func() string
	Ahead         func(projectRoot, runningCommit string) (bool, error)
	LaneActive    func() (bool, error)
	Rebuild       func(projectRoot string) error
	ReExecTarget  func(projectRoot string) (string, error)
	Provenance    func(projectRoot string) (string, phaseintegrity.ProvenanceVerified)
	Argv          func() []string
	Environ       func() []string
	Flush         func()
	ReExec        func(argv0 string, argv, envv []string) error
}

// Refresher is the boundary binary refresh (cycle 1314): a chained or
// multi-wave loop that runs for many boundaries could keep executing a
// running binary that fell behind HEAD while fixes landed on main. It is
// called only BETWEEN batches (the loop body is single-threaded per
// boundary, so no lock is needed); every degrade returns refreshed=false and
// the CURRENT binary keeps running the loop — it never halts.
type Refresher struct {
	roots  Roots
	deps   RefreshDeps
	stderr io.Writer
	opts   options
}

// NewRefresher builds the refresher over its roots, deps and the warn writer
// the two kept report lines print to.
func NewRefresher(roots Roots, deps RefreshDeps, warn io.Writer, opts ...Option) *Refresher {
	return &Refresher{roots: roots, deps: deps, stderr: warn, opts: newOptions(opts)}
}

// SignalsWired reports whether the refresher currently reaches a Center.
func (r *Refresher) SignalsWired() bool { return r.opts.center() != nil }

// skip is the Result Object of a degrade branch: the step that refused, the
// sentence the branch has always printed, and the cause.
type skip struct {
	step   string
	reason string
	err    error
}

// Refresh runs the boundary-refresh sequence for boundary `batch` as a
// Template Method: guards (ahead-check, fleet-lane guard, loop breaker) →
// rebuildAndRepin (rebuild, re-exec target, provenance-gated re-pin, audit
// log) → armAndExec (argv, arm the breaker, flush, exec). Every degrade is
// ONE LOOP_BOUNDARY_REFRESH_SKIPPED naming its step; not-ahead is silent.
func (r *Refresher) Refresh(batch int) (refreshed bool) {
	commit, ahead, s := r.guards()
	if s != nil {
		return r.skipped(batch, commit, s)
	}
	if !ahead {
		return false
	}
	fmt.Fprintf(r.stderr, "[chain] boundary-refresh: HEAD has advanced past the running binary's build commit (%.12s) — rebuilding\n", commit)
	target, res, s := r.rebuildAndRepin(batch)
	if s != nil {
		return r.skipped(batch, commit, s)
	}
	if s := r.armAndExec(batch, commit, target, res); s != nil {
		return r.skipped(batch, commit, s)
	}
	return true
}

// guards resolves the running commit and refuses before any rebuild: an
// ahead-check error, an active sibling lane (or an unverifiable check — not
// proof the plane is idle; the standing rule is NEVER rebuild the plane
// binary mid-batch), and the loop breaker (a second attempt carrying the
// same build commit means the previous re-exec came back on a binary that
// had not moved — refuse rather than livelock at zero batches).
func (r *Refresher) guards() (commit string, ahead bool, s *skip) {
	commit = r.deps.RunningCommit()
	ahead, err := r.deps.Ahead(r.roots.ProjectRoot, commit)
	if err != nil {
		return commit, false, &skip{"ahead_check", fmt.Sprintf("ahead-check failed (%v) — skipping refresh, continuing on the current binary", err), err}
	}
	if !ahead {
		return commit, false, nil
	}
	if active, lerr := r.deps.LaneActive(); lerr != nil {
		return commit, true, &skip{"lane_check", fmt.Sprintf("WARN fleet lane check unverifiable (%v) — cannot prove the plane is idle, refusing to rebuild the shared binary; continuing on the current binary", lerr), lerr}
	} else if active {
		return commit, true, &skip{"lane_active", "a sibling fleet lane is active (fresh run lease) — refusing to rebuild the plane binary mid-batch; continuing on the current binary", nil}
	}
	if r.alreadyAttempted(commit) {
		return commit, true, &skip{"breaker", fmt.Sprintf("REFUSED — a refresh was already performed for build commit %.12s and the running binary still reports it; the rebuild did not move the binary. Continuing on the current binary rather than re-execing again (see .evolve/%s)", commit, r.opts.attemptFile), nil}
	}
	return commit, true, nil
}

// rebuildAndRepin rebuilds, resolves the re-exec target BEFORE the re-pin
// (if there is nothing safe to come back on, the pin must stay), then re-pins
// through phaseintegrity.RepinIfDrifted — the ONE shared detect-drift +
// provenance-gate + re-pin path the boot healer and the post-build re-pin
// use: it hashes the REBUILT binary and consults a REAL provenance closure;
// unverified provenance refuses and leaves the pin untouched. The audit
// entry is appended right after the pin moves (its failure is its own code
// and never aborts — the pin already moved).
func (r *Refresher) rebuildAndRepin(batch int) (target string, res phaseintegrity.RepinResult, s *skip) {
	if err := r.deps.Rebuild(r.roots.ProjectRoot); err != nil {
		return "", res, &skip{"rebuild", fmt.Sprintf("rebuild failed (%v) — skipping refresh, continuing on the current binary", err), err}
	}
	target, err := r.deps.ReExecTarget(r.roots.ProjectRoot)
	if err != nil {
		return "", res, &skip{"target", fmt.Sprintf("no re-exec target (%v) — skipping refresh, continuing on the current binary", err), err}
	}
	repinCommit, prov := r.deps.Provenance(r.roots.ProjectRoot)
	res, err = phaseintegrity.RepinIfDrifted(filepath.Join(r.roots.EvolveDir, "state.json"), target, repinCommit, "", prov)
	if err != nil {
		return "", res, &skip{"repin", fmt.Sprintf("repin refused (%v) — skipping refresh, continuing on the current binary", err), err}
	}
	if err := r.appendLog(batch, res); err != nil {
		r.warn("Refresher.Refresh", CodeBoundaryRefreshAuditFailed, fmt.Sprintf("WARN: audit log write failed (%v) — pin already moved", err),
			map[string]string{"batch": strconv.Itoa(batch), "path": r.logPath(), "error": err.Error()})
	}
	return target, res, nil
}

// armAndExec resolves the argv, arms the breaker BEFORE the exec (a
// successful exec never returns, so anything written after it never
// happens), flushes the Center so no queued signal is lost to the exec, and
// replaces the process image. The audit entry lands before the argv check
// (Q-C2): an empty argv leaves a log entry with no re-exec.
func (r *Refresher) armAndExec(batch int, commit, target string, res phaseintegrity.RepinResult) *skip {
	argv := r.deps.Argv()
	if len(argv) == 0 {
		return &skip{"argv", "empty re-exec argv — skipping refresh", nil}
	}
	if err := r.recordAttempt(commit, batch); err != nil {
		return &skip{"arm", fmt.Sprintf("cannot arm the re-exec loop breaker (%v) — skipping refresh rather than risking an unbounded re-exec loop", err), err}
	}
	fmt.Fprintf(r.stderr, "[chain] boundary-refresh: re-pinned %.12s -> %.12s — re-executing %s to pick up the rebuilt binary\n", res.OldSHA, res.NewSHA, target)
	r.deps.Flush()
	if err := r.deps.ReExec(target, argv, r.deps.Environ()); err != nil {
		return &skip{"reexec", fmt.Sprintf("re-exec failed (%v) — continuing on the current binary", err), err}
	}
	return nil
}

// skipped reports a degrade branch as ONE loop.warning WARN and returns
// refreshed=false.
func (r *Refresher) skipped(batch int, commit string, s *skip) bool {
	fields := map[string]string{"batch": strconv.Itoa(batch), "step": s.step}
	if commit != "" {
		fields["commit"] = commit
	}
	if s.err != nil {
		fields["error"] = s.err.Error()
	}
	r.warn("Refresher.Refresh", CodeBoundaryRefreshSkipped, s.reason, fields)
	return false
}

// warn is the refresher's producer: a loop.warning WARN under module loop.
func (r *Refresher) warn(origin string, code signalcenter.Code, reason string, fields map[string]string) {
	emit(r.opts.center(), origin, signalcenter.KindLoopWarning, signalcenter.SeverityWarn, code, reason, fields)
}
