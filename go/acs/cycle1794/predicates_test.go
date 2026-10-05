//go:build acs

package cycle1794

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/runlease"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
	"github.com/mickeyyaya/evolve-loop/go/internal/sizeratchet"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
	"github.com/mickeyyaya/evolve-loop/go/test/structure"
)

const (
	baseSHA     = "0f072732bb7a4493220ded7239c3a20758687d94"
	usageExit   = 10
	streamGo    = "go/internal/signalcenter/stream.go"
	tailGo      = "go/cmd/evolve/cmd_signals_tail.go"
	verdictFail = signalcenter.Code("ORCHESTRATOR_PHASE_VERDICT_FAIL")
	contractOK  = signalcenter.Code("GATE_CONTRACT_VERIFIED")
	cycleFailed = signalcenter.Code("ORCHESTRATOR_CYCLE_FAILED")
	outcome     = signalcenter.KindPhaseOutcome
	dispatched  = signalcenter.KindPhaseDispatched
)

func TestMain(m *testing.M) {
	code := m.Run()
	for _, b := range []*builtBinary{evolveBuild, probeBuild, commentauditBuild, apicoverBuild} {
		if b.dir == "" {
			continue
		}
		if err := os.RemoveAll(b.dir); err != nil {
			fmt.Fprintf(os.Stderr, "cycle1794: remove %s: %v\n", b.dir, err)
		}
	}
	os.Exit(code)
}

type builtBinary struct {
	once      sync.Once
	pkg, name string
	dir, path string
	failure   string
}

var (
	evolveBuild       = &builtBinary{pkg: "./cmd/evolve", name: "evolve"}
	probeBuild        = &builtBinary{pkg: "./acs/cycle1794/testdata/streamprobe", name: "streamprobe"}
	commentauditBuild = &builtBinary{pkg: "./cmd/commentaudit", name: "commentaudit"}
	apicoverBuild     = &builtBinary{pkg: "./cmd/apicover", name: "apicover"}
)

func (b *builtBinary) get(t *testing.T) string {
	t.Helper()
	b.once.Do(func() { b.failure = b.build(filepath.Join(acsassert.RepoRoot(t), "go")) })
	if b.failure != "" {
		t.Fatalf("go build %s: %s", b.pkg, b.failure)
	}
	return b.path
}

func (b *builtBinary) build(goDir string) string {
	dir, err := os.MkdirTemp("", "cycle1794-"+b.name+"-")
	if err != nil {
		return err.Error()
	}
	b.dir, b.path = dir, filepath.Join(dir, b.name)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, "go", "build", "-o", b.path, b.pkg)
	cmd.Dir = goDir
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Sprintf("%v\n%s", err, out)
	}
	return ""
}

func evolveBin(t *testing.T) string { return evolveBuild.get(t) }

type result struct {
	stdout, stderr string
	code           int
}

func (r result) String() string {
	return fmt.Sprintf("exit=%d\n--- stdout ---\n%s--- stderr ---\n%s", r.code, r.stdout, r.stderr)
}

func boundedCtx(t *testing.T) context.Context {
	t.Helper()
	parent, cancelParent := context.WithTimeout(context.Background(), 2*time.Minute)
	t.Cleanup(cancelParent)
	deadline, ok := t.Deadline()
	if !ok {
		return parent
	}
	ctx, cancel := context.WithDeadline(parent, deadline.Add(-10*time.Second))
	t.Cleanup(cancel)
	return ctx
}

func exitResult(err error, stdout, stderr string) result {
	r := result{stdout: stdout, stderr: stderr}
	var exitErr *exec.ExitError
	switch {
	case err == nil:
	case errors.As(err, &exitErr):
		r.code = exitErr.ExitCode()
	default:
		r.code, r.stderr = -1, stderr+"\n"+err.Error()
	}
	return r
}

func run(t *testing.T, dir string, env []string, stdin string, bin string, args ...string) result {
	t.Helper()
	cmd := exec.CommandContext(boundedCtx(t), bin, args...)
	cmd.Dir, cmd.Env, cmd.WaitDelay = dir, env, 5*time.Second
	cmd.Stdin = strings.NewReader(stdin)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	return exitResult(err, stdout.String(), stderr.String())
}

func scrubbedEnv(root string) []string {
	var env []string
	for _, kv := range os.Environ() {
		if !strings.HasPrefix(kv, "EVOLVE_") {
			env = append(env, kv)
		}
	}
	return append(env, "EVOLVE_PROJECT_ROOT="+root)
}

func tail(t *testing.T, root string, args ...string) result {
	t.Helper()
	return run(t, root, scrubbedEnv(root), "", evolveBin(t), append([]string{"signals", "tail"}, args...)...)
}

func newProject(t *testing.T) string {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return root
}

func runsDir(root string) string { return filepath.Join(root, ".evolve", "runs") }

func laneDir(root string, cycle int) string {
	return filepath.Join(runsDir(root), "cycle-"+strconv.Itoa(cycle))
}

func laneStream(root string, cycle int) string {
	return filepath.Join(laneDir(root, cycle), signalcenter.StreamFileName)
}

func rootStream(root string) string {
	return filepath.Join(root, ".evolve", signalcenter.StreamFileName)
}

func at(clock string) string { return "2026-10-05T10:" + clock + "Z" }

func ev(cycle int, seq uint64, ts string, kind signalcenter.Kind, phase string) signalcenter.Event {
	return signalcenter.Event{
		SchemaVersion: "signal/1.0", Seq: seq, PID: 7, TS: ts, Cycle: cycle, RunID: "run-" + strconv.Itoa(cycle),
		Phase: phase, Module: signalcenter.ModuleOrchestrator, Origin: "Fixture.emit", Kind: kind,
		Severity: signalcenter.SeverityInfo, Reason: string(kind) + " " + phase,
	}
}

func coded(e signalcenter.Event, code signalcenter.Code, severity signalcenter.Severity) signalcenter.Event {
	e.Code, e.Severity = code, severity
	return e
}

func withFields(e signalcenter.Event, kv ...string) signalcenter.Event {
	e.Fields = map[string]string{}
	for i := 0; i+1 < len(kv); i += 2 {
		e.Fields[kv[i]] = kv[i+1]
	}
	return e
}

func sealed(cycle int, seq uint64, ts string) signalcenter.Event {
	return withFields(coded(ev(cycle, seq, ts, signalcenter.KindCycleSealed, ""), cycleFailed, signalcenter.SeverityWarn), "final_verdict", "FAIL")
}

func cycleThreeEvents() []signalcenter.Event {
	audit := coded(ev(3, 5, at("00:06"), outcome, "audit"), verdictFail, signalcenter.SeverityWarn)
	audit.Attempt = 2
	landed := withFields(ev(3, 6, at("00:07.25"), signalcenter.KindShipLanded, "ship"), "sha", "c0ffee1794")
	landed.Module = signalcenter.ModuleShip
	return []signalcenter.Event{
		ev(3, 1, at("00:05"), dispatched, "build"),
		ev(3, 2, at("00:05.1"), signalcenter.KindLedgerAppended, ""),
		withFields(ev(3, 3, at("00:05.4396"), outcome, "build"), "verdict", "PASS"),
		coded(ev(3, 4, at("00:05.43965"), signalcenter.KindGatePassed, "audit"), contractOK, signalcenter.SeverityInfo),
		withFields(audit, "verdict", "FAIL"),
		landed,
		sealed(3, 7, at("00:08")),
	}
}

func pick(events []signalcenter.Event, idx ...int) []signalcenter.Event {
	out := make([]signalcenter.Event, 0, len(idx))
	for _, i := range idx {
		out = append(out, events[i])
	}
	return out
}

func shuffled(events []signalcenter.Event) []signalcenter.Event {
	return pick(events, 5, 2, 0, 6, 1, 4, 3)
}

func ndjson(t *testing.T, events ...signalcenter.Event) []byte {
	t.Helper()
	var b bytes.Buffer
	for _, e := range events {
		line, err := json.Marshal(e)
		if err != nil {
			t.Fatal(err)
		}
		b.Write(line)
		b.WriteByte('\n')
	}
	return b.Bytes()
}

func human(e signalcenter.Event) string { return e.TS + " " + signalcenter.FormatLine(e) }

func humanLines(events ...signalcenter.Event) string {
	var b strings.Builder
	for _, e := range events {
		b.WriteString(human(e) + "\n")
	}
	return b.String()
}

func writeFile(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func appendFile(t *testing.T, path string, data []byte) {
	t.Helper()
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.Write(data); err != nil {
		_ = f.Close()
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
}

func liveLease(t *testing.T, dir string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := runlease.Write(dir, runlease.Lease{RunID: "live", OwnerPID: os.Getpid()}, time.Now()); err != nil {
		t.Fatal(err)
	}
}

func staleLease(t *testing.T, dir string) {
	t.Helper()
	longAgo := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	if err := runlease.Write(dir, runlease.Lease{RunID: "stale", OwnerPID: os.Getpid()}, longAgo); err != nil {
		t.Fatal(err)
	}
}

type follower struct {
	cmd    *exec.Cmd
	ctx    context.Context
	lines  chan string
	stdout []string
	stderr bytes.Buffer
}

func startTail(t *testing.T, root string, args ...string) *follower {
	t.Helper()
	ctx := boundedCtx(t)
	cmd := exec.CommandContext(ctx, evolveBin(t), append([]string{"signals", "tail"}, args...)...)
	cmd.Dir, cmd.Env, cmd.WaitDelay = root, scrubbedEnv(root), 5*time.Second
	f := &follower{cmd: cmd, ctx: ctx, lines: make(chan string, 256)}
	cmd.Stderr = &f.stderr
	out, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	go func() {
		sc := bufio.NewScanner(out)
		sc.Buffer(make([]byte, 0, 64*1024), 16*1024*1024)
		for sc.Scan() {
			f.lines <- sc.Text()
		}
		close(f.lines)
	}()
	t.Cleanup(func() { f.reap() })
	return f
}

func (f *follower) reap() {
	if f.cmd.ProcessState != nil {
		return
	}
	_ = f.cmd.Process.Kill()
	go func() {
		for range f.lines {
		}
	}()
	_ = f.cmd.Wait()
}

func (f *follower) waitFor(t *testing.T, want string) {
	t.Helper()
	for {
		select {
		case line, ok := <-f.lines:
			if !ok {
				t.Fatalf("tail ended before printing %q:\n%s", want, f.finish())
			}
			f.stdout = append(f.stdout, line)
			if line == want {
				return
			}
		case <-f.ctx.Done():
			t.Fatalf("tail never printed %q within the test's bound:\n%s", want, f.finish())
		}
	}
}

func (f *follower) finish() result {
	for line := range f.lines {
		f.stdout = append(f.stdout, line)
	}
	err := f.cmd.Wait()
	var out strings.Builder
	for _, line := range f.stdout {
		out.WriteString(line + "\n")
	}
	return exitResult(err, out.String(), f.stderr.String())
}

func countLine(stdout, line string) int {
	n := 0
	for _, got := range strings.Split(stdout, "\n") {
		if got == line {
			n++
		}
	}
	return n
}

func absent(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("signals tail must be read-only, but %s now exists (stat err=%v)", path, err)
	}
}

func TestC1794_001_CycleModePrintsEveryEventOfThatStreamInInstantOrder(t *testing.T) {
	root := newProject(t)
	events := cycleThreeEvents()
	writeFile(t, laneStream(root, 3), ndjson(t, shuffled(events)...))
	writeFile(t, laneStream(root, 4), ndjson(t, ev(4, 1, at("00:05.2"), outcome, "scout")))
	r := tail(t, root, "--cycle", "3")
	if want := humanLines(events...); r.code != 0 || r.stdout != want {
		t.Fatalf("--cycle 3 must print exactly cycle 3's events, ordered by parsed instant, as TS+\" \"+FormatLine:\n%s\n--- want stdout ---\n%s", r, want)
	}
}

func TestC1794_002_CycleWithoutStreamExitsOneNamingThePath(t *testing.T) {
	root := newProject(t)
	r := tail(t, root, "--cycle", "41")
	if r.code != 1 || r.stdout != "" || !strings.Contains(r.stderr, laneStream(root, 41)) {
		t.Errorf("a --cycle with no run dir must exit 1 and name %s on stderr:\n%s", laneStream(root, 41), r)
	}
	absent(t, filepath.Join(root, ".evolve"))
	if err := os.MkdirAll(laneDir(root, 42), 0o755); err != nil {
		t.Fatal(err)
	}
	r = tail(t, root, "--cycle", "42")
	if r.code != 1 || r.stdout != "" || !strings.Contains(r.stderr, laneStream(root, 42)) {
		t.Errorf("a run dir without signals.ndjson must exit 1 and name %s on stderr:\n%s", laneStream(root, 42), r)
	}
	absent(t, laneStream(root, 42))
}

func TestC1794_003_WaveModeMergesOnlyLiveLanesByInstant(t *testing.T) {
	root := newProject(t)
	nine := []signalcenter.Event{
		ev(9, 1, at("00:05.1"), dispatched, "scout"),
		ev(9, 2, at("00:05.43965"), outcome, "scout"),
		ev(9, 3, at("00:07"), outcome, "build"),
	}
	ten := []signalcenter.Event{
		ev(10, 1, at("00:05"), dispatched, "scout"),
		ev(10, 2, at("00:05.4396"), outcome, "scout"),
		withFields(ev(10, 3, at("00:07"), signalcenter.KindShipLanded, "ship"), "sha", "feed10"),
	}
	writeFile(t, laneStream(root, 9), ndjson(t, nine...))
	liveLease(t, laneDir(root, 9))
	writeFile(t, laneStream(root, 10), ndjson(t, ten...))
	liveLease(t, laneDir(root, 10))
	liveLease(t, laneDir(root, 12))
	writeFile(t, laneStream(root, 11), ndjson(t, ev(11, 1, at("00:06"), outcome, "scout")))
	writeFile(t, laneStream(root, 8), ndjson(t, ev(8, 1, at("00:06.5"), outcome, "scout")))
	staleLease(t, laneDir(root, 8))
	r := tail(t, root)
	want := humanLines(ten[0], nine[0], ten[1], nine[1], nine[2], ten[2])
	if r.code != 0 || r.stdout != want {
		t.Fatalf("wave mode must merge only the leased-live lanes 9 and 10 by parsed instant, ties in cycle-number order, excluding unleased cycle-11 and stale-leased cycle-8:\n%s\n--- want stdout ---\n%s", r, want)
	}
}

func TestC1794_004_WaveFallbackReadsTheNumericallyNewestStream(t *testing.T) {
	root := newProject(t)
	writeFile(t, laneStream(root, 99), ndjson(t, ev(99, 1, at("00:01"), outcome, "scout")))
	newest := []signalcenter.Event{ev(1000, 1, at("00:02"), outcome, "build"), sealed(1000, 2, at("00:03"))}
	writeFile(t, laneStream(root, 1000), ndjson(t, newest...))
	writeFile(t, filepath.Join(runsDir(root), "cycle-5000x", signalcenter.StreamFileName), ndjson(t, ev(5000, 1, at("00:04"), outcome, "scout")))
	writeFile(t, filepath.Join(runsDir(root), "notacycle", signalcenter.StreamFileName), ndjson(t, ev(7000, 1, at("00:05"), outcome, "scout")))
	if err := os.MkdirAll(laneDir(root, 2000), 0o755); err != nil {
		t.Fatal(err)
	}
	r := tail(t, root)
	if want := humanLines(newest...); r.code != 0 || r.stdout != want {
		t.Errorf("with no live lane the wave is the numerically newest cycle-<int> dir that has a stream (cycle-1000, not cycle-99, cycle-5000x, notacycle or the streamless cycle-2000):\n%s\n--- want stdout ---\n%s", r, want)
	}
	empty := newProject(t)
	r = tail(t, empty)
	if r.code != 0 || r.stdout != "" || !strings.Contains(r.stderr, "no signal streams") {
		t.Errorf("a project with no streams must exit 0, print nothing and say \"no signal streams\" on stderr:\n%s", r)
	}
	absent(t, filepath.Join(empty, ".evolve"))
}

func TestC1794_005_KindAndCodeFiltersGateOnlyTheOutput(t *testing.T) {
	root := newProject(t)
	events := cycleThreeEvents()
	writeFile(t, laneStream(root, 3), ndjson(t, shuffled(events)...))
	cases := []struct {
		args []string
		want []signalcenter.Event
	}{
		{[]string{"--kind", "ship.landed"}, pick(events, 5)},
		{[]string{"--kind", "phase.outcome,cycle.sealed"}, pick(events, 2, 4, 6)},
		{[]string{"--kind", "ledger.appended"}, pick(events, 1)},
		{[]string{"--kind", "loop.halt"}, nil},
		{[]string{"--code", string(verdictFail)}, pick(events, 4)},
		{[]string{"--code", string(contractOK) + "," + string(cycleFailed)}, pick(events, 3, 6)},
		{[]string{"--kind", "phase.outcome", "--code", string(verdictFail)}, pick(events, 4)},
	}
	for _, c := range cases {
		r := tail(t, root, append([]string{"--cycle", "3"}, c.args...)...)
		if want := humanLines(c.want...); r.code != 0 || r.stdout != want {
			t.Errorf("signals tail --cycle 3 %s:\n%s\n--- want stdout ---\n%s", strings.Join(c.args, " "), r, want)
		}
	}
}

func decodeJSONLines(t *testing.T, stdout string) []signalcenter.Event {
	t.Helper()
	var events []signalcenter.Event
	for _, line := range strings.Split(strings.TrimSuffix(stdout, "\n"), "\n") {
		if line == "" {
			continue
		}
		dec := json.NewDecoder(strings.NewReader(line))
		dec.DisallowUnknownFields()
		var e signalcenter.Event
		if err := dec.Decode(&e); err != nil {
			t.Fatalf("--json line is not one signal/1.0 Event: %v\n%s", err, line)
		}
		if !strings.Contains(line, `"schema_version":"signal/1.0"`) {
			t.Errorf("--json line lost the schema version: %s", line)
		}
		events = append(events, e)
	}
	return events
}

func TestC1794_006_JSONEmitsOneSignalEventPerLine(t *testing.T) {
	root := newProject(t)
	events := cycleThreeEvents()
	writeFile(t, laneStream(root, 3), ndjson(t, shuffled(events)...))
	r := tail(t, root, "--cycle", "3", "--json")
	if r.code != 0 {
		t.Fatalf("--json must exit 0:\n%s", r)
	}
	if got := decodeJSONLines(t, r.stdout); !reflect.DeepEqual(got, events) {
		t.Errorf("--json must re-encode every event unchanged in instant order:\n%s", r)
	}
	r = tail(t, root, "--cycle", "3", "--json", "--kind", "ship.landed")
	got := decodeJSONLines(t, r.stdout)
	if r.code != 0 || len(got) != 1 || got[0].Kind != signalcenter.KindShipLanded || got[0].Fields["sha"] != "c0ffee1794" {
		t.Errorf("--json --kind ship.landed must print exactly the landing event with its sha field:\n%s", r)
	}
}

func TestC1794_007_MalformedLinesAreSkippedAndReported(t *testing.T) {
	root := newProject(t)
	good := []signalcenter.Event{
		ev(5, 1, at("00:01"), outcome, "scout"),
		ev(5, 3, at("00:03"), outcome, "triage"),
		ev(5, 4, at("00:04"), outcome, "build"),
	}
	var data []byte
	data = append(data, ndjson(t, good[0])...)
	data = append(data, "{not json\n\n"...)
	data = append(data, ndjson(t, ev(5, 2, "yesterday", outcome, "tdd"))...)
	data = append(data, ndjson(t, good[1])...)
	data = append(data, "   \n"...)
	data = append(data, ndjson(t, good[2])...)
	writeFile(t, laneStream(root, 5), data)
	r := tail(t, root, "--cycle", "5")
	if want := humanLines(good...); r.code != 0 || r.stdout != want {
		t.Errorf("the decodable events must still print and the tail exit 0:\n%s\n--- want stdout ---\n%s", r, want)
	}
	if !strings.Contains(r.stderr, "skipped 2 malformed line(s)") || !strings.Contains(r.stderr, laneStream(root, 5)) {
		t.Errorf("one bad-JSON line and one unparsable-ts line must be reported as \"skipped 2 malformed line(s)\" naming the stream, blank lines uncounted:\n%s", r)
	}
	writeFile(t, laneStream(root, 6), nil)
	r = tail(t, root, "--cycle", "6")
	if r.code != 0 || r.stdout != "" || strings.Contains(r.stderr, "skipped") {
		t.Errorf("an empty stream prints nothing and exits 0:\n%s", r)
	}
}

func TestC1794_008_UnreadableStreamOrRunsDirExitsTwo(t *testing.T) {
	root := newProject(t)
	if err := os.MkdirAll(laneStream(root, 5), 0o755); err != nil {
		t.Fatal(err)
	}
	r := tail(t, root, "--cycle", "5")
	if r.code != 2 || r.stdout != "" || r.stderr == "" {
		t.Errorf("--cycle on a stream path that is a directory is an I/O fault (exit 2), not a missing stream:\n%s", r)
	}
	liveLease(t, laneDir(root, 5))
	r = tail(t, root)
	if r.code != 2 || r.stdout != "" || r.stderr == "" {
		t.Errorf("wave mode with a live lane whose stream is a directory must exit 2:\n%s", r)
	}
	fileRuns := newProject(t)
	writeFile(t, runsDir(fileRuns), []byte("not a directory\n"))
	r = tail(t, fileRuns)
	if r.code != 2 || r.stdout != "" || r.stderr == "" {
		t.Errorf("wave mode whose runs path is a regular file is a listing fault (exit 2):\n%s", r)
	}
}

func TestC1794_009_BadFlagsAndArgumentsAreUsageErrors(t *testing.T) {
	root := newProject(t)
	writeFile(t, laneStream(root, 3), ndjson(t, cycleThreeEvents()...))
	if r := tail(t, root, "--cycle", "3"); r.code != 0 {
		t.Fatalf("control: valid flags against an existing stream must not be a usage error:\n%s", r)
	}
	for _, args := range [][]string{
		{"--bogus"}, {"stray"}, {"--cycle", "3", "stray"}, {"-h"},
		{"--cycle", "0"}, {"--cycle", "-3"}, {"--cycle", "abc"},
		{"--kind", "not.a.kind"}, {"--kind", "phase.outcome,"}, {"--kind", "phase.outcome,,cycle.sealed"},
		{"--code", "lower_case"}, {"--code", string(verdictFail) + ",nope"},
	} {
		r := tail(t, root, args...)
		if r.code != usageExit || r.stdout != "" || r.stderr == "" {
			t.Errorf("signals tail %s must be a usage error (exit %d, message on stderr):\n%s", strings.Join(args, " "), usageExit, r)
		}
	}
}

func TestC1794_010_CycleFollowPrintsAppendedEventsAndEndsOnTheSealDespiteAKindFilter(t *testing.T) {
	root := newProject(t)
	stream := laneStream(root, 12)
	backlog := ev(12, 2, at("00:02"), outcome, "scout")
	writeFile(t, stream, ndjson(t, ev(12, 1, at("00:01"), dispatched, "scout"), backlog))
	f := startTail(t, root, "--cycle", "12", "--follow", "--kind", "phase.outcome")
	f.waitFor(t, human(backlog))
	appended := ev(12, 3, at("00:03"), outcome, "build")
	appendFile(t, stream, ndjson(t, appended))
	f.waitFor(t, human(appended))
	appendFile(t, stream, ndjson(t, ev(12, 4, at("00:04"), signalcenter.KindLedgerAppended, ""), sealed(12, 5, at("00:05"))))
	if r := f.finish(); r.code != 0 || r.stdout != humanLines(backlog, appended) {
		t.Errorf("--follow must print the appended phase.outcome and end by itself (exit 0) on the filtered-out cycle.sealed:\n%s", r)
	}
	done := []signalcenter.Event{ev(13, 1, at("00:01"), outcome, "scout"), sealed(13, 2, at("00:02"))}
	writeFile(t, laneStream(root, 13), ndjson(t, done...))
	if r := tail(t, root, "--cycle", "13", "--follow"); r.code != 0 || r.stdout != humanLines(done...) {
		t.Errorf("--follow on an already-sealed cycle must print its backlog and exit 0:\n%s", r)
	}
}

func TestC1794_011_WaveFollowHoldsFragmentsJoinsNewLanesAndRootAppends(t *testing.T) {
	root := newProject(t)
	history := ev(0, 1, at("00:00"), signalcenter.KindLoopWave, "")
	writeFile(t, rootStream(root), ndjson(t, history))
	b20, b21 := ev(20, 1, at("00:01"), outcome, "scout"), ev(21, 1, at("00:02"), outcome, "scout")
	writeFile(t, laneStream(root, 20), ndjson(t, b20))
	liveLease(t, laneDir(root, 20))
	writeFile(t, laneStream(root, 21), ndjson(t, b21))
	liveLease(t, laneDir(root, 21))
	staleBacklog := ev(19, 1, at("00:01.5"), outcome, "scout")
	writeFile(t, laneStream(root, 19), ndjson(t, staleBacklog))
	f := startTail(t, root, "--follow")
	f.waitFor(t, human(b20))
	f.waitFor(t, human(b21))
	fragment := ev(20, 2, at("00:03"), outcome, "build")
	fragmentLine := ndjson(t, fragment)
	half := len(fragmentLine) / 2
	appendFile(t, laneStream(root, 20), fragmentLine[:half])
	expected := []signalcenter.Event{b20, b21, fragment}
	for i, clock := range []string{"00:03.1", "00:03.2"} {
		marker := ev(21, uint64(2+i), at(clock), signalcenter.KindLedgerAppended, "")
		appendFile(t, laneStream(root, 21), ndjson(t, marker))
		f.waitFor(t, human(marker))
		expected = append(expected, marker)
	}
	appendFile(t, laneStream(root, 20), fragmentLine[half:])
	f.waitFor(t, human(fragment))
	wave := ev(0, 2, at("00:04"), signalcenter.KindLoopWave, "")
	appendFile(t, rootStream(root), ndjson(t, wave))
	f.waitFor(t, human(wave))
	staleAppend := ev(19, 2, at("00:04.5"), outcome, "build")
	appendFile(t, laneStream(root, 19), ndjson(t, staleAppend))
	joined := ev(22, 1, at("00:05"), outcome, "scout")
	liveLease(t, laneDir(root, 22))
	writeFile(t, laneStream(root, 22), ndjson(t, joined))
	f.waitFor(t, human(joined))
	seals := []signalcenter.Event{sealed(20, 3, at("00:06")), sealed(21, 4, at("00:06.1")), sealed(22, 2, at("00:06.2"))}
	for _, s := range seals {
		appendFile(t, laneStream(root, s.Cycle), ndjson(t, s))
	}
	r := f.finish()
	if r.code != 0 || strings.Contains(r.stderr, "skipped") {
		t.Errorf("wave --follow must end by itself (exit 0) once every followed lane is sealed, never reporting the held fragment as malformed:\n%s", r)
	}
	for _, e := range append(append(expected, wave, joined), seals...) {
		if n := countLine(r.stdout, human(e)); n != 1 {
			t.Errorf("expected exactly one %q, got %d:\n%s", human(e), n, r)
		}
	}
	for _, e := range []signalcenter.Event{history, staleBacklog, staleAppend} {
		if n := countLine(r.stdout, human(e)); n != 0 {
			t.Errorf("root-stream history and the unleased lane must never print, got %d of %q:\n%s", n, human(e), r)
		}
	}
}

func TestC1794_012_FollowEndsCleanlyOnInterruptOrTerminate(t *testing.T) {
	for _, sig := range []os.Signal{os.Interrupt, syscall.SIGTERM} {
		root := newProject(t)
		backlog := ev(30, 1, at("00:01"), outcome, "scout")
		writeFile(t, laneStream(root, 30), ndjson(t, backlog))
		f := startTail(t, root, "--cycle", "30", "--follow")
		f.waitFor(t, human(backlog))
		if err := f.cmd.Process.Signal(sig); err != nil {
			t.Fatal(err)
		}
		if r := f.finish(); r.code != 0 || r.stdout != humanLines(backlog) {
			t.Errorf("%v must end an unsealed --follow with exit 0:\n%s", sig, r)
		}
	}
}

func snapshot(t *testing.T, root string) map[string]string {
	t.Helper()
	state := map[string]string{}
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		state[path] = fmt.Sprintf("%v %d %d", info.Mode(), info.Size(), info.ModTime().UnixNano())
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return state
}

func TestC1794_013_TailNeverWritesUnderTheProject(t *testing.T) {
	root := newProject(t)
	events := cycleThreeEvents()
	writeFile(t, laneStream(root, 3), ndjson(t, events...))
	liveLease(t, laneDir(root, 3))
	writeFile(t, rootStream(root), ndjson(t, ev(0, 1, at("00:00"), signalcenter.KindLoopWave, "")))
	before := snapshot(t, root)
	if r := tail(t, root, "--cycle", "3"); r.code != 0 || r.stdout != humanLines(events...) {
		t.Fatalf("precondition: --cycle 3 must print the stream:\n%s", r)
	}
	for _, args := range [][]string{{}, {"--json"}, {"--cycle", "3", "--follow"}, {"--follow"}, {"--cycle", "77"}} {
		if r := tail(t, root, args...); r.code != 0 && r.code != 1 {
			t.Errorf("signals tail %s: unexpected exit:\n%s", strings.Join(args, " "), r)
		}
	}
	if after := snapshot(t, root); !reflect.DeepEqual(before, after) {
		t.Errorf("signals tail changed the project tree (leases, streams or dirs):\nbefore=%v\nafter=%v", before, after)
	}
}

type probeRead struct {
	Events   []signalcenter.Event `json:"events"`
	Next     int64                `json:"next"`
	Skipped  int                  `json:"skipped"`
	Err      string               `json:"err"`
	NotExist bool                 `json:"not_exist"`
}

func readStream(t *testing.T, path string, from int64) probeRead {
	t.Helper()
	r := run(t, filepath.Dir(path), os.Environ(), "", probeBuild.get(t), "read", path, strconv.FormatInt(from, 10))
	var got probeRead
	if r.code != 0 || json.Unmarshal([]byte(r.stdout), &got) != nil {
		t.Fatalf("streamprobe read %s %d:\n%s", path, from, r)
	}
	return got
}

func sameJSON(t *testing.T, got, want any) bool {
	t.Helper()
	g, gerr := json.Marshal(got)
	w, werr := json.Marshal(want)
	if gerr != nil || werr != nil {
		t.Fatalf("marshal: %v %v", gerr, werr)
	}
	return bytes.Equal(g, w)
}

func TestC1794_014_ReadStreamConsumesOnlyCompleteLinesFromAnOffset(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, signalcenter.StreamFileName)
	first, last := ev(1, 1, at("00:01"), outcome, "scout"), ev(1, 4, at("00:04"), outcome, "audit")
	huge := ev(1, 2, at("00:02"), outcome, "build")
	huge.Reason = strings.Repeat("r", 2<<20)
	var complete []byte
	complete = append(complete, ndjson(t, first)...)
	complete = append(complete, "{\"ts\":\n"...)
	complete = append(complete, ndjson(t, ev(1, 3, "not-a-time", outcome, "tdd"))...)
	complete = append(complete, ndjson(t, huge)...)
	complete = append(complete, "  \t\n"...)
	complete = append(complete, ndjson(t, last)...)
	pending := sealed(1, 5, at("00:05"))
	pendingLine := ndjson(t, pending)
	writeFile(t, path, append(append([]byte{}, complete...), pendingLine[:len(pendingLine)-1]...))
	got := readStream(t, path, 0)
	if got.Err != "" || got.Skipped != 2 || got.Next != int64(len(complete)) || !sameJSON(t, got.Events, []signalcenter.Event{first, huge, last}) {
		t.Errorf("ReadStream(0) must decode the 3 good lines (a 2 MiB line included), skip 2, ignore blanks and stop Next before the unterminated fragment: next=%d skipped=%d err=%q events=%d", got.Next, got.Skipped, got.Err, len(got.Events))
	}
	if again := readStream(t, path, int64(len(complete))); again.Err != "" || len(again.Events) != 0 || again.Skipped != 0 || again.Next != int64(len(complete)) {
		t.Errorf("a fragment-only remainder yields nothing and keeps Next: %+v", again)
	}
	appendFile(t, path, []byte("\n"))
	total := int64(len(complete) + len(pendingLine))
	if done := readStream(t, path, int64(len(complete))); done.Err != "" || done.Next != total || !sameJSON(t, done.Events, []signalcenter.Event{pending}) {
		t.Errorf("the completed fragment must be returned exactly once: next=%d events=%d err=%q", done.Next, len(done.Events), done.Err)
	}
	for _, from := range []int64{-1, total + 100} {
		if re := readStream(t, path, from); re.Next != total || re.Skipped != 2 || !sameJSON(t, re.Events, []signalcenter.Event{first, huge, last, pending}) {
			t.Errorf("ReadStream(%d) on a shorter-than-offset or negative offset must re-read from 0: next=%d skipped=%d events=%d", from, re.Next, re.Skipped, len(re.Events))
		}
	}
	if miss := readStream(t, filepath.Join(dir, "absent.ndjson"), 7); miss.Err == "" || !miss.NotExist || miss.Next != 7 || len(miss.Events) != 0 || miss.Skipped != 0 {
		t.Errorf("a missing stream must be an fs.ErrNotExist error that keeps Next == from: %+v", miss)
	}
	if isDir := readStream(t, dir, 7); isDir.Err == "" || isDir.NotExist || isDir.Next != 7 || len(isDir.Events) != 0 {
		t.Errorf("a directory path must be an I/O error that is not fs.ErrNotExist and keeps Next == from: %+v", isDir)
	}
}

type probeMerge struct {
	Merged                    []signalcenter.Event   `json:"merged"`
	InputsAfterCall           [][]signalcenter.Event `json:"inputs_after_call"`
	InputsAfterMutatingResult [][]signalcenter.Event `json:"inputs_after_mutating_result"`
}

func mergeByTS(t *testing.T, streams [][]signalcenter.Event) probeMerge {
	t.Helper()
	in, err := json.Marshal(streams)
	if err != nil {
		t.Fatal(err)
	}
	r := run(t, t.TempDir(), os.Environ(), string(in), probeBuild.get(t), "merge")
	var got probeMerge
	if r.code != 0 || json.Unmarshal([]byte(r.stdout), &got) != nil {
		t.Fatalf("streamprobe merge:\n%s", r)
	}
	return got
}

func TestC1794_015_MergeByTSOrdersByInstantStablyWithoutTouchingItsInputs(t *testing.T) {
	a1, a2, a3 := ev(1, 1, at("00:05.1"), outcome, "a"), ev(1, 3, at("00:05.43965"), outcome, "a"), ev(1, 4, at("00:07"), outcome, "a")
	aBad := ev(1, 2, "bogus", outcome, "a")
	b1, b2, b3 := ev(2, 1, at("00:05"), outcome, "b"), ev(2, 2, at("00:05.4396"), outcome, "b"), ev(2, 3, at("00:07"), outcome, "b")
	bBad := ev(2, 4, "garbled", outcome, "b")
	streams := [][]signalcenter.Event{{a1, aBad, a2, a3}, {}, {b1, b2, b3, bBad}}
	got := mergeByTS(t, streams)
	want := []signalcenter.Event{aBad, bBad, b1, a1, b2, a2, a3, b3}
	if !sameJSON(t, got.Merged, want) {
		t.Errorf("MergeByTS must order by parsed instant (unparsable first), equal instants in argument order:\n got %v\nwant %v", got.Merged, want)
	}
	if !sameJSON(t, got.InputsAfterCall, streams) || !sameJSON(t, got.InputsAfterMutatingResult, streams) {
		t.Errorf("MergeByTS must leave its inputs untouched and return a fresh slice")
	}
	single := [][]signalcenter.Event{{b2, b1}}
	if one := mergeByTS(t, single); !sameJSON(t, one.Merged, []signalcenter.Event{b1, b2}) || !sameJSON(t, one.InputsAfterMutatingResult, single) {
		t.Errorf("a single stream must come back sorted in a fresh slice: %+v", one)
	}
	for _, none := range [][][]signalcenter.Event{{}, {{}, {}}} {
		if empty := mergeByTS(t, none); len(empty.Merged) != 0 {
			t.Errorf("MergeByTS over %d empty argument(s) must be empty: %v", len(none), empty.Merged)
		}
	}
}

func coverFloor(t *testing.T, root, pkg string) float64 {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(root, "go", ".cover-strict"))
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range strings.Split(string(raw), "\n") {
		if f := strings.Fields(line); len(f) == 2 && f[0] == pkg {
			floor, err := strconv.ParseFloat(f[1], 64)
			if err != nil {
				t.Fatal(err)
			}
			return floor
		}
	}
	t.Fatalf("%s has no floor in go/.cover-strict", pkg)
	return 0
}

func funcCoverage(funcOut, file, name string) string {
	for _, line := range strings.Split(funcOut, "\n") {
		f := strings.Fields(line)
		if len(f) == 3 && strings.Contains(f[0], file+":") && f[1] == name {
			return f[2]
		}
	}
	return ""
}

func TestC1794_016_StreamAPIIsFullyCoveredAndPassesTheEnforcedApicoverGate(t *testing.T) {
	root := acsassert.RepoRoot(t)
	goDir, tmp := filepath.Join(root, "go"), t.TempDir()
	profile, funcs := filepath.Join(tmp, "coverage.txt"), filepath.Join(tmp, "coverage.func.txt")
	if r := run(t, goDir, os.Environ(), "", "go", "test", "-count=1", "-tags", "integration", "-coverprofile="+profile, "./internal/signalcenter"); r.code != 0 {
		t.Fatalf("signalcenter tests must pass:\n%s", r)
	}
	fn := run(t, goDir, os.Environ(), "", "go", "tool", "cover", "-func="+profile)
	if fn.code != 0 {
		t.Fatalf("go tool cover:\n%s", fn)
	}
	for _, name := range []string{"ReadStream", "MergeByTS"} {
		if got := funcCoverage(fn.stdout, "/internal/signalcenter/stream.go", name); got != "100.0%" {
			t.Errorf("signalcenter/stream.go %s must exist and be fully covered, got %q", name, got)
		}
	}
	total := strings.TrimSuffix(funcCoverage(fn.stdout, "total", "(statements)"), "%")
	if pct, err := strconv.ParseFloat(total, 64); err != nil || pct < coverFloor(t, root, "./internal/signalcenter") {
		t.Errorf("signalcenter total coverage %q is below its .cover-strict floor", total)
	}
	writeFile(t, funcs, []byte(fn.stdout))
	if r := run(t, goDir, os.Environ(), "", apicoverBuild.get(t), "-enforce", "-cover", funcs, filepath.Join(goDir, "internal", "signalcenter")); r.code != 0 {
		t.Errorf("apicover -enforce over the enrolled signalcenter package must pass (StreamChunk, ReadStream and MergeByTS named and covered):\n%s", r)
	}
}

func requireFiles(t *testing.T, root string, rels ...string) {
	t.Helper()
	for _, rel := range rels {
		if _, err := os.Stat(filepath.Join(root, rel)); err != nil {
			t.Fatalf("contract file %s is missing: %v", rel, err)
		}
	}
}

func TestC1794_017_TheLaneAddsNoCommentsToItsGoFiles(t *testing.T) {
	root := acsassert.RepoRoot(t)
	requireFiles(t, root, streamGo, tailGo)
	r := run(t, root, os.Environ(), "", commentauditBuild.get(t), "comments", "-base", baseSHA, "go/cmd/evolve", "go/internal/signalcenter")
	if r.code != 0 {
		t.Errorf("commentaudit comments must find changed Go files under both dirs and zero added comments:\n%s", r)
	}
}

func TestC1794_018_NewCodeFitsTheSizeRatchetAndKeepsSignalcenterALeaf(t *testing.T) {
	root := acsassert.RepoRoot(t)
	requireFiles(t, root, streamGo, tailGo)
	if err := sizeratchet.Scan(filepath.Join(root, "go")); err != nil {
		t.Errorf("function-size ratchet: %v", err)
	}
	if r := run(t, root, os.Environ(), "", "git", "-C", root, "diff", "--quiet", baseSHA, "--", "go/internal/sizeratchet/offenders.json"); r.code != 0 {
		t.Errorf("offenders.json must not change:\n%s", r)
	}
	if err := structure.CheckLimits(filepath.Join(root, "go", "internal", "signalcenter")); err != nil {
		t.Errorf("signalcenter limits: %v", err)
	}
	file, err := parser.ParseFile(token.NewFileSet(), filepath.Join(root, streamGo), nil, parser.ImportsOnly)
	if err != nil {
		t.Fatal(err)
	}
	for _, imp := range file.Imports {
		path, _ := strconv.Unquote(imp.Path.Value)
		if first, _, _ := strings.Cut(path, "/"); strings.Contains(first, ".") {
			t.Errorf("stream.go must import the standard library only, found %s", path)
		}
	}
}

func TestC1794_019_RuntimeReferenceDocumentsTheTailCommand(t *testing.T) {
	// acs-predicate: config-check
	root := acsassert.RepoRoot(t)
	doc := filepath.Join(root, "docs", "operations", "runtime-reference.md")
	ok, err := acsassert.LineContainsAllChecked(doc, "evolve signals tail", "--follow", "--cycle", "--kind", "--code", "--json")
	if err != nil || !ok {
		t.Errorf("runtime-reference.md needs one entry naming `evolve signals tail` with --follow, --cycle, --kind, --code and --json (err=%v)", err)
	}
}

func TestC1794_020_SignalsDispatchKeepsCodesAndAdvertisesTail(t *testing.T) {
	root := acsassert.RepoRoot(t)
	env := scrubbedEnv(root)
	if r := run(t, root, env, "", evolveBin(t), "signals", "codes", "check"); r.code != 0 || !strings.Contains(r.stdout, "codes in sync") {
		t.Errorf("`signals codes check` must keep working:\n%s", r)
	}
	for _, args := range [][]string{{"signals", "codes", "frobnicate"}, {"signals", "codes"}} {
		if r := run(t, root, env, "", evolveBin(t), args...); r.code != usageExit {
			t.Errorf("%s must stay a usage error:\n%s", strings.Join(args, " "), r)
		}
	}
	r := run(t, root, env, "", evolveBin(t), "signals")
	if r.code != usageExit || !strings.Contains(r.stderr, "evolve signals codes <generate|check>") || !strings.Contains(r.stderr, "evolve signals tail") {
		t.Errorf("bare `signals` must exit %d with a usage line naming both codes and tail:\n%s", usageExit, r)
	}
}
