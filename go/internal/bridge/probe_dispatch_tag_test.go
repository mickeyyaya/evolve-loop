package bridge

import (
	"context"
	"os"
	"regexp"
	"slices"
	"syscall"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/gc"
	"github.com/mickeyyaya/evolve-loop/go/internal/ipcenv"
	"github.com/mickeyyaya/evolve-loop/go/internal/proctree"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

const (
	probeShellPid = 700
	probeChildPid = 701
	probeCycle    = 2218
)

var probeTagLine = regexp.MustCompile(`^export ` + ipcenv.DispatchIDKey + `='?([^']+)'?$`)

type paneTreeProbe struct {
	treeDir string
	keys    func() []string
	tag     string
	tree    []proctree.Identity
	treeErr error
	taken   bool
}

func (p *paneTreeProbe) snapshotAtKill() {
	p.tag = probeTagOf(p.keys())
	p.taken = true
	if p.tag != "" {
		p.tree, p.treeErr = proctree.LoadTree(p.treeDir, p.tag)
	}
}

func probeTagOf(keys []string) string {
	for _, k := range keys {
		if m := probeTagLine.FindStringSubmatch(k); m != nil {
			return m[1]
		}
	}
	return ""
}

type probeTmux struct {
	fakeTmux
	pids       []int
	snap       *paneTreeProbe
	keysAtBoot int
	booted     bool
}

func (f *probeTmux) PanePIDs(context.Context, string) ([]int, error) { return f.pids, nil }

func (f *probeTmux) CapturePane(ctx context.Context, name string, scrollback int) (string, error) {
	if !f.booted {
		f.booted, f.keysAtBoot = true, len(f.sentKeys)
	}
	return f.fakeTmux.CapturePane(ctx, name, scrollback)
}

func (f *probeTmux) KillSession(ctx context.Context, name string) error {
	f.snap.snapshotAtKill()
	return f.fakeTmux.KillSession(ctx, name)
}

type mismatchProbeTmux struct {
	*FakeTmuxController
	pids []int
	snap *paneTreeProbe
}

func (f *mismatchProbeTmux) PanePIDs(context.Context, string) ([]int, error) { return f.pids, nil }

func (f *mismatchProbeTmux) KillSession(ctx context.Context, name string) error {
	f.snap.snapshotAtKill()
	return f.FakeTmuxController.KillSession(ctx, name)
}

type probeRun struct {
	root    string
	keys    []string
	bootAt  int
	snap    *paneTreeProbe
	sent    []int
	leaked  []proctree.Process
	runCode int
}

func paneProcesses() []proctree.Process {
	return []proctree.Process{
		{Pid: probeShellPid, Ppid: 1, Pgid: probeShellPid, Started: reapT0, Comm: "zsh", Env: map[string]string{}},
		{Pid: probeChildPid, Ppid: probeShellPid, Pgid: probeShellPid, Started: reapT0, Comm: "claude", Env: map[string]string{}},
	}
}

func probeHostDeps(deps Deps, host *hostTable) Deps {
	deps.ListProcesses = host.list
	deps.SignalProcess = host.signal
	deps.ListCwds = func(context.Context) ([]proctree.Cwd, error) { return nil, nil }
	deps.Signals = signalcenter.New()
	deps.Sleep = func(time.Duration) {}
	return deps
}

func leakProbeConfig(t *testing.T) *Config {
	t.Helper()
	cfg := namedCaptureCfg(t, "")
	cfg.SessionName = ""
	cfg.Worktree = t.TempDir()
	cfg.ProjectRoot = t.TempDir()
	cfg.RunID = "01RUN"
	cfg.Cycle = probeCycle
	return cfg
}

const probeReadyPane = "ready ❯\nSelect model\n1. Default  Opus 4.8"

func runEphemeralProbe(t *testing.T, probe func(context.Context, *Config, Deps) error) probeRun {
	t.Helper()
	cfg := leakProbeConfig(t)
	var events []string
	host := &hostTable{tables: [][]proctree.Process{paneProcesses()}, events: &events}
	snap := &paneTreeProbe{treeDir: proctree.TreeDir(cfg.ProjectRoot)}
	tx := &probeTmux{fakeTmux: fakeTmux{paneSeq: []string{probeReadyPane}}, pids: []int{probeShellPid}, snap: snap}
	snap.keys = func() []string { return tx.sentKeys }

	if err := probe(context.Background(), cfg, probeHostDeps(covDeps(), host).withTmux(tx)); err != nil {
		t.Fatalf("probe: %v", err)
	}
	return probeRun{root: cfg.ProjectRoot, keys: tx.sentKeys, bootAt: tx.keysAtBoot, snap: snap, sent: host.sent, leaked: paneProcesses()}
}

func (d Deps) withTmux(tm TmuxController) Deps {
	d.Tmux = tm
	return d
}

func runMismatchProbe(t *testing.T) probeRun {
	t.Helper()
	cfg := fixtureConfig(t)
	cfg.AllowBypass, cfg.Agent, cfg.Cycle, cfg.RunID = true, "auditor", probeCycle, "01RUN"
	cfg.CLI = "agy-claude-tmux"
	cfg.Realization = RealizeFor("agy-claude-tmux", LaunchIntent{ModelTier: "deep", Permission: "bypass"})
	var events []string
	host := &hostTable{tables: [][]proctree.Process{paneProcesses()}, events: &events}
	snap := &paneTreeProbe{treeDir: proctree.TreeDir(cfg.ProjectRoot)}
	frames := slices.Repeat([]string{agyFooterGeminiFlash}, 400)
	fake := &FakeTmuxController{CaptureFrames: frames}
	tx := &mismatchProbeTmux{FakeTmuxController: fake, pids: []int{probeShellPid}, snap: snap}
	snap.keys = func() []string { return fake.SentKeys }
	d, ok := LookupDriver("agy-claude-tmux")
	if !ok {
		t.Fatal("agy-claude-tmux is not a registered driver")
	}
	deps := probeHostDeps(Deps{Tmux: tx, Stderr: testLogWriter{t}, LookupEnv: mapLookup(map[string]string{"EVOLVE_PHASE_RECOVERY": "off"})}, host).withDefaults()

	code, err := launchSwept(context.Background(), d, cfg, deps)
	if code != ExitModelMismatch {
		t.Fatalf("launchSwept exit = %d (err %v), want ExitModelMismatch %d", code, err, ExitModelMismatch)
	}
	return probeRun{root: cfg.ProjectRoot, keys: fake.SentKeys, bootAt: len(fake.SentKeys), snap: snap, sent: host.sent, leaked: paneProcesses(), runCode: code}
}

type testLogWriter struct{ t *testing.T }

func (w testLogWriter) Write(p []byte) (int, error) { w.t.Log(string(p)); return len(p), nil }

func assertProbeTagged(t *testing.T, run probeRun) {
	t.Helper()
	tag := probeTagOf(run.keys)
	if tag == "" {
		t.Fatalf("no `export %s=` line was sent to the probe pane; sent %q", ipcenv.DispatchIDKey, run.keys)
	}
	if _, ok := proctree.ParseDispatchID(tag); !ok {
		t.Errorf("probe tag %q is not a dispatch id gc can parse", tag)
	}
	if !slices.Contains(run.keys, "export EVOLVE_PROJECT_ROOT="+shellQuotePOSIX(run.root)) {
		t.Errorf("no `export EVOLVE_PROJECT_ROOT=%s` line was sent to the probe pane; sent %q", run.root, run.keys)
	}
	if at := slices.IndexFunc(run.keys, func(k string) bool { return probeTagLine.MatchString(k) }); run.bootAt > 0 && at >= run.bootAt-1 {
		t.Errorf("the tag line is key #%d but the CLI launch is key #%d: the tag must be exported before the CLI starts", at, run.bootAt-1)
	}
	if !run.snap.taken {
		t.Fatal("kill-session was never reached")
	}
	if run.snap.tag != tag {
		t.Errorf("at kill-session the persisted-tree id is %q, want the pane tag %q", run.snap.tag, tag)
	}
	if run.snap.treeErr != nil || !slices.ContainsFunc(run.snap.tree, func(i proctree.Identity) bool { return i.Pid == probeChildPid }) {
		t.Errorf("tree saved for %q at kill-session = %v (err %v), want it to hold the pane child %d", tag, run.snap.tree, run.snap.treeErr, probeChildPid)
	}
}

func TestProbeTag_CaptureHelpTagsItsPaneAndPersistsTheTreeBeforeKillSession(t *testing.T) {
	t.Parallel()
	run := runEphemeralProbe(t, func(ctx context.Context, cfg *Config, deps Deps) error {
		_, err := CaptureHelp(ctx, cfg, deps, "claude-tmux")
		return err
	})
	assertProbeTagged(t, run)
}

func TestProbeTag_CaptureModelPickerTagsItsPaneAndPersistsTheTreeBeforeKillSession(t *testing.T) {
	t.Parallel()
	run := runEphemeralProbe(t, func(ctx context.Context, cfg *Config, deps Deps) error {
		_, err := CaptureModelPicker(ctx, cfg, deps, "claude-tmux")
		return err
	})
	assertProbeTagged(t, run)
}

func TestProbeTag_AnUnnamedLaunchModelMismatchKillTagsItsPaneAndPersistsTheTreeBeforeKillSession(t *testing.T) {
	t.Parallel()
	assertProbeTagged(t, runMismatchProbe(t))
}

func TestProbeTag_TheProbeStillStopsTheRecordedPaneChildAtTheEndOfTheProbe(t *testing.T) {
	t.Parallel()
	run := runEphemeralProbe(t, func(ctx context.Context, cfg *Config, deps Deps) error {
		_, err := CaptureHelp(ctx, cfg, deps, "claude-tmux")
		return err
	})
	if !slices.Contains(run.sent, probeChildPid) {
		t.Errorf("signals went to %v, want the pane child %d stopped by the injected sender", run.sent, probeChildPid)
	}
}

func leakedProbeTable(tag, root string, tagged bool) []proctree.Process {
	table := paneProcesses()
	for i := range table {
		if tagged {
			table[i].Env = map[string]string{ipcenv.DispatchIDKey: tag, gc.ProjectRootEnvKey: root}
		}
		table[i].Ppid = 1
	}
	return table
}

func gcSweepOfLeak(root string, ownerAlive, cycleClosed bool, table []proctree.Process) (planned []gc.DispatchProcessItem, signalled []int) {
	opts := gc.DispatchProcessOptions{
		ProjectRoot: root,
		PidAlive:    func(int) bool { return ownerAlive },
		CycleClosed: func(int) bool { return cycleClosed },
		RecordedTree: func(id string) []proctree.Identity {
			tree, _ := proctree.LoadTree(proctree.TreeDir(root), id)
			return tree
		},
		Now: reapT0.Add(48 * time.Hour),
	}
	planned = gc.PlanDispatchProcesses(table, opts)
	reaper := proctree.Reaper{
		List:   func(context.Context) ([]proctree.Process, error) { return withSelf(table), nil },
		Signal: func(pid int, _ syscall.Signal) error { signalled = append(signalled, pid); return nil },
		Sleep:  func(time.Duration) {}, Grace: time.Millisecond, Self: os.Getpid(),
	}
	reaper.Reap(context.Background(), gc.StaleDispatch(opts))
	return planned, signalled
}

func assertGCReapsTheLeak(t *testing.T, run probeRun) {
	t.Helper()
	tag := probeTagOf(run.keys)
	if tag == "" {
		t.Fatalf("the probe exported no %s, so gc has no tag to match; sent %q", ipcenv.DispatchIDKey, run.keys)
	}
	planned, signalled := gcSweepOfLeak(run.root, false, true, leakedProbeTable(tag, run.root, true))
	if !slices.ContainsFunc(planned, func(it gc.DispatchProcessItem) bool {
		return it.Process.Pid == probeChildPid && it.Rule == gc.RuleStaleDispatch
	}) {
		t.Errorf("gc planned %v, want the leaked probe child %d under rule %s", planned, probeChildPid, gc.RuleStaleDispatch)
	}
	if !slices.Contains(signalled, probeChildPid) {
		t.Errorf("the injected sender got %v, want the leaked probe child %d", signalled, probeChildPid)
	}
}

func TestProbeGC_ALeakedHelpProbeSessionIsReapedByGCThroughAnInjectedSender(t *testing.T) {
	t.Parallel()
	assertGCReapsTheLeak(t, runEphemeralProbe(t, func(ctx context.Context, cfg *Config, deps Deps) error {
		_, err := CaptureHelp(ctx, cfg, deps, "claude-tmux")
		return err
	}))
}

func TestProbeGC_ALeakedModelPickerProbeSessionIsReapedByGCThroughAnInjectedSender(t *testing.T) {
	t.Parallel()
	assertGCReapsTheLeak(t, runEphemeralProbe(t, func(ctx context.Context, cfg *Config, deps Deps) error {
		_, err := CaptureModelPicker(ctx, cfg, deps, "claude-tmux")
		return err
	}))
}

func TestProbeGC_ALeakedLaunchModelMismatchProbeSessionIsReapedByGCThroughAnInjectedSender(t *testing.T) {
	t.Parallel()
	assertGCReapsTheLeak(t, runMismatchProbe(t))
}

func TestProbeGC_ALeakedProbeTagIsNoWarrantToStopALiveDispatchAnotherProjectOrAnUntaggedProcess(t *testing.T) {
	t.Parallel()
	run := runEphemeralProbe(t, func(ctx context.Context, cfg *Config, deps Deps) error {
		_, err := CaptureHelp(ctx, cfg, deps, "claude-tmux")
		return err
	})
	tag := probeTagOf(run.keys)
	if tag == "" {
		t.Fatalf("the probe exported no %s; sent %q", ipcenv.DispatchIDKey, run.keys)
	}
	_, liveOwner := gcSweepOfLeak(run.root, true, false, leakedProbeTable(tag, run.root, true))
	if len(liveOwner) != 0 {
		t.Errorf("gc signalled %v for a probe whose owner is alive and whose cycle is open", liveOwner)
	}
	_, otherProject := gcSweepOfLeak("/other/project", false, true, leakedProbeTable(tag, run.root, true))
	if len(otherProject) != 0 {
		t.Errorf("gc signalled %v for a probe tagged with another project root", otherProject)
	}
	_, untagged := gcSweepOfLeak(run.root, false, true, leakedProbeTable(tag, run.root, false))
	if len(untagged) != 0 {
		t.Errorf("gc signalled %v for processes that carry no dispatch tag", untagged)
	}
}
