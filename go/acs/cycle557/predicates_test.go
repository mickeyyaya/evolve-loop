//go:build acs

package cycle557

import (
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

var baselineCoverageRe = regexp.MustCompile(`gathering baseline coverage: (\d+)/(\d+) completed`)

const (
	clihealthPkg = "github.com/mickeyyaya/evolve-loop/go/internal/clihealth"
	bridgePkg    = "github.com/mickeyyaya/evolve-loop/go/internal/bridge"
)

func requireListed(t *testing.T, pkg, funcName string) {
	t.Helper()
	stdout, stderr, code, err := acsassert.SubprocessOutput("go", "test", "-list", "^"+funcName+"$", pkg)
	if err != nil && code == 0 {
		t.Fatalf("go test -list failed to launch: %v\nstderr:\n%s", err, stderr)
	}
	if !strings.Contains(stdout, funcName) {
		t.Errorf("go test -list '^%s$' %s did not list %s (fuzz harness missing or renamed):\nstdout:\n%s\nstderr:\n%s",
			funcName, pkg, funcName, stdout, stderr)
	}
}

func requireFuzzGreen(t *testing.T, pkg, funcName string, minSeeds int) {
	t.Helper()
	pattern := "^" + funcName + "$"
	stdout, stderr, code, _ := acsassert.SubprocessOutput(
		"go", "test", "-run", pattern, "-fuzz", pattern, "-fuzztime=3s", "-v", pkg)
	out := stdout + "\n" + stderr

	matches := baselineCoverageRe.FindAllStringSubmatch(out, -1)
	if len(matches) == 0 {
		t.Errorf("no fuzz run detected for %s in %s (missing \"gathering baseline coverage\" line) — harness did not run:\n%s", funcName, pkg, out)
		return
	}
	total, _ := strconv.Atoi(matches[len(matches)-1][2])
	if total < minSeeds {
		t.Errorf("only %d corpus entrie(s) loaded for %s (need >= %d) — seed corpus missing or too small:\n%s",
			total, funcName, minSeeds, out)
		return
	}
	if code != 0 || strings.Contains(out, "--- FAIL") || strings.Contains(out, "FAIL\t") {
		t.Errorf("%s in %s failed a bounded fuzz run (exit=%d):\n%s", funcName, pkg, code, out)
	}
}

func TestC557_001_FuzzParseResetHintRegistered(t *testing.T) {
	requireListed(t, clihealthPkg, "FuzzParseResetHint")
}

func TestC557_002_FuzzParseResetHintGreenSeededFromGoldens(t *testing.T) {
	requireFuzzGreen(t, clihealthPkg, "FuzzParseResetHint", 15)
}

func TestC557_003_FuzzClassifyExhaustedRegistered(t *testing.T) {
	requireListed(t, bridgePkg, "FuzzClassifyExhausted")
}

func TestC557_004_FuzzClassifyExhaustedGreenSeededFromGoldens(t *testing.T) {
	requireFuzzGreen(t, bridgePkg, "FuzzClassifyExhausted", 8)
}
