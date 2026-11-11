package sandbox

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func stubBinary(t *testing.T, script string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "stub")
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestParseReports_ReadsTheOperationTheTargetAndTheProcess(t *testing.T) {
	t.Parallel()
	raw := `{"eventMessage":"Sandbox: git(41) deny(1) file-read-data \/lane\/.git\/HEAD\nevolve-cell=gaterun-1 rule=gate.git inv=I0"}` + "\n" +
		`not json` + "\n" +
		`{"eventMessage":"unrelated"}` + "\n" +
		`{"eventMessage":"Sandbox: git(41) deny(1) file-read-data \/lane\/.git\/HEAD\nevolve-cell=gaterun-1 rule=gate.git inv=I0"}` + "\n" +
		`{"eventMessage":"Sandbox: allow process-exec* \/usr\/bin\/git\nevolve-cell=gaterun-1 rule=gate.exec inv=I0"}` + "\n"
	got := ParseReports([]byte(raw))
	if len(got) != 2 {
		t.Fatalf("ParseReports = %+v, want two reports", got)
	}
	want := Report{Denied: true, Operation: "file-read-data", Target: "/lane/.git/HEAD", Rule: "gate.git", Invariant: "I0", Process: "git", PID: "41"}
	if got[0] != want {
		t.Errorf("first report = %+v, want %+v", got[0], want)
	}
	if got[1].Denied || got[1].Operation != "process-exec*" || got[1].Process != "" {
		t.Errorf("second report = %+v, want an allowed process-exec* with no process", got[1])
	}
}

func TestParseReports_CapsTheReportsOfOneCell(t *testing.T) {
	t.Parallel()
	var b strings.Builder
	for i := range ReportCap + 5 {
		b.WriteString(`{"eventMessage":"Sandbox: cat(1) allow file-read-data /x/` + strings.Repeat("a", i+1) + `\nevolve-cell=d rule=read.home.secret inv=I5"}` + "\n")
	}
	if got := ParseReports([]byte(b.String())); len(got) != ReportCap {
		t.Fatalf("parsed %d reports, want the cap %d", len(got), ReportCap)
	}
}

func TestParseReports_NoiseOfOneRuleCannotHideTheReportOfAnother(t *testing.T) {
	t.Parallel()
	var b strings.Builder
	for i := range ReportCap + 5 {
		b.WriteString(`{"eventMessage":"Sandbox: x(1) allow file-read-data /etc/` + strings.Repeat("a", i+1) + `\nevolve-cell=g rule=gate.default inv=I0"}` + "\n")
	}
	b.WriteString(`{"eventMessage":"Sandbox: x(1) allow file-read-data /lane/.git/HEAD\nevolve-cell=g rule=gate.git inv=I0"}` + "\n")
	got := ParseReports([]byte(b.String()))
	if len(got) != ReportCap+1 || got[ReportCap].Rule != "gate.git" {
		t.Errorf("parsed %d reports, last %+v: the cap is per rule, so the gate.git report after %d gate.default reports stays", len(got), got[len(got)-1], ReportCap+5)
	}
}

func TestReports_RunsLogShowForTheTagOfTheCell(t *testing.T) {
	t.Parallel()
	stub := stubBinary(t, "#!/bin/sh\nprintf '%s\\n' \"$@\"\n")
	out, err := Reports(context.Background(), stub, "d42", time.Date(2026, 10, 26, 3, 4, 5, 0, time.Local))
	if err != nil {
		t.Fatal(err)
	}
	want := "show\n--style\nndjson\n--start\n2026-10-26 03:04:05\n--predicate\n" + `sender == "Sandbox" AND eventMessage CONTAINS "evolve-cell=d42 "` + "\n"
	if string(out) != want {
		t.Fatalf("log argv =\n%s\nwant\n%s", out, want)
	}
	if _, err := Reports(context.Background(), filepath.Join(t.TempDir(), "absent"), "d42", time.Now()); err == nil {
		t.Fatal("a missing log binary must fail")
	}
}

func TestWriteFence_RunsTheBinaryWithAProfileThatNamesTheTag(t *testing.T) {
	t.Parallel()
	stub := stubBinary(t, "#!/bin/sh\nprintf '%s' \"$2\" > \"$0.out\"\n")
	if err := WriteFence(context.Background(), stub, "run1/7"); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(stub + ".out")
	if !strings.Contains(string(got), FenceMessage("run1/7")) {
		t.Errorf("profile = %q, want the fence message of the tag", got)
	}
	if err := WriteFence(context.Background(), filepath.Join(t.TempDir(), "absent"), "d1"); err == nil {
		t.Error("a missing binary must fail")
	}
}

func TestReportTools_AreTheAbsoluteMacOSBinariesAndTheFenceNamesItsRule(t *testing.T) {
	t.Parallel()
	if LogShowPath != "/usr/bin/log" || SandboxExecPath != "/usr/bin/sandbox-exec" {
		t.Errorf("LogShowPath = %q, SandboxExecPath = %q: a PATH lookup lets a planted binary forge the reports", LogShowPath, SandboxExecPath)
	}
	if !strings.HasSuffix(FenceMessage("d1"), " rule="+RuleCellFence) {
		t.Errorf("FenceMessage = %q, want the rule %s", FenceMessage("d1"), RuleCellFence)
	}
}

func TestFenced_FindsOnlyTheFenceOfItsOwnTag(t *testing.T) {
	t.Parallel()
	raw := []byte(`{"eventMessage":"Sandbox: x\n` + FenceMessage("other") + `"}` + "\nnot json\n")
	if Fenced(raw, "d1") || !Fenced(raw, "other") {
		t.Errorf("Fenced must match the tag of its own fence only")
	}
}

type clock struct{ now time.Time }

func (c *clock) Now() time.Time        { return c.now }
func (c *clock) Sleep(d time.Duration) { c.now = c.now.Add(d) }

func TestReportSource_CellWithoutAFenceReadsOnce(t *testing.T) {
	t.Parallel()
	s := ReportSource{Reports: func(context.Context, string, time.Time) ([]byte, error) { return []byte("r"), nil }}
	raw, complete, err := s.Cell(context.Background(), "d", time.Now())
	if string(raw) != "r" || !complete || err != nil {
		t.Errorf("Cell = %q, %v, %v", raw, complete, err)
	}
}

func TestReportSource_CellWhenTheFenceFailsWarnsAndIsIncomplete(t *testing.T) {
	t.Parallel()
	var warned string
	s := ReportSource{
		Reports: func(context.Context, string, time.Time) ([]byte, error) { return []byte("r"), nil },
		Fence:   func(context.Context, string) error { return errors.New("no fence") },
		Warn:    func(f string, a ...any) { warned = f },
	}
	_, complete, err := s.Cell(context.Background(), "d", time.Now())
	if complete || err != nil || !strings.Contains(warned, "fence") {
		t.Errorf("Cell = %v, %v, warned %q", complete, err, warned)
	}
}

func TestReportSource_CellPollsUntilTheFenceShowsAndStopsAtTheDeadline(t *testing.T) {
	t.Parallel()
	c := &clock{now: time.Unix(0, 0)}
	calls := 0
	s := ReportSource{
		Reports: func(context.Context, string, time.Time) ([]byte, error) {
			calls++
			if calls == 3 {
				return []byte(`{"eventMessage":"` + FenceMessage("d") + `"}`), nil
			}
			return nil, nil
		},
		Fence: func(context.Context, string) error { return nil }, Now: c.Now, Sleep: c.Sleep,
	}
	if _, complete, err := s.Cell(context.Background(), "d", time.Now()); !complete || err != nil || calls != 3 {
		t.Errorf("fenced run: complete %v err %v calls %d", complete, err, calls)
	}
	var warned bool
	s = ReportSource{
		Reports: func(context.Context, string, time.Time) ([]byte, error) { return nil, nil },
		Fence:   func(context.Context, string) error { return nil }, Now: c.Now, Sleep: c.Sleep, Deadline: 3 * time.Second,
		Warn: func(string, ...any) { warned = true },
	}
	if _, complete, err := s.Cell(context.Background(), "d", time.Now()); complete || err != nil || !warned {
		t.Errorf("deadline run: complete %v err %v warned %v", complete, err, warned)
	}
	s.Warn = nil
	if _, complete, _ := s.Cell(context.Background(), "d", time.Now()); complete {
		t.Error("a missing warner must not change the result")
	}
}

func TestReportSource_CellReturnsAReaderError(t *testing.T) {
	t.Parallel()
	s := ReportSource{
		Reports: func(context.Context, string, time.Time) ([]byte, error) { return nil, errors.New("log: denied") },
		Fence:   func(context.Context, string) error { return nil }, Now: time.Now, Sleep: func(time.Duration) {},
	}
	if _, complete, err := s.Cell(context.Background(), "d", time.Now()); err == nil || !complete {
		t.Errorf("Cell = %v, %v, want the reader error", complete, err)
	}
}
