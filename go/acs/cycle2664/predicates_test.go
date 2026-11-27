//go:build acs

package cycle2664

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/phases/ship/landing"
	"github.com/mickeyyaya/evolve-loop/go/internal/runlease"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
	"github.com/mickeyyaya/evolve-loop/go/internal/stelint"
	"github.com/mickeyyaya/evolve-loop/go/internal/wave"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	cycleFloor        = 2580
	singleFlight      = "verify-single-flight-unavailable"
	signalsSuffix     = ".evolve/signals.ndjson"
	triggerNoShip     = "no-ship"
	triggerAbnormal   = "abnormal-kind"
	triggerLaneStalls = "lane-stalls"
	stallKind         = "stall"
	pollsWaited       = 90 * time.Second
	watchLifetime     = 5 * time.Minute
)

var (
	wave185Start  = time.Date(2026, 11, 21, 8, 26, 1, 0, time.UTC)
	wave185WakeBy = time.Date(2026, 11, 21, 10, 0, 0, 0, time.UTC)
)

var evolveBuild struct {
	once      sync.Once
	dir, path string
	failure   string
}

func TestMain(m *testing.M) {
	code := m.Run()
	if evolveBuild.dir != "" {
		if err := os.RemoveAll(evolveBuild.dir); err != nil {
			fmt.Fprintf(os.Stderr, "cycle2664: remove %s: %v\n", evolveBuild.dir, err)
		}
	}
	os.Exit(code)
}

func evolveBin(t *testing.T) string {
	t.Helper()
	evolveBuild.once.Do(func() {
		dir, err := os.MkdirTemp("", "cycle2664-evolve-")
		if err != nil {
			evolveBuild.failure = err.Error()
			return
		}
		evolveBuild.dir, evolveBuild.path = dir, filepath.Join(dir, "evolve")
		stdout, stderr, code, err := acsassert.SubprocessOutputIn(acsassert.GoModuleDir(t), "go", "build", "-o", evolveBuild.path, "./cmd/evolve")
		if err != nil || code != 0 {
			evolveBuild.failure = fmt.Sprintf("exit %d err %v\n%s%s", code, err, stdout, stderr)
		}
	})
	if evolveBuild.failure != "" {
		t.Fatalf("go build ./cmd/evolve: %s", evolveBuild.failure)
	}
	return evolveBuild.path
}

type stallLine struct {
	lane  string
	atMin int
}

type abnormalEvent struct {
	kind, severity string
	cycle, atMin   int
}

type replay struct {
	number  int
	age     time.Duration
	live    int
	cycles  int
	stalls  []stallLine
	events  []abnormalEvent
	shipped []int
	policy  string
}

func cycleID(n int) int { return cycleFloor + n }

func (r replay) signal(at time.Time, code signalcenter.Code, severity signalcenter.Severity, reason string, fields map[string]string) string {
	e := signalcenter.Event{
		SchemaVersion: signalcenter.SchemaVersion, PID: 9, TS: at.UTC().Format(time.RFC3339Nano), Module: signalcenter.ModuleLoop,
		Origin: "replay", Kind: signalcenter.KindLoopWave, Code: code, Severity: severity, Reason: reason, Fields: fields,
	}
	raw, err := json.Marshal(e)
	if err != nil {
		panic(err)
	}
	return string(raw) + "\n"
}

func (r replay) marker(at time.Time, n int) string {
	return r.signal(at, "LOOP_ORCH_POOL_UNDERFILLED", signalcenter.SeverityWarn, "marker-"+strconv.Itoa(n), map[string]string{"live": strconv.Itoa(r.live)})
}

func (r replay) write(t *testing.T) (root string, start time.Time) {
	t.Helper()
	root = t.TempDir()
	evolveDir := filepath.Join(root, ".evolve")
	now := time.Now().UTC()
	start = now.Add(-r.age)
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	must(os.MkdirAll(filepath.Join(evolveDir, "runs", "loop-holder"), 0o755))
	must(wave.NewStore(evolveDir).Save(wave.Record{Number: r.number, RunID: "r" + strconv.Itoa(r.number), PID: 9, CycleFloor: cycleFloor, StartedAt: start}))
	must(runlease.Write(filepath.Join(evolveDir, "runs", "loop-holder"), runlease.Lease{OwnerPID: os.Getpid()}, now))
	if r.policy != "" {
		must(os.WriteFile(filepath.Join(evolveDir, "policy.json"), []byte(r.policy), 0o644))
	}
	var signals strings.Builder
	for i := 1; i <= r.live; i++ {
		lane := "lane-" + strconv.Itoa(i)
		signals.WriteString(r.signal(start.Add(time.Duration(i-1)*time.Minute), "LOOP_ORCH_LANE_LAUNCHED", signalcenter.SeverityInfo, "pool launched "+lane,
			map[string]string{"todo": lane, "live": strconv.Itoa(i)}))
	}
	for _, s := range r.stalls {
		signals.WriteString(r.signal(start.Add(time.Duration(s.atMin)*time.Minute), "LOOP_ORCH_LANE_STALLED", signalcenter.SeverityWarn, "lane "+s.lane+" has had no event",
			map[string]string{"todo": s.lane, "live": strconv.Itoa(r.live), "stall_minutes": "30"}))
	}
	signals.WriteString(r.marker(now.Add(-time.Second), 1))
	must(os.WriteFile(filepath.Join(evolveDir, "signals.ndjson"), []byte(signals.String()), 0o644))
	perCycle := map[int]*strings.Builder{}
	for n := 1; n <= r.cycles; n++ {
		must(os.MkdirAll(filepath.Join(evolveDir, "runs", "cycle-"+strconv.Itoa(cycleID(n))), 0o755))
		perCycle[cycleID(n)] = &strings.Builder{}
	}
	for _, e := range r.events {
		line := fmt.Sprintf(`{"event_type":%q,"timestamp":%q,"source_phase":"verify","severity":%q,"details":"replay","cycle":%d}`+"\n",
			e.kind, start.Add(time.Duration(e.atMin)*time.Minute).Format(time.RFC3339), e.severity, e.cycle)
		perCycle[e.cycle].WriteString(line)
	}
	for id, body := range perCycle {
		if body.Len() > 0 {
			must(os.WriteFile(filepath.Join(evolveDir, "runs", "cycle-"+strconv.Itoa(id), "abnormal-events.jsonl"), []byte(body.String()), 0o644))
		}
	}
	for _, id := range r.shipped {
		must(landing.WriteIntent(landing.IntentPath(evolveDir, id), landing.Intent{Cycle: id, Status: landing.IntentComplete}))
	}
	return root, start
}

type watchEvent struct {
	Kind     string   `json:"kind"`
	Detail   string   `json:"detail"`
	Trigger  string   `json:"trigger"`
	Lanes    []string `json:"lanes"`
	Evidence []string `json:"evidence"`
}

type session struct {
	lines chan string
	seen  []string
}

func startWatch(t *testing.T, root string, extra ...string) *session {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), watchLifetime)
	cmd := exec.CommandContext(ctx, evolveBin(t), append([]string{"wave", "watch", "--project-root", root}, extra...)...)
	cmd.Dir, cmd.WaitDelay = t.TempDir(), 10*time.Second
	pipe, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		cancel()
		t.Fatal(err)
	}
	s := &session{lines: make(chan string, 4096)}
	done := make(chan struct{})
	go func() {
		defer close(done)
		defer close(s.lines)
		sc := bufio.NewScanner(pipe)
		sc.Buffer(nil, 1<<20)
		for sc.Scan() {
			s.lines <- sc.Text()
		}
	}()
	t.Cleanup(func() {
		cancel()
		<-done
		_ = cmd.Wait()
	})
	return s
}

func (s *session) waitFor(ctx context.Context, needle string) bool {
	for _, line := range s.seen {
		if strings.Contains(line, needle) {
			return true
		}
	}
	for {
		select {
		case line, ok := <-s.lines:
			if !ok {
				return false
			}
			s.seen = append(s.seen, line)
			if strings.Contains(line, needle) {
				return true
			}
		case <-ctx.Done():
			return false
		}
	}
}

func observe(t *testing.T, r replay, completedPolls int, extra ...string) (root string, lines []string) {
	t.Helper()
	root, _ = r.write(t)
	s := startWatch(t, root, extra...)
	ctx, cancel := context.WithTimeout(context.Background(), pollsWaited)
	defer cancel()
	for n := 1; n <= completedPolls+1; n++ {
		if n > 1 {
			f, err := os.OpenFile(filepath.Join(root, ".evolve", "signals.ndjson"), os.O_APPEND|os.O_WRONLY, 0o644)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := f.WriteString(r.marker(time.Now().UTC(), n)); err != nil {
				t.Fatal(err)
			}
			if err := f.Close(); err != nil {
				t.Fatal(err)
			}
		}
		if !s.waitFor(ctx, "marker-"+strconv.Itoa(n)) {
			t.Fatalf("the watch never printed marker-%d, so poll %d did not finish; it printed:\n%s", n, n-1, strings.Join(s.seen, "\n"))
		}
	}
	return root, s.seen
}

func jsonEvents(t *testing.T, lines []string) []watchEvent {
	t.Helper()
	var out []watchEvent
	for _, line := range lines {
		var envelope struct {
			Schema string     `json:"schema"`
			Data   watchEvent `json:"data"`
		}
		if err := json.Unmarshal([]byte(line), &envelope); err != nil {
			t.Fatalf("watch --json printed a line that is not JSON: %q: %v", line, err)
		}
		if envelope.Schema == "" || envelope.Data.Kind == "" {
			t.Fatalf("watch --json printed a line with no schema or no data.kind: %q", line)
		}
		out = append(out, envelope.Data)
	}
	return out
}

func stalls(events []watchEvent) []watchEvent {
	var out []watchEvent
	for _, e := range events {
		if e.Kind == stallKind {
			out = append(out, e)
		}
	}
	return out
}

func minutesOf(d time.Duration) int { return int(d / time.Minute) }

func wave185() replay {
	age := wave185WakeBy.Sub(wave185Start) - time.Minute
	r := replay{number: 185, age: age, live: 8, cycles: 9}
	for i, m := 0, 43; m < minutesOf(age); i, m = i+1, m+30 {
		r.stalls = append(r.stalls, stallLine{lane: "lane-" + strconv.Itoa(i%8+1), atMin: m})
	}
	for i, m := 0, 14; m < minutesOf(age); i, m = i+1, m+4 {
		r.events = append(r.events, abnormalEvent{kind: singleFlight, severity: "WARN", cycle: cycleID(i%9 + 1), atMin: m})
	}
	return r
}

func normalWaves() map[string]replay {
	wave184 := replay{number: 184, age: 300 * time.Minute, live: 8, cycles: 9, shipped: []int{cycleID(1), cycleID(3), cycleID(5), cycleID(7)},
		stalls: []stallLine{{"lane-3", 70}, {"lane-5", 190}}}
	for i, m := range []int{20, 65, 110, 155, 200, 245} {
		wave184.events = append(wave184.events, abnormalEvent{kind: singleFlight, severity: "WARN", cycle: cycleID(2 + 2*(i%3)), atMin: m})
	}
	for i := 0; i < 4; i++ {
		wave184.events = append(wave184.events, abnormalEvent{kind: "subscription-auth-mode", severity: "INFO", cycle: cycleID(8), atMin: 35 + 60*i})
	}
	wave183 := replay{number: 183, age: 100 * time.Minute, live: 8, cycles: 5, stalls: []stallLine{{"lane-1", 30}, {"lane-2", 80}}}
	for i := 0; i < 14; i++ {
		wave183.events = append(wave183.events, abnormalEvent{kind: singleFlight, severity: "WARN", cycle: cycleID(i%5 + 1), atMin: 20 + 5*i})
	}
	wave182 := replay{number: 182, age: 360 * time.Minute, live: 8, cycles: 6, shipped: []int{cycleID(1), cycleID(3)}, stalls: []stallLine{{"lane-1", 100}, {"lane-2", 110}}}
	for i := 0; i < 40; i++ {
		wave182.events = append(wave182.events, abnormalEvent{kind: "verify-failed", severity: "WARN", cycle: cycleID(2 + 2*(i%2)), atMin: 5 + 8*i})
	}
	for i := 0; i < 60; i++ {
		wave182.events = append(wave182.events, abnormalEvent{kind: "classification", severity: "INFO", cycle: cycleID(i%6 + 1), atMin: 2 + 5*i})
	}
	wave181 := replay{number: 181, age: 150 * time.Minute, live: 3, cycles: 4}
	return map[string]replay{"wave 184": wave184, "wave 183": wave183, "wave 182": wave182, "wave 181": wave181}
}

func TestC2664_001_TheWave185ReplayWakesTheWatchBefore1000Z(t *testing.T) {
	r := wave185()
	if r.age >= 120*time.Minute {
		t.Fatalf("the replay is %v old at 10:00Z, want under the 120 minute no-ship age so that the abnormal-kind trigger carries the wake", r.age)
	}
	_, lines := observe(t, r, 1, "--json")
	woke := stalls(jsonEvents(t, lines))
	if len(woke) == 0 {
		t.Fatalf("RED: the watch printed no stall event for wave 185 at %v of age (0 ships, 8 live lanes, %d abnormal events of %s)", r.age, len(r.events), singleFlight)
	}
	if woke[0].Trigger != triggerAbnormal {
		t.Errorf("first stall trigger = %q, want %q", woke[0].Trigger, triggerAbnormal)
	}
}

func TestC2664_002_ReplaysOfWave184AndEarlierWavesStaySilentOnNormalVolume(t *testing.T) {
	for name, r := range normalWaves() {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			_, lines := observe(t, r, 2, "--json")
			if woke := stalls(jsonEvents(t, lines)); len(woke) != 0 {
				t.Errorf("%s woke the watch: %+v", name, woke)
			}
		})
	}
}

func TestC2664_003_TheWakeNamesTheTriggerTheLanesAndTheEvidencePaths(t *testing.T) {
	noShip := replay{number: 186, age: 130 * time.Minute, live: 8, cycles: 9}
	laneStalls := replay{number: 187, age: 60 * time.Minute, live: 8, cycles: 4, stalls: []stallLine{{"lane-1", 20}, {"lane-2", 30}, {"lane-3", 40}}}
	cases := map[string]triggerCase{"abnormal kind": {wave185(), triggerAbnormal}, "no ship": {noShip, triggerNoShip}, "lane stalls": {laneStalls, triggerLaneStalls}}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			root, lines := observe(t, c.r, 1, "--json")
			woke := stalls(jsonEvents(t, lines))
			if len(woke) == 0 {
				t.Fatalf("RED: no stall event for the %s replay", name)
			}
			e := woke[0]
			if e.Trigger != c.trigger {
				t.Errorf("trigger = %q, want %q", e.Trigger, c.trigger)
			}
			if len(e.Lanes) == 0 {
				t.Errorf("the wake names no lane")
			}
			if len(e.Evidence) == 0 {
				t.Fatalf("the wake names no evidence path")
			}
			for _, p := range e.Evidence {
				if !filepath.IsAbs(p) {
					p = filepath.Join(root, p)
				}
				if _, err := os.Stat(p); err != nil {
					t.Errorf("evidence path %s does not exist: %v", p, err)
				}
			}
			c.checkNamed(t, e)
		})
	}
}

type triggerCase struct {
	r       replay
	trigger string
}

func (c triggerCase) checkNamed(t *testing.T, e watchEvent) {
	t.Helper()
	switch c.trigger {
	case triggerLaneStalls:
		if want := []string{"lane-1", "lane-2", "lane-3"}; !slices.Equal(slices.Sorted(slices.Values(e.Lanes)), want) {
			t.Errorf("lanes = %v, want the stalled lanes %v", e.Lanes, want)
		}
		if !hasSuffixIn(e.Evidence, signalsSuffix) {
			t.Errorf("evidence = %v, want it to hold %s", e.Evidence, signalsSuffix)
		}
	case triggerAbnormal:
		held := map[string]bool{}
		for _, ev := range c.r.events {
			held["cycle-"+strconv.Itoa(ev.cycle)] = true
		}
		for _, lane := range e.Lanes {
			if !held[lane] {
				t.Errorf("lane %q holds no event of %s; the lanes of the wake must be cycles that hold the events", lane, singleFlight)
			}
		}
		if !strings.Contains(e.Detail, singleFlight) {
			t.Errorf("detail = %q, want it to name the kind %s", e.Detail, singleFlight)
		}
		if !hasSuffixIn(e.Evidence, "/abnormal-events.jsonl") {
			t.Errorf("evidence = %v, want an abnormal-events.jsonl path", e.Evidence)
		}
	case triggerNoShip:
		if !hasSuffixIn(e.Evidence, signalsSuffix) {
			t.Errorf("evidence = %v, want it to hold %s", e.Evidence, signalsSuffix)
		}
	}
}

func hasSuffixIn(paths []string, suffix string) bool {
	return slices.ContainsFunc(paths, func(p string) bool { return strings.HasSuffix(filepath.ToSlash(p), suffix) })
}

func TestC2664_004_TheTextWakeLineNamesTheTriggerTheLanesAndTheEvidencePath(t *testing.T) {
	r := replay{number: 187, age: 60 * time.Minute, live: 8, cycles: 4, stalls: []stallLine{{"lane-1", 20}, {"lane-2", 30}, {"lane-3", 40}}}
	_, lines := observe(t, r, 1)
	var wake string
	for _, line := range lines {
		if strings.Contains(line, triggerLaneStalls) {
			wake = line
		}
	}
	if wake == "" {
		t.Fatalf("RED: no text line names the trigger %s; the watch printed:\n%s", triggerLaneStalls, strings.Join(lines, "\n"))
	}
	for _, need := range []string{"lane-1", "lane-2", "lane-3", "signals.ndjson"} {
		if !strings.Contains(wake, need) {
			t.Errorf("wake line %q does not name %q", wake, need)
		}
	}
}

func TestC2664_005_TheWakePrintsOncePerTriggerAcrossPolls(t *testing.T) {
	_, lines := observe(t, wave185(), 2, "--json")
	if woke := stalls(jsonEvents(t, lines)); len(woke) != 1 {
		t.Errorf("stall events after two full polls = %d, want exactly 1 for the one standing trigger: %+v", len(woke), woke)
	}
}

func TestC2664_006_TheWatchTakesItsBoundsFromPolicy(t *testing.T) {
	loose := wave185()
	loose.policy = `{"wave":{"checkin":{"abnormal_min_events":1000,"stall_lines":1000,"stall_min_age_minutes":100000}}}`
	tight := normalWaves()["wave 183"]
	tight.policy = `{"wave":{"checkin":{"abnormal_min_events":5,"abnormal_min_cycles":2}}}`
	t.Run("loose bounds silence the wave 185 replay", func(t *testing.T) {
		t.Parallel()
		_, lines := observe(t, loose, 2, "--json")
		if woke := stalls(jsonEvents(t, lines)); len(woke) != 0 {
			t.Errorf("a policy with wide bounds still woke the watch: %+v", woke)
		}
	})
	t.Run("tight bounds wake on normal volume", func(t *testing.T) {
		t.Parallel()
		_, lines := observe(t, tight, 1, "--json")
		woke := stalls(jsonEvents(t, lines))
		if len(woke) == 0 {
			t.Fatalf("RED: a policy with abnormal_min_events 5 did not wake the watch on 14 events of one kind in 5 cycles")
		}
		if woke[0].Trigger != triggerAbnormal {
			t.Errorf("trigger = %q, want %q", woke[0].Trigger, triggerAbnormal)
		}
	})
}

// acs-predicate: config-check
func TestC2664_007_TheOperationsDocNamesTheZeroShipFirstLook(t *testing.T) {
	root := acsassert.RepoRoot(t)
	raw, err := os.ReadFile(filepath.Join(root, "docs", "operations", "wave-operations.md"))
	if err != nil {
		t.Fatal(err)
	}
	section := sectionOf(string(raw), "zero-ship first look")
	if section == "" {
		t.Fatalf("RED: wave-operations.md has no heading that names the zero-ship first look")
	}
	for _, need := range []string{"LOOP_ORCH_LANE_STALLED", "abnormal-events.jsonl", "evolve wave watch"} {
		if !strings.Contains(section, need) {
			t.Errorf("the zero-ship section does not name %q", need)
		}
	}
	words, err := stelint.LoadStandard(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range stelint.Check([]byte(section), stelint.Options{Words: words}) {
		t.Errorf("STE finding in the zero-ship section: line %d %s %s: %s", f.Line, f.Rule, f.Message, f.Excerpt)
	}
}

func sectionOf(doc, heading string) string {
	lines := strings.Split(doc, "\n")
	depthOf := func(l string) int { return len(l) - len(strings.TrimLeft(l, "#")) }
	for i, l := range lines {
		if !strings.HasPrefix(l, "#") || !strings.Contains(strings.ToLower(l), heading) {
			continue
		}
		end := len(lines)
		for j := i + 1; j < len(lines); j++ {
			if strings.HasPrefix(lines[j], "#") && depthOf(lines[j]) <= depthOf(l) {
				end = j
				break
			}
		}
		return strings.Join(lines[i:end], "\n")
	}
	return ""
}
