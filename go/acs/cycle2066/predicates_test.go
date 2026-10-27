//go:build acs

package cycle2066

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	modulePrefix         = "github.com/mickeyyaya/evolve-loop/go/"
	macJobName           = "build + test (Go 1.23, macOS, the darwin package set)"
	macTestStep          = "test the darwin set (go/.darwin-set; race + cover, incl. integration tier)"
	ubuntuJobName        = "build + test (Go 1.23, ubuntu, every package and every gate)"
	ubuntuTestStep       = "test (race + cover, incl. integration tier)"
	ubuntu127Job         = "build + test (Go 1.27, ubuntu, every package, no gates)"
	vetJobName           = "vet + gofmt (Go 1.23)"
	exitUsageCode        = 10
	telemetryErrorPrefix = "evolve ci telemetry: "
	commandTimeout       = 90 * time.Second
	buildTimeout         = 10 * time.Minute
	citelemetryPkg       = "./internal/citelemetry"
	coverageRequired     = 100.0
)

var (
	buildOnce sync.Once
	builtPath string
	buildErr  error
	buildDir  string
)

func TestMain(m *testing.M) {
	code := m.Run()
	if buildDir != "" {
		_ = os.RemoveAll(buildDir)
	}
	os.Exit(code)
}

func goDir(t *testing.T) string {
	t.Helper()
	return filepath.Join(acsassert.RepoRoot(t), "go")
}

func evolveBinary(t *testing.T) string {
	t.Helper()
	dir := goDir(t)
	buildOnce.Do(func() {
		buildDir, buildErr = os.MkdirTemp("", "cycle2066-evolve-")
		if buildErr != nil {
			return
		}
		builtPath = filepath.Join(buildDir, "evolve")
		ctx, cancel := context.WithTimeout(context.Background(), buildTimeout)
		defer cancel()
		cmd := exec.CommandContext(ctx, "go", "build", "-o", builtPath, "./cmd/evolve")
		cmd.Dir = dir
		cmd.WaitDelay = 5 * time.Second
		var out bytes.Buffer
		cmd.Stdout, cmd.Stderr = &out, &out
		if err := cmd.Run(); err != nil {
			buildErr = fmt.Errorf("go build ./cmd/evolve: %w\n%s", err, acsassert.OutputTail(out.String(), 30))
		}
	})
	if buildErr != nil {
		t.Fatalf("cannot build the production binary: %v", buildErr)
	}
	return builtPath
}

type result struct {
	stdout string
	stderr string
	code   int
}

func runEvolve(t *testing.T, args ...string) result {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), commandTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, evolveBinary(t), args...)
	cmd.Dir = t.TempDir()
	cmd.WaitDelay = 5 * time.Second
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	code := 0
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		code = exitErr.ExitCode()
	} else if err != nil {
		t.Fatalf("run evolve %v: %v", args, err)
	}
	return result{stdout: stdout.String(), stderr: stderr.String(), code: code}
}

func at(clock string) string {
	return "2026-10-20T" + clock + "Z"
}

type step struct {
	name, from, to string
}

type job struct {
	name, created, started, completed string
	steps                             []step
}

func jobsDocument(jobs []job) []byte {
	var rows []map[string]any
	for _, j := range jobs {
		var steps []map[string]any
		for i, s := range j.steps {
			steps = append(steps, map[string]any{
				"name": s.name, "number": i + 1, "status": "completed", "conclusion": "success",
				"started_at": at(s.from), "completed_at": at(s.to),
			})
		}
		rows = append(rows, map[string]any{
			"name": j.name, "status": "completed", "conclusion": "success",
			"created_at": at(j.created), "started_at": at(j.started), "completed_at": at(j.completed),
			"steps": steps,
		})
	}
	raw, _ := json.Marshal(map[string]any{"total_count": len(rows), "jobs": rows})
	return raw
}

func fixtureJobs() []job {
	return []job{
		{macJobName, "10:00:00", "10:10:00", "10:32:30", []step{
			{"Set up job", "10:10:00", "10:10:05"},
			{"build", "10:10:05", "10:11:00"},
			{macTestStep, "10:11:00", "10:30:00"},
			{"e2e tier (no race; live sub-tier self-skips without live flag)", "10:30:00", "10:32:00"},
		}},
		{ubuntuJobName, "10:00:00", "10:00:20", "10:13:00", []step{
			{"Set up job", "10:00:20", "10:00:25"},
			{"build", "10:00:25", "10:01:00"},
			{ubuntuTestStep, "10:01:00", "10:11:00"},
			{"changed functions at 100% (cover-changed; hard-fail)", "10:11:00", "10:12:30"},
		}},
		{ubuntu127Job, "10:00:00", "10:00:15", "10:10:15", []step{
			{"build", "10:00:15", "10:01:00"},
			{ubuntuTestStep, "10:01:00", "10:09:00"},
		}},
		{vetJobName, "10:00:00", "10:00:10", "10:01:10", []step{
			{"vet (linux)", "10:00:10", "10:01:10"},
		}},
	}
}

type streamLines struct {
	lines []string
}

func (s *streamLines) event(clock, action, pkg, test string, elapsed float64) {
	e := map[string]any{"Time": at(clock), "Action": action, "Package": pkg}
	if test != "" {
		e["Test"] = test
	}
	if action == "pass" || action == "fail" {
		e["Elapsed"] = elapsed
	}
	raw, _ := json.Marshal(e)
	s.lines = append(s.lines, string(raw))
}

func (s *streamLines) serial(clock, pkg, test, action string, elapsed float64) {
	s.event(clock, "run", pkg, test, 0)
	s.event(clock, action, pkg, test, elapsed)
}

func (s *streamLines) parallel(clock, pkg, test string, elapsed float64) {
	s.event(clock, "run", pkg, test, 0)
	s.event(clock, "pause", pkg, test, 0)
	s.event(clock, "cont", pkg, test, 0)
	s.event(clock, "pass", pkg, test, elapsed)
}

func (s *streamLines) text() string {
	return strings.Join(s.lines, "\n") + "\n"
}

func macStream() string {
	var s streamLines
	evolve := modulePrefix + "cmd/evolve"
	s.event("10:11:10", "start", evolve, "", 0)
	s.serial("10:11:20", evolve, "TestSerialSlow", "pass", 30)
	s.serial("10:11:50", evolve, "TestSerialFast", "pass", 20)
	s.parallel("10:12:10", evolve, "TestParallelA", 10)
	s.parallel("10:12:15", evolve, "TestParallelB", 6)
	s.event("10:12:20", "pass", evolve, "", 70)

	core := modulePrefix + "internal/core"
	s.event("10:12:30", "start", core, "", 0)
	s.serial("10:12:40", core, "TestRedOne", "fail", 5)
	s.serial("10:12:50", core, "TestOk", "pass", 3)
	s.event("10:13:10", "fail", core, "", 40)

	bridge := modulePrefix + "internal/bridge"
	s.event("10:13:20", "start", bridge, "", 0)
	s.serial("10:13:30", bridge, "TestBridge", "pass", 4)
	s.event("10:13:50", "pass", bridge, "", 30)

	audit := modulePrefix + "internal/phases/audit"
	s.event("10:14:00", "start", audit, "", 0)
	s.serial("10:14:10", audit, "TestAudit", "pass", 4)
	s.event("10:14:20", "pass", audit, "", 20)

	idle := modulePrefix + "internal/idle"
	s.event("10:14:30", "start", idle, "", 0)
	s.serial("10:14:35", idle, "TestIdle", "pass", 2)
	s.event("10:14:40", "pass", idle, "", 10)

	lookalike := modulePrefix + "internal/xcmd/evolve"
	s.event("10:14:50", "start", lookalike, "", 0)
	s.serial("10:14:52", lookalike, "TestLookalike", "pass", 1)
	s.event("10:14:55", "pass", lookalike, "", 5)

	s.event("10:15:00", "start", modulePrefix+"internal/hung", "", 0)
	return s.text()
}

func rusageLine(pkg string, wall, user, system, rss float64, exit int) string {
	raw, _ := json.Marshal(map[string]any{
		"package": pkg, "wall_s": wall, "user_s": user, "system_s": system,
		"peak_rss_mb": rss, "block_in": 0, "block_out": 0,
		"ctx_voluntary": 10, "ctx_involuntary": 5, "exit_code": exit,
	})
	return string(raw)
}

func macRusage() string {
	return strings.Join([]string{
		rusageLine("cmd/evolve", 70, 40, 10, 300, 0),
		rusageLine("internal/core", 40, 3, 9, 512, 1),
		rusageLine("internal/bridge", 30, 5, 1, 256, 0),
		rusageLine("internal/phases/audit", 20, 10, 4, 400, 0),
		rusageLine("internal/idle", 10, 1, 0.5, 64, 0),
		rusageLine("internal/ghost", 99, 99, 99, 9999, 0),
	}, "\n") + "\n"
}

func sampleLine(clock string, cpu, swap float64) string {
	raw, _ := json.Marshal(map[string]any{
		"ts": at(clock), "cpu_busy_pct": cpu, "mem_used_mb": 3000.0, "swap_used_mb": swap,
	})
	return string(raw)
}

func macSamples() string {
	var lines []string
	for _, c := range []string{"10:11:10", "10:11:20", "10:11:30", "10:11:40", "10:11:50", "10:12:00", "10:12:10", "10:12:20"} {
		lines = append(lines, sampleLine(c, 60, 512))
	}
	lines = append(lines, sampleLine("10:12:29", 99, 0))
	for _, c := range []string{"10:12:30", "10:12:40", "10:12:50", "10:13:00", "10:13:10"} {
		lines = append(lines, sampleLine(c, 55, 0))
	}
	lines = append(lines, sampleLine("10:13:11", 99, 0))
	for _, c := range []string{"10:13:20", "10:13:30", "10:13:40", "10:13:50"} {
		lines = append(lines, sampleLine(c, 40, 2048))
	}
	lines = append(lines, sampleLine("10:14:00", 95, 0), sampleLine("10:14:10", 96, 0), sampleLine("10:14:20", 97, 0))
	lines = append(lines, sampleLine("10:14:30", 20, 0), sampleLine("10:14:40", 20, 0))
	lines = append(lines, sampleLine("10:14:50", 30, 0))
	return strings.Join(lines, "\n") + "\n"
}

type fixture struct {
	jobsFile string
	macDir   string
}

func writeFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func newFixture(t *testing.T) fixture {
	t.Helper()
	root := t.TempDir()
	f := fixture{jobsFile: filepath.Join(root, "jobs.json"), macDir: filepath.Join(root, "artifacts", "mac")}
	writeFile(t, f.jobsFile, string(jobsDocument(fixtureJobs())))
	writeFile(t, filepath.Join(f.macDir, "test-integration.json"), macStream())
	writeFile(t, filepath.Join(f.macDir, "test-rusage.ndjson"), macRusage())
	writeFile(t, filepath.Join(f.macDir, "runner-samples.ndjson"), macSamples())
	return f
}

func (f fixture) telemetryArgs(extra ...string) []string {
	args := []string{"ci", "telemetry", "--jobs", f.jobsFile, "--job-artifacts", macJobName + "=" + f.macDir}
	return append(args, extra...)
}

type pathStep struct {
	Level   string  `json:"level"`
	Unit    string  `json:"unit"`
	Seconds float64 `json:"seconds"`
}

type pkgRow struct {
	Package string  `json:"package"`
	Result  string  `json:"result"`
	WallS   float64 `json:"wall_s"`
	Limit   string  `json:"limit"`
	Rusage  *struct {
		WallS     float64 `json:"wall_s"`
		UserS     float64 `json:"user_s"`
		SystemS   float64 `json:"system_s"`
		PeakRSSMB float64 `json:"peak_rss_mb"`
		ExitCode  int     `json:"exit_code"`
	} `json:"rusage"`
	Sampler *struct {
		Samples       int     `json:"samples"`
		CPUBusyMaxPct float64 `json:"cpu_busy_max_pct"`
		SwapUsedMaxMB float64 `json:"swap_used_max_mb"`
	} `json:"sampler"`
}

type report struct {
	G1 struct {
		MacosJobS       float64 `json:"macos_job_s"`
		UbuntuTestStepS float64 `json:"ubuntu_test_step_s"`
	} `json:"g1"`
	CriticalPath []pathStep `json:"critical_path"`
	Packages     []pkgRow   `json:"packages"`
	Incomplete   []string   `json:"incomplete"`
}

func decodeReport(t *testing.T, r result) report {
	t.Helper()
	if r.code != 0 {
		t.Fatalf("evolve ci telemetry --json exit=%d stderr=%q", r.code, acsassert.OutputTail(r.stderr, 10))
	}
	var rep report
	if err := json.Unmarshal([]byte(r.stdout), &rep); err != nil {
		t.Fatalf("stdout is not the report JSON: %v\n%s", err, acsassert.OutputTail(r.stdout, 10))
	}
	return rep
}

func (rep report) pkg(t *testing.T, rel string) pkgRow {
	t.Helper()
	for _, p := range rep.Packages {
		if p.Package == modulePrefix+rel {
			return p
		}
	}
	t.Fatalf("report has no package %s; got %d packages", rel, len(rep.Packages))
	return pkgRow{}
}

func near(a, b float64) bool {
	return math.Abs(a-b) < 0.01
}

func TestC2066_001_FixtureGivesTheExpectedL5ToL1Path(t *testing.T) {
	f := newFixture(t)
	rep := decodeReport(t, runEvolve(t, f.telemetryArgs("--json")...))
	want := []pathStep{
		{"L5", macJobName, 1350},
		{"L4", macTestStep, 1140},
		{"L3", modulePrefix + "cmd/evolve", 70},
		{"L2", "serial", 50},
		{"L1", "TestSerialSlow", 30},
	}
	if len(rep.CriticalPath) != len(want) {
		t.Fatalf("critical_path has %d entries, want %d: %+v", len(rep.CriticalPath), len(want), rep.CriticalPath)
	}
	for i, w := range want {
		got := rep.CriticalPath[i]
		if got.Level != w.Level || got.Unit != w.Unit || !near(got.Seconds, w.Seconds) {
			t.Errorf("critical_path[%d] = %+v, want %+v", i, got, w)
		}
	}
}

func TestC2066_002_TheMarkdownReportListsTheLevelsFromL5DownToL1(t *testing.T) {
	f := newFixture(t)
	r := runEvolve(t, f.telemetryArgs()...)
	if r.code != 0 {
		t.Fatalf("exit=%d stderr=%q", r.code, acsassert.OutputTail(r.stderr, 10))
	}
	cursor := 0
	for _, marker := range []string{"L5", macJobName, "L4", macTestStep, "L3", "cmd/evolve", "L2", "serial", "L1", "TestSerialSlow"} {
		idx := strings.Index(r.stdout[cursor:], marker)
		if idx < 0 {
			t.Fatalf("markdown lacks %q after offset %d; output:\n%s", marker, cursor, acsassert.OutputTail(r.stdout, 40))
		}
		cursor += idx + len(marker)
	}
}

func TestC2066_003_WithoutArtifactsThePathStopsAtL4(t *testing.T) {
	f := newFixture(t)
	rep := decodeReport(t, runEvolve(t, "ci", "telemetry", "--jobs", f.jobsFile, "--json"))
	levels := make([]string, 0, len(rep.CriticalPath))
	for _, s := range rep.CriticalPath {
		levels = append(levels, s.Level)
	}
	if strings.Join(levels, ",") != "L5,L4" {
		t.Errorf("levels without artifacts = %v, want [L5 L4]", levels)
	}
	if len(rep.Packages) != 0 {
		t.Errorf("packages without artifacts = %d, want 0", len(rep.Packages))
	}
}

func TestC2066_004_TheRedPackageJoinsItsRusageAndSamplerRecords(t *testing.T) {
	f := newFixture(t)
	rep := decodeReport(t, runEvolve(t, f.telemetryArgs("--json")...))
	core := rep.pkg(t, "internal/core")
	if core.Result != "fail" {
		t.Errorf("internal/core result = %q, want fail", core.Result)
	}
	if core.Rusage == nil {
		t.Fatalf("internal/core has no rusage record joined")
	}
	if !near(core.Rusage.SystemS, 9) || !near(core.Rusage.UserS, 3) || !near(core.Rusage.PeakRSSMB, 512) || core.Rusage.ExitCode != 1 {
		t.Errorf("internal/core rusage = %+v, want user 3 system 9 rss 512 exit 1", *core.Rusage)
	}
	if core.Sampler == nil {
		t.Fatalf("internal/core has no sampler records joined")
	}
	if core.Sampler.Samples != 5 {
		t.Errorf("internal/core sampler samples = %d, want 5 (the window is inclusive and excludes the records one second outside it)", core.Sampler.Samples)
	}
	if !near(core.Sampler.CPUBusyMaxPct, 55) {
		t.Errorf("internal/core cpu_busy_max_pct = %v, want 55", core.Sampler.CPUBusyMaxPct)
	}
	if core.Limit != "subprocesses-and-syscalls" {
		t.Errorf("internal/core limit = %q, want subprocesses-and-syscalls (system CPU 9 s above user CPU 3 s)", core.Limit)
	}
}

func TestC2066_005_EachPackageGetsTheLimitOfTheFirstRuleThatHolds(t *testing.T) {
	f := newFixture(t)
	rep := decodeReport(t, runEvolve(t, f.telemetryArgs("--json")...))
	cases := []struct{ rel, limit, why string }{
		{"internal/bridge", "memory", "macOS swap 2048 MB is above 1024 MB"},
		{"internal/phases/audit", "cpu-saturation", "every sample in the window is above 90% busy"},
		{"internal/idle", "wait", "CPU efficiency 0.15 is below 0.5"},
		{"cmd/evolve", "cpu", "macOS swap 512 MB is normal and no other rule holds"},
		{"internal/xcmd/evolve", "unknown", "no rusage record belongs to this package"},
	}
	for _, c := range cases {
		if got := rep.pkg(t, c.rel).Limit; got != c.limit {
			t.Errorf("%s limit = %q, want %q (%s)", c.rel, got, c.limit, c.why)
		}
	}
}

func TestC2066_006_ARusageRecordJoinsOnlyTheExactPackagePath(t *testing.T) {
	f := newFixture(t)
	rep := decodeReport(t, runEvolve(t, f.telemetryArgs("--json")...))
	if lookalike := rep.pkg(t, "internal/xcmd/evolve"); lookalike.Rusage != nil {
		t.Errorf("internal/xcmd/evolve took the rusage record of cmd/evolve: %+v", *lookalike.Rusage)
	}
	for _, p := range rep.Packages {
		if strings.HasSuffix(p.Package, "internal/ghost") {
			t.Errorf("a rusage record with no test stream became a package: %s", p.Package)
		}
	}
}

func TestC2066_007_APackageWithoutAResultIsListedAsIncomplete(t *testing.T) {
	f := newFixture(t)
	rep := decodeReport(t, runEvolve(t, f.telemetryArgs("--json")...))
	hung := modulePrefix + "internal/hung"
	if len(rep.Incomplete) != 1 || rep.Incomplete[0] != hung {
		t.Errorf("incomplete = %v, want [%s]", rep.Incomplete, hung)
	}
	for _, p := range rep.Packages {
		if p.Package == hung {
			t.Errorf("an unfinished package is also listed with a wall time: %+v", p)
		}
	}
}

func TestC2066_008_TheReportGivesTheG1MacosJobAndUbuntuTestStepTimes(t *testing.T) {
	f := newFixture(t)
	rep := decodeReport(t, runEvolve(t, f.telemetryArgs("--json")...))
	if !near(rep.G1.MacosJobS, 1350) {
		t.Errorf("g1.macos_job_s = %v, want 1350 (started to completed of the macOS job)", rep.G1.MacosJobS)
	}
	if !near(rep.G1.UbuntuTestStepS, 600) {
		t.Errorf("g1.ubuntu_test_step_s = %v, want 600 (the longest ubuntu test step; the Go 1.27 step takes 480)", rep.G1.UbuntuTestStepS)
	}
}

func TestC2066_009_TheMarkdownReportShowsTheG1Times(t *testing.T) {
	f := newFixture(t)
	r := runEvolve(t, f.telemetryArgs()...)
	if r.code != 0 {
		t.Fatalf("exit=%d stderr=%q", r.code, acsassert.OutputTail(r.stderr, 10))
	}
	for _, want := range []string{"G1", "1350", "600"} {
		if !strings.Contains(r.stdout, want) {
			t.Errorf("markdown lacks %q:\n%s", want, acsassert.OutputTail(r.stdout, 40))
		}
	}
}

func TestC2066_010_BadInputFailsLoudlyAndPrintsNoReport(t *testing.T) {
	f := newFixture(t)
	malformed := filepath.Join(t.TempDir(), "jobs.json")
	writeFile(t, malformed, "{not json")
	cases := []struct {
		name     string
		args     []string
		wantCode int
	}{
		{"no jobs flag", []string{"ci", "telemetry"}, exitUsageCode},
		{"missing jobs file", []string{"ci", "telemetry", "--jobs", filepath.Join(t.TempDir(), "absent.json")}, -1},
		{"malformed jobs file", []string{"ci", "telemetry", "--jobs", malformed}, -1},
		{"artifact binding without equals", []string{"ci", "telemetry", "--jobs", f.jobsFile, "--job-artifacts", "no-separator"}, -1},
		{"artifact directory missing", []string{"ci", "telemetry", "--jobs", f.jobsFile, "--job-artifacts", macJobName + "=" + filepath.Join(t.TempDir(), "absent")}, -1},
		{"unknown job name", []string{"ci", "telemetry", "--jobs", f.jobsFile, "--job-artifacts", "no such job=" + f.macDir}, -1},
	}
	for _, c := range cases {
		r := runEvolve(t, append(c.args, "--json")...)
		if r.code == 0 {
			t.Errorf("%s: exit 0, want a failure", c.name)
		}
		if c.wantCode >= 0 && r.code != c.wantCode {
			t.Errorf("%s: exit %d, want %d", c.name, r.code, c.wantCode)
		}
		if !strings.Contains(r.stderr, telemetryErrorPrefix) {
			t.Errorf("%s: stderr lacks %q, the failure is silent: %q", c.name, telemetryErrorPrefix, acsassert.OutputTail(r.stderr, 4))
		}
		if strings.TrimSpace(r.stdout) != "" {
			t.Errorf("%s: stdout is not empty on failure: %q", c.name, acsassert.OutputTail(r.stdout, 3))
		}
	}
}

func TestC2066_011_TheCiUsageNamesTheTelemetryVerb(t *testing.T) {
	r := runEvolve(t, "ci")
	if r.code == 0 {
		t.Fatalf("evolve ci with no verb exit 0, want a usage failure")
	}
	if !strings.Contains(r.stderr, "evolve ci telemetry") {
		t.Errorf("usage lacks the telemetry verb: %q", acsassert.OutputTail(r.stderr, 6))
	}
}

func TestC2066_012_EveryFunctionOfTheTelemetryPackageIsFullyCovered(t *testing.T) {
	dir := goDir(t)
	profile := filepath.Join(t.TempDir(), "cover.out")
	testRun := runIn(t, dir, "go", "test", "-count=1", "-coverprofile="+profile, citelemetryPkg)
	if testRun.code != 0 {
		t.Fatalf("go test %s failed (exit %d):\n%s", citelemetryPkg, testRun.code, acsassert.OutputTail(testRun.stdout+testRun.stderr, 25))
	}
	funcs := runIn(t, dir, "go", "tool", "cover", "-func="+profile)
	if funcs.code != 0 {
		t.Fatalf("go tool cover -func failed: %s", acsassert.OutputTail(funcs.stderr, 10))
	}
	names := coveredFunctionNames(funcs.stdout)
	if len(names) == 0 {
		t.Fatalf("the cover report lists no function of %s", citelemetryPkg)
	}
	for _, name := range names {
		pct, err := acsassert.FuncCoverage(funcs.stdout, name)
		if err != nil {
			t.Errorf("%v", err)
			continue
		}
		if pct < coverageRequired {
			t.Errorf("%s is covered at %.1f%%, want %.0f%%", name, pct, coverageRequired)
		}
	}
}

func TestC2066_013_TheTelemetryPackageGraduatesIntoBothCoverageGates(t *testing.T) {
	dir := goDir(t)
	strict := filepath.Join(dir, ".cover-strict")
	apicover := filepath.Join(dir, ".apicover-enforce")
	named := filepath.Join(dir, "internal", "citelemetry", "apicover_named_test.go")
	acsassert.AllOf(t,
		func(tb acsassert.TB) bool {
			return acsassert.FileMatchesRegex(tb, strict, `(?m)^\./internal/citelemetry 100$`)
		},
		func(tb acsassert.TB) bool {
			return acsassert.FileMatchesRegex(tb, apicover, `(?m)^\./internal/citelemetry$`)
		},
		func(tb acsassert.TB) bool { return acsassert.FileExists(tb, named) },
	)
}

func runIn(t *testing.T, dir, name string, args ...string) result {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), buildTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	cmd.WaitDelay = 5 * time.Second
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	code := 0
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		code = exitErr.ExitCode()
	} else if err != nil {
		t.Fatalf("run %s %v: %v", name, args, err)
	}
	return result{stdout: stdout.String(), stderr: stderr.String(), code: code}
}

func coveredFunctionNames(coverFuncOutput string) []string {
	seen := map[string]bool{}
	var names []string
	for _, line := range strings.Split(coverFuncOutput, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 3 || !strings.Contains(fields[0], "internal/citelemetry/") {
			continue
		}
		if !seen[fields[1]] {
			seen[fields[1]] = true
			names = append(names, fields[1])
		}
	}
	return names
}
