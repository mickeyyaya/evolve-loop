package main

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/codeindex/gotypes"
	"github.com/mickeyyaya/evolve-loop/go/internal/codeindex"
)

func benchProject(t *testing.T, source string) (root, indexDir string) {
	t.Helper()
	root = t.TempDir()
	indexDir = filepath.Join(root, "idx")
	module := filepath.Join(indexDir, "go")
	for rel, body := range map[string]string{
		"go.mod":     "module example.com/bench\n\ngo 1.22\n",
		"lib/lib.go": source,
	} {
		path := filepath.Join(module, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Mkdir(filepath.Join(indexDir, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeCodePolicy(t, root, `{"code_index":{"index_dir":"`+indexDir+`","trail_bin":"/nonexistent/trail"}}`)
	if err := gotypes.Build(context.Background(), module, gotypes.IndexPath(indexDir)); err != nil {
		t.Fatalf("gotypes.Build: %v", err)
	}
	return root, indexDir
}

const benchLibSource = "package lib\n\nfunc Target() int { return 1 }\n\nfunc Use() int { return Target() }\n"

func runBenchVerb(t *testing.T, host codeIndexHost, args ...string) (int, string, string) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	rc := codeBenchVerb(args, &stdout, &stderr, host)
	return rc, stdout.String(), stderr.String()
}

func benchHost(load string) codeIndexHost {
	return codeIndexHost{run: scriptedRun(load), env: []string{"PATH=/usr/bin"}, stat: os.Stat}
}

func TestCodeBench_MeasuresGotypesAgainstTheTruthAndPrintsTheTable(t *testing.T) {
	t.Parallel()
	root, _ := benchProject(t, benchLibSource)

	rc, stdout, stderr := runBenchVerb(t, benchHost("{ 2.50 1.00 1.00 }"), "--backends", "gotypes", "--project-root", root, "--symbols", "5")

	if rc != 0 || !strings.Contains(stdout, "gotypes  callers  1.00") || !strings.Contains(stdout, "host load average") {
		t.Errorf("rc=%d stdout=%q stderr=%q, want a gotypes callers row at 1.00 and the load average", rc, stdout, stderr)
	}
}

func TestCodeBench_ATruthThatTheGrepCheckRefusesFailsTheRun(t *testing.T) {
	t.Parallel()
	root, indexDir := benchProject(t, benchLibSource)
	changed := "package lib\n\nfunc Target() int { return 1 }\n\nfunc Use() int { return Target() }\n\nfunc Extra() int { return Target() }\n"
	if err := os.WriteFile(filepath.Join(indexDir, "go", "lib", "lib.go"), []byte(changed), 0o644); err != nil {
		t.Fatal(err)
	}

	rc, stdout, stderr := runBenchVerb(t, benchHost(""), "--backends", "gotypes", "--project-root", root)

	if rc != codeExitFailed || stdout != "" || !strings.Contains(stderr, "disagree") {
		t.Errorf("rc=%d stdout=%q stderr=%q, want exit %d, no table and the disagreement", rc, stdout, stderr, codeExitFailed)
	}
}

func TestCodeBench_AnAbsentGotypesIndexIsExitAbsent(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeCodePolicy(t, root, `{"code_index":{"index_dir":"`+filepath.Join(root, "none")+`","trail_bin":"/nonexistent/trail"}}`)

	rc, _, stderr := runBenchVerb(t, benchHost(""), "--project-root", root)

	if rc != codeExitAbsent || !strings.Contains(stderr, "absent") {
		t.Errorf("rc=%d stderr=%q, want exit %d", rc, stderr, codeExitAbsent)
	}
}

func TestCodeBench_AnUnknownBackendOrAFlagErrorIsAUsageError(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeCodePolicy(t, root, `{"code_index":{"index_dir":"/abs/index","trail_bin":"/abs/trail"}}`)

	rc, _, stderr := runBenchVerb(t, benchHost(""), "--backends", "zzz", "--project-root", root)
	if rc != codeExitUsage || !strings.Contains(stderr, "unknown backend") {
		t.Errorf("rc=%d stderr=%q, want a usage exit naming the unknown backend", rc, stderr)
	}
	if rc, _, _ := runBenchVerb(t, benchHost(""), "--bogus"); rc != codeExitUsage {
		t.Errorf("a bad flag: rc=%d, want %d", rc, codeExitUsage)
	}
	if rc, _, _ := runBenchVerb(t, benchHost(""), "extra"); rc != codeExitUsage {
		t.Errorf("a stray argument: rc=%d, want %d", rc, codeExitUsage)
	}
}

func TestCodeBench_APolicyThatCannotLoadFailsTheRun(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeCodePolicy(t, root, `{"code_index":{"bogus":1}}`)

	rc, _, stderr := runBenchVerb(t, benchHost(""), "--project-root", root)

	if rc != codeExitFailed || stderr == "" {
		t.Errorf("rc=%d stderr=%q, want exit %d with a message", rc, stderr, codeExitFailed)
	}
}

func TestCodeBenchNames_BlankMeansEveryKnownBackend(t *testing.T) {
	t.Parallel()

	if got := codeBenchNames(" ", map[string]codeindex.Factory{"b": nil, "a": nil}); strings.Join(got, ",") != "a,b" {
		t.Errorf("blank names = %v, want a,b", got)
	}
	if got := codeBenchNames("x,y", nil); strings.Join(got, ",") != "x,y" {
		t.Errorf("named = %v, want x,y", got)
	}
}

func TestParseLoadAverage_ReadsTheFirstFigureOfEachHostFormat(t *testing.T) {
	t.Parallel()
	for raw, want := range map[string]float64{"{ 2.50 1.00 1.00 }\n": 2.5, "0.75 0.50 0.25 1/100 99\n": 0.75, "": 0, "{ x }": 0} {
		if got := parseLoadAverage(raw); got != want {
			t.Errorf("parseLoadAverage(%q) = %v, want %v", raw, got, want)
		}
	}
}

func TestHostLoadAverage_PrefersTheProcFileThenFallsBackToSysctl(t *testing.T) {
	t.Parallel()
	noProc := func(string) ([]byte, error) { return nil, os.ErrNotExist }
	proc := func(string) ([]byte, error) { return []byte("0.75 0.50 0.25 1/100 99\n"), nil }
	failing := codeIndexHost{run: func(context.Context, string, string, []string, []string, io.Reader, io.Writer, io.Writer) (int, error) {
		return 1, os.ErrNotExist
	}}

	got := []float64{
		hostLoadAverage(context.Background(), benchHost(""), proc),
		hostLoadAverage(context.Background(), benchHost("{ 4.25 1.00 1.00 }"), noProc),
		hostLoadAverage(context.Background(), failing, noProc),
	}

	if got[0] != 0.75 || got[1] != 4.25 || got[2] != 0 {
		t.Errorf("load = %v, want 0.75, 4.25 and 0", got)
	}
}

func TestRunCodeCommand_RoutesTheBenchVerb(t *testing.T) {
	t.Parallel()
	var stdout, stderr bytes.Buffer

	rc := runCodeCommand([]string{"bench", "extra"}, nil, &stdout, &stderr)

	if rc != codeExitUsage || !strings.Contains(stderr.String(), "bench") {
		t.Errorf("rc=%d stderr=%q, want the bench usage exit", rc, stderr.String())
	}
}

type failingStdout struct{}

func (failingStdout) Write([]byte) (int, error) { return 0, io.ErrClosedPipe }

func TestCodeBench_AStdoutThatCannotBeWrittenFailsTheRun(t *testing.T) {
	t.Parallel()
	root, _ := benchProject(t, benchLibSource)
	var stderr bytes.Buffer

	rc := codeBenchVerb([]string{"--backends", "gotypes", "--project-root", root, "--symbols", "5"}, failingStdout{}, &stderr, benchHost("{ 2.50 1.00 1.00 }"))

	if rc != codeExitFailed || !strings.Contains(stderr.String(), "evolve code bench:") {
		t.Errorf("rc=%d stderr=%q, want codeExitFailed and the named write error", rc, stderr.String())
	}
}
