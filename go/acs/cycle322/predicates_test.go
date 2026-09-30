//go:build acs

package cycle322

import (
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/modelcatalog"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

func goDir(t *testing.T) string {
	t.Helper()
	return filepath.Join(acsassert.RepoRoot(t), "go")
}

const modelcatalogPkg = "./internal/modelcatalog/..."

func tail(s string, n int) string {
	lines := strings.Split(s, "\n")
	if len(lines) <= n {
		return s
	}
	return strings.Join(lines[len(lines)-n:], "\n")
}

func runGoTest(t *testing.T, args ...string) (combined string, code int) {
	t.Helper()
	full := append([]string{"test", "-C", goDir(t)}, args...)
	stdout, stderr, code, err := acsassert.SubprocessOutput("go", full...)
	if err != nil {
		t.Fatalf("go test failed to launch (not a test failure): %v\n%s", err, tail(stderr, 30))
	}
	return stdout + stderr, code
}

var coverTotalRe = regexp.MustCompile(`(?m)^total:\s+\S+\s+([0-9.]+)%`)

func coverFuncOutput(t *testing.T) string {
	t.Helper()
	profile := filepath.Join(t.TempDir(), "c.cover")
	combined, code := runGoTest(t, "-count=1", "-coverprofile="+profile, modelcatalogPkg)
	if code != 0 {
		t.Fatalf("RED: %s test run failed (exit=%d) — add the createTemp seam + the "+
			"tmp.Write/Sync/Close failure tests / fix regressions:\n%s", modelcatalogPkg, code, tail(combined, 40))
	}
	stdout, stderr, code2, err := acsassert.SubprocessOutput("go", "tool", "cover", "-func="+profile)
	if err != nil || code2 != 0 {
		t.Fatalf("go tool cover failed (exit=%d, err=%v):\n%s", code2, err, tail(stderr, 20))
	}
	return stdout
}

func totalCoverage(t *testing.T, out string) float64 {
	t.Helper()
	m := coverTotalRe.FindStringSubmatch(out)
	if m == nil {
		t.Fatalf("could not parse total coverage from:\n%s", tail(out, 20))
	}
	pct, err := strconv.ParseFloat(m[1], 64)
	if err != nil {
		t.Fatalf("unparsable total coverage %q: %v", m[1], err)
	}
	return pct
}

func funcCoverage(t *testing.T, out, fn string) float64 {
	t.Helper()
	re := regexp.MustCompile(`(?m)^\S+:\d+:\s+` + regexp.QuoteMeta(fn) + `\s+([0-9.]+)%`)
	m := re.FindStringSubmatch(out)
	if m == nil {
		t.Fatalf("could not find %s() in coverage output (renamed/removed?):\n%s", fn, tail(out, 20))
	}
	pct, err := strconv.ParseFloat(m[1], 64)
	if err != nil {
		t.Fatalf("unparsable %s coverage %q: %v", fn, m[1], err)
	}
	return pct
}

func TestC322_001_ModelcatalogCoverageFloor(t *testing.T) {
	const floor = 92.0
	out := coverFuncOutput(t)
	if pct := totalCoverage(t, out); pct < floor {
		t.Errorf("RED: internal/modelcatalog coverage %.1f%% < %.1f%% floor — exercise store.Write's "+
			"dark tmp.Write / tmp.Sync / tmp.Close error exits via the createTemp seam. Baseline 87.3%%.", pct, floor)
	}
}

func TestC322_002_StoreWriteCoverageFloor(t *testing.T) {
	const floor = 85.0
	out := coverFuncOutput(t)
	if pct := funcCoverage(t, out, "Write"); pct < floor {
		t.Errorf("RED: store.Write coverage %.1f%% < %.1f%% — add tests that fail tmp.Write, tmp.Sync, "+
			"and tmp.Close via the createTemp seam. Baseline 66.7%%; the marshal-error arm is unreachable "+
			"so ~96%% (not 100%%) is the practical ceiling.", pct, floor)
	}
}

func TestC322_003_ReadOnlyDirWriteErrorTestPasses(t *testing.T) {
	combined, code := runGoTest(t, "-run", "TestWriteCreateTempFailsInReadOnlyDir",
		"-count=1", modelcatalogPkg)
	if code != 0 {
		t.Errorf("RED: TestWriteCreateTempFailsInReadOnlyDir does not pass (exit=%d) — the read-only-dir "+
			"write-error contract must hold (and the package must compile):\n%s", code, tail(combined, 30))
	}
}

func TestC322_004_ExistingContractSuiteGreen(t *testing.T) {
	combined, code := runGoTest(t, "-run",
		"TestRead|TestWriteThenReadRoundTrip|TestWriteCreates|TestWriteIsAtomic|TestLookup|TestBuildFromSnapshots|TestDispatch",
		"-count=1", modelcatalogPkg)
	if code != 0 {
		t.Errorf("RED: the existing modelcatalog contract suite fails (exit=%d) — the new temp-file "+
			"error-path tests must not break the round-trip / atomic-write / lookup contract:\n%s",
			code, tail(combined, 30))
	}
}

func TestC322_005_NoTempLeakOnFailedWrite(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, modelcatalog.FileName)
	if err := os.MkdirAll(filepath.Join(target, "child"), 0o755); err != nil {
		t.Fatalf("arrange: mkdir target-as-nonempty-dir: %v", err)
	}

	err := modelcatalog.Write(dir, modelcatalog.Catalog{})
	if err == nil {
		t.Fatalf("RED: Write onto a non-empty dir must return a non-nil rename error")
	}

	entries, rerr := os.ReadDir(dir)
	if rerr != nil {
		t.Fatalf("ReadDir after failed write: %v", rerr)
	}
	for _, e := range entries {
		if filepath.Ext(e.Name()) == ".tmp" {
			t.Errorf("RED: temp file leaked after failed write: %s — cleanup() must remove it", e.Name())
		}
	}
}
