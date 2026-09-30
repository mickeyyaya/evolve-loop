//go:build acs

package cycle476

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const corePkg = "github.com/mickeyyaya/evolve-loop/go/internal/core"
const runnerPkg = "github.com/mickeyyaya/evolve-loop/go/internal/phases/runner"

func runGoTest(t *testing.T, runFilter string, race bool, pkgs ...string) (out string, code int) {
	t.Helper()
	args := []string{"test", "-count=1", "-v"}
	if race {
		args = append(args, "-race")
	}
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

func TestC476_001_RealPersonaComposedPromptCarriesTierAndCLI(t *testing.T) {
	out, code := runGoTest(t, "TestComposePlanPrompt_RealPersonaExistingExampleCarriesTierAndCLI", true, corePkg)
	requireTestsRan(t, out, 1)
	if code != 0 {
		t.Errorf("real-persona composed-prompt liveness golden is red (exit=%d) — a bare existing-phase example survives in agents/evolve-router.md, competing with the Go {cli,tier} example\n%s", code, out)
	}
}

func TestC476_002_PersonaFrontmatterAndDegradeContract(t *testing.T) {
	out, code := runGoTest(t,
		"TestRealPersonaFrontmatterOutputFormatEnumeratesTierCLI|TestParsePhasePlan_AbsentCLITierFieldsStayEmpty|TestSanitizeAdvisorTier_RejectsHighTopAndRawModel",
		true, corePkg)
	requireTestsRan(t, out, 3)
	if code != 0 {
		t.Errorf("frontmatter-enumeration + degrade + tier-confinement contract is red (exit=%d)\n%s", code, out)
	}
}

func TestC476_003_OverlayLogGoldensRemainGreen(t *testing.T) {
	out, code := runGoTest(t, "TestRunner_AdvisorOverlayAppliedLogsLine|TestRunner_NoAdvisorOverlayLogsProfileDefault", true, runnerPkg)
	requireTestsRan(t, out, 2)
	if code != 0 {
		t.Errorf("overlay-log goldens are red (exit=%d) — the locked observability shapes changed\n%s", code, out)
	}
}

func TestC476_004_CIParityCoreRaceVetApicover(t *testing.T) {
	out, code := runGoTest(t, "", true, corePkg)
	if code != 0 {
		t.Errorf("full-package -race regression on internal/core is red (exit=%d)\n%s", code, out)
	}
	root := acsassert.RepoRoot(t)
	goDir := filepath.Join(root, "go")
	vetOut, _, vetCode, _ := acsassert.SubprocessOutput("bash", "-c", "cd "+goDir+" && go vet ./internal/core/...")
	if vetCode != 0 {
		t.Errorf("go vet ./internal/core/... is red (exit=%d)\n%s", vetCode, vetOut)
	}
	apicoverCmd := "cd " + goDir + " && " +
		"go build -o bin/apicover ./cmd/apicover && " +
		"go test -coverprofile=coverage.core476.txt ./internal/core/ >/dev/null && " +
		"go tool cover -func=coverage.core476.txt > coverage.core476.func.txt && " +
		"bin/apicover -enforce -cover coverage.core476.func.txt $(go list -f '{{.Dir}}' ./internal/core)"
	apiOut, _, apiCode, _ := acsassert.SubprocessOutput("bash", "-c", apicoverCmd)
	if apiCode != 0 {
		t.Errorf("apicover -enforce over internal/core is red (exit=%d)\n%s", apiCode, apiOut)
	}
}
