//go:build acs

package cycle2379

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	workflowRel     = ".github/workflows/go.yml"
	samplerNoteRel  = "docs/architecture/packages/cmd-hostsampler.md"
	latencyNoteRel  = "docs/architecture/packages/cmd-testlatency.md"
	samplesBaseName = "runner-samples.ndjson"
	megabyte        = 1 << 20

	fiveCoreHeader  = `{"host":{"os":"darwin","cpus":5,"mem_bytes":17179869184}}`
	threeCoreHeader = `{"host":{"os":"darwin","cpus":3,"mem_bytes":7516192768}}`
	oneSampleLine   = `{"ts":"2026-11-14T10:00:01Z","cpu_busy_pct":40,"mem_used_mb":2048}`

	wallStream = `{"Action":"run","Package":"x/pkg","Test":"TestA"}
{"Action":"pass","Package":"x/pkg","Test":"TestA","Elapsed":4}
{"Action":"pass","Package":"x/pkg","Elapsed":4.5}
`
)

type builtBinary struct {
	once sync.Once
	path string
	err  string
}

var (
	samplerBuild builtBinary
	latencyBuild builtBinary
	buildRoot    string
)

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "cycle2379-bin-")
	if err == nil {
		buildRoot = dir
	}
	code := m.Run()
	if buildRoot != "" {
		os.RemoveAll(buildRoot)
	}
	os.Exit(code)
}

func (b *builtBinary) build(t *testing.T, pkg string) string {
	t.Helper()
	b.once.Do(func() {
		if buildRoot == "" {
			b.err = "no build directory"
			return
		}
		goDir := filepath.Join(acsassert.RepoRoot(t), "go")
		out := filepath.Join(buildRoot, filepath.Base(pkg))
		_, stderr, code, err := acsassert.SubprocessOutput("go", "-C", goDir, "build", "-o", out, pkg)
		if err != nil || code != 0 {
			b.err = "go build " + pkg + ": " + acsassert.OutputTail(stderr, 10)
			return
		}
		b.path = out
	})
	if b.err != "" {
		t.Fatalf("RED: %s", b.err)
	}
	return b.path
}

type hostFields struct {
	OS       string `json:"os"`
	CPUs     int    `json:"cpus"`
	MemBytes uint64 `json:"mem_bytes"`
}

type record struct {
	Host *hostFields `json:"host"`
	Ts   string      `json:"ts"`
}

func runSampler(t *testing.T) (lines []string, records []record) {
	t.Helper()
	bin := samplerBuild.build(t, "./cmd/hostsampler")
	out := filepath.Join(t.TempDir(), samplesBaseName)
	_, stderr, code, err := acsassert.SubprocessOutput(bin, "-out", out, "-interval", "200ms", "-duration", "900ms")
	if err != nil || code != 0 {
		t.Fatalf("hostsampler exit=%d err=%v stderr=%q", code, err, stderr)
	}
	raw, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("RED: hostsampler wrote no file: %v", err)
	}
	for _, line := range strings.Split(strings.TrimSpace(string(raw)), "\n") {
		if line == "" {
			continue
		}
		var r record
		if err := json.Unmarshal([]byte(line), &r); err != nil {
			t.Fatalf("not JSON: %q: %v", line, err)
		}
		lines = append(lines, line)
		records = append(records, r)
	}
	if len(records) == 0 {
		t.Fatalf("RED: hostsampler wrote no record")
	}
	return lines, records
}

func trimmed(t *testing.T, name string, args ...string) string {
	t.Helper()
	out, stderr, code, err := acsassert.SubprocessOutput(name, args...)
	if err != nil || code != 0 {
		t.Fatalf("%s %v: exit=%d err=%v stderr=%q", name, args, code, err, stderr)
	}
	return strings.TrimSpace(out)
}

func hostReference(t *testing.T) (cpus int, memBytes uint64) {
	t.Helper()
	switch runtime.GOOS {
	case "darwin":
		n, err := strconv.Atoi(trimmed(t, "sysctl", "-n", "hw.ncpu"))
		if err != nil {
			t.Fatalf("hw.ncpu: %v", err)
		}
		m, err := strconv.ParseUint(trimmed(t, "sysctl", "-n", "hw.memsize"), 10, 64)
		if err != nil {
			t.Fatalf("hw.memsize: %v", err)
		}
		return n, m
	case "linux":
		n, err := strconv.Atoi(trimmed(t, "nproc"))
		if err != nil {
			t.Fatalf("nproc: %v", err)
		}
		raw, err := os.ReadFile("/proc/meminfo")
		if err != nil {
			t.Fatalf("meminfo: %v", err)
		}
		m := regexp.MustCompile(`(?m)^MemTotal:\s+([0-9]+) kB`).FindStringSubmatch(string(raw))
		if m == nil {
			t.Fatalf("meminfo has no MemTotal")
		}
		kb, _ := strconv.ParseUint(m[1], 10, 64)
		return n, kb * 1024
	}
	t.Skipf("no host class source on %s", runtime.GOOS)
	return 0, 0
}

func TestC2379_001_SamplerHeaderHoldsTheCPUCountAndMemorySizeOfTheHost(t *testing.T) {
	wantCPUs, wantMem := hostReference(t)
	_, records := runSampler(t)
	head := records[0].Host
	if head == nil {
		t.Fatalf("RED: the first record of the sampler file has no host object")
	}
	if head.OS != runtime.GOOS {
		t.Errorf("host.os = %q, want %q", head.OS, runtime.GOOS)
	}
	if head.CPUs != wantCPUs {
		t.Errorf("host.cpus = %d, want %d (hw.ncpu on macOS, nproc on Linux)", head.CPUs, wantCPUs)
	}
	if head.MemBytes != wantMem {
		t.Errorf("host.mem_bytes = %d, want %d (hw.memsize on macOS, MemTotal on Linux)", head.MemBytes, wantMem)
	}
}

func TestC2379_002_HostRecordIsWrittenOnceAndSamplesFollowIt(t *testing.T) {
	_, records := runSampler(t)
	hostRecords, samples := 0, 0
	for i, r := range records {
		if r.Host != nil {
			hostRecords++
			if i != 0 {
				t.Errorf("host record at line %d, want only line 1", i+1)
			}
			continue
		}
		if r.Ts == "" {
			t.Errorf("line %d is neither a host record nor a sample", i+1)
		}
		samples++
	}
	if hostRecords != 1 {
		t.Errorf("host records = %d, want exactly 1", hostRecords)
	}
	if samples < 2 {
		t.Errorf("samples after the header = %d, want at least 2 (the sampler must still sample)", samples)
	}
}

type latencyRun struct {
	stdout string
	stderr string
	code   int
}

func runLatency(t *testing.T, samplesBody *string, extra ...string) latencyRun {
	t.Helper()
	bin := latencyBuild.build(t, "./cmd/testlatency")
	dir := t.TempDir()
	stdin := filepath.Join(dir, "events.json")
	if err := os.WriteFile(stdin, []byte(wallStream), 0o644); err != nil {
		t.Fatal(err)
	}
	args := []string{"-budget", "180s", "-title", "Package wall time"}
	if samplesBody != nil {
		path := filepath.Join(dir, samplesBaseName)
		if err := os.WriteFile(path, []byte(*samplesBody), 0o644); err != nil {
			t.Fatal(err)
		}
		args = append(args, "-samples", path)
	}
	args = append(args, extra...)
	cmd := "cd " + shellQuote(dir) + " && " + shellQuote(bin) + " " + joinQuoted(args) + " < " + shellQuote(stdin)
	out, stderr, code, err := acsassert.SubprocessOutput("sh", "-c", cmd)
	if err != nil && code == 0 {
		t.Fatalf("testlatency did not run: %v", err)
	}
	return latencyRun{stdout: out, stderr: stderr, code: code}
}

func shellQuote(s string) string { return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'" }

func joinQuoted(args []string) string {
	quoted := make([]string, len(args))
	for i, a := range args {
		quoted[i] = shellQuote(a)
	}
	return strings.Join(quoted, " ")
}

func hostClassLine(out string) string {
	for _, line := range strings.Split(out, "\n") {
		if strings.Contains(strings.ToLower(line), "host class") {
			return line
		}
	}
	return ""
}

func requireWallReport(t *testing.T, r latencyRun) {
	t.Helper()
	if r.code != 0 {
		t.Fatalf("testlatency exit=%d stderr=%q", r.code, acsassert.OutputTail(r.stderr, 5))
	}
	if !strings.Contains(r.stdout, "# Package wall time") || !strings.Contains(r.stdout, "Aggregate package wall time") {
		t.Fatalf("package wall report missing from stdout: %q", acsassert.OutputTail(r.stdout, 12))
	}
}

func TestC2379_010_JobSummaryPrintsTheHostClassBesideThePackageWallReport(t *testing.T) {
	body := fiveCoreHeader + "\n" + oneSampleLine + "\n"
	r := runLatency(t, &body)
	requireWallReport(t, r)
	line := hostClassLine(r.stdout)
	if line == "" {
		t.Fatalf("RED: the report has no Host class line: %q", acsassert.OutputTail(r.stdout, 12))
	}
	memMB := strconv.Itoa(17179869184 / megabyte)
	for _, want := range []string{"5", memMB, "darwin"} {
		if !strings.Contains(line, want) {
			t.Errorf("host class line %q lacks %q", line, want)
		}
	}
}

func TestC2379_011_HostClassFollowsTheHeaderNotAFixedText(t *testing.T) {
	body := oneSampleLine + "\n" + threeCoreHeader + "\n"
	r := runLatency(t, &body)
	requireWallReport(t, r)
	line := hostClassLine(r.stdout)
	if line == "" {
		t.Fatalf("RED: the report has no Host class line: %q", acsassert.OutputTail(r.stdout, 12))
	}
	memMB := strconv.Itoa(7516192768 / megabyte)
	for _, want := range []string{"3", memMB} {
		if !strings.Contains(line, want) {
			t.Errorf("host class line %q lacks %q", line, want)
		}
	}
	if strings.Contains(line, "16384") {
		t.Errorf("host class line %q shows the memory of another class", line)
	}
}

func TestC2379_012_NoSamplesFlagLeavesTheReportWithoutAHostClassLine(t *testing.T) {
	r := runLatency(t, nil)
	requireWallReport(t, r)
	if line := hostClassLine(r.stdout); line != "" {
		t.Errorf("report without -samples prints a host class: %q", line)
	}
}

func TestC2379_013_SamplesWithoutAHeaderGiveNotRecordedAndTheReportStillRuns(t *testing.T) {
	headerless := oneSampleLine + "\n"
	cases := map[string]latencyRun{
		"no header":    runLatency(t, &headerless),
		"empty file":   runLatency(t, ptr("")),
		"missing file": runLatency(t, nil, "-samples", filepath.Join(t.TempDir(), "absent.ndjson")),
	}
	for name, r := range cases {
		requireWallReport(t, r)
		line := hostClassLine(r.stdout)
		if line == "" || !strings.Contains(strings.ToLower(line), "not recorded") {
			t.Errorf("%s: host class line = %q, want one that says not recorded", name, line)
		}
	}
}

func ptr(s string) *string { return &s }

// acs-predicate: config-check
func TestC2379_014_EveryWallBudgetStepOfTheWorkflowReadsTheSamplerFile(t *testing.T) {
	root := acsassert.RepoRoot(t)
	raw, err := os.ReadFile(filepath.Join(root, workflowRel))
	if err != nil {
		t.Fatal(err)
	}
	flagPattern := regexp.MustCompile(`-samples\s+\S*` + regexp.QuoteMeta(samplesBaseName))
	steps := 0
	for _, line := range strings.Split(string(raw), "\n") {
		if !strings.Contains(line, "cmd/testlatency -budget") {
			continue
		}
		steps++
		if !flagPattern.MatchString(line) {
			t.Errorf("RED: wall budget step does not pass -samples <path>/%s: %s", samplesBaseName, strings.TrimSpace(line))
		}
	}
	if steps < 2 {
		t.Errorf("wall budget steps = %d, want at least 2 (the ubuntu job and the macOS job)", steps)
	}
}

func TestC2379_020_PackageNotesNameTheHostClassSourcesAndPassSteLint(t *testing.T) {
	root := acsassert.RepoRoot(t)
	wants := map[string][]string{
		samplerNoteRel: {"hw.ncpu", "hw.memsize", "nproc"},
		latencyNoteRel: {"-samples", "host class"},
	}
	for rel, terms := range wants {
		raw, err := os.ReadFile(filepath.Join(root, rel))
		if err != nil {
			t.Fatal(err)
		}
		text := strings.ToLower(string(raw))
		for _, term := range terms {
			if !strings.Contains(text, strings.ToLower(term)) {
				t.Errorf("RED: %s does not name %q", rel, term)
			}
		}
	}
	out, errOut, code, err := acsassert.SubprocessOutput("go", "-C", filepath.Join(root, "go"), "run", "./cmd/evolve",
		"docs", "ste-lint", "--strict", "--project-root", root, samplerNoteRel, latencyNoteRel)
	if err != nil {
		t.Fatalf("ste-lint did not run: %v", err)
	}
	if code != 0 || !strings.Contains(out, "ste-lint: 0 finding(s)") {
		t.Errorf("ste-lint rc=%d out=%q stderr=%q", code, acsassert.OutputTail(out, 20), acsassert.OutputTail(errOut, 5))
	}
}
