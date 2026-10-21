package bridge

import (
	"context"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/ipcenv"
	"github.com/mickeyyaya/evolve-loop/go/internal/proctree"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

const (
	CodeDispatchProcessSurvived signalcenter.Code = "BRIDGE_DISPATCH_PROCESS_SURVIVED"
	CodeDispatchReapFailed      signalcenter.Code = "BRIDGE_DISPATCH_REAP_FAILED"
	CodeDispatchSweepRefused    signalcenter.Code = "BRIDGE_DISPATCH_SWEEP_REFUSED"

	dispatchReapGrace   = 2 * time.Second
	dispatchReapTimeout = 30 * time.Second
	dispatchReapOrigin  = "dispatchSweep.finish"
)

func init() {
	signalcenter.RegisterCode(signalcenter.ModuleBridge, CodeDispatchProcessSurvived, "a process of a finished dispatch was still alive after SIGTERM, the grace time and SIGKILL; it carried the dispatch tag or was in the pane tree recorded before kill-session; fields dispatch, pids")
	signalcenter.RegisterCode(signalcenter.ModuleBridge, CodeDispatchReapFailed, "the sweep at dispatch end could not list the processes or could not send a signal; the processes of the dispatch can still run, and gc is the backstop; fields dispatch, errors")
	signalcenter.RegisterCode(signalcenter.ModuleBridge, CodeDispatchSweepRefused, "the sweep at dispatch end did not look for orphans by cwd, because the worktree is not a safe base: it is empty or relative, it cannot be resolved, or it holds the project root or the home directory; the tag and pane-tree proofs still run; fields dispatch, reason")
}

var dispatchNonce atomic.Uint64

func mintDispatchID(cfg *Config, pid int) string {
	nonce := strconv.FormatUint(dispatchNonce.Add(1), 36) + "t" + randomStamp()
	return proctree.DispatchID{Run: orDefault(cfg.RunID, "norun"), Cycle: cfg.Cycle, Agent: orDefault(cfg.Agent, "agent"), Owner: pid, Nonce: nonce}.String()
}

func dispatchTagLine(id string) string {
	if id == "" {
		return "unset " + ipcenv.DispatchIDKey
	}
	return "export " + ipcenv.DispatchIDKey + "=" + shellQuotePOSIX(id)
}

func dispatchEnv(deps Deps, cfg *Config) []string {
	env := slices.DeleteFunc(driverEnv(deps, cfg.Realization.Env), isDispatchTag)
	if cfg.DispatchID == "" {
		return env
	}
	return append(env, ipcenv.DispatchIDKey+"="+cfg.DispatchID)
}

func isDispatchTag(kv string) bool { return strings.HasPrefix(kv, ipcenv.DispatchIDKey+"=") }

type dispatchSweep struct {
	id          string
	treeDir     string
	cwdProofDir string
	refusal     string
	started     time.Time
	panes       []proctree.Identity
	recorded    []proctree.Identity
	preserved   bool
}

type panePIDLister interface {
	PanePIDs(ctx context.Context, session string) ([]int, error)
}

func (s *dispatchSweep) preserve() {
	if s != nil {
		s.preserved = true
	}
}

func (s *dispatchSweep) recordPane(ctx context.Context, deps Deps, session string) {
	lister, ok := deps.Tmux.(panePIDLister)
	if s == nil || !ok {
		return
	}
	roots, err := lister.PanePIDs(ctx, session)
	if err != nil {
		fmt.Fprintf(deps.Stderr, "[bridge] WARN dispatch %s: cannot read the pane pids of session %s: %v\n", s.id, session, err)
		return
	}
	table, err := deps.ListProcesses(ctx)
	if err != nil {
		fmt.Fprintf(deps.Stderr, "[bridge] WARN dispatch %s: cannot record the pane tree of session %s: %v\n", s.id, session, err)
		return
	}
	s.panes, _ = union(s.panes, proctree.Identities(proctree.Select(table, pidIn(roots))))
	grown := false
	s.recorded, grown = union(s.recorded, proctree.Identities(proctree.Descendants(table, roots)))
	if grown && s.treeDir != "" && s.id != "" {
		if err := proctree.SaveTree(s.treeDir, s.id, s.recorded); err != nil {
			fmt.Fprintf(deps.Stderr, "[bridge] WARN dispatch %s: cannot persist the pane tree for gc: %v\n", s.id, err)
		}
	}
}

func union(known, members []proctree.Identity) ([]proctree.Identity, bool) {
	seen := proctree.Recorded(known)
	out, grown := known, false
	for _, m := range members {
		if !seen(proctree.Process{Pid: m.Pid, Started: m.Started}) {
			out, grown = append(out, m), true
		}
	}
	return out, grown
}

func pidIn(pids []int) proctree.Proof {
	return func(p proctree.Process) bool { return slices.Contains(pids, p.Pid) }
}

func (s *dispatchSweep) finish(ctx context.Context, deps Deps) {
	if s == nil || s.preserved || (s.id == "" && len(s.recorded) == 0) {
		return
	}
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), dispatchReapTimeout)
	defer cancel()
	reaper := proctree.Reaper{List: deps.ListProcesses, Signal: deps.SignalProcess, Sleep: deps.Sleep,
		Grace: dispatchReapGrace, Self: os.Getpid()}
	rep := reaper.Reap(ctx, s.ownership(ctx, deps))
	s.report(deps, rep)
	if len(rep.Errors) == 0 && len(rep.Survivors) == 0 && s.treeDir != "" && s.id != "" {
		if err := proctree.RemoveTree(s.treeDir, s.id); err != nil {
			fmt.Fprintf(deps.Stderr, "[bridge] WARN dispatch %s: %v\n", s.id, err)
		}
	}
}

func (s *dispatchSweep) ownership(ctx context.Context, deps Deps) proctree.Proof {
	owned := proctree.OwnedByDispatch(s.id, s.recorded, s.livePaneTree(ctx, deps))
	if s.cwdProofDir == "" {
		s.refuse(deps)
		return owned
	}
	cwds, err := deps.ListCwds(ctx)
	if err != nil {
		fmt.Fprintf(deps.Stderr, "[bridge] WARN dispatch %s: cannot list the process cwds, so an untagged orphan in %s can outlive the dispatch: %v\n", s.id, s.cwdProofDir, err)
		return owned
	}
	return proctree.AnyOf(owned, proctree.OrphanUnder(s.cwdProofDir, s.started, cwds))
}

func (s *dispatchSweep) refuse(deps Deps) {
	if s.refusal == "" {
		return
	}
	fmt.Fprintf(deps.Stderr, "[bridge] WARN dispatch %s: no orphan sweep by cwd: %s\n", s.id, s.refusal)
	s.emit(deps, CodeDispatchSweepRefused, signalcenter.SeverityWarn, "no orphan sweep by cwd", map[string]string{"reason": s.refusal})
}

func (s *dispatchSweep) livePaneTree(ctx context.Context, deps Deps) []proctree.Identity {
	if len(s.panes) == 0 {
		return nil
	}
	table, err := deps.ListProcesses(ctx)
	if err != nil {
		fmt.Fprintf(deps.Stderr, "[bridge] WARN dispatch %s: cannot list the live pane tree, so each shared helper is spared: %v\n", s.id, err)
		return nil
	}
	roots := proctree.Select(table, proctree.Recorded(s.panes))
	pids := make([]int, 0, len(roots))
	for _, r := range roots {
		pids = append(pids, r.Pid)
	}
	return proctree.Identities(proctree.Descendants(table, pids))
}

func (s *dispatchSweep) report(deps Deps, rep proctree.Report) {
	if len(rep.Terminated) > 0 {
		fmt.Fprintf(deps.Stderr, "[bridge] dispatch %s: stopped %d process(es) at dispatch end (SIGKILL: %d)\n", s.id, len(rep.Terminated), len(rep.Killed))
	}
	if len(rep.Errors) > 0 {
		fmt.Fprintf(deps.Stderr, "[bridge] WARN dispatch %s: the sweep at dispatch end is not complete: %s\n", s.id, strings.Join(rep.Errors, "; "))
		s.emit(deps, CodeDispatchReapFailed, signalcenter.SeverityWarn, "the sweep at dispatch end is not complete", map[string]string{"errors": strings.Join(rep.Errors, "; ")})
	}
	if len(rep.Survivors) > 0 {
		pids := pidList(rep.Survivors)
		fmt.Fprintf(deps.Stderr, "[bridge] ERROR dispatch %s: process(es) %s are alive after SIGKILL\n", s.id, pids)
		s.emit(deps, CodeDispatchProcessSurvived, signalcenter.SeverityIncident, "processes of a finished dispatch are alive after SIGKILL", map[string]string{"pids": pids})
	}
}

func (s *dispatchSweep) emit(deps Deps, code signalcenter.Code, sev signalcenter.Severity, reason string, fields map[string]string) {
	fields["dispatch"] = s.id
	deps.Signals.Emit(signalcenter.Event{Module: signalcenter.ModuleBridge, Origin: dispatchReapOrigin, Kind: signalcenter.KindBridgeWarning,
		Severity: sev, Code: code, Reason: reason, Fields: fields})
}

func pidList(procs []proctree.Process) string {
	out := make([]string, 0, len(procs))
	for _, p := range procs {
		out = append(out, strconv.Itoa(p.Pid))
	}
	return strings.Join(out, ",")
}

func randomStamp() string {
	b := make([]byte, 6)
	if _, err := randRead(b); err != nil {
		return strconv.FormatInt(time.Now().UnixNano(), 36)
	}
	return hex.EncodeToString(b)
}

func launchSwept(ctx context.Context, driver Driver, cfg *Config, deps Deps) (int, error) {
	cfg.DispatchID = mintDispatchID(cfg, os.Getpid())
	observeDispatchID(deps, cfg.DispatchID)
	home, _ := os.UserHomeDir()
	deps.sweep = newDispatchSweep(cfg.DispatchID, cfg.ProjectRoot, cfg.Worktree, home, time.Now())
	defer deps.sweep.finish(ctx, deps)
	return driver.Launch(ctx, cfg, deps)
}

var defaultSignalProcess proctree.Signaler = syscall.Kill

func (d Deps) withProcessDefaults() Deps {
	if d.ListProcesses == nil {
		d.ListProcesses = proctree.ExecLister()
	}
	if d.ListCwds == nil {
		d.ListCwds = proctree.ExecCwdLister()
	}
	if d.SignalProcess == nil {
		d.SignalProcess = defaultSignalProcess
	}
	return d
}

func newDispatchSweep(id, projectRoot, worktree, home string, started time.Time) *dispatchSweep {
	s := &dispatchSweep{id: id, treeDir: dispatchTreeDir(projectRoot), started: started.Truncate(time.Second)}
	s.cwdProofDir, s.refusal = cwdProofBase(projectRoot, worktree, home)
	return s
}

func cwdProofBase(projectRoot, worktree, home string) (base, refusal string) {
	if worktree == "" {
		return "", ""
	}
	if !filepath.IsAbs(projectRoot) || !filepath.IsAbs(worktree) || !filepath.IsAbs(home) {
		return "", fmt.Sprintf("the project root %q, the worktree %q and the home directory %q are not all absolute", projectRoot, worktree, home)
	}
	base, err := filepath.EvalSymlinks(worktree)
	if err != nil {
		return "", fmt.Sprintf("cannot resolve the worktree: %v", err)
	}
	for _, kept := range []string{projectRoot, home} {
		if proctree.Under(base, kept) {
			return "", fmt.Sprintf("the worktree %s holds %s", base, kept)
		}
	}
	return base, ""
}

func dispatchTreeDir(projectRoot string) string {
	if projectRoot == "" {
		return ""
	}
	return proctree.TreeDir(projectRoot)
}

func killSessionSwept(ctx context.Context, deps Deps, session string) error {
	deps = deps.withProcessDefaults()
	sweep := deps.sweep
	if sweep == nil {
		sweep = &dispatchSweep{}
		defer sweep.finish(ctx, deps)
	}
	sweep.preserved = false
	sweep.recordPane(ctx, deps, session)
	return deps.Tmux.KillSession(ctx, session)
}

func boundedCommand(ctx context.Context, name string, args ...string) *exec.Cmd {
	cmd := sysexec.Command(ctx, name, args...)
	cmd.Env = slices.DeleteFunc(os.Environ(), isDispatchTag)
	return cmd
}

func (w replWaiter) pace() {
	w.deps.Sleep(artifactWaitInterval)
	w.deps.sweep.recordPane(w.ctx, w.deps, w.launch.session)
}
