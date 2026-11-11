//go:build acs

package cycle2320

import (
	"bufio"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/codeindex"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

var lintedPackages = []string{"./internal/core/...", "./internal/codeindex/...", "./internal/gc/..."}

var errDiskFull = errors.New("disk full")

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, errDiskFull }

type recordingWriter struct{ buf strings.Builder }

func (w *recordingWriter) Write(p []byte) (int, error) { return w.buf.Write(p) }

func goDir(t *testing.T) string {
	t.Helper()
	return filepath.Join(acsassert.RepoRoot(t), "go")
}

func runIn(t *testing.T, dir, name string, args ...string) (string, int) {
	t.Helper()
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err == nil {
		return string(out), 0
	}
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		return string(out), exit.ExitCode()
	}
	t.Fatalf("run %s: %v", name, err)
	return "", -1
}

func TestC2320_001_GolangciLintReportsZeroIssuesOnCoreCodeindexAndGc(t *testing.T) {
	if _, err := exec.LookPath("golangci-lint"); err != nil {
		t.Fatalf("golangci-lint is not on PATH: %v", err)
	}
	out, code := runIn(t, goDir(t), "golangci-lint", append([]string{"run"}, lintedPackages...)...)
	if code != 0 {
		t.Errorf("golangci-lint exit=%d, want 0 issues:\n%s", code, acsassert.OutputTail(out, 20))
	}
}

func benchReportWithOneRow() codeindex.BenchReport {
	return codeindex.BenchReport{
		LoadAvg: 1.5,
		Rows: []codeindex.BenchRow{{
			Backend: "gotypes", Query: codeindex.QueryCallers,
			TP: 3, FP: 1, FN: 2, Precision: 0.75, Recall: 0.6, MedianBytes: 120,
		}},
	}
}

func writeTableMethod(t *testing.T) reflect.Value {
	t.Helper()
	method := reflect.ValueOf(benchReportWithOneRow()).MethodByName("WriteTable")
	if !method.IsValid() {
		t.Fatalf("BenchReport has no WriteTable(io.Writer) error: Table swallows the tabwriter Flush error")
	}
	signature := method.Type()
	writerType := reflect.TypeOf((*interface{ Write([]byte) (int, error) })(nil)).Elem()
	if signature.NumIn() != 1 || !writerType.AssignableTo(signature.In(0)) ||
		signature.NumOut() != 1 || signature.Out(0).String() != "error" {
		t.Fatalf("WriteTable has signature %s, want func(io.Writer) error", signature)
	}
	return method
}

func callWriteTable(t *testing.T, w any) error {
	t.Helper()
	got := writeTableMethod(t).Call([]reflect.Value{reflect.ValueOf(w)})[0]
	if got.IsNil() {
		return nil
	}
	return got.Interface().(error)
}

func TestC2320_002_ABenchTableWriteThatFailsIsReturnedNotSwallowed(t *testing.T) {
	err := callWriteTable(t, failingWriter{})
	if err == nil {
		t.Fatal("WriteTable returned nil although the writer failed on every write")
	}
	if !errors.Is(err, errDiskFull) {
		t.Errorf("WriteTable error %q does not wrap the writer failure", err)
	}
}

func TestC2320_003_AWorkingWriterGetsTheFullTableAndNoError(t *testing.T) {
	w := &recordingWriter{}
	if err := callWriteTable(t, w); err != nil {
		t.Fatalf("WriteTable on a working writer: %v", err)
	}
	for _, want := range []string{"backend", "precision", "gotypes", "callers", "0.75", "0.60", "host load average: 1.50"} {
		if !strings.Contains(w.buf.String(), want) {
			t.Errorf("table lacks %q:\n%s", want, w.buf.String())
		}
	}
}

func TestC2320_004_TheTableStringStaysTheSameAsTheWrittenTable(t *testing.T) {
	w := &recordingWriter{}
	if err := callWriteTable(t, w); err != nil {
		t.Fatalf("WriteTable: %v", err)
	}
	table := reflect.ValueOf(benchReportWithOneRow()).MethodByName("Table")
	if !table.IsValid() {
		return
	}
	if got := table.Call(nil)[0].String(); got != w.buf.String() {
		t.Errorf("Table() and WriteTable disagree:\n%q\n%q", got, w.buf.String())
	}
}

func TestC2320_005_EveryBenchFuncInCodeindexIsFullyCovered(t *testing.T) {
	profile := filepath.Join(t.TempDir(), "codeindex.cover")
	dir := goDir(t)
	if out, code := runIn(t, dir, "go", "test", "-count=1", "-coverprofile="+profile, "./internal/codeindex"); code != 0 {
		t.Fatalf("go test ./internal/codeindex exit=%d:\n%s", code, acsassert.OutputTail(out, 20))
	}
	out, code := runIn(t, dir, "go", "tool", "cover", "-func="+profile)
	if code != 0 {
		t.Fatalf("go tool cover exit=%d:\n%s", code, out)
	}
	seen := 0
	for _, line := range strings.Split(out, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 3 || !strings.Contains(fields[0], "/codeindex/bench.go:") {
			continue
		}
		seen++
		if fields[len(fields)-1] != "100.0%" {
			t.Errorf("%s %s is covered %s, want 100.0%%", fields[0], fields[1], fields[len(fields)-1])
		}
	}
	if seen == 0 {
		t.Fatalf("no bench.go function in the cover report:\n%s", acsassert.OutputTail(out, 10))
	}
	if cover, err := acsassert.FuncCoverage(out, "WriteTable"); err != nil || cover < 100 {
		t.Errorf("WriteTable coverage = %v, err=%v, want 100", cover, err)
	}
}

type coverBlock struct {
	startLine, endLine int
	count              int
}

var coverLine = regexp.MustCompile(`^(.+?):(\d+)\.\d+,(\d+)\.\d+ \d+ (\d+)$`)

func blocksOf(t *testing.T, profile, fileSuffix string) []coverBlock {
	t.Helper()
	f, err := os.Open(profile)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	var blocks []coverBlock
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		m := coverLine.FindStringSubmatch(scanner.Text())
		if m == nil || !strings.HasSuffix(m[1], fileSuffix) {
			continue
		}
		start, _ := strconv.Atoi(m[2])
		end, _ := strconv.Atoi(m[3])
		count, _ := strconv.Atoi(m[4])
		blocks = append(blocks, coverBlock{start, end, count})
	}
	return blocks
}

func writeTableCallerBounds(t *testing.T, cmdDir string) (file string, start, end int) {
	t.Helper()
	files, err := filepath.Glob(filepath.Join(cmdDir, "*.go"))
	if err != nil {
		t.Fatal(err)
	}
	funcHead := regexp.MustCompile(`^func `)
	for _, path := range files {
		if strings.HasSuffix(path, "_test.go") {
			continue
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		lines := strings.Split(string(raw), "\n")
		for i, line := range lines {
			if !strings.Contains(line, ".WriteTable(") {
				continue
			}
			start = i
			for start > 0 && !funcHead.MatchString(lines[start]) {
				start--
			}
			end = i + 1
			for end < len(lines) && !funcHead.MatchString(lines[end]) {
				end++
			}
			return filepath.Base(path), start + 1, end
		}
	}
	t.Fatalf("no production file under %s calls .WriteTable(: the bench verb never reaches the error-returning writer", cmdDir)
	return "", 0, 0
}

func TestC2320_006_TheBenchVerbReachesWriteTableAndItsWholeCallerIsCovered(t *testing.T) {
	dir := goDir(t)
	file, funcStart, funcEnd := writeTableCallerBounds(t, filepath.Join(dir, "cmd", "evolve"))
	profile := filepath.Join(t.TempDir(), "evolve.cover")
	out, code := runIn(t, dir, "go", "test", "-count=1", "-run", "Bench|RunCodeCommand", "-coverprofile="+profile, "./cmd/evolve")
	if code != 0 {
		t.Fatalf("go test ./cmd/evolve exit=%d:\n%s", code, acsassert.OutputTail(out, 20))
	}
	inFunc := 0
	for _, b := range blocksOf(t, profile, "cmd/evolve/"+file) {
		if b.startLine < funcStart || b.endLine > funcEnd {
			continue
		}
		inFunc++
		if b.count == 0 {
			t.Errorf("%s lines %d-%d never run by the Bench/RunCodeCommand tests: the WriteTable caller is not fully covered", file, b.startLine, b.endLine)
		}
	}
	if inFunc == 0 {
		t.Fatalf("no coverage block found for the WriteTable caller in %s lines %d-%d", file, funcStart, funcEnd)
	}
}

func TestC2320_007_TheRouterWallBenchFailureStaysASignalNamingTheFamily(t *testing.T) {
	names := "^(TestBenchQuotaFamilies_AStoreThatCannotBeWrittenIsAnErrorThatNamesTheFamily|TestRunCycle_ABenchThatFailsAtTheRouterWallStillPausesAndNamesTheFamily)$"
	out, code := runIn(t, goDir(t), "go", "test", "-count=1", "-v", "-run", names, "./internal/core")
	if code != 0 {
		t.Fatalf("router wall tests exit=%d:\n%s", code, acsassert.OutputTail(out, 20))
	}
	if got := strings.Count(out, "--- PASS"); got != 2 {
		t.Errorf("%d router wall tests passed, want 2:\n%s", got, acsassert.OutputTail(out, 20))
	}
}
