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

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/sandbox"
	"github.com/mickeyyaya/evolve-loop/go/internal/inputindex"
	"github.com/mickeyyaya/evolve-loop/go/internal/isolation"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
	"github.com/mickeyyaya/evolve-loop/go/internal/tokenusage"
)

type readCell struct {
	root, ws, runs, home string
}

func realDir(t *testing.T) string {
	t.Helper()
	d, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return d
}

func newReadCell(t *testing.T) readCell {
	t.Helper()
	c := readCell{root: realDir(t), home: realDir(t)}
	c.runs = filepath.Join(c.root, ".evolve", "runs")
	c.ws = filepath.Join(c.runs, "cycle-7")
	for path, body := range map[string]string{
		filepath.Join(c.runs, "cycle-6", "lane-scope.json"):         `{"todo_ids":["item-a"]}`,
		filepath.Join(c.runs, "cycle-6", "retrospective-report.md"): "# lesson\n",
		filepath.Join(c.ws, "lane-scope.json"):                      `{"todo_ids":["item-a"]}`,
		filepath.Join(c.ws, "scout-stdout.log"):                     "raw\n",
		filepath.Join(c.ws, "scout-report.md"):                      "# s\n",
		filepath.Join(c.home, ".ssh", "id_fixture"):                 "secret\n",
		filepath.Join(c.home, ".netrc"):                             "secret\n",
		filepath.Join(c.ws, ".bridge", "scout-stdout.log"):          "raw\n",
	} {
		writeFixtureFile(t, path, body)
	}
	return c
}

func writeFixtureFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func (c readCell) index(t *testing.T) {
	t.Helper()
	idx, err := inputindex.Build(inputindex.Request{ProjectRoot: c.root, Workspace: c.ws, Cycle: 7, Phase: "build"})
	if err != nil {
		t.Fatal(err)
	}
	if err := inputindex.Write(c.ws, idx); err != nil {
		t.Fatal(err)
	}
}

func (c readCell) request() SandboxWrapRequest {
	return SandboxWrapRequest{Phase: "build", CLI: "claude-tmux", Workspace: c.ws, RepoRoot: c.root, Tag: "d42"}
}

func TestReadIsolation_GrantsTheIndexedPathsAndGuardsTheRunsTheRawOutputAndTheSecrets(t *testing.T) {
	t.Parallel()
	c := newReadCell(t)
	c.index(t)
	ri := Deps{ReadStage: "enforce"}.readIsolation(c.request(), c.home)
	want := sandbox.ReadIsolation{
		Enforce: true, Tag: "d42", RunsDir: c.runs, Workspace: c.ws,
		Grants:      []string{inputindex.Path(c.ws), filepath.Join(c.ws, "scout-report.md"), filepath.Join(c.runs, "cycle-6", "retrospective-report.md")},
		RawSuffixes: inputindex.RawOutputSuffixes(),
		RawFiles:    []string{filepath.Join(c.ws, "scout-stdout.log")},
		RawDirs:     []string{filepath.Join(c.ws, ".bridge")},
		SecretDirs:  []string{filepath.Join(c.home, ".ssh")},
		SecretFiles: []string{filepath.Join(c.home, ".netrc")},
	}
	if !readIsolationEqual(ri, want) {
		t.Fatalf("readIsolation =\n%+v\nwant\n%+v", ri, want)
	}
}

func readIsolationEqual(a, b sandbox.ReadIsolation) bool {
	return a.Enforce == b.Enforce && a.Tag == b.Tag && a.RunsDir == b.RunsDir && a.Workspace == b.Workspace &&
		slices.Equal(a.Grants, b.Grants) && slices.Equal(a.RawSuffixes, b.RawSuffixes) && slices.Equal(a.RawFiles, b.RawFiles) &&
		slices.Equal(a.RawDirs, b.RawDirs) && slices.Equal(a.SecretDirs, b.SecretDirs) && slices.Equal(a.SecretFiles, b.SecretFiles)
}

func TestReadIsolation_OffOrNoWorkspaceGivesNoRulesAndShadowDeniesNothing(t *testing.T) {
	t.Parallel()
	c := newReadCell(t)
	if ri := (Deps{ReadStage: "off"}).readIsolation(c.request(), c.home); ri.Workspace != "" {
		t.Fatalf("off must give no read isolation, got %+v", ri)
	}
	req := c.request()
	req.Workspace = ""
	if ri := (Deps{ReadStage: "enforce"}).readIsolation(req, c.home); ri.Workspace != "" {
		t.Fatalf("a launch with no workspace gets no read isolation, got %+v", ri)
	}
	if err := os.RemoveAll(filepath.Join(c.ws, ".bridge")); err != nil {
		t.Fatal(err)
	}
	if ri := (Deps{ReadStage: "enforce"}).readIsolation(c.request(), c.home); len(ri.RawDirs) != 0 {
		t.Fatalf("a missing .bridge directory gets no tmpfs, so bwrap makes no mount point in the workspace: %v", ri.RawDirs)
	}
	req = c.request()
	req.RepoRoot = realDir(t)
	if ri := (Deps{ReadStage: "enforce"}).readIsolation(req, c.home); ri.RunsDir != "" || ri.Workspace != c.ws {
		t.Fatalf("a root with no runs directory gets no runs rule, so bwrap mounts no tmpfs on a missing path: %+v", ri)
	}
	ri := Deps{ReadStage: "shadow"}.readIsolation(c.request(), "")
	if ri.Enforce || ri.Workspace != c.ws || len(ri.SecretDirs)+len(ri.SecretFiles) != 0 || len(ri.Grants) != 0 {
		t.Fatalf("shadow with no index and no home: %+v", ri)
	}
}

func TestReadIsolation_AMalformedIndexGrantsNothingAndWarns(t *testing.T) {
	t.Parallel()
	c := newReadCell(t)
	writeFixtureFile(t, inputindex.Path(c.ws), "{")
	var stderr bytes.Buffer
	ri := Deps{ReadStage: "enforce", Stderr: &stderr}.readIsolation(c.request(), c.home)
	if len(ri.Grants) != 0 || !strings.Contains(stderr.String(), "WARN input index") {
		t.Fatalf("grants = %v, stderr = %q; a malformed index must grant nothing and warn", ri.Grants, stderr.String())
	}
}

func TestSandboxWrap_TheProfileCarriesTheReadIsolationOfTheCellAndItsTag(t *testing.T) {
	t.Parallel()
	c := newReadCell(t)
	c.index(t)
	deps := Deps{Env: map[string]string{"EVOLVE_SANDBOX": "on"}, ReadStage: "enforce", ExecStage: "off"}
	req := c.request()
	req.Worktree = realDir(t)
	prefix, ok := defaultSandboxWrapWithProbe(deps, fakeProbe("darwin", true))(req)
	if !ok {
		t.Fatal("the wrap was refused")
	}
	sbpl, err := os.ReadFile(prefix[2])
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		`(deny file-read-data (subpath "` + c.runs + `") (with message "evolve-cell=d42 rule=read.runs.unrelated inv=I5"))`,
		`(allow file-read* file-read-data (literal "` + filepath.Join(c.runs, "cycle-6", "retrospective-report.md") + `"))`,
	} {
		if !strings.Contains(string(sbpl), want) {
			t.Fatalf("the profile must carry %s:\n%s", want, sbpl)
		}
	}
}

func TestSandboxPrefixForLaunch_TagsTheCellWithItsDispatchID(t *testing.T) {
	t.Parallel()
	var got SandboxWrapRequest
	deps := Deps{SandboxWrap: func(req SandboxWrapRequest) ([]string, bool) { got = req; return nil, false }}
	sandboxPrefixForLaunch(deps, &Config{Worktree: "/wt", DispatchID: "d-77"}, "")
	if got.Tag != "d-77" {
		t.Fatalf("the wrap request tag = %q, want the dispatch id d-77", got.Tag)
	}
}

const sbplReportFixture = `{"eventMessage":"Sandbox: cat(1810) allow file-read-data \/repo\/.evolve\/runs\/cycle-9\/audit-report.md\nevolve-cell=d42 rule=read.runs.unrelated inv=I5"}
{"eventMessage":"Sandbox: cat(1811) allow file-read-data \/repo\/.evolve\/runs\/cycle-9\/audit-report.md\nevolve-cell=d42 rule=read.runs.unrelated inv=I5"}
{"eventMessage":"Sandbox: sh(1812) deny(1) file-read-data \/repo\/.evolve\/runs\/cycle-7\/scout-stdout.log\nevolve-cell=d42 rule=read.raw.output inv=I4"}
{"eventMessage":"Sandbox: mds(1330) deny(1) system-info vfs.disk-space"}
not json
{"count":3,"finished":1}
`

func readViolations(center *signalcenter.Center) []signalcenter.Event {
	var out []signalcenter.Event
	for _, e := range center.Recent() {
		if e.Kind == signalcenter.KindIsolationViolation {
			out = append(out, e)
		}
	}
	return out
}

func TestReportReadViolations_EmitsOneSignalForEachGuardedReadOfTheCell(t *testing.T) {
	t.Parallel()
	center := signalcenter.New()
	var gotTag string
	deps := Deps{Signals: center, ReadStage: "shadow", hostOS: "darwin", Env: map[string]string{},
		SandboxReports: func(_ context.Context, tag string, _ time.Time) ([]byte, error) {
			gotTag = tag
			return []byte(sbplReportFixture), nil
		}}
	cfg := &Config{CLI: "agy", Agent: "build", Cycle: 7, Workspace: "/repo/.evolve/runs/cycle-7", ProjectRoot: "/repo", Worktree: "/wt", DispatchID: "d42"}
	deps.reportReadViolations(context.Background(), cfg, time.Now())
	got := readViolations(center)
	if gotTag != "d42" || len(got) != 2 {
		t.Fatalf("tag %q, violations %+v; want the reports of d42, one per rule and target", gotTag, got)
	}
	logged, refused := got[0], got[1]
	if logged.Code != isolation.ViolationCode(5) || logged.Fields["action"] != "logged" || logged.Fields["target"] != ".evolve/runs/cycle-9/audit-report.md" ||
		logged.Fields["rule"] != "read.runs.unrelated" || logged.Fields["source"] != "sbpl-report" || logged.Fields["stage"] != "shadow" || logged.Fields["cell"] != "build" || logged.Fields["family"] != "agy" || logged.Cycle != 7 {
		t.Fatalf("the allowed read must be an I5 violation with action=logged: %+v", logged)
	}
	if refused.Code != isolation.ViolationCode(4) || refused.Fields["action"] != "refused" || refused.Fields["invariant"] != "I4" {
		t.Fatalf("the denied read of raw output must be an I4 violation with action=refused: %+v", refused)
	}
}

func TestReportReadViolations_SkipsACellThatHasNoReadReport(t *testing.T) {
	t.Parallel()
	cfg := &Config{Workspace: "/ws", Worktree: "/wt", DispatchID: "d1"}
	for name, c := range map[string]struct {
		deps Deps
		cfg  Config
	}{
		"linux":           {Deps{hostOS: "linux", ReadStage: "shadow"}, *cfg},
		"off stage":       {Deps{hostOS: "darwin", ReadStage: "off"}, *cfg},
		"no dispatch id":  {Deps{hostOS: "darwin", ReadStage: "shadow"}, Config{Workspace: "/ws", Worktree: "/wt"}},
		"no workspace":    {Deps{hostOS: "darwin", ReadStage: "shadow"}, Config{Worktree: "/wt", DispatchID: "d1"}},
		"no sandbox":      {Deps{hostOS: "darwin", ReadStage: "shadow"}, Config{Workspace: "/ws", DispatchID: "d1"}},
		"sandbox opt-out": {Deps{hostOS: "darwin", ReadStage: "shadow", Env: map[string]string{envSandboxMode: "off"}}, *cfg},
	} {
		called := false
		c.deps.SandboxReports = func(context.Context, string, time.Time) ([]byte, error) { called = true; return nil, nil }
		c.deps.reportReadViolations(context.Background(), &c.cfg, time.Now())
		if called {
			t.Errorf("%s: the reports must not be read", name)
		}
	}
}

func TestReportReadViolations_WarnsWhenTheReportsCannotBeRead(t *testing.T) {
	t.Parallel()
	var stderr bytes.Buffer
	deps := Deps{hostOS: "darwin", ReadStage: "enforce", Stderr: &stderr,
		SandboxReports: func(context.Context, string, time.Time) ([]byte, error) { return nil, errors.New("log: denied") }}
	deps.reportReadViolations(context.Background(), &Config{Workspace: "/ws", RequireSandbox: true, DispatchID: "d1"}, time.Now())
	if !strings.Contains(stderr.String(), "WARN isolation read reports: log: denied") {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

func TestParseReadReports_CapsTheViolationsOfOneCell(t *testing.T) {
	t.Parallel()
	var b strings.Builder
	for i := range readReportCap + 5 {
		b.WriteString(`{"eventMessage":"Sandbox: cat(1) allow file-read-data /x/` + strings.Repeat("a", i+1) + `\nevolve-cell=d rule=read.home.secret inv=I5"}` + "\n")
	}
	if got := parseReadReports([]byte(b.String())); len(got) != readReportCap {
		t.Fatalf("parsed %d reports, want the cap %d", len(got), readReportCap)
	}
}

func TestLogShowReports_RunsLogShowForTheTagOfTheCell(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	stub := filepath.Join(dir, "log")
	writeFixtureFile(t, stub, "#!/bin/sh\nprintf '%s\\n' \"$@\"\n")
	if err := os.Chmod(stub, 0o755); err != nil {
		t.Fatal(err)
	}
	out, err := logShowReports(stub)(context.Background(), "d42", time.Date(2026, 10, 26, 3, 4, 5, 0, time.Local))
	if err != nil {
		t.Fatal(err)
	}
	want := "show\n--style\nndjson\n--start\n2026-10-26 03:04:05\n--predicate\n" + `sender == "Sandbox" AND eventMessage CONTAINS "evolve-cell=d42 "` + "\n"
	if string(out) != want {
		t.Fatalf("log argv =\n%s\nwant\n%s", out, want)
	}
	if _, err := logShowReports(filepath.Join(dir, "absent"))(context.Background(), "d42", time.Now()); err == nil {
		t.Fatal("a missing log binary must fail")
	}
}

func TestReportReadViolations_ADepsWithoutAReaderEmitsNothingAndTheDefaultsWireLogShow(t *testing.T) {
	t.Parallel()
	center := signalcenter.New()
	Deps{Signals: center, hostOS: "darwin", ReadStage: "shadow"}.reportReadViolations(context.Background(), &Config{Workspace: "/ws", Worktree: "/wt", DispatchID: "d1"}, time.Now())
	if got := readViolations(center); len(got) != 0 {
		t.Fatalf("no reader, no violations: %+v", got)
	}
	if NewEngine(Deps{TokenResolver: func(tokenusage.Window) (tokenusage.Result, error) { return tokenusage.Result{}, nil }}).deps.SandboxReports == nil {
		t.Fatal("the engine defaults must read the sandbox reports through log show")
	}
}

func TestRelToProject_NamesATargetUnderTheRootRelativelyAndKeepsAnyOtherPath(t *testing.T) {
	t.Parallel()
	for _, c := range []struct{ root, target, want string }{
		{"/repo", "/repo/.evolve/runs/cycle-9/audit-report.md", ".evolve/runs/cycle-9/audit-report.md"},
		{"", "/repo/a", "/repo/a"},
		{"/repo", "/home/u/.ssh/id", "/home/u/.ssh/id"},
		{"/repo", "relative", "relative"},
	} {
		if got := relToProject(c.root, c.target); got != c.want {
			t.Errorf("relToProject(%q, %q) = %q, want %q", c.root, c.target, got, c.want)
		}
	}
}

func TestReadIsolation_GrantsOnlyIndexedPathsThatResolveInsideTheRunsAndAreNotRawOutput(t *testing.T) {
	t.Parallel()
	c := newReadCell(t)
	link := filepath.Join(c.runs, "cycle-6", "audit-report.md")
	if err := os.Symlink(filepath.Join(c.home, ".netrc"), link); err != nil {
		t.Fatal(err)
	}
	idx := inputindex.Index{Schema: inputindex.Schema, Entries: []inputindex.Entry{
		{Path: ".evolve/runs/cycle-6/audit-report.md"}, {Path: ".evolve/runs/cycle-6/build-stdout.log"},
		{Path: ".evolve/instincts/lessons/a.yaml"}, {Path: ".evolve/runs/cycle-6/retrospective-report.md"},
	}}
	if err := inputindex.Write(c.ws, idx); err != nil {
		t.Fatal(err)
	}
	got := Deps{ReadStage: "enforce"}.readIsolation(c.request(), c.home).Grants
	want := []string{inputindex.Path(c.ws), filepath.Join(c.runs, "cycle-6", "retrospective-report.md")}
	if !slices.Equal(got, want) {
		t.Fatalf("grants = %v, want %v: a symlink out of the runs, raw output and a path outside the runs are never granted", got, want)
	}
}

func TestReportReadViolations_ReadsTheReportsAfterACancelledLaunchAndRefusesAnUnsafeTag(t *testing.T) {
	t.Parallel()
	var ctxErr error
	calls := 0
	deps := Deps{hostOS: "darwin", ReadStage: "shadow", SandboxReports: func(ctx context.Context, _ string, _ time.Time) ([]byte, error) {
		calls++
		ctxErr = ctx.Err()
		return nil, nil
	}}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	deps.reportReadViolations(ctx, &Config{Workspace: "/ws", Worktree: "/wt", DispatchID: "d1"}, time.Now())
	deps.reportReadViolations(context.Background(), &Config{Workspace: "/ws", Worktree: "/wt", DispatchID: `d" OR 1`}, time.Now())
	if calls != 1 || ctxErr != nil {
		t.Fatalf("calls = %d ctx err = %v; a cancelled launch must still read its reports, and an unsafe tag must not reach the predicate", calls, ctxErr)
	}
}

func TestCanonicalOrSelf_KeepsAPathThatCannotBeResolved(t *testing.T) {
	t.Parallel()
	file := filepath.Join(realDir(t), "file")
	writeFixtureFile(t, file, "x")
	if got := canonicalOrSelf(file + "/below"); got != file+"/below" {
		t.Fatalf("canonicalOrSelf = %q, want the path itself", got)
	}
}
