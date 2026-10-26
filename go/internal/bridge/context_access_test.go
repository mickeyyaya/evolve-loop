package bridge

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/inputindex"
	"github.com/mickeyyaya/evolve-loop/go/internal/isolation"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

func grantReport(tag, path string) string {
	return `{"eventMessage":"Sandbox: cat(1810) allow file-read-data ` + path + `\nevolve-cell=` + tag + ` rule=grant.index"}` + "\n"
}

func contextEvents(center *signalcenter.Center) []signalcenter.Event {
	var out []signalcenter.Event
	for _, e := range center.Recent() {
		if e.Kind == signalcenter.KindIsolationContext {
			out = append(out, e)
		}
	}
	return out
}

func TestReportReads_WritesOneAccessRecordForEachOpenedEntry(t *testing.T) {
	t.Parallel()
	c := newReadCell(t)
	c.index(t)
	idx, err := inputindex.Read(c.ws)
	if err != nil {
		t.Fatal(err)
	}
	tag := "run1/7/build/p12n3"
	scout := filepath.Join(c.ws, "scout-report.md")
	center := signalcenter.New()
	deps := Deps{Signals: center, ReadStage: "shadow", hostOS: "darwin", Env: map[string]string{},
		SandboxReports: func(context.Context, string, time.Time) ([]byte, error) {
			return []byte(grantReport(tag, scout) + grantReport(tag, scout) + sbplReportFixture), nil
		}}
	cfg := &Config{CLI: "claude", Agent: "build", Cycle: 7, Workspace: c.ws, ProjectRoot: c.root, Worktree: "/wt", DispatchID: tag}
	deps.reportReadViolations(context.Background(), cfg, time.Now())
	records, err := inputindex.ReadAccess(c.root)
	if err != nil {
		t.Fatal(err)
	}
	var opens []string
	var cell inputindex.AccessRecord
	for _, r := range records {
		if r.Type == inputindex.AccessOpen {
			opens = append(opens, r.Kind)
		} else {
			cell = r
		}
	}
	if !slices.Equal(opens, []string{"scout-report"}) || cell.Source != inputindex.AccessSourceSandbox || cell.Dispatch != tag {
		t.Fatalf("records = %+v, want one open of the scout report from the sandbox", records)
	}
	unopened := slices.DeleteFunc(slices.Clone(idx.Required), func(id string) bool { return slices.Contains(cell.Opened, id) })
	events := contextEvents(center)
	if len(unopened) == 0 || len(events) != 1 || events[0].Code != isolation.CodeContextRequiredUnopened || events[0].Severity != signalcenter.SeverityInfo ||
		events[0].Fields["unopened"] != strings.Join(unopened, ",") {
		t.Fatalf("events = %+v, want one INFO that names the unopened required ids %v", events, unopened)
	}
	if got := readViolations(center); len(got) != 2 {
		t.Fatalf("the grant reports must never become violations: %+v", got)
	}
}

func TestReportReads_ACellWithNoReportSourceWritesASummaryWithSourceNone(t *testing.T) {
	t.Parallel()
	c := newReadCell(t)
	c.index(t)
	var stderr bytes.Buffer
	for name, deps := range map[string]Deps{
		"linux":              {hostOS: "linux", ReadStage: "shadow"},
		"unreadable reports": {hostOS: "darwin", ReadStage: "shadow", Stderr: &stderr, SandboxReports: func(context.Context, string, time.Time) ([]byte, error) { return nil, errors.New("log: denied") }},
	} {
		center := signalcenter.New()
		deps.Signals = center
		deps.reportReadViolations(context.Background(), &Config{Agent: "build", Cycle: 7, Workspace: c.ws, ProjectRoot: c.root, Worktree: "/wt", DispatchID: "run1/7/build/p1n" + name[:1]}, time.Now())
		if len(contextEvents(center)) != 0 {
			t.Errorf("%s: a cell with no report source emits no unopened signal", name)
		}
	}
	records, _ := inputindex.ReadAccess(c.root)
	if len(records) != 2 || records[0].Source != inputindex.AccessSourceNone || records[1].Source != inputindex.AccessSourceNone || records[0].Type != inputindex.AccessCell {
		t.Fatalf("records = %+v, want two cell summaries with source none", records)
	}
}

func TestReportReads_SkipsTheAccessLogOfAnotherPhaseOrOfNoIndexAndWarnsOnAWriteFailure(t *testing.T) {
	t.Parallel()
	c := newReadCell(t)
	deps := Deps{hostOS: "linux", ReadStage: "shadow"}
	deps.reportReadViolations(context.Background(), &Config{Agent: "build", Cycle: 7, Workspace: c.ws, ProjectRoot: c.root}, time.Now())
	c.index(t)
	deps.reportReadViolations(context.Background(), &Config{Agent: "phase-advisor", Cycle: 7, Workspace: c.ws, ProjectRoot: c.root}, time.Now())
	if records, _ := inputindex.ReadAccess(c.root); len(records) != 0 {
		t.Fatalf("no index, or the index of another phase, writes no access record: %+v", records)
	}
	writeFixtureFile(t, filepath.Join(c.root, ".evolve", "context-access"), "a file where the directory belongs")
	var stderr bytes.Buffer
	writeFixtureFile(t, inputindex.Path(filepath.Join(c.runs, "cycle-5")), "{")
	Deps{hostOS: "linux", ReadStage: "shadow", Stderr: &stderr}.reportReadViolations(context.Background(), &Config{Agent: "build", Cycle: 5, Workspace: filepath.Join(c.runs, "cycle-5"), ProjectRoot: c.root}, time.Now())
	if !strings.Contains(stderr.String(), "WARN context access log") || !strings.Contains(stderr.String(), "no access record") {
		t.Fatalf("a corrupt index must warn, got %q", stderr.String())
	}
	stderr.Reset()
	deps.Stderr = &stderr
	deps.reportReadViolations(context.Background(), &Config{Agent: "build", Cycle: 7, Workspace: c.ws, ProjectRoot: c.root}, time.Now())
	if !strings.Contains(stderr.String(), "WARN context access log") {
		t.Fatalf("a failed write of the access log must warn, got %q", stderr.String())
	}
}

func TestReportReads_EmitsTheBudgetOverflowOfEachLayer(t *testing.T) {
	t.Parallel()
	c := newReadCell(t)
	idx, err := inputindex.Build(inputindex.Request{ProjectRoot: c.root, Workspace: c.ws, Cycle: 7, Phase: "build", Context: inputindex.ContextPolicy{Budget: inputindex.Budget{L1Tokens: 1}}})
	if err != nil {
		t.Fatal(err)
	}
	if err := inputindex.Write(c.ws, idx); err != nil {
		t.Fatal(err)
	}
	center := signalcenter.New()
	Deps{Signals: center, hostOS: "linux", ReadStage: "shadow"}.reportReadViolations(context.Background(), &Config{Agent: "build", Cycle: 7, Workspace: c.ws, ProjectRoot: c.root}, time.Now())
	events := contextEvents(center)
	if len(events) != 1 || events[0].Code != isolation.CodeContextBudgetOver || events[0].Severity != signalcenter.SeverityWarn || events[0].Fields["layer"] != "L1" || events[0].Fields["limit_tokens"] != "1" {
		t.Fatalf("events = %+v, want one WARN for the L1 overflow", events)
	}
}

func TestCellTag_AcceptsAMintedDispatchIDAndRefusesAQuote(t *testing.T) {
	t.Parallel()
	if id := mintDispatchID(&Config{RunID: "r1", Cycle: 7, Agent: "build"}, 42); !cellTag.MatchString(id) {
		t.Fatalf("a minted dispatch id %q must be a valid cell tag, or the read reports of a real cell are never read", id)
	}
	for _, bad := range []string{`d42" OR 1`, "d42 x", "d42\nx", `d42\x`, ""} {
		if cellTag.MatchString(bad) {
			t.Fatalf("%q must not reach the log predicate or the SBPL message", bad)
		}
	}
}

func TestIndexGrants_ReportsTheReadsOfAnIndexedRepoPathWithoutAGrant(t *testing.T) {
	t.Parallel()
	c := newReadCell(t)
	lesson := filepath.Join(c.root, ".evolve", "instincts", "lessons", "a.yaml")
	writeFixtureFile(t, lesson, "summary: a\n")
	idx := inputindex.Index{Schema: inputindex.Schema, Entries: []inputindex.Entry{{Path: ".evolve/instincts/lessons/a.yaml"}, {Path: ".evolve/runs/cycle-6/retrospective-report.md"}}}
	if err := inputindex.Write(c.ws, idx); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(c.root, ".evolve", "instincts"), 0o755); err != nil {
		t.Fatal(err)
	}
	ri := Deps{ReadStage: "enforce"}.readIsolation(c.request(), c.home)
	if !slices.Equal(ri.ReportOnly, []string{lesson}) || slices.Contains(ri.Grants, lesson) {
		t.Fatalf("grants %v report-only %v: a repo path gets a report rule and no grant", ri.Grants, ri.ReportOnly)
	}
}
