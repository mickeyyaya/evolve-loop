//go:build acs

package cycle1751

import (
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/sizeratchet"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const cmdEvolvePkg = "github.com/mickeyyaya/evolve-loop/go/cmd/evolve"

func moduleRoot(t *testing.T) string {
	t.Helper()
	return filepath.Join(acsassert.RepoRoot(t), "go")
}

func runNamedGoTestFamily(t *testing.T, pattern string, names []string) {
	t.Helper()
	cmd := exec.Command("go", "test", "-count=1", "-v", "-run", pattern, cmdEvolvePkg)
	cmd.Dir = moduleRoot(t)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go test -run %s %s failed: %v\n%s", pattern, cmdEvolvePkg, err, out)
	}
	output := string(out)
	for _, name := range names {
		if !strings.Contains(output, "--- PASS: "+name) {
			t.Errorf("required subtest did not pass: %s\n%s", name, output)
		}
	}
}

func spanLines(t *testing.T, root, key string) (int, bool) {
	t.Helper()
	spans, err := sizeratchet.Walk(root)
	if err != nil {
		t.Fatalf("sizeratchet.Walk(%s): %v", root, err)
	}
	found := false
	lines := 0
	for _, s := range spans {
		if s.Key == key && s.Lines > lines {
			lines = s.Lines
			found = true
		}
	}
	return lines, found
}

func TestC1751_001_ParseLoopArgsShrunkToAllowance(t *testing.T) {
	root := moduleRoot(t)
	lines, found := spanLines(t, root, "cmd/evolve.parseLoopArgs")
	if !found {
		t.Fatalf("cmd/evolve.parseLoopArgs not found under %s", root)
	}
	if lines > sizeratchet.MaxLines {
		t.Errorf("cmd/evolve.parseLoopArgs is %d lines > %d: shrink it via behavior-preserving extraction", lines, sizeratchet.MaxLines)
	}
}

func TestC1751_002_DefaultMatrixDepsShrunkToAllowance(t *testing.T) {
	root := moduleRoot(t)
	lines, found := spanLines(t, root, "cmd/evolve.defaultMatrixDeps")
	if !found {
		t.Fatalf("cmd/evolve.defaultMatrixDeps not found under %s", root)
	}
	if lines > sizeratchet.MaxLines {
		t.Errorf("cmd/evolve.defaultMatrixDeps is %d lines > %d: shrink it via behavior-preserving extraction", lines, sizeratchet.MaxLines)
	}
}

func TestC1751_003_DetectQuotaPauseShrunkToAllowance(t *testing.T) {
	root := moduleRoot(t)
	lines, found := spanLines(t, root, "cmd/evolve.detectQuotaPause")
	if !found {
		t.Fatalf("cmd/evolve.detectQuotaPause not found under %s", root)
	}
	if lines > sizeratchet.MaxLines {
		t.Errorf("cmd/evolve.detectQuotaPause is %d lines > %d: shrink it via behavior-preserving extraction", lines, sizeratchet.MaxLines)
	}
}

func offendersLack(t *testing.T, key string) bool {
	t.Helper()
	offenders, err := sizeratchet.LoadOffenders(filepath.Join(moduleRoot(t), "internal/sizeratchet/offenders.json"))
	if err != nil {
		t.Fatalf("LoadOffenders: %v", err)
	}
	_, listed := offenders[key]
	return !listed
}

func TestC1751_004_OffendersJSONDropsParseLoopArgs(t *testing.T) {
	if !offendersLack(t, "cmd/evolve.parseLoopArgs") {
		t.Errorf("offenders.json still lists cmd/evolve.parseLoopArgs: entry must be removed once the function fits the ratchet, not resized")
	}
}

func TestC1751_005_OffendersJSONDropsDefaultMatrixDeps(t *testing.T) {
	if !offendersLack(t, "cmd/evolve.defaultMatrixDeps") {
		t.Errorf("offenders.json still lists cmd/evolve.defaultMatrixDeps: entry must be removed once the function fits the ratchet, not resized")
	}
}

func TestC1751_006_OffendersJSONDropsDetectQuotaPause(t *testing.T) {
	if !offendersLack(t, "cmd/evolve.detectQuotaPause") {
		t.Errorf("offenders.json still lists cmd/evolve.detectQuotaPause: entry must be removed once the function fits the ratchet, not resized")
	}
}

func TestC1751_007_ParseLoopArgsTestFamilyPasses(t *testing.T) {
	runNamedGoTestFamily(t, "^TestParseLoopArgs_", []string{
		"TestParseLoopArgs_GoalSources",
		"TestParseLoopArgs_PositionalCyclesStrategy",
		"TestParseLoopArgs_StrategyValidation",
		"TestParseLoopArgs_FlagPrecedence",
		"TestParseLoopArgs_DryRun",
		"TestParseLoopArgs_BudgetFlagsAreNoOps",
		"TestParseLoopArgs_BudgetFlagWarnsRemoved",
		"TestParseLoopArgs_BudgetAliasAccepted",
		"TestParseLoopArgs_NegativeBudgetAccepted",
		"TestParseLoopArgs_LegacyPositionalIntegerWarn",
		"TestParseLoopArgs_ExplicitCyclesNoWarn",
		"TestParseLoopArgs_PerAgentCLIFlag",
		"TestParseLoopArgs_PerAgentModelFlag",
		"TestParseLoopArgs_MalformedCLIFlagRejected",
		"TestParseLoopArgs_ProjectRootResolvedAbsolute",
		"TestParseLoopArgs_UntilInboxEmpty",
		"TestParseLoopArgs_FlagParseError",
		"TestParseLoopArgs_MaxCyclesFlag",
		"TestParseLoopArgs_MaxCyclesExplicit",
	})
}

func TestC1751_008_DefaultMatrixDepsTestFamilyPasses(t *testing.T) {
	runNamedGoTestFamily(t, "^TestVerifyReleaseCLIMatrix_", []string{
		"TestVerifyReleaseCLIMatrix_AllPass",
		"TestVerifyReleaseCLIMatrix_MissingSubcommand",
		"TestVerifyReleaseCLIMatrix_OneCLIFails",
		"TestVerifyReleaseCLIMatrix_AllFailVisible",
	})
}

func TestC1751_009_DetectQuotaPauseTestFamilyPasses(t *testing.T) {
	runNamedGoTestFamily(t, "^TestDetectQuotaPause_", []string{
		"TestDetectQuotaPause_EmitsNonEmptyWakeAtAndSource",
		"TestDetectQuotaPause_EmptySourceReadsAsUnknown",
		"TestDetectQuotaPause_HappyPath",
		"TestDetectQuotaPause_NotFlagged",
		"TestDetectQuotaPause_FallbackFields",
	})
}

func TestC1751_010_BuildAndVetClean(t *testing.T) {
	root := moduleRoot(t)
	build := exec.Command("go", "build", "./...")
	build.Dir = root
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("go build ./... failed: %v\n%s", err, out)
	}
	vet := exec.Command("go", "vet", "./cmd/evolve/...")
	vet.Dir = root
	if out, err := vet.CombinedOutput(); err != nil {
		t.Errorf("go vet ./cmd/evolve/... failed: %v\n%s", err, out)
	}
}
