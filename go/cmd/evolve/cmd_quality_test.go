package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/events/channel"
	"github.com/mickeyyaya/evolve-loop/go/internal/events/rootwire"
	"github.com/mickeyyaya/evolve-loop/go/internal/paths"
	"github.com/mickeyyaya/evolve-loop/go/internal/qualitypanel"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

func fakeQualityEnv(readers ...qualityReader) qualityEnv {
	return qualityEnv{scopes: map[string][]qualityReader{"wave": readers, "tree": nil}, newCenter: publishingQualityCenter}
}

func publishingQualityCenter(root, evolveDir string, console io.Writer) (*signalcenter.Center, func()) {
	c := newRootSignalCenter(root, evolveDir, console)
	return c, rootwire.Attach(c, "quality", evolveDir, console)
}

func readerOf(index string, readings []qualitypanel.Reading, err error) qualityReader {
	return qualityReader{index: index, read: func(string) ([]qualitypanel.Reading, error) { return readings, err }}
}

func qualityProject(t *testing.T, policyJSON string) string {
	t.Helper()
	root := t.TempDir()
	dir := filepath.Join(root, ".evolve")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if policyJSON != "" {
		if err := os.WriteFile(filepath.Join(dir, "policy.json"), []byte(policyJSON), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func channelEvents(t *testing.T, root, name string) []signalcenter.Event {
	t.Helper()
	log, err := channel.New(paths.EventChannelsRoot(paths.EvolveDirOf(root)), name, channel.Config{})
	if err != nil {
		t.Fatal(err)
	}
	batch, err := log.Read(0)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("read channel %s: %v", name, err)
	}
	var out []signalcenter.Event
	for _, r := range batch.Records {
		if r.Signal != nil {
			out = append(out, *r.Signal)
		}
	}
	return out
}

func runQualityReport(t *testing.T, env qualityEnv, root string) (int, string) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	rc := runQualityWith(env, []string{"report", "--scope", "wave", "--project-root", root}, &stdout, &stderr)
	return rc, stdout.String()
}

func TestQualityReport_AKnownKAndNReachesTheReadingChannelWithTheExactWilsonBounds(t *testing.T) {
	t.Parallel()
	root := qualityProject(t, "")
	reading := qualitypanel.Reading{Index: "PQ1", Interval: qualitypanel.Wilson(3, 5), Method: qualitypanel.MethodWilson95, N: 5, Window: "waves:140", Source: ".evolve/runs"}

	rc, stdout := runQualityReport(t, fakeQualityEnv(readerOf("PQ1", []qualitypanel.Reading{reading}, nil)), root)

	if rc != 0 || !strings.Contains(stdout, "1 reading(s), 0 breach(es), 0 warning(s)") {
		t.Fatalf("rc %d stdout %q, want 0 and one reading", rc, stdout)
	}
	events := channelEvents(t, root, "quality.reading")
	if len(events) != 1 || events[0].Kind != signalcenter.KindQualityReading {
		t.Fatalf("channel quality.reading = %+v, want one reading", events)
	}
	f := events[0].Fields
	if f["value"] != "0.600" || f["lo"] != "0.231" || f["hi"] != "0.882" || f["stage"] != "report" {
		t.Errorf("fields = %v, want 0.600 [0.231, 0.882] at the default stage report", f)
	}
	if got := channelEvents(t, root, "quality.breach"); len(got) != 0 {
		t.Errorf("channel quality.breach = %+v, want nothing for an inconclusive reading", got)
	}
}

func TestQualityReport_ACycleReadingGoesToItsCycleStream(t *testing.T) {
	t.Parallel()
	root := qualityProject(t, "")
	reading := qualitypanel.Reading{Index: "TQ2", Interval: qualitypanel.Wilson(9, 10), Method: qualitypanel.MethodWilson95, N: 10, Cycle: 2028}

	if rc, _ := runQualityReport(t, fakeQualityEnv(readerOf("TQ2", []qualitypanel.Reading{reading}, nil)), root); rc != 0 {
		t.Fatalf("rc = %d", rc)
	}
	chunk, err := signalcenter.ReadStream(filepath.Join(paths.RunWorkspace(root, 2028), signalcenter.StreamFileName), 0)
	if err != nil || len(chunk.Events) != 1 || chunk.Events[0].Fields["index"] != "TQ2" {
		t.Errorf("cycle stream = %+v, %v; want the TQ2 reading", chunk.Events, err)
	}
}

func TestQualityReport_TheStageComesOnlyFromPolicy(t *testing.T) {
	t.Parallel()
	breach := qualitypanel.Reading{Index: "PQ1", Interval: qualitypanel.Wilson(5, 60), Method: qualitypanel.MethodWilson95, N: 60}
	cases := []struct {
		policy     string
		wantBreach int
	}{
		{"", 1},
		{`{"quality":{"indexes":{"PQ1":{"stage":"shadow"}}}}`, 0},
		{`{"quality":{"indexes":{"PQ1":{"stage":"gate"}}}}`, 1},
	}
	for _, c := range cases {
		root := qualityProject(t, c.policy)

		_, stdout := runQualityReport(t, fakeQualityEnv(readerOf("PQ1", []qualitypanel.Reading{breach}, nil)), root)

		if !strings.Contains(stdout, fmt.Sprintf(" %d breach(es)", c.wantBreach)) || len(channelEvents(t, root, "quality.breach")) != c.wantBreach {
			t.Errorf("policy %q: stdout %q, want %d breach(es) on the breach channel", c.policy, stdout, c.wantBreach)
		}
	}
}

func TestQualityReport_ACollectorFailureWarnsAndExitsZero(t *testing.T) {
	t.Parallel()
	absent := &fs.PathError{Op: "open", Path: ".evolve/waves", Err: fs.ErrNotExist}
	root := qualityProject(t, "")

	rc, stdout := runQualityReport(t, fakeQualityEnv(readerOf("PQ1", nil, absent), readerOf("JQ3", nil, errors.New("bad verdict line"))), root)

	if rc != 0 || !strings.Contains(stdout, "2 warning(s)") {
		t.Fatalf("rc %d stdout %q, want exit 0 with two warnings", rc, stdout)
	}
	codes := map[signalcenter.Code]string{}
	for _, e := range channelEvents(t, root, "quality.breach") {
		codes[e.Code] = e.Fields["index"] + "|" + e.Fields["source"]
	}
	want := map[signalcenter.Code]string{qualitypanel.CodeSourceAbsent: "PQ1|.evolve/waves", qualitypanel.CodeSourceUnreadable: "JQ3|"}
	for code, v := range want {
		if codes[code] != v {
			t.Errorf("%s -> %q, want %q (all: %v)", code, codes[code], v, codes)
		}
	}
}

func TestQualityReport_ABadPolicyWarnsWithItsSource(t *testing.T) {
	t.Parallel()
	root := qualityProject(t, `{"quality":{"indexes":{"PQ1":{"stage":"on"}}}}`)

	rc, _ := runQualityReport(t, fakeQualityEnv(), root)

	events := channelEvents(t, root, "quality.breach")
	if rc != 0 || len(events) != 1 || events[0].Code != qualitypanel.CodeSourceUnreadable || events[0].Fields["source"] != ".evolve/policy.json" {
		t.Errorf("rc %d events = %+v, want exit 0 and one QUALITY_SOURCE_UNREADABLE naming .evolve/policy.json", rc, events)
	}
}

func TestQualityReport_AScopeWithNoReadersSaysSo(t *testing.T) {
	t.Parallel()
	var stdout, stderr bytes.Buffer

	rc := runQualityWith(fakeQualityEnv(), []string{"report", "--scope", "tree", "--project-root", qualityProject(t, "")}, &stdout, &stderr)

	if rc != 0 || !strings.Contains(stdout.String(), "scope tree has no index readers yet") {
		t.Errorf("rc %d stdout %q", rc, stdout.String())
	}
}

func TestQualityReport_UsageErrors(t *testing.T) {
	t.Parallel()
	root := qualityProject(t, "")
	for _, args := range [][]string{nil, {"show"}, {"report", "--scope", "galaxy", "--project-root", root}, {"report", "--scope", "wave"}, {"report", "--scope", "wave", "--project-root", root, "extra"}, {"report", "--scope"}} {
		var stdout, stderr bytes.Buffer
		if rc := runQualityWith(fakeQualityEnv(), args, &stdout, &stderr); rc != exitUsage || !strings.Contains(stderr.String(), "usage: evolve quality report") {
			t.Errorf("args %q: rc %d stderr %q, want %d with the usage", args, rc, stderr.String(), exitUsage)
		}
	}
}

func TestQualityReport_IsVisibleInSignalsTailByKind(t *testing.T) {
	t.Parallel()
	root := qualityProject(t, "")
	reading := qualitypanel.Reading{Index: "JQ3", Interval: qualitypanel.Wilson(41, 98), Method: qualitypanel.MethodWilson95, N: 98}
	runQualityReport(t, fakeQualityEnv(readerOf("JQ3", []qualitypanel.Reading{reading}, nil), readerOf("PQ1", nil, errors.New("bad"))), root)
	var stdout, stderr bytes.Buffer

	rc := runSignalsTail(context.Background(), []string{"--channel", "signals", "--kind", "quality.reading,quality.breach,quality.warning"}, tailEnv{ProjectRoot: root}, &stdout, &stderr)

	out := stdout.String()
	if rc != 0 || strings.Count(out, "\n") != 2 || !strings.Contains(out, "quality.reading") || !strings.Contains(out, "JQ3") || !strings.Contains(out, "QUALITY_SOURCE_UNREADABLE") {
		t.Errorf("rc %d tail output %q (stderr %q), want the JQ3 reading and the PQ1 warning", rc, out, stderr.String())
	}
}

func TestNewQualityCenter_AttachesThePublisherUnderTheQualityRole(t *testing.T) {
	root := qualityProject(t, "")
	var roles []string
	prevAttach := attachRootEventsFn
	t.Cleanup(func() { attachRootEventsFn = prevAttach })
	attachRootEventsFn = func(c *signalcenter.Center, role, _ string, _ io.Writer) func() {
		roles = append(roles, role)
		return c.Flush
	}

	center, closeCenter := newQualityCenter(root, paths.EvolveDirOf(root), io.Discard)
	center.Emit(qualitypanel.Events(qualitypanel.Reading{Index: "OQ3", Method: qualitypanel.MethodWilson95, N: 695, Interval: qualitypanel.Wilson(0, 695)}, qualitypanel.StageReport)[0])
	closeCenter()

	if len(roles) != 1 || roles[0] != "quality" {
		t.Errorf("attached roles = %v, want [quality]", roles)
	}
}

func TestBoundary_RunsTheWaveQualityReportBeforeGC(t *testing.T) {
	t.Parallel()
	steps := boundaryHaltSteps(boundaryArgs{}, "/plane")
	q := steps[len(steps)-2]

	if q.verb != "quality" || strings.Join(q.args, " ") != "report --scope wave --project-root /plane" || steps[len(steps)-1].verb != "gc" {
		t.Errorf("step before gc = %s %v, want quality report --scope wave --project-root /plane", q.verb, q.args)
	}
	var stdout, stderr bytes.Buffer
	if rc := dispatchBoundaryVerb("quality", []string{"report"}, &stdout, &stderr); rc != exitUsage || !strings.Contains(stderr.String(), qualityUsage) {
		t.Errorf("the boundary dispatches quality to runQuality: rc %d stderr %q", rc, stderr.String())
	}
}

func TestDispatchBoundaryVerb_AnUnknownVerbIsAnIOError(t *testing.T) {
	t.Parallel()
	var stdout, stderr bytes.Buffer
	if rc := dispatchBoundaryVerb("telescope", nil, &stdout, &stderr); rc != exitIO || !strings.Contains(stderr.String(), `no handler for step "telescope"`) {
		t.Errorf("rc %d stderr %q, want %d and the verb named", rc, stderr.String(), exitIO)
	}
}

func TestQualityArgs_ARelativeRootBecomesAbsoluteForThePublisher(t *testing.T) {
	t.Parallel()
	_, root, err := parseQualityArgs([]string{"report", "--scope", "wave", "--project-root", "."}, qualityScopes)
	c, cerr := parseQualityCompare([]string{"--index", "PQ1", "--a", "wave:1", "--b", "wave:2", "--project-root", "."})
	if err != nil || cerr != nil || !filepath.IsAbs(root) || !filepath.IsAbs(c.root) {
		t.Errorf("report root %q (%v), compare root %q (%v); want absolute paths, since the publisher refuses a relative evolve directory", root, err, c.root, cerr)
	}
}

func TestQualityReport_TheTreeScopeReadsDepthDuplicationAndJudgmentSurface(t *testing.T) {
	t.Parallel()
	root := pipelineProject(t)
	for rel, body := range map[string]string{"go/internal/a/a.go": "package a\n\nfunc A() int { return 1 }\n", "agents/x.md": "- If asked, answer.\n"} {
		p := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	env := qualityEnv{scopes: map[string][]qualityReader{"wave": treeQualityReaders()}, newCenter: publishingQualityCenter}

	rc, stdout := runQualityReport(t, env, root)

	if rc != 0 || !strings.Contains(stdout, "4 reading(s)") {
		t.Fatalf("rc %d stdout %q, want CQ3, CQ4 tree, CQ4 added and CQ5", rc, stdout)
	}
	got := map[string]string{}
	for _, e := range channelEvents(t, root, "quality.reading") {
		got[e.Fields["index"]+" "+e.Fields["window"]] = e.Fields["value"] + " " + e.Fields["stage"]
	}
	want := map[string]string{"CQ3 tree": "1.000 shadow", "CQ5 tree": "1.000 shadow", "CQ4 tree": "0.000 shadow", "CQ4 wave:2/added": "0.000 shadow"}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("%s = %q, want %q (all %v)", k, got[k], v, got)
		}
	}
	if len(qualityScopes["tree"]) != 2 {
		t.Errorf("the production tree scope has %d readers, want 2: one scan for CQ3 and CQ4, and CQ5", len(qualityScopes["tree"]))
	}
}

func TestTreeQualityReaders_AMissingWaveRecordWarnsForTheAddedForm(t *testing.T) {
	t.Parallel()
	root := qualityProject(t, "")
	if err := os.MkdirAll(filepath.Join(root, "go"), 0o755); err != nil {
		t.Fatal(err)
	}
	env := qualityEnv{scopes: map[string][]qualityReader{"wave": treeQualityReaders()}, newCenter: publishingQualityCenter}

	_, stdout := runQualityReport(t, env, root)

	if !strings.Contains(stdout, "3 reading(s)") || !strings.Contains(stdout, "4 warning(s)") {
		t.Errorf("stdout %q", stdout)
	}
	absent := 0
	for _, e := range channelEvents(t, root, "quality.breach") {
		if e.Code == "QUALITY_SOURCE_ABSENT" && e.Fields["index"] == "CQ4" {
			absent++
		}
	}
	if absent != 1 {
		t.Errorf("CQ4 absent warnings = %d, want 1 for the missing wave records", absent)
	}
}

func TestTreeQualityReaders_AFailedShipDiffWarnsAndAddsNoEmptyReading(t *testing.T) {
	t.Parallel()
	root := qualityProject(t, "")
	for rel, body := range map[string]string{
		"go/internal/a/a.go":                     "package a\n\nfunc A() int { return 1 }\n",
		".evolve/waves/wave-1.json":              `{"number":1,"cycle_floor":1}`,
		".evolve/runs/cycle-1/run.json":          `{"shipped":true}`,
		".evolve/runs/cycle-1/ship-binding.json": `{"commit_sha":"0000000000000000000000000000000000000000"}`,
	} {
		p := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	env := qualityEnv{scopes: map[string][]qualityReader{"wave": treeQualityReaders()}, newCenter: publishingQualityCenter}

	_, stdout := runQualityReport(t, env, root)

	for _, e := range channelEvents(t, root, "quality.reading") {
		if e.Fields["window"] == "wave:1/added" || e.Fields["window"] == "" {
			t.Errorf("reading %v, want no added-form reading when its diff failed", e.Fields)
		}
	}
	unreadable := 0
	for _, e := range channelEvents(t, root, "quality.breach") {
		if e.Code == "QUALITY_SOURCE_UNREADABLE" && e.Fields["index"] == "CQ4" {
			unreadable++
		}
	}
	if !strings.Contains(stdout, "3 reading(s)") || unreadable != 1 {
		t.Errorf("stdout %q, CQ4 unreadable warnings %d; want CQ3, CQ4 tree and CQ5 and one warning for the failed git show", stdout, unreadable)
	}
}
