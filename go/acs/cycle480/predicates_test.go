//go:build acs

package cycle480

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const routerPkg = "github.com/mickeyyaya/evolve-loop/go/internal/router"

func runGoTest(t *testing.T, runFilter string, pkgs ...string) (out string, code int) {
	t.Helper()
	args := []string{"test", "-count=1", "-race", "-v"}
	if runFilter != "" {
		args = append(args, "-run", runFilter)
	}
	args = append(args, pkgs...)
	stdout, stderr, code, _ := acsassert.SubprocessOutput("go", args...)
	return stdout + "\n" + stderr, code
}

func requireTestsRan(t *testing.T, out string, min int) {
	t.Helper()
	if strings.Contains(out, "no tests to run") {
		t.Errorf("no tests matched the -run filter (\"no tests to run\") — required tests are unwritten or renamed")
		return
	}
	if got := strings.Count(out, "=== RUN"); got < min {
		t.Errorf("only %d test(s) ran, need >= %d", got, min)
	}
}

func TestC480_001_NilEnvelopeFloorClampsUpUniversally(t *testing.T) {
	out, code := runGoTest(t,
		"TestClampPlanModelRouting_NilEnvelopeFloorClampsUp|TestClampPlanModelRouting_NilEnvelopeFloorAppliesAcrossPhases",
		routerPkg)
	requireTestsRan(t, out, 2)
	if code != 0 {
		t.Errorf("universal-envelope-floor clamp-up is red (exit=%d) — a nil-envelope profile still lets a below-floor tier through\n%s", code, out)
	}
}

func TestC480_002_EnvelopeFloorDoesNotOverClamp(t *testing.T) {
	out, code := runGoTest(t,
		"TestClampPlanModelRouting_ExplicitEnvelopeNotOverriddenByDefault|TestClampPlanModelRouting_NilEnvelopeWithinCeilingPassesThrough",
		routerPkg)
	requireTestsRan(t, out, 2)
	if code != 0 {
		t.Errorf("envelope-floor over-clamp guard is red (exit=%d) — the default floor either overrode an explicit envelope or clamped a within-ceiling tier\n%s", code, out)
	}
}

func TestC480_003_RouterCIParity(t *testing.T) {
	out, code := runGoTest(t, "", routerPkg)
	if code != 0 {
		t.Errorf("full-package -race regression on internal/router is red (exit=%d)\n%s", code, out)
	}
	root := acsassert.RepoRoot(t)
	goDir := filepath.Join(root, "go")
	vetOut, _, vetCode, _ := acsassert.SubprocessOutput("bash", "-c", "cd "+goDir+" && go vet ./internal/router/...")
	if vetCode != 0 {
		t.Errorf("go vet ./internal/router/... is red (exit=%d)\n%s", vetCode, vetOut)
	}
	apicoverCmd := "cd " + goDir + " && " +
		"go build -o bin/apicover ./cmd/apicover && " +
		"go test -coverprofile=coverage.router480.txt ./internal/router/ >/dev/null && " +
		"go tool cover -func=coverage.router480.txt > coverage.router480.func.txt && " +
		"bin/apicover -enforce -cover coverage.router480.func.txt $(go list -f '{{.Dir}}' ./internal/router)"
	apiOut, _, apiCode, _ := acsassert.SubprocessOutput("bash", "-c", apicoverCmd)
	if apiCode != 0 {
		t.Errorf("apicover -enforce over internal/router is red (exit=%d)\n%s", apiCode, apiOut)
	}
}
