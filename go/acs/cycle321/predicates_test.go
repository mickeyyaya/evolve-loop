//go:build acs

package cycle321

import (
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/clihealth"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

func goDir(t *testing.T) string {
	t.Helper()
	return filepath.Join(acsassert.RepoRoot(t), "go")
}

const clihealthPkg = "./internal/clihealth/..."

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
	combined, code := runGoTest(t, "-count=1", "-coverprofile="+profile, clihealthPkg)
	if code != 0 {
		t.Fatalf("RED: %s test run failed (exit=%d) — add the write/NewStore/Load I/O error-path "+
			"tests / fix regressions:\n%s", clihealthPkg, code, tail(combined, 40))
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

func TestC321_001_ClihealthCoverageFloor(t *testing.T) {
	const floor = 94.0
	out := coverFuncOutput(t)
	if pct := totalCoverage(t, out); pct < floor {
		t.Errorf("RED: internal/clihealth coverage %.1f%% < %.1f%% floor — exercise the dark write "+
			"error exits, NewStore nil-clock default, and Load degradation arms. Baseline 90.1%%.", pct, floor)
	}
}

func TestC321_002_StoreWriteCoverageFloor(t *testing.T) {
	const floor = 90.0
	out := coverFuncOutput(t)
	if pct := funcCoverage(t, out, "write"); pct < floor {
		t.Errorf("RED: Store.write coverage %.1f%% < %.1f%% — add tests that fail os.MkdirAll "+
			"(read-only parent), os.WriteFile (unwritable temp dir), and os.Rename (target is a "+
			"non-empty dir). Baseline 63.6%%; the marshal-error arm is unreachable so 90%% is the ceiling.", pct, floor)
	}
}

func TestC321_003_NewStoreCoverageFloor(t *testing.T) {
	const floor = 90.0
	out := coverFuncOutput(t)
	if pct := funcCoverage(t, out, "NewStore"); pct < floor {
		t.Errorf("RED: NewStore coverage %.1f%% < %.1f%% — add a test calling NewStore(root, nil) so "+
			"the `now = time.Now` default arm runs. Baseline 66.7%% (only the injected-clock arm).", pct, floor)
	}
}

func TestC321_004_StoreLoadCoverageFloor(t *testing.T) {
	const floor = 90.0
	out := coverFuncOutput(t)
	if pct := funcCoverage(t, out, "Load"); pct < floor {
		t.Errorf("RED: Store.Load coverage %.1f%% < %.1f%% — add tests for the read-error WARN path "+
			"(path is a directory) and the valid-JSON-but-nil-benches arm. Baseline 76.9%%.", pct, floor)
	}
}

func TestC321_005_ExistingContractSuiteGreen(t *testing.T) {
	combined, code := runGoTest(t, "-run", "TestStore|TestCooldown|TestBench|TestNewBenchEntry|TestParseResetHint",
		"-count=1", clihealthPkg)
	if code != 0 {
		t.Errorf("RED: the existing clihealth contract suite fails (exit=%d) — the new I/O error-path "+
			"tests must not break the bench/cooldown/parse contract:\n%s", code, tail(combined, 30))
	}
}

func TestC321_006_BenchOnReadOnlyRootReturnsError(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("running as root bypasses directory permissions; read-only-dir negative test is meaningless")
	}
	root := t.TempDir()
	if err := os.Chmod(root, 0o500); err != nil {
		t.Fatalf("chmod read-only root: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(root, 0o755) })

	s := clihealth.NewStore(root, nil)
	err := s.Bench(clihealth.Entry{Family: "codex", Reason: "rate_limit"})
	if err == nil {
		t.Errorf("RED: Bench on a read-only root must return a non-nil error (write must not " +
			"swallow the os.MkdirAll failure)")
	}
}
