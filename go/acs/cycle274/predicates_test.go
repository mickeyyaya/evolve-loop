//go:build acs

package cycle274

import (
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

var (
	bridgeOnce sync.Once
	bridgeOut  string
	coreOnce   sync.Once
	coreOut    string
)

func goDir(t *testing.T) string {
	t.Helper()
	return filepath.Join(acsassert.RepoRoot(t), "go")
}

func runBridgeSuite(t *testing.T) string {
	t.Helper()
	dir := goDir(t)
	bridgeOnce.Do(func() {
		stdout, stderr, _, _ := acsassert.SubprocessOutput(
			"go", "test", "-C", dir, "-count=1", "-v", "./internal/bridge/")
		bridgeOut = stdout + "\n" + stderr
	})
	return bridgeOut
}

func runCoreSuite(t *testing.T) string {
	t.Helper()
	dir := goDir(t)
	coreOnce.Do(func() {
		stdout, stderr, _, _ := acsassert.SubprocessOutput(
			"go", "test", "-C", dir, "-count=1", "-v",
			"-run", "TestIsLegitimateMainTreePath|TestGuardCatchesInsertedPhaseLeak|TestGuardIgnoresLegitimateWorkspaceWrite",
			"./internal/core/")
		coreOut = stdout + "\n" + stderr
	})
	return coreOut
}

var (
	topPassRe  = regexp.MustCompile(`(?m)^--- PASS: (Test\w+)`)
	anyFailRe  = regexp.MustCompile(`(?m)^\s*--- FAIL:`)
	covTotalRe = regexp.MustCompile(`(?m)^total:\s+\(statements\)\s+([0-9.]+)%`)
)

func topLevelPassed(out, name string) bool {
	for _, m := range topPassRe.FindAllStringSubmatch(out, -1) {
		if m[1] == name {
			return true
		}
	}
	return false
}

func subPasses(out, parent string) map[string]bool {
	re := regexp.MustCompile(`(?m)^\s+--- PASS: ` + regexp.QuoteMeta(parent) + `/(\S+)`)
	seen := map[string]bool{}
	for _, m := range re.FindAllStringSubmatch(out, -1) {
		seen[m[1]] = true
	}
	return seen
}

// acs-predicate: structural-absence — this is NOT a magic-string presence grep
func TestC274_001_NoCLINameTransportLeaks(t *testing.T) {
	root := acsassert.RepoRoot(t)
	stdout, _, _, _ := acsassert.SubprocessOutput(
		"git", "-C", root, "grep", "-nE", `HasSuffix\([^)]*-tmux`,
		"--", "go/internal/swarm", "go/internal/adapters", "go/internal/looppreflight",
		":(exclude)*_test.go")
	if leaks := nonEmptyLines(stdout); len(leaks) > 0 {
		t.Errorf("RED: %d CLI-name transport leak site(s) remain (must route through bridge.IsTmuxDriver):\n%s",
			len(leaks), stdout)
	}
	defOut, _, _, _ := acsassert.SubprocessOutput(
		"git", "-C", root, "grep", "-n", "func isTmuxFamilyCLI",
		"--", "go/internal/adapters")
	if strings.TrimSpace(defOut) != "" {
		t.Errorf("RED: isTmuxFamilyCLI still defined — delete it and route through bridge.IsTmuxDriver:\n%s", defOut)
	}
}

// acs-predicate: config-check — asserts the new `transport` field exists in the
func TestC274_002_ManifestsDeclareTransport(t *testing.T) {
	root := acsassert.RepoRoot(t)
	dir := filepath.Join(root, "go", "internal", "bridge", "manifests")
	tmuxManifests := []string{"agy-tmux", "claude-tmux", "codex-tmux", "ollama-tmux"}
	headlessManifests := []string{"agy", "claude-p", "codex"}
	for _, name := range tmuxManifests {
		p := filepath.Join(dir, name+".json")
		if !acsassert.FileMatchesRegex(t, p, `"transport"\s*:\s*"tmux"`) {
			t.Errorf("RED: %s.json must declare \"transport\": \"tmux\" (single-source transport data)", name)
		}
	}
	for _, name := range headlessManifests {
		p := filepath.Join(dir, name+".json")
		if !acsassert.FileMatchesRegex(t, p, `"transport"\s*:\s*"headless"`) {
			t.Errorf("RED: %s.json must declare \"transport\": \"headless\" (single-source transport data)", name)
		}
	}
}

func TestC274_003_IsTmuxDriverBehavior(t *testing.T) {
	out := runBridgeSuite(t)
	if anyFailRe.MatchString(out) {
		t.Errorf("RED/REGRESSION: bridge suite has a FAIL line:\n%s", out)
	}
	if !topLevelPassed(out, "TestIsTmuxDriver") {
		t.Errorf("RED: TestIsTmuxDriver did not run+PASS — bridge.IsTmuxDriver(cli) is not implemented/tested")
	}
	if subs := subPasses(out, "TestIsTmuxDriver"); len(subs) < 3 {
		t.Errorf("RED: TestIsTmuxDriver has %d passing sub-cases, want >= 3 "+
			"(a *-tmux positive, a headless negative, and the empty/unknown->false edge — R3)", len(subs))
	}
	if !topLevelPassed(out, "TestManifestIsTmux") {
		t.Errorf("RED: TestManifestIsTmux did not run+PASS — Manifest.IsTmux() projection is not implemented/tested")
	}
}

func TestC274_004_LegitMainTreePathClassifierExtracted(t *testing.T) {
	out := runCoreSuite(t)
	if anyFailRe.MatchString(out) {
		t.Errorf("RED/REGRESSION: core guard suite has a FAIL line:\n%s", out)
	}
	if !topLevelPassed(out, "TestIsLegitimateMainTreePath") {
		t.Errorf("RED: TestIsLegitimateMainTreePath did not run+PASS — the shared legitimate-path classifier (R9) is not extracted/tested")
	}
	if subs := subPasses(out, "TestIsLegitimateMainTreePath"); len(subs) < 2 {
		t.Errorf("RED: TestIsLegitimateMainTreePath has %d passing sub-cases, want >= 2 "+
			"(a legit `.evolve/**`/build-artifact path AND a real source path that must NOT be legit)", len(subs))
	}
}

func TestC274_005_GuardCatchesInsertedUntrackedLeak(t *testing.T) {
	out := runCoreSuite(t)
	if !topLevelPassed(out, "TestGuardCatchesInsertedPhaseLeak") {
		t.Errorf("RED: TestGuardCatchesInsertedPhaseLeak did not run+PASS — an inserted/non-worktree phase's untracked main-tree leak still slips past the guard (cycle-270 defect, R5+R6)")
	}
	if subs := subPasses(out, "TestGuardCatchesInsertedPhaseLeak"); len(subs) < 2 {
		t.Errorf("RED: TestGuardCatchesInsertedPhaseLeak has %d passing sub-cases, want >= 2 "+
			"(an untracked-file leak detected [R6] AND an inserted-phase-identity leak detected [R5])", len(subs))
	}
}

func TestC274_006_GuardIgnoresLegitimateWorkspaceWrite(t *testing.T) {
	out := runCoreSuite(t)
	if !topLevelPassed(out, "TestGuardIgnoresLegitimateWorkspaceWrite") {
		t.Errorf("RED: TestGuardIgnoresLegitimateWorkspaceWrite did not run+PASS — the every-boundary guard must NOT false-fire on legitimate `.evolve/` workspace writes (R7)")
	}
}

func TestC274_007_BridgeCoverageAtLeast95(t *testing.T) {
	dir := goDir(t)
	prof := filepath.Join(t.TempDir(), "cover.out")
	_, stderr, _, _ := acsassert.SubprocessOutput(
		"go", "test", "-C", dir, "-short", "-count=1",
		"-coverprofile="+prof, "./internal/bridge/...")
	funcOut, funcErr, _, _ := acsassert.SubprocessOutput("go", "tool", "cover", "-func="+prof)
	cov := parseCoverTotal(funcOut)
	if cov < 0 {
		t.Fatalf("RED: no `total: (statements) N%%` line — coverage profile not produced.\ntest stderr:\n%s\ncover stderr:\n%s\ncover stdout:\n%s",
			stderr, funcErr, funcOut)
	}
	if cov < 95.0 {
		t.Errorf("RED: go/internal/bridge total coverage = %.1f%%, want >= 95.0%% (baseline 94.5%%)", cov)
	}
}

func nonEmptyLines(s string) []string {
	var out []string
	for _, ln := range strings.Split(s, "\n") {
		if strings.TrimSpace(ln) != "" {
			out = append(out, ln)
		}
	}
	return out
}

func parseCoverTotal(out string) float64 {
	m := covTotalRe.FindStringSubmatch(out)
	if m == nil {
		return -1
	}
	v, err := strconv.ParseFloat(m[1], 64)
	if err != nil {
		return -1
	}
	return v
}
