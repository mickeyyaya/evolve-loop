package observerengine

// observerengine_test.go — the leaf's fixtures and the construction, layout,
// reporter and registry tests (ADR-0103 unit 12 §6 tests 16-17, 33-35).
// Every test uses t.TempDir() only, a fixed or stepped clock, fake ports and
// a recording Center: no syscall, no inbox, no tmux, no ticker.

import (
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

const fixturePID = 4242

var fixtureAt = time.Date(2026, 6, 1, 9, 0, 0, 0, time.UTC)

func fixedClock(at time.Time) func() time.Time { return func() time.Time { return at } }

// settingsFixture is the ONE complete Settings every leaf test starts from:
// every threshold set (the engine defaults nothing — the host's withDefaults
// and policy's ObserverConfig are the belief owners).
func settingsFixture(ws string) Settings {
	return Settings{
		Cycle: 7, Phase: "build", Agent: "builder", Scope: ScopePhase, PID: fixturePID,
		PollS: 1, StallS: 600, EOFGraceS: 9999, HeartbeatEvery: 12,
		NudgeBody: "You appear stalled. Summarize your current state, then either continue or finalize your artifact.",
		Paths:     PathsFor(ws, "builder"),
	}
}

// depsFixture wires every port to a no-op fake and a fixed clock.
func depsFixture() Deps {
	return Deps{
		Now:   fixedClock(fixtureAt),
		Kill:  func(int, syscall.Signal) error { return nil },
		Nudge: func(string) error { return nil },
	}
}

// recordingCenter captures every event a real Center delivers.
type recordingCenter struct {
	c      *signalcenter.Center
	mu     sync.Mutex
	events []signalcenter.Event
}

func newRecordingCenter() *recordingCenter {
	r := &recordingCenter{c: signalcenter.New()}
	r.c.Subscribe(func(e signalcenter.Event) {
		r.mu.Lock()
		defer r.mu.Unlock()
		r.events = append(r.events, e)
	})
	return r
}

func (r *recordingCenter) accessor() func() *signalcenter.Center {
	return func() *signalcenter.Center { return r.c }
}

func (r *recordingCenter) all() []signalcenter.Event {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]signalcenter.Event(nil), r.events...)
}

func (r *recordingCenter) byCode(code signalcenter.Code) []signalcenter.Event {
	var out []signalcenter.Event
	for _, e := range r.all() {
		if e.Code == code {
			out = append(out, e)
		}
	}
	return out
}

// newEngine builds an engine over a temp workspace, a recording Center and the
// fixtures, after letting the test mutate Settings/Deps.
func newEngine(t *testing.T, mutate func(s *Settings, d *Deps)) (*Engine, *recordingCenter, string) {
	t.Helper()
	ws := t.TempDir()
	s, d := settingsFixture(ws), depsFixture()
	if mutate != nil {
		mutate(&s, &d)
	}
	rc := newRecordingCenter()
	return New(s, d, WithSignals(rc.accessor())), rc, ws
}

func readEvents(t *testing.T, path string) []map[string]any {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var out []map[string]any
	for _, line := range strings.Split(strings.TrimSpace(string(raw)), "\n") {
		var env map[string]any
		if err := json.Unmarshal([]byte(line), &env); err != nil {
			t.Fatalf("bad envelope %q: %v", line, err)
		}
		out = append(out, env)
	}
	return out
}

func typesOf(events []map[string]any) []string {
	out := make([]string, 0, len(events))
	for _, e := range events {
		out = append(out, e["type"].(string))
	}
	return out
}

func seedLog(t *testing.T, path string, lines ...string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

var threeLines = []string{
	`{"type":"assistant","message":{"content":[{"type":"tool_use","name":"Read","input":{"path":"x"}}]}}`,
	`{"type":"user","message":{"content":[{"type":"tool_result","is_error":false}]}}`,
	`{"type":"result","total_cost_usd":0.5,"usage":{"cache_read_input_tokens":10,"cache_creation_input_tokens":2}}`,
}

// --- 16: the layout projection ---------------------------------------------

// TestPathsFor_ProjectsTheThreeSuffixes is the consumer pin for the one
// spelling of the per-agent layout (the host and the live adapter project it;
// the two glob readers are follow-up F3). Kills M23 (a suffix misspelt).
func TestPathsFor_ProjectsTheThreeSuffixes(t *testing.T) {
	t.Parallel()
	got := PathsFor("/ws", "builder")
	want := Paths{Stdout: "/ws/builder-stdout.log", Events: "/ws/builder-observer-events.ndjson", Report: "/ws/builder-observer-report.json"}
	if got != want {
		t.Fatalf("PathsFor = %+v, want %+v", got, want)
	}
	if StdoutSuffix != "-stdout.log" || EventsSuffix != "-observer-events.ndjson" || ReportSuffix != "-observer-report.json" {
		t.Error("the three suffix literals are the layout the runner, the adapter and the watchdog share")
	}
	if ScopePhase != "phase" || ScopeCycle != "cycle" {
		t.Error("the two scope spellings the host aliases")
	}
}

// --- 17: construction and the start event -----------------------------------

// TestNew_StartEmitsObserverStartedAndSetsTheClocks — fixed clock, PID 4242:
// the observer_started line is byte-exact; traceID, startedAt and the two
// liveness clocks are set from the ONE construction read. Kills M24 (a dropped
// start field), M25 (the trace format), M26 (a zero clock).
func TestNew_StartEmitsObserverStartedAndSetsTheClocks(t *testing.T) {
	t.Parallel()
	e, rc, ws := newEngine(t, nil)
	e.Start()
	raw, err := os.ReadFile(filepath.Join(ws, "builder-observer-events.ndjson"))
	if err != nil {
		t.Fatal(err)
	}
	want := `{"data":{"enforce":false,"poll_s":1,"scope":"phase","stall_s":600},"id":"obs_1780304400000000000_4242_0","schema_version":"1.0","severity":"INFO","source":{"agent":"builder","component":"phase-observer","cycle":7,"observer_pid":4242,"phase":"build"},"trace_id":"cycle-7-build-1780304400","ts":"2026-06-01T09:00:00Z","type":"observer_started"}` + "\n"
	if string(raw) != want {
		t.Fatalf("observer_started:\n got: %s\nwant: %s", raw, want)
	}
	if e.traceID != fmt.Sprintf("cycle-7-build-%d", fixtureAt.Unix()) || e.startedAt != fixtureAt || e.startedAtISO != "2026-06-01T09:00:00Z" {
		t.Errorf("trace/started: %q %v %q", e.traceID, e.startedAt, e.startedAtISO)
	}
	if e.lastEventTS != fixtureAt || e.lastProgressTS != fixtureAt {
		t.Errorf("both liveness clocks start at construction time: %v %v", e.lastEventTS, e.lastProgressTS)
	}
	if len(rc.all()) != 0 {
		t.Errorf("the start event never enters the Center: %+v", rc.all())
	}
	if !e.SignalsWired() {
		t.Error("SignalsWired reports the recording Center")
	}
}

// --- 33: the reporter ---------------------------------------------------------

// TestReporter_NullObjectAndSeverityTable — a nil accessor and an accessor
// returning nil never panic and report Wired false; wired, one event per call
// with the severity from the table (INCIDENT only for the stall kill) and the
// origin/cycle/phase as passed; an unregistered code degrades to WARN and the
// Center's own drift stamp. Kills M37 (WARN↔INCIDENT), M49 (a nil deref).
func TestReporter_NullObjectAndSeverityTable(t *testing.T) {
	t.Parallel()
	var nilRep *Reporter
	for i, r := range []*Reporter{nilRep, NewReporter(nil), NewReporter(func() *signalcenter.Center { return nil })} {
		if r.Wired() {
			t.Errorf("reporter %d must not be wired", i)
		}
		r.Report("Engine.Tick", 1, "build", CodeKillFailed, "no center", nil) // must not panic
	}
	rc := newRecordingCenter()
	r := NewReporter(rc.accessor())
	if !r.Wired() {
		t.Fatal("wired reporter")
	}
	r.Report("Engine.Tick", 9, "audit", CodeStallKillSent, "killing", map[string]string{"step": "respond"})
	r.Report("Engine.WriteReport", 9, "audit", CodeReportWriteFailed, "rename", map[string]string{"step": "report"})
	r.Report("Engine.Tick", 9, "audit", signalcenter.Code("OBSERVER_NOT_A_REGISTERED_CODE"), "drift", nil)
	got := rc.all()
	if len(got) != 3 {
		t.Fatalf("one event per call, got %d", len(got))
	}
	if got[0].Severity != signalcenter.SeverityIncident || got[0].Module != signalcenter.ModuleObserver || got[0].Kind != signalcenter.KindObserverWarning || got[0].Origin != "Engine.Tick" || got[0].Cycle != 9 || got[0].Phase != "audit" || got[0].Fields["step"] != "respond" {
		t.Errorf("stall kill: %+v", got[0])
	}
	if got[1].Severity != signalcenter.SeverityWarn || got[1].Origin != "Engine.WriteReport" {
		t.Errorf("report fault: %+v", got[1])
	}
	if got[2].Severity != signalcenter.SeverityWarn || got[2].Fields["raw_code"] != "OBSERVER_NOT_A_REGISTERED_CODE" {
		t.Errorf("an unregistered code is WARN with the Center's drift stamp, never a panic: %+v", got[2])
	}
}

// --- 34/35: the severity table and the registry -------------------------------

// leafCodeConsts scans the leaf's non-test sources for every `Code*` const
// declared with the signalcenter.Code type.
func leafCodeConsts(t *testing.T) []signalcenter.Code {
	t.Helper()
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	var out []signalcenter.Code
	fset := token.NewFileSet()
	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, name, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		for _, decl := range f.Decls {
			gd, ok := decl.(*ast.GenDecl)
			if !ok || gd.Tok != token.CONST {
				continue
			}
			for _, spec := range gd.Specs {
				vs := spec.(*ast.ValueSpec)
				for i, n := range vs.Names {
					if strings.HasPrefix(n.Name, "Code") && len(vs.Values) > i {
						lit := vs.Values[i].(*ast.BasicLit)
						out = append(out, signalcenter.Code(strings.Trim(lit.Value, `"`)))
					}
				}
			}
		}
	}
	return out
}

// TestSeverityTable_CompleteAndRegistered — every Code* const has a severityOf
// row, is registered under ModuleObserver, BelongsTo the module, and carries
// the §5 severity (INCIDENT for the kill, WARN otherwise). Kills M50 (a code
// without a row), M51 (a mis-prefixed code).
func TestSeverityTable_CompleteAndRegistered(t *testing.T) {
	t.Parallel()
	codes := leafCodeConsts(t)
	if len(codes) != 8 {
		t.Fatalf("the unit declares eight codes, found %d: %v", len(codes), codes)
	}
	for _, c := range codes {
		sev, ok := severityOf[c]
		if !ok {
			t.Errorf("%s has no severityOf row", c)
		}
		want := signalcenter.SeverityWarn
		if c == CodeStallKillSent {
			want = signalcenter.SeverityIncident
		}
		if sev != want {
			t.Errorf("%s severity = %s, want %s", c, sev, want)
		}
		if m, ok := signalcenter.IsRegistered(c); !ok || m != signalcenter.ModuleObserver {
			t.Errorf("%s registered under %q (%v), want observer", c, m, ok)
		}
		if !c.BelongsTo(signalcenter.ModuleObserver) {
			t.Errorf("%s does not carry the OBSERVER_ prefix", c)
		}
	}
	if len(severityOf) != len(codes) {
		t.Errorf("severityOf has %d rows for %d codes", len(severityOf), len(codes))
	}
}

// TestCodes_RegisteredOnceUnderModuleObserver — the registry lists exactly the
// eight, each with a non-empty doc. Kills M52 (a RegisterCode removed).
func TestCodes_RegisteredOnceUnderModuleObserver(t *testing.T) {
	t.Parallel()
	docs := signalcenter.RegisteredCodes()[signalcenter.ModuleObserver]
	want := []signalcenter.Code{CodeEventsSinkOpenFailed, CodeEventAppendFailed, CodeKillFailed, CodeNudgeAppendFailed, CodeReportWriteFailed, CodeStallKillSent, CodeStdoutTailFailed, CodeWatcherLeaked}
	if len(docs) != len(want) {
		t.Fatalf("registered observer codes = %d, want %d: %+v", len(docs), len(want), docs)
	}
	for i, d := range docs {
		if d.Code != want[i] || d.Doc == "" {
			t.Errorf("registry[%d] = %+v, want %s with a doc", i, d, want[i])
		}
	}
}

// TestWithSignals_ReadsTheCenterLive — the accessor is read at every use, so a
// Center installed after construction is reached. Kills M-snapshot.
func TestWithSignals_ReadsTheCenterLive(t *testing.T) {
	t.Parallel()
	var live *signalcenter.Center
	e := New(settingsFixture(t.TempDir()), depsFixture(), WithSignals(func() *signalcenter.Center { return live }))
	if e.SignalsWired() {
		t.Fatal("no Center yet")
	}
	rc := newRecordingCenter()
	live = rc.c
	if !e.SignalsWired() {
		t.Fatal("the accessor is read live")
	}
	e.reportFault("Engine.Tick", CodeKillFailed, "x", nil)
	if len(rc.byCode(CodeKillFailed)) != 1 {
		t.Errorf("reported into the late-bound Center: %+v", rc.all())
	}
	if New(settingsFixture(t.TempDir()), depsFixture()).SignalsWired() {
		t.Error("an engine without WithSignals is the Null Object")
	}
}
