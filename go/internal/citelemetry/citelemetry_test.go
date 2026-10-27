package citelemetry

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const jobsJSON = `{"jobs":[
{"name":"macOS job","started_at":"2026-10-20T10:10:00Z","completed_at":"2026-10-20T10:32:30Z","steps":[
 {"name":"build","started_at":"2026-10-20T10:10:00Z","completed_at":"2026-10-20T10:11:00Z"},
 {"name":"test darwin","started_at":"2026-10-20T10:11:00Z","completed_at":"2026-10-20T10:30:00Z"},
 {"name":"skipped","started_at":null,"completed_at":null}]},
{"name":"ubuntu 1.23","started_at":"2026-10-20T10:00:00Z","completed_at":"2026-10-20T10:13:00Z","steps":[
 {"name":"test race","started_at":"2026-10-20T10:01:00Z","completed_at":"2026-10-20T10:11:00Z"},
 {"name":"gates","started_at":"2026-10-20T10:11:00Z","completed_at":"2026-10-20T10:12:30Z"}]},
{"name":"ubuntu 1.27","started_at":"2026-10-20T10:00:00Z","completed_at":"2026-10-20T10:10:00Z","steps":[
 {"name":"test race","started_at":"2026-10-20T10:01:00Z","completed_at":"2026-10-20T10:09:00Z"}]}]}`

const (
	mod    = "example.com/m/go/"
	stream = `{"Time":"2026-10-20T10:11:10Z","Action":"start","Package":"example.com/m/go/cmd/x"}
{"Time":"2026-10-20T10:11:20Z","Action":"run","Package":"example.com/m/go/cmd/x","Test":"TestSlow"}
{"Time":"2026-10-20T10:11:20Z","Action":"pass","Package":"example.com/m/go/cmd/x","Test":"TestSlow","Elapsed":30}
{"Time":"2026-10-20T10:11:21Z","Action":"pass","Package":"example.com/m/go/cmd/x","Test":"TestSlow/sub","Elapsed":99}
{"Time":"2026-10-20T10:11:22Z","Action":"pause","Package":"example.com/m/go/cmd/x","Test":"TestPar"}
{"Time":"2026-10-20T10:11:23Z","Action":"pass","Package":"example.com/m/go/cmd/x","Test":"TestPar","Elapsed":10}
{"Time":"2026-10-20T10:12:20Z","Action":"pass","Package":"example.com/m/go/cmd/x","Elapsed":70}
{"Time":"2026-10-20T10:12:30Z","Action":"start","Package":"example.com/m/go/internal/y"}
{"Time":"2026-10-20T10:12:40Z","Action":"fail","Package":"example.com/m/go/internal/y","Elapsed":5}
{"Time":"2026-10-20T10:13:00Z","Action":"start","Package":"example.com/m/go/internal/hung"}
`
	rusage = `{"package":"cmd/x","wall_s":70,"user_s":40,"system_s":10,"peak_rss_mb":300,"exit_code":0}` + "\n\n" +
		`{"package":"internal/ghost","wall_s":9,"user_s":9,"system_s":9,"exit_code":0}` + "\n"
	samples = `{"ts":"2026-10-20T10:11:10Z","cpu_busy_pct":60,"swap_used_mb":0}
{"ts":"2026-10-20T10:12:20Z","cpu_busy_pct":80,"swap_used_mb":512}
{"ts":"2026-10-20T10:12:21Z","cpu_busy_pct":99,"swap_used_mb":9999}
`
)

func writeDir(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for name, body := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func fixtureReport(t *testing.T) Report {
	t.Helper()
	jobs := writeDir(t, map[string]string{"jobs.json": jobsJSON})
	art := writeDir(t, map[string]string{testJSONFile: stream, rusageFile: rusage, samplesFile: samples})
	rep, err := Run(filepath.Join(jobs, "jobs.json"), []string{"macOS job=" + art})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	return rep
}

func TestRun_FixtureGivesTheL5ToL1PathAndTheJoin(t *testing.T) {
	rep := fixtureReport(t)
	want := []PathStep{
		{"L5", "macOS job", 1350}, {"L4", "test darwin", 1140}, {"L3", mod + "cmd/x", 70}, {"L2", "serial", 30}, {"L1", "TestSlow", 30},
	}
	if len(rep.CriticalPath) != len(want) {
		t.Fatalf("path = %+v, want %+v", rep.CriticalPath, want)
	}
	for i, w := range want {
		if rep.CriticalPath[i] != w {
			t.Errorf("path[%d] = %+v, want %+v", i, rep.CriticalPath[i], w)
		}
	}
	if rep.G1.MacosJobS != 1350 || rep.G1.UbuntuTestStepS != 600 {
		t.Errorf("g1 = %+v, want 1350 and 600", rep.G1)
	}
	if len(rep.Incomplete) != 1 || rep.Incomplete[0] != mod+"internal/hung" {
		t.Errorf("incomplete = %v", rep.Incomplete)
	}
	if len(rep.Packages) != 2 || rep.Packages[0].Package != mod+"cmd/x" || rep.Packages[1].Result != "fail" {
		t.Fatalf("packages = %+v", rep.Packages)
	}
	x := rep.Packages[0]
	if x.Rusage == nil || x.Rusage.SystemS != 10 || x.Sampler == nil || x.Sampler.Samples != 2 || x.Sampler.CPUBusyMaxPct != 80 || x.Sampler.SwapUsedMaxMB != 512 || x.Limit != "cpu" {
		t.Errorf("cmd/x row = %+v rusage=%+v sampler=%+v", x, x.Rusage, x.Sampler)
	}
	if y := rep.Packages[1]; y.Rusage != nil || y.Sampler != nil || y.Limit != "unknown" {
		t.Errorf("internal/y row = %+v, want no join and unknown", y)
	}
}

func TestMarkdown_ListsPathG1PackagesAndIncomplete(t *testing.T) {
	md := fixtureReport(t).Markdown()
	cursor := 0
	for _, marker := range []string{"L5", "macOS job", "L4", "test darwin", "L3", "cmd/x", "L2", "serial", "L1", "TestSlow", "G1", "1350", "600", "## Packages", "fail", "## Incomplete", "internal/hung"} {
		idx := strings.Index(md[cursor:], marker)
		if idx < 0 {
			t.Fatalf("markdown lacks %q after offset %d:\n%s", marker, cursor, md)
		}
		cursor += idx + len(marker)
	}
	bare := Report{}.Markdown()
	if strings.Contains(bare, "## Packages") || strings.Contains(bare, "## Incomplete") {
		t.Errorf("an empty report prints empty sections:\n%s", bare)
	}
}

func TestBuild_PathStopsWhereTheEvidenceStops(t *testing.T) {
	steps := []jobStep{{"test", 5}}
	cases := []struct {
		name   string
		in     input
		levels int
	}{
		{"job without steps", input{jobs: []job{{name: "a", seconds: 3}}}, 1},
		{"job without artifacts", input{jobs: []job{{name: "a", seconds: 3, steps: steps}}}, 2},
		{"stream without a finished package", input{jobs: []job{{name: "a", seconds: 3, steps: steps}},
			artifacts: map[string]artifacts{"a": {testJSON: []byte(`{"Time":"2026-10-20T10:00:00Z","Action":"start","Package":"p"}`)}}}, 2},
		{"package without tests", input{jobs: []job{{name: "a", seconds: 3, steps: steps}},
			artifacts: map[string]artifacts{"a": {testJSON: []byte(`{"Time":"2026-10-20T10:00:00Z","Action":"pass","Package":"p","Elapsed":4}`)}}}, 3},
	}
	for _, c := range cases {
		rep, err := build(c.in)
		if err != nil || len(rep.CriticalPath) != c.levels {
			t.Errorf("%s: path = %+v err = %v, want %d levels", c.name, rep.CriticalPath, err, c.levels)
		}
	}
}

func TestBuild_ParallelPartDominatesWhenItIsLongerThanTheSerialSum(t *testing.T) {
	events := `{"Time":"2026-10-20T10:00:00Z","Action":"pause","Package":"p","Test":"TestP"}
{"Time":"2026-10-20T10:00:01Z","Action":"pass","Package":"p","Test":"TestP","Elapsed":9}
{"Time":"2026-10-20T10:00:02Z","Action":"skip","Package":"p","Test":"TestS","Elapsed":2}
{"Time":"2026-10-20T10:00:03Z","Action":"pass","Package":"p","Elapsed":9}`
	rep, err := build(input{jobs: []job{{name: "a", seconds: 3, steps: []jobStep{{"test", 5}}}},
		artifacts: map[string]artifacts{"a": {testJSON: []byte(events)}}})
	if err != nil || len(rep.CriticalPath) != 5 {
		t.Fatalf("path = %+v err = %v", rep.CriticalPath, err)
	}
	if mode, test := rep.CriticalPath[3], rep.CriticalPath[4]; mode.Unit != "parallel" || mode.Seconds != 9 || test.Unit != "TestP" {
		t.Errorf("mode = %+v test = %+v, want parallel 9 and TestP", mode, test)
	}
}

func TestBuild_RejectsAnInputThatHasNoJobsOrNamesAnUnknownJob(t *testing.T) {
	if _, err := build(input{}); !errors.Is(err, errNoJobs) {
		t.Errorf("no jobs: err = %v, want errNoJobs", err)
	}
	_, err := build(input{jobs: []job{{name: "a"}}, artifacts: map[string]artifacts{"b": {}}})
	if !errors.Is(err, errUnknownJob) || !strings.Contains(err.Error(), `"b"`) {
		t.Errorf("unknown job: err = %v, want errUnknownJob naming b", err)
	}
}

func TestG1_TakesTheLongestMatchingJobAndTestStep(t *testing.T) {
	g := g1Of([]job{
		{name: "a macOS", seconds: 5}, {name: "b macOS", seconds: 9},
		{name: "u ubuntu", steps: []jobStep{{"test one", 4}, {"test two", 7}, {"lint", 99}}},
		{name: "other", seconds: 100, steps: []jobStep{{"test", 100}}},
	})
	if g.MacosJobS != 9 || g.UbuntuTestStepS != 7 {
		t.Errorf("g1 = %+v, want 9 and 7", g)
	}
}

func TestLimitOf_FirstRuleThatHoldsWins(t *testing.T) {
	cases := []struct {
		name string
		r    *Rusage
		s    *Sampler
		want string
	}{
		{"no rusage", nil, nil, "unknown"},
		{"swap above limit", &Rusage{WallS: 10, UserS: 9}, &Sampler{SwapUsedMaxMB: 2048, cpuBusyMin: 99}, "memory"},
		{"saturated window", &Rusage{WallS: 10, UserS: 9}, &Sampler{cpuBusyMin: 91}, "cpu-saturation"},
		{"window at 60 is not saturated", &Rusage{WallS: 10, UserS: 9}, &Sampler{cpuBusyMin: 60}, "cpu"},
		{"system above user", &Rusage{WallS: 40, UserS: 3, SystemS: 9}, nil, "subprocesses-and-syscalls"},
		{"tiny cpu skips the system rule", &Rusage{WallS: 1, UserS: 0.001, SystemS: 0.01}, nil, "wait"},
		{"low efficiency", &Rusage{WallS: 10, UserS: 1, SystemS: 0.5}, nil, "wait"},
		{"busy binary", &Rusage{WallS: 10, UserS: 8}, nil, "cpu"},
	}
	for _, c := range cases {
		if got := limitOf(c.r, c.s); got != c.want {
			t.Errorf("%s: limit = %q, want %q", c.name, got, c.want)
		}
	}
}

func TestJoinRusage_UsesTheModulePrefixWithTheMostVotesAndTheExactPath(t *testing.T) {
	runs := []*packageRun{{name: "a/go/cmd/x"}, {name: "a/go/internal/cmd/x"}, {name: "a/go/internal/xcmd/x"}, {name: "b/go/cmd/x"}}
	joined := joinRusage([]rusageRecord{{Package: "cmd/x"}, {Package: "internal/cmd/x"}}, runs)
	if len(joined) != 2 || joined["a/go/cmd/x"] == nil || joined["a/go/internal/cmd/x"] == nil {
		t.Errorf("joined = %v, want the a/go prefix and nothing else", joined)
	}
	if got := modulePrefix([]rusageRecord{{Package: "cmd/x"}}, runs); got != "a/go" {
		t.Errorf("prefix = %q, want the lexically first of the tied prefixes a/go", got)
	}
	if got := modulePrefix([]rusageRecord{{Package: "cmd/none"}}, runs); got != "" {
		t.Errorf("prefix = %q, want empty without a vote", got)
	}
}

func TestRun_FailsLoudlyOnBadInput(t *testing.T) {
	good := writeDir(t, map[string]string{"jobs.json": jobsJSON})
	jobsFile := filepath.Join(good, "jobs.json")
	art := writeDir(t, map[string]string{testJSONFile: stream})
	brokenStream := writeDir(t, map[string]string{testJSONFile: "{bad"})
	brokenRusage := writeDir(t, map[string]string{testJSONFile: stream, rusageFile: "{bad"})
	brokenSamples := writeDir(t, map[string]string{testJSONFile: stream, samplesFile: "{bad"})
	unreadable := writeDir(t, map[string]string{testJSONFile: stream})
	if err := os.Mkdir(filepath.Join(unreadable, rusageFile), 0o755); err != nil {
		t.Fatal(err)
	}
	unreadableSamples := writeDir(t, map[string]string{testJSONFile: stream, rusageFile: ""})
	if err := os.Mkdir(filepath.Join(unreadableSamples, samplesFile), 0o755); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name     string
		jobsFile string
		bindings []string
	}{
		{"missing jobs file", filepath.Join(good, "absent.json"), nil},
		{"malformed jobs file", writeDir(t, map[string]string{"j": "{bad"}) + "/j", nil},
		{"bad job time", writeDir(t, map[string]string{"j": `{"jobs":[{"name":"a","started_at":"x","completed_at":"2026-10-20T10:00:00Z"}]}`}) + "/j", nil},
		{"bad job end time", writeDir(t, map[string]string{"j": `{"jobs":[{"name":"a","started_at":"2026-10-20T10:00:00Z","completed_at":"x"}]}`}) + "/j", nil},
		{"bad step time", writeDir(t, map[string]string{"j": `{"jobs":[{"name":"a","steps":[{"name":"s","started_at":"x","completed_at":"y"}]}]}`}) + "/j", nil},
		{"binding without equals", jobsFile, []string{"mac"}},
		{"artifact directory missing", jobsFile, []string{"macOS job=" + filepath.Join(good, "absent")}},
		{"unknown job name", jobsFile, []string{"nope=" + art}},
		{"malformed test stream", jobsFile, []string{"macOS job=" + brokenStream}},
		{"malformed rusage", jobsFile, []string{"macOS job=" + brokenRusage}},
		{"malformed samples", jobsFile, []string{"macOS job=" + brokenSamples}},
		{"unreadable rusage", jobsFile, []string{"macOS job=" + unreadable}},
		{"unreadable samples", jobsFile, []string{"macOS job=" + unreadableSamples}},
	}
	for _, c := range cases {
		if rep, err := Run(c.jobsFile, c.bindings); err == nil {
			t.Errorf("%s: no error, report %+v", c.name, rep)
		}
	}
}
