package loopchain

import (
	"context"
	"fmt"
	"io"
	"path/filepath"
	"strconv"

	"github.com/mickeyyaya/evolve-loop/go/internal/phaseintegrity"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

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

type Refresher struct {
	roots  Roots
	deps   RefreshDeps
	stderr io.Writer
	opts   options
}

func NewRefresher(roots Roots, deps RefreshDeps, warn io.Writer, opts ...Option) *Refresher {
	return &Refresher{roots: roots, deps: deps, stderr: warn, opts: newOptions(opts)}
}

func (r *Refresher) SignalsWired() bool { return r.opts.center() != nil }

type skip struct {
	step   string
	reason string
	err    error
}

func (r *Refresher) Refresh(ctx context.Context, batch int) (refreshed bool) {
	commit, ahead, s := r.guards(ctx)
	if s != nil {
		return r.skipped(batch, commit, s)
	}
	if !ahead {
		return false
	}
	fmt.Fprintf(r.stderr, "[chain] boundary-refresh: HEAD has advanced past the running binary's build commit (%.12s) — rebuilding\n", commit)
	target, res, s := r.rebuildAndRepin(ctx, batch)
	if s != nil {
		return r.skipped(batch, commit, s)
	}
	if s := r.armAndExec(ctx, batch, commit, target, res); s != nil {
		return r.skipped(batch, commit, s)
	}
	return true
}

func (r *Refresher) guards(ctx context.Context) (commit string, ahead bool, s *skip) {
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
	return commit, true, interrupted(ctx)
}

func (r *Refresher) rebuildAndRepin(ctx context.Context, batch int) (target string, res phaseintegrity.RepinResult, s *skip) {
	if err := r.deps.Rebuild(r.roots.ProjectRoot); err != nil {
		return "", res, &skip{"rebuild", fmt.Sprintf("rebuild failed (%v) — skipping refresh, continuing on the current binary", err), err}
	}
	if s := interrupted(ctx); s != nil {
		return "", res, s
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

func (r *Refresher) armAndExec(ctx context.Context, batch int, commit, target string, res phaseintegrity.RepinResult) *skip {
	if s := interrupted(ctx); s != nil {
		return s
	}
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

func interrupted(ctx context.Context) *skip {
	if err := ctx.Err(); err != nil {
		return &skip{"interrupted", fmt.Sprintf("interrupted (%v) — skipping the refresh without a re-exec, so the pending interrupt stops the loop on the current binary", err), err}
	}
	return nil
}

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

func (r *Refresher) warn(origin string, code signalcenter.Code, reason string, fields map[string]string) {
	emit(r.opts.center(), origin, signalcenter.KindLoopWarning, signalcenter.SeverityWarn, code, reason, fields)
}
