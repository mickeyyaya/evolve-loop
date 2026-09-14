package defectledger

// helpers_test.go — the leaf's test fixtures: a Ledger over stub collaborators
// reporting into a recording Center, the continuation lineage the goldens were
// captured on (8e8f080f, zz_golden_capture_test.go), and the golden readers.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/continuation"
	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

// recording returns a Center accessor and the events it delivered, in order.
func recording() (func() *signalcenter.Center, *[]signalcenter.Event) {
	c := signalcenter.New()
	got := &[]signalcenter.Event{}
	c.Subscribe(func(e signalcenter.Event) { *got = append(*got, e) })
	return func() *signalcenter.Center { return c }, got
}

// scopeOf is a lane-scope reader stub returning the given ids for any workspace.
func scopeOf(ids ...string) LaneScopeReader {
	return func(string) []string { return ids }
}

var locatorRE = regexp.MustCompile(`:[0-9]+(-[0-9]+)?$`)

// resolveUnder is the citation policy stub: every ";"-joined fragment must
// name a file (minus up to two line locators) under root; the miss reason is
// the production resolver's project-root wording so the goldens replay.
func resolveUnder(root string) Resolver {
	return func(evidence string, _ Request) (bool, string) {
		for _, frag := range strings.Split(evidence, ";") {
			path := strings.TrimSpace(frag)
			for i := 0; i < 2; i++ {
				path = locatorRE.ReplaceAllString(path, "")
			}
			if _, err := os.Stat(filepath.Join(root, path)); err != nil {
				return false, "evidence " + strconv.Quote(evidence) + " resolves to no file under the project root"
			}
		}
		return true, ""
	}
}

func resolveNever(_ string, _ Request) (bool, string) { return false, "stub: never" }

// observed builds a ledger over the stubs, reporting into a recording Center.
func observed(scope LaneScopeReader, resolve Resolver) (*Ledger, *[]signalcenter.Event) {
	acc, got := recording()
	return New(scope, resolve, WithSignals(acc)), got
}

// mustEmit runs a clean Emit: a zero Verdict — no diagnostic, nothing blocked.
func mustEmit(t *testing.T, l *Ledger, req Request, r Rejection) {
	t.Helper()
	if v := l.Emit(req, r); len(v.Diagnostics) != 0 || v.Blocked {
		t.Fatalf("a clean emit returns a zero Verdict: %+v", v)
	}
}

func codesOf(events []signalcenter.Event) []signalcenter.Code {
	var out []signalcenter.Code
	for _, e := range events {
		out = append(out, e.Code)
	}
	return out
}

// only returns the one event carrying code and fails when there are zero or several.
func only(t *testing.T, events []signalcenter.Event, code signalcenter.Code) signalcenter.Event {
	t.Helper()
	var hits []signalcenter.Event
	for _, e := range events {
		if e.Code == code {
			hits = append(hits, e)
		}
	}
	if len(hits) != 1 {
		t.Fatalf("want exactly one %s, got %d in %v", code, len(hits), codesOf(events))
	}
	return hits[0]
}

// fixture is <root>/.evolve/runs/cycle-1270 continuing cycle-1255 — the
// lineage the goldens were captured on.
type fixture struct {
	root, ws, ancestorWS string
	req                  Request
}

func newFixture(t *testing.T) fixture {
	t.Helper()
	root := t.TempDir()
	f := fixture{root: root, ancestorWS: filepath.Join(root, ".evolve", "runs", "cycle-1255"), ws: filepath.Join(root, ".evolve", "runs", "cycle-1270")}
	for _, d := range []string{f.ancestorWS, f.ws, filepath.Join(root, "go")} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	for _, rel := range []string{"go/x.go", "go/y.go"} {
		f.write(t, rel, "package x\n")
	}
	if err := continuation.WriteManifest(f.ws, continuation.Continuation{Cycle: 1255, Branch: "cycle-1255", SnapshotSHA: "deadbeef", BaseSHA: "cafebabe"}); err != nil {
		t.Fatal(err)
	}
	f.req = Request{Cycle: 1270, Workspace: f.ws, ProjectRoot: root}
	return f
}

func (f fixture) write(t *testing.T, rel, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(f.root, rel), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func (f fixture) ancestorLedger(t *testing.T, body string) {
	f.write(t, ".evolve/runs/cycle-1255/defect-ledger.json", body)
}
func (f fixture) ownLedger(t *testing.T, body string) {
	f.write(t, ".evolve/runs/cycle-1270/defect-ledger.json", body)
}
func (f fixture) claims(t *testing.T, body string) {
	f.write(t, ".evolve/runs/cycle-1270/defect-dispositions.json", body)
}

func (f fixture) removeManifest(t *testing.T) {
	t.Helper()
	if err := os.Remove(filepath.Join(f.ws, "continuation-manifest.json")); err != nil {
		t.Fatal(err)
	}
}

// bindRegistry writes the root-owned registry binding the lane "lane-scope-id" to cycle.
func (f fixture) bindRegistry(t *testing.T, cycle int) {
	t.Helper()
	binding := map[string]any{"cycle": cycle, "branch": "cycle-" + strconv.Itoa(cycle), "snapshot_sha": "deadbeef", "base_sha": "cafebabe"}
	body, err := json.Marshal(map[string]any{"lane-scope-id": binding})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(continuation.RegistryPath(f.root), body, 0o644); err != nil {
		t.Fatal(err)
	}
}

const (
	ancestorOneOpen    = `{"origin_cycle":1255,"entries":[{"id":"d1","text":"first defect","status":"OPEN"}]}`
	ancestorTwoOpen    = `{"origin_cycle":1255,"entries":[{"id":"d1","text":"first defect","status":"OPEN"},{"id":"d2","text":"second defect","status":"OPEN"}]}`
	reconciledAncestor = `{"origin_cycle":1250,"entries":[
{"id":"d1","text":"fixed one","status":"OPEN"},
{"id":"d2","text":"deferred one","status":"OPEN"},
{"id":"d3","text":"unaccounted one","status":"OPEN"},
{"id":"d4","text":"shadowed one","status":"OPEN"},
{"id":"d5","text":"closed upstream","status":"FIXED","evidence":"docs/x.md","reason":"done"}]}`
	reconciledCurrent = `{"origin_cycle":1270,"entries":[{"id":"own1","text":"this cycle's own defect","status":"OPEN"},{"id":"d4","text":"planted different text","status":"FIXED","evidence":"x"}]}`
	reconciledClaims  = `{"dispositions":[{"id":"d1","status":"FIXED","evidence":["go/x.go:12","go/y.go"]},{"id":"d2","status":"DEFERRED","reason":"out of scope"},{"id":"d4","status":"FIXED","evidence":"go/x.go"}]}`
)

// scenario is one row of diagnostics.golden.json (captured on 8e8f080f).
type scenario struct {
	Diagnostics []cyclestate.Diagnostic `json:"diagnostics"`
	Blocked     bool                    `json:"blocked"`
	Lineage     []int                   `json:"lineage"`
}

func goldenBytes(t *testing.T, name string) []byte {
	t.Helper()
	body, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	return body
}

var tmpNameRE = regexp.MustCompile(`\.defect-ledger\.json\.[0-9]+\.tmp`)

// goldenScenario returns the named G4 row with the fixture's paths substituted.
func goldenScenario(t *testing.T, name string, f fixture) scenario {
	t.Helper()
	var all map[string]scenario
	if err := json.Unmarshal(goldenBytes(t, "diagnostics.golden.json"), &all); err != nil {
		t.Fatal(err)
	}
	s, ok := all[name]
	if !ok {
		t.Fatalf("no golden scenario %q", name)
	}
	for i := range s.Diagnostics {
		s.Diagnostics[i].Message = strings.ReplaceAll(strings.ReplaceAll(s.Diagnostics[i].Message, "{{WS}}", f.ws), "{{ROOT}}", f.root)
	}
	return s
}

// assertVerdict compares a Verdict with a golden row, normalizing the random temp name.
func assertVerdict(t *testing.T, name string, got Verdict, want scenario) {
	t.Helper()
	if got.Blocked != want.Blocked || len(got.Diagnostics) != len(want.Diagnostics) || strings.Join(ints(got.LineageCycles), ",") != strings.Join(ints(want.Lineage), ",") {
		t.Fatalf("%s: blocked=%v lineage=%v diagnostics=%d; want blocked=%v lineage=%v diagnostics=%d\n%s", name, got.Blocked, got.LineageCycles, len(got.Diagnostics), want.Blocked, want.Lineage, len(want.Diagnostics), diagsText(got.Diagnostics))
	}
	for i := range got.Diagnostics {
		g, w := got.Diagnostics[i], want.Diagnostics[i]
		if g.Severity != w.Severity || tmpNameRE.ReplaceAllString(g.Message, ".defect-ledger.json.N.tmp") != w.Message {
			t.Fatalf("%s: diagnostic %d\n got %s: %s\nwant %s: %s", name, i, g.Severity, g.Message, w.Severity, w.Message)
		}
	}
}

func ints(in []int) []string {
	out := make([]string, len(in))
	for i, n := range in {
		out[i] = strconv.Itoa(n)
	}
	return out
}

func diagsText(diags []cyclestate.Diagnostic) string {
	var b strings.Builder
	for _, d := range diags {
		b.WriteString(d.Severity + ": " + d.Message + "\n")
	}
	return b.String()
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(body)
}
