package bridge

import (
	"bytes"
	"context"
	"errors"
	"os"
	"reflect"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/ipcenv"
	"github.com/mickeyyaya/evolve-loop/go/internal/proctree"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

var reapT0 = time.Date(2026, 10, 8, 14, 0, 0, 0, time.UTC)

type panePIDTmux struct {
	fakeTmux
	panePIDs []int
	events   *[]string
}

func (f *panePIDTmux) PanePIDs(_ context.Context, session string) ([]int, error) {
	*f.events = append(*f.events, "pane-pids:"+session)
	return f.panePIDs, nil
}

func (f *panePIDTmux) KillSession(ctx context.Context, name string) error {
	*f.events = append(*f.events, "kill-session:"+name)
	return f.fakeTmux.KillSession(ctx, name)
}

type hostTable struct {
	tables [][]proctree.Process
	calls  int
	events *[]string
	sent   []int
	sigs   []syscall.Signal
}

func (h *hostTable) list(context.Context) ([]proctree.Process, error) {
	*h.events = append(*h.events, "list")
	i := min(h.calls, len(h.tables)-1)
	h.calls++
	return withSelf(h.tables[i]), nil
}

func withSelf(table []proctree.Process) []proctree.Process {
	return append(slices.Clone(table), hostProc(os.Getpid(), os.Getppid(), ""))
}

func (h *hostTable) signal(pid int, sig syscall.Signal) error {
	h.sent, h.sigs = append(h.sent, pid), append(h.sigs, sig)
	return nil
}

func hostProc(pid, ppid int, tag string) proctree.Process {
	p := proctree.Process{Pid: pid, Ppid: ppid, Pgid: pid, Started: reapT0, Comm: "x", Env: map[string]string{}}
	if tag != "" {
		p.Env[ipcenv.DispatchIDKey] = tag
	}
	return p
}

func reapDeps(h *hostTable, tmux TmuxController, stderr *bytes.Buffer, center *signalcenter.Center) Deps {
	return Deps{Tmux: tmux, ListProcesses: h.list, SignalProcess: h.signal, Sleep: func(time.Duration) {}, Stderr: stderr, Signals: center}
}

const thisDispatch = "01RUN/1835/build/p4242n1t1"

func TestTmuxCleanup_RecordsThePaneTreeBeforeKillSessionAndTheSweepStopsOnlyOwnedProcesses(t *testing.T) {
	var events []string
	table := []proctree.Process{
		hostProc(500, 400, ""),
		hostProc(501, 500, thisDispatch),
		hostProc(503, 501, ""),
		hostProc(502, 1, thisDispatch),
		hostProc(600, 1, ""),
		hostProc(700, 1, "01RUN/1835/build/p4242n2t1"),
	}
	h := &hostTable{tables: [][]proctree.Process{table, table, table, nil}, events: &events}
	tmux := &panePIDTmux{fakeTmux: fakeTmux{existing: map[string]bool{"s1": true}}, panePIDs: []int{500}, events: &events}
	var stderr bytes.Buffer
	deps := reapDeps(h, tmux, &stderr, nil)
	deps.sweep = &dispatchSweep{id: thisDispatch}

	tmuxCleanup(context.Background(), deps, "agy-tmux", "s1", t.TempDir()+"/scrollback.txt", false, 10)
	deps.sweep.finish(context.Background(), deps)

	if want := []string{"pane-pids:s1", "list", "kill-session:s1"}; len(events) < 3 || !reflect.DeepEqual(events[:3], want) {
		t.Fatalf("events = %v, want the pane tree recorded before kill-session (%v)", events, want)
	}
	slices.Sort(h.sent)
	if want := []int{500, 501, 502, 503}; !reflect.DeepEqual(h.sent, want) {
		t.Errorf("SIGTERM went to %v, want %v: the recorded pane tree and the tagged orphan, never 600 or the other dispatch's 700", h.sent, want)
	}
}

func TestTmuxCleanup_ANamedSessionIsNeverSwept(t *testing.T) {
	var events []string
	h := &hostTable{tables: [][]proctree.Process{{hostProc(501, 500, thisDispatch)}}, events: &events}
	tmux := &panePIDTmux{fakeTmux: fakeTmux{existing: map[string]bool{"evolve-bridge-named-x": true}}, panePIDs: []int{500}, events: &events}
	var stderr bytes.Buffer
	deps := reapDeps(h, tmux, &stderr, nil)
	deps.sweep = &dispatchSweep{id: thisDispatch}

	tmuxCleanup(context.Background(), deps, "claude-tmux", "evolve-bridge-named-x", t.TempDir()+"/sb.txt", true, 10)
	deps.sweep.finish(context.Background(), deps)

	if len(h.sent) != 0 || h.calls != 0 {
		t.Errorf("signals=%v listings=%d, want none: a named session keeps its processes for the next dispatch", h.sent, h.calls)
	}
}

func TestDispatchSweep_ASurvivorIsAnIncidentSignal(t *testing.T) {
	var events []string
	survivor := hostProc(502, 1, thisDispatch)
	h := &hostTable{tables: [][]proctree.Process{{survivor}}, events: &events}
	center := signalcenter.New()
	var stderr bytes.Buffer
	deps := reapDeps(h, &fakeTmux{}, &stderr, center)
	sweep := &dispatchSweep{id: thisDispatch}

	sweep.finish(context.Background(), deps)
	center.Flush()

	var got []signalcenter.Event
	for _, e := range center.Recent() {
		if e.Code == CodeDispatchProcessSurvived {
			got = append(got, e)
		}
	}
	if len(got) != 1 || got[0].Severity != signalcenter.SeverityIncident || got[0].Fields["pids"] != "502" || got[0].Fields["dispatch"] != thisDispatch {
		t.Fatalf("survivor events = %+v, want one INCIDENT that names pid 502 and the dispatch", got)
	}
	if !strings.Contains(stderr.String(), "ERROR") || !strings.Contains(stderr.String(), "502") {
		t.Errorf("stderr = %q, want a loud ERROR line that names the survivor", stderr.String())
	}
}

func TestDispatchSweep_AFailedListingIsAWarnSignal(t *testing.T) {
	center := signalcenter.New()
	var stderr bytes.Buffer
	deps := Deps{ListProcesses: func(context.Context) ([]proctree.Process, error) { return nil, errors.New("ps: not found") },
		SignalProcess: func(int, syscall.Signal) error { t.Fatal("signal without a listing"); return nil },
		Sleep:         func(time.Duration) {}, Stderr: &stderr, Signals: center}

	(&dispatchSweep{id: thisDispatch}).finish(context.Background(), deps)
	center.Flush()

	if !slices.ContainsFunc(center.Recent(), func(e signalcenter.Event) bool { return e.Code == CodeDispatchReapFailed }) {
		t.Errorf("events = %+v, want %s", center.Recent(), CodeDispatchReapFailed)
	}
}

func TestDriverEnv_DropsAnInheritedDispatchTag(t *testing.T) {
	t.Setenv(ipcenv.DispatchIDKey, "stale/1/build/p1n1t1")

	env := driverEnv(Deps{}, nil)

	for _, kv := range env {
		if strings.HasPrefix(kv, ipcenv.DispatchIDKey+"=") {
			t.Fatalf("driverEnv kept %q: a child must not carry the tag of a different dispatch", kv)
		}
	}
}

func TestDispatchEnv_CarriesExactlyTheTagOfThisDispatch(t *testing.T) {
	t.Setenv(ipcenv.DispatchIDKey, "stale/1/build/p1n1t1")
	deps := Deps{Env: map[string]string{ipcenv.DispatchIDKey: "overlay/1/build/p1n2t1"}}
	cfg := &Config{DispatchID: thisDispatch}

	env := dispatchEnv(deps, cfg)

	var tags []string
	for _, kv := range env {
		if v, ok := strings.CutPrefix(kv, ipcenv.DispatchIDKey+"="); ok {
			tags = append(tags, v)
		}
	}
	if !reflect.DeepEqual(tags, []string{thisDispatch}) {
		t.Errorf("tags = %v, want only %s", tags, thisDispatch)
	}
}

var dispatchIDRE = regexp.MustCompile(`^export EVOLVE_DISPATCH_ID='?(norun/0/[^/]+/p\d+n[0-9a-z]+t[0-9a-z]+)'?$`)

func exportedDispatchIDs(keys []string) []string {
	var ids []string
	for _, k := range keys {
		if m := dispatchIDRE.FindStringSubmatch(k); m != nil {
			ids = append(ids, m[1])
		}
	}
	return ids
}

func TestBootTmuxREPL_ExportsTheDispatchTagOfThisLaunch(t *testing.T) {
	fx := newFixture(t, "claude-tmux", "plan")
	tmux := &fakeTmux{}

	runTmux(t, fx, tmux, nil, "--worktree="+t.TempDir())

	ids := exportedDispatchIDs(tmux.sentKeys)
	if len(ids) != 1 || !strings.Contains(ids[0], "/p"+strconv.Itoa(os.Getpid())+"n") {
		t.Fatalf("exported ids = %v from %q, want one id with this bridge's pid", ids, tmux.sentKeys)
	}
	launchAt := slices.IndexFunc(tmux.sentKeys, func(k string) bool { return strings.Contains(k, "claude --model") })
	exportAt := slices.IndexFunc(tmux.sentKeys, func(k string) bool { return dispatchIDRE.MatchString(k) })
	if exportAt < 0 || launchAt < exportAt {
		t.Errorf("export at %d, launch at %d: the tag must be in the shell before the CLI starts", exportAt, launchAt)
	}
}

func TestBootTmuxREPL_AnEmptyDispatchIDUnsetsTheTag(t *testing.T) {
	if got := dispatchTagLine(""); got != "unset "+ipcenv.DispatchIDKey {
		t.Errorf("dispatchTagLine(\"\") = %q, want the unset line: the server env can hold a stale tag", got)
	}
}

func TestLaunchArgs_EachLaunchMintsANewDispatchID(t *testing.T) {
	fx := newFixture(t, "claude-tmux", "plan")
	first, second := &fakeTmux{}, &fakeTmux{}

	runTmux(t, fx, first, nil, "--worktree="+t.TempDir())
	runTmux(t, fx, second, nil, "--worktree="+t.TempDir())

	a, b := exportedDispatchIDs(first.sentKeys), exportedDispatchIDs(second.sentKeys)
	if len(a) != 1 || len(b) != 1 || a[0] == b[0] {
		t.Errorf("ids = %v and %v, want two different ids", a, b)
	}
}

func TestLaunchArgs_SweepsTheTagAfterTheDriverReturns(t *testing.T) {
	fx := newFixture(t, "claude-tmux", "plan")
	tmux := &fakeTmux{}
	var sent []int
	list := func(context.Context) ([]proctree.Process, error) {
		ids := exportedDispatchIDs(tmux.sentKeys)
		if len(ids) == 0 {
			return withSelf(nil), nil
		}
		if len(sent) > 0 {
			return withSelf(nil), nil
		}
		return withSelf([]proctree.Process{hostProc(31337, 1, ids[0]), hostProc(31338, 1, "")}), nil
	}
	eng := NewEngine(Deps{Tmux: tmux, Sleep: func(time.Duration) {}, LookupEnv: mapLookup(nil), CaptureBaseline: zeroBaselineCapture,
		ListProcesses: list, SignalProcess: func(pid int, _ syscall.Signal) error { sent = append(sent, pid); return nil }})
	var stdout, stderr bytes.Buffer

	eng.LaunchArgs(context.Background(), fx.args("claude-tmux", "--worktree="+t.TempDir()), nil, &stdout, &stderr)

	if !reflect.DeepEqual(sent, []int{31337}) {
		t.Errorf("signaled %v, want [31337]: the tagged orphan of this launch, not the untagged 31338", sent)
	}
}

func TestParsePanePIDs_ReadsOnePidPerLineAndRefusesGarbage(t *testing.T) {
	got, err := parsePanePIDs("30465\n30470\n\n")
	if err != nil || !reflect.DeepEqual(got, []int{30465, 30470}) {
		t.Errorf("parsePanePIDs = %v, %v, want [30465 30470]", got, err)
	}
	if _, err := parsePanePIDs("30465\nno server running\n"); err == nil {
		t.Errorf("parsePanePIDs accepted a line that is not a pid")
	}
}

func TestMintDispatchID_ParsesBackToThisLaunch(t *testing.T) {
	id := mintDispatchID(&Config{RunID: "01RUN", Cycle: 1835, Agent: "build"}, 4242)

	got, ok := proctree.ParseDispatchID(id)

	if !ok || got.Run != "01RUN" || got.Cycle != 1835 || got.Agent != "build" || got.Owner != 4242 {
		t.Errorf("ParseDispatchID(%q) = %+v, %v: gc reads the owner and the cycle from this tag", id, got, ok)
	}
}

func sharedHelpers(tag string) []proctree.Process {
	daemon := hostProc(800, 1, tag)
	daemon.Comm, daemon.Args = "claude", []string{"claude", "daemon", "run"}
	chrome := hostProc(801, 1, tag)
	chrome.Comm = "Google Chrome Helper (Renderer)"
	server := hostProc(802, 1, tag)
	server.Comm, server.Args = "/opt/homebrew/bin/tmux", []string{"tmux", "-L", "evolve-bridge", "new-session"}
	return []proctree.Process{daemon, chrome, server}
}

func TestDispatchSweep_NeverStopsATaggedSharedHelperOutsideTheRecordedTree(t *testing.T) {
	var events []string
	helpers := sharedHelpers(thisDispatch)
	table := append(helpers, hostProc(803, 1, thisDispatch))
	h := &hostTable{tables: [][]proctree.Process{table, nil}, events: &events}
	var stderr bytes.Buffer

	(&dispatchSweep{id: thisDispatch, recorded: []proctree.Identity{helpers[0].Identity()}}).finish(context.Background(), reapDeps(h, &fakeTmux{}, &stderr, nil))

	if want := []int{803}; !reflect.DeepEqual(h.sent, want) {
		t.Errorf("signaled %v, want %v: the shared helpers are spared; the daemon is in the recorded tree but is no live descendant of a pane", h.sent, want)
	}
}

func TestRecordPane_KeepsEveryMemberAndPersistsTheTreeForGC(t *testing.T) {
	var events []string
	root := t.TempDir()
	first := []proctree.Process{hostProc(500, 400, ""), hostProc(501, 500, "")}
	second := []proctree.Process{hostProc(500, 400, ""), hostProc(501, 1, ""), hostProc(502, 500, "")}
	h := &hostTable{tables: [][]proctree.Process{first, second}, events: &events}
	tmux := &panePIDTmux{panePIDs: []int{500}, events: &events}
	var stderr bytes.Buffer
	deps := reapDeps(h, tmux, &stderr, nil)
	sweep := newDispatchSweep(thisDispatch, root)

	sweep.recordPane(context.Background(), deps, "s1")
	sweep.recordPane(context.Background(), deps, "s1")

	got, err := proctree.LoadTree(dispatchTreeDir(root), thisDispatch)
	if err != nil || len(got) != 3 {
		t.Fatalf("persisted tree = %+v, %v, want 500, 501 and 502: 501 left the pane tree but stays a member", got, err)
	}
}

func TestDispatchSweep_ACleanSweepRemovesThePersistedTree(t *testing.T) {
	var events []string
	root := t.TempDir()
	h := &hostTable{tables: [][]proctree.Process{{hostProc(500, 400, "")}, nil}, events: &events}
	var stderr bytes.Buffer
	deps := reapDeps(h, &panePIDTmux{panePIDs: []int{500}, events: &events}, &stderr, nil)
	sweep := newDispatchSweep(thisDispatch, root)
	sweep.recordPane(context.Background(), deps, "s1")

	sweep.finish(context.Background(), deps)

	if _, err := os.Stat(proctree.TreeFile(dispatchTreeDir(root), thisDispatch)); !os.IsNotExist(err) {
		t.Errorf("the tree file is still there after a clean sweep: %v", err)
	}
}

func TestKillSessionSwept_StopsTheRecordedTreeOutsideALaunch(t *testing.T) {
	var events []string
	tree := []proctree.Process{hostProc(500, 400, ""), hostProc(501, 500, "")}
	h := &hostTable{tables: [][]proctree.Process{tree, tree, tree, nil}, events: &events}
	var stderr bytes.Buffer
	deps := reapDeps(h, &panePIDTmux{panePIDs: []int{500}, events: &events}, &stderr, nil)

	killSessionSwept(context.Background(), deps, "evolve-recipe-x")

	if len(events) < 3 || !reflect.DeepEqual(events[:3], []string{"pane-pids:evolve-recipe-x", "list", "kill-session:evolve-recipe-x"}) || !reflect.DeepEqual(h.sent, []int{500, 501}) {
		t.Errorf("events=%v signals=%v, want record, kill-session, then SIGTERM to 500 and 501", events, h.sent)
	}
}

func TestKillSessionSwept_AKilledNamedSessionIsSweptAfterAll(t *testing.T) {
	var events []string
	h := &hostTable{tables: [][]proctree.Process{{hostProc(500, 400, "")}}, events: &events}
	var stderr bytes.Buffer
	deps := reapDeps(h, &panePIDTmux{panePIDs: []int{500}, events: &events}, &stderr, nil)
	deps.sweep = &dispatchSweep{id: thisDispatch, preserved: true}

	killSessionSwept(context.Background(), deps, "evolve-bridge-named-x")

	if deps.sweep.preserved {
		t.Errorf("a named session that was killed is not preserved: its processes must be swept")
	}
}

func TestReplWaiterPace_RefreshesThePaneTree(t *testing.T) {
	var events []string
	h := &hostTable{tables: [][]proctree.Process{{hostProc(500, 400, "")}}, events: &events}
	var stderr bytes.Buffer
	deps := reapDeps(h, &panePIDTmux{panePIDs: []int{500}, events: &events}, &stderr, nil)
	deps.sweep = &dispatchSweep{id: thisDispatch}
	w := replWaiter{ctx: context.Background(), deps: deps, launch: tmuxLaunch{session: "s1"}}

	w.pace()

	if len(deps.sweep.recorded) != 1 || deps.sweep.recorded[0].Pid != 500 {
		t.Errorf("recorded = %+v, want the pane pid 500 after one poll", deps.sweep.recorded)
	}
}

func TestBoundedCommand_StartsTmuxWithoutTheDispatchTag(t *testing.T) {
	t.Setenv(ipcenv.DispatchIDKey, "parent/1/build/p9n1")

	cmd := boundedCommand(context.Background(), "tmux", "new-session", "-d")

	if slices.ContainsFunc(cmd.Env, isDispatchTag) || len(cmd.Env) == 0 {
		t.Errorf("tmux env carries the tag or is empty (%d entries): a tmux server started with a tag is a tagged shared helper", len(cmd.Env))
	}
}

type goneAtCleanupTmux struct{ panePIDTmux }

func (f *goneAtCleanupTmux) HasSession(context.Context, string) bool { return false }

func (f *goneAtCleanupTmux) PasteBuffer(ctx context.Context, session string) error {
	*f.events = append(*f.events, "paste")
	return f.fakeTmux.PasteBuffer(ctx, session)
}

func TestLaunchArgs_ASessionGoneBeforeCleanupStillHasItsRecordedTree(t *testing.T) {
	fx := newFixture(t, "claude-tmux", "plan")
	var events []string
	tmux := &goneAtCleanupTmux{panePIDTmux{fakeTmux: fakeTmux{paneSeq: []string{"❯ "}}, panePIDs: []int{500}, events: &events}}
	var sent []int
	tree := []proctree.Process{hostProc(500, 400, ""), hostProc(501, 500, "")}
	list := func(context.Context) ([]proctree.Process, error) {
		if len(sent) > 0 {
			return withSelf(nil), nil
		}
		return withSelf(tree), nil
	}
	eng := NewEngine(Deps{Tmux: tmux, Sleep: func(time.Duration) {}, LookupEnv: mapLookup(nil), CaptureBaseline: zeroBaselineCapture,
		ListProcesses: list, SignalProcess: func(pid int, _ syscall.Signal) error { sent = append(sent, pid); return nil }})
	var stdout, stderr bytes.Buffer

	eng.LaunchArgs(context.Background(), fx.args("claude-tmux", "--worktree="+t.TempDir()), nil, &stdout, &stderr)

	paste := slices.Index(events, "paste")
	if paste < 1 || !strings.HasPrefix(events[0], "pane-pids:") {
		t.Errorf("events = %v, want the pane tree recorded at dispatch, before the prompt paste", events)
	}
	if !reflect.DeepEqual(sent, []int{500, 501}) {
		t.Errorf("signaled %v (events %v), want the tree recorded at dispatch and at each poll: 500 and 501; stderr=%s", sent, events, stderr.String())
	}
}

func TestDispatchSweep_ARecordedSharedHelperIsStoppedOnlyWhileItIsALiveDescendantOfThePane(t *testing.T) {
	var events []string
	daemon := hostProc(800, 500, "")
	daemon.Comm, daemon.Args = "claude", []string{"claude", "daemon", "run"}
	detached := daemon
	detached.Ppid = 1
	recordTime := []proctree.Process{hostProc(500, 400, ""), daemon}
	cases := []struct {
		name  string
		sweep []proctree.Process
		want  []int
	}{
		{"still under the pane", recordTime, []int{500, 800}},
		{"detached from the pane", []proctree.Process{hostProc(500, 400, ""), detached}, []int{500}},
	}
	for _, tc := range cases {
		h := &hostTable{tables: [][]proctree.Process{recordTime, tc.sweep, tc.sweep, nil}, events: &events}
		var stderr bytes.Buffer
		deps := reapDeps(h, &panePIDTmux{panePIDs: []int{500}, events: &events}, &stderr, nil)
		sweep := &dispatchSweep{id: thisDispatch}
		sweep.recordPane(context.Background(), deps, "s1")

		sweep.finish(context.Background(), deps)

		if !reflect.DeepEqual(h.sent, tc.want) {
			t.Errorf("%s: signaled %v, want %v", tc.name, h.sent, tc.want)
		}
	}
}
