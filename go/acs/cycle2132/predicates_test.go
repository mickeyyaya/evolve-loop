//go:build acs

package cycle2132

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const runtimeReference = "docs/operations/runtime-reference.md"

var (
	buildOnce  sync.Once
	builtPath  string
	buildError error
)

func evolveBinary(t *testing.T) string {
	t.Helper()
	buildOnce.Do(func() {
		moduleDir, err := acsassert.ModuleDir(filepath.Join(acsassert.RepoRoot(t), "go"))
		if err != nil {
			buildError = err
			return
		}
		dir, err := os.MkdirTemp("", "evolve-cycle2132-")
		if err != nil {
			buildError = err
			return
		}
		builtPath = filepath.Join(dir, "evolve")
		cmd := exec.Command("go", "build", "-o", builtPath, "./cmd/evolve")
		cmd.Dir = moduleDir
		if out, err := cmd.CombinedOutput(); err != nil {
			buildError = &buildFailure{tail: acsassert.OutputTail(string(out), 15)}
		}
	})
	if buildError != nil {
		t.Fatalf("building the evolve binary: %v", buildError)
	}
	return builtPath
}

type buildFailure struct{ tail string }

func (b *buildFailure) Error() string { return b.tail }

func writeLLMCalls(t *testing.T, projectRoot, cycle string, lines ...string) {
	t.Helper()
	ws := filepath.Join(projectRoot, ".evolve", "runs", "cycle-"+cycle)
	if err := os.MkdirAll(ws, 0o755); err != nil {
		t.Fatal(err)
	}
	body := strings.Join(lines, "\n") + "\n"
	if err := os.WriteFile(filepath.Join(ws, "llm-calls.ndjson"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

const (
	scoutCallServed = `{"ts":"2026-10-20T00:00:00Z","agent":"scout","phase":"scout","cli":"agy-tmux","model":"balanced","requested_model":"balanced","dispatched_model":"gemini-3.8-flash-high","dispatch_source":"argv","attempt":1,"served_model":"Gemini 3.8 Flash (High)"}`
	buildCallServed = `{"ts":"2026-10-20T00:01:00Z","agent":"build","phase":"build","cli":"agy-tmux","model":"deep","requested_model":"deep","dispatched_model":"gemini-4-argon-medium","dispatch_source":"argv","attempt":1,"served_model":"Gemini 4 Argon (Medium)"}`
	auditCallBare   = `{"ts":"2026-10-20T00:02:00Z","agent":"audit","phase":"audit","cli":"agy-tmux","model":"fast","requested_model":"fast","dispatched_model":"gemini-3.8-flash-low","dispatch_source":"argv","attempt":1}`
)

func runEvolve(t *testing.T, args ...string) (string, string, int) {
	t.Helper()
	stdout, stderr, code, err := acsassert.SubprocessOutput(evolveBinary(t), args...)
	if err != nil && code == 0 {
		t.Fatalf("running evolve %v: %v", args, err)
	}
	return stdout, stderr, code
}

func rowFor(out, phase string) string {
	for _, line := range strings.Split(out, "\n") {
		fields := strings.Fields(line)
		if len(fields) > 0 && fields[0] == phase {
			return line
		}
	}
	return ""
}

func TestC2132_001_CycleModelsPrintsTierSelectorAndServedModelPerPhase(t *testing.T) {
	root := t.TempDir()
	writeLLMCalls(t, root, "7", scoutCallServed, buildCallServed)
	out, stderr, code := runEvolve(t, "cycle", "models", "--project-root", root, "--cycle", "7")
	if code != 0 {
		t.Fatalf("exit=%d stderr=%s", code, stderr)
	}
	scout, build := rowFor(out, "scout"), rowFor(out, "build")
	for _, want := range []string{"balanced", "gemini-3.8-flash-high", "Gemini 3.8 Flash (High)"} {
		if !strings.Contains(scout, want) {
			t.Errorf("scout row %q lacks %q", scout, want)
		}
	}
	for _, want := range []string{"deep", "gemini-4-argon-medium", "Gemini 4 Argon (Medium)"} {
		if !strings.Contains(build, want) {
			t.Errorf("build row %q lacks %q", build, want)
		}
	}
	if strings.Contains(scout, "deep") || strings.Contains(build, "balanced") {
		t.Errorf("rows leak into each other:\n%s", out)
	}
}

func TestC2132_002_ALaunchThroughAFakeDriverRecordsTheServedModelOnTheCallLine(t *testing.T) {
	moduleDir, err := acsassert.ModuleDir(filepath.Join(acsassert.RepoRoot(t), "go"))
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("go", "test", "-count=1", "-run",
		"^(TestTryRecord_ServedModelIsTheModelTheTranscriptReportsNotTheTier|TestTryRecord_ServedModelIsTheVerifiedFooterLabel)$",
		"./internal/bridge")
	cmd.Dir = moduleDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("served-model recording tests failed: %v\n%s", err, acsassert.OutputTail(string(out), 20))
	}
	if !strings.Contains(string(out), "ok") {
		t.Errorf("no ok line in go test output: %s", out)
	}
}

func TestC2132_003_RecordsWithoutAServingModelPrintUnreportedAndBadInputFailsLoudly(t *testing.T) {
	root := t.TempDir()
	writeLLMCalls(t, root, "8", scoutCallServed, auditCallBare)
	out, stderr, code := runEvolve(t, "cycle", "models", "--project-root", root, "--cycle", "8")
	if code != 0 {
		t.Fatalf("exit=%d stderr=%s", code, stderr)
	}
	if audit := rowFor(out, "audit"); !strings.HasSuffix(strings.TrimSpace(audit), "unreported") {
		t.Errorf("audit row %q must end with unreported", audit)
	}
	if scout := rowFor(out, "scout"); strings.Contains(scout, "unreported") {
		t.Errorf("scout row %q reported a served model and must not print unreported", scout)
	}

	if _, _, code := runEvolve(t, "cycle", "models", "--project-root", root, "--cycle", "999"); code != 1 {
		t.Errorf("a cycle without records exited %d, want 1", code)
	}
	if _, _, code := runEvolve(t, "cycle", "models", "--project-root", root, "--no-such-flag"); code != 10 {
		t.Errorf("an unknown flag exited %d, want 10", code)
	}
}

func documentedBullet(t *testing.T) string {
	t.Helper()
	path := filepath.Join(acsassert.RepoRoot(t), runtimeReference)
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var bullets []string
	for _, line := range strings.Split(string(raw), "\n") {
		if strings.Contains(line, "`evolve cycle models") {
			bullets = append(bullets, line)
		}
	}
	if len(bullets) != 1 {
		t.Fatalf("%s documents `evolve cycle models` on %d lines, want exactly 1", runtimeReference, len(bullets))
	}
	return bullets[0]
}

func TestC2132_004_RuntimeReferenceDocumentsEveryFlagAndExitCodeTheCommandHas(t *testing.T) {
	bullet := documentedBullet(t)
	helpOut, helpErr, _ := runEvolve(t, "cycle", "models", "-h")
	help := helpOut + helpErr
	flagPattern := regexp.MustCompile(`--[a-z][a-z-]*`)
	documented := flagPattern.FindAllString(bullet, -1)
	if len(documented) == 0 {
		t.Fatalf("the bullet names no flag: %s", bullet)
	}
	for _, flag := range documented {
		if !strings.Contains(help, flag) {
			t.Errorf("the bullet documents %s, which `evolve cycle models -h` does not offer", flag)
		}
	}
	for _, mustDocument := range []string{"--cycle", "--json"} {
		if !strings.Contains(bullet, mustDocument) {
			t.Errorf("the bullet omits the %s flag", mustDocument)
		}
	}
	if !strings.Contains(bullet, "unreported") {
		t.Errorf("the bullet omits the unreported marker")
	}
	root := t.TempDir()
	_, _, noRecords := runEvolve(t, "cycle", "models", "--project-root", root, "--cycle", "1")
	_, _, badArgs := runEvolve(t, "cycle", "models", "--project-root", root, "--no-such-flag")
	for _, code := range []int{noRecords, badArgs} {
		if !regexp.MustCompile(`\b` + strconv.Itoa(code) + `\b`).MatchString(bullet) {
			t.Errorf("the bullet omits the measured exit code %d", code)
		}
	}
}

func TestC2132_005_TheDocumentedBulletPassesStrictSteLintAndSitsBesideModelsPerformance(t *testing.T) {
	bullet := documentedBullet(t)
	dir := t.TempDir()
	file := filepath.Join(dir, "bullet.md")
	if err := os.WriteFile(file, []byte(bullet+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	out, stderr, code := runEvolve(t, "docs", "ste-lint", "--strict", "--project-root", filepath.Join(acsassert.RepoRoot(t)), file)
	if code != 0 {
		t.Errorf("ste-lint --strict exit=%d\n%s%s", code, out, stderr)
	}

	raw, err := os.ReadFile(filepath.Join(acsassert.RepoRoot(t), runtimeReference))
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(string(raw), "\n")
	cycleAt, perfAt := -1, -1
	for i, line := range lines {
		switch {
		case strings.Contains(line, "`evolve cycle models"):
			cycleAt = i
		case strings.Contains(line, "`evolve models performance"):
			perfAt = i
		}
	}
	if perfAt < 0 || cycleAt < 0 || cycleAt < perfAt-3 || cycleAt > perfAt+3 {
		t.Errorf("the cycle models bullet (line %d) must sit within 3 lines of the models performance bullet (line %d)", cycleAt+1, perfAt+1)
	}
}

const (
	prRefreshIntegrationTest = "go/cmd/evolve/cmd_pr_refresh_integration_test.go"
	researchIndex            = "docs/research-index.md"
)

func gitLines(t *testing.T, root string, args ...string) []string {
	t.Helper()
	out, stderr, code, err := acsassert.SubprocessOutput("git", append([]string{"-C", root}, args...)...)
	if err != nil || code != 0 {
		t.Fatalf("git %v: code=%d err=%v stderr=%s", args, code, err, stderr)
	}
	var lines []string
	for _, line := range strings.Split(out, "\n") {
		if strings.TrimSpace(line) != "" {
			lines = append(lines, line)
		}
	}
	return lines
}

func TestC2132_006_TheOutOfScopePRRefreshIntegrationTestIsUnchangedFromBase(t *testing.T) {
	root := acsassert.RepoRoot(t)
	if changed := gitLines(t, root, "diff", "HEAD", "--name-only", "--", prRefreshIntegrationTest); len(changed) != 0 {
		t.Errorf("%s differs from its base; the precondition was relaxed to pass on a stale base: %v", prRefreshIntegrationTest, changed)
	}
	if untracked := gitLines(t, root, "ls-files", "--others", "--exclude-standard", "--", prRefreshIntegrationTest); len(untracked) != 0 {
		t.Errorf("%s is untracked: %v", prRefreshIntegrationTest, untracked)
	}
}

func TestC2132_007_TheResearchIndexLosesNoRowAgainstBase(t *testing.T) {
	root := acsassert.RepoRoot(t)
	var removed []string
	for _, line := range gitLines(t, root, "diff", "HEAD", "--unified=0", "--", researchIndex) {
		if strings.HasPrefix(line, "-") && !strings.HasPrefix(line, "---") {
			removed = append(removed, line)
		}
	}
	if len(removed) != 0 {
		t.Errorf("%s drops %d line(s) present at base, for example %q", researchIndex, len(removed), removed[0])
	}
	linkPattern := regexp.MustCompile(`\]\((research/[^)\s]+\.md)\)`)
	current, err := os.ReadFile(filepath.Join(root, researchIndex))
	if err != nil {
		t.Fatal(err)
	}
	have := map[string]bool{}
	for _, m := range linkPattern.FindAllStringSubmatch(string(current), -1) {
		have[m[1]] = true
	}
	baseText, _, code, err := acsassert.SubprocessOutput("git", "-C", root, "show", "HEAD:"+researchIndex)
	if err != nil || code != 0 {
		t.Fatalf("reading the base index: code=%d err=%v", code, err)
	}
	for _, m := range linkPattern.FindAllStringSubmatch(baseText, -1) {
		if !have[m[1]] {
			t.Errorf("the index row for %s is gone", m[1])
		}
	}
}
