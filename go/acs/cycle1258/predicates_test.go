//go:build acs

package cycle1258

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/flagregistry"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
	"gopkg.in/yaml.v3"
)

const evalRelPath = ".evolve/evals/artifact-ready-crosspoll-debounce.md"

const bridgePkg = "./internal/bridge"

const subprocessBudget = 4 * time.Minute

func goTestDir(t *testing.T) string {
	t.Helper()
	dir := filepath.Join(acsassert.RepoRoot(t), "go")
	if _, err := os.Stat(filepath.Join(dir, "go.mod")); err != nil {
		t.Fatalf("worktree go module not found at %s: %v", dir, err)
	}
	return dir
}

func runNarrowGoTest(t *testing.T, pattern string) (string, bool) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), subprocessBudget)
	defer cancel()

	cmd := exec.CommandContext(ctx, "go", "test", "-count=1", "-run", pattern, bridgePkg)
	cmd.Dir = goTestDir(t)
	cmd.WaitDelay = 10 * time.Second
	out, err := cmd.CombinedOutput()
	return string(out), err == nil
}

func assertNamedTestsRan(t *testing.T, out string, want ...string) {
	t.Helper()
	if strings.Contains(out, "no tests to run") || strings.Contains(out, "no test files") {
		t.Errorf("go test matched NO tests — the contract it enforces has been renamed or deleted:\n%s", out)
		return
	}
	for _, name := range want {
		if !strings.Contains(out, name) {
			t.Errorf("test %s did not appear in the run output — it was renamed or removed, "+
				"so this predicate is no longer enforcing anything:\n%s", name, out)
		}
	}
}

func verboseRun(t *testing.T, pattern string) (string, bool) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), subprocessBudget)
	defer cancel()

	cmd := exec.CommandContext(ctx, "go", "test", "-count=1", "-v", "-run", pattern, bridgePkg)
	cmd.Dir = goTestDir(t)
	cmd.WaitDelay = 10 * time.Second
	out, err := cmd.CombinedOutput()
	return string(out), err == nil
}

func TestC1258_001_ArtifactDebounceStabilityWindow(t *testing.T) {
	const pattern = `^TestArtifactDetector_(ReadyOnlyAfterCrossPollStability|NotReadyWhileArtifactStillGrowing|NotReadyOnSameSizeRewrite)$`
	out, ok := verboseRun(t, pattern)
	assertNamedTestsRan(t, out,
		"TestArtifactDetector_ReadyOnlyAfterCrossPollStability",
		"TestArtifactDetector_NotReadyWhileArtifactStillGrowing",
		"TestArtifactDetector_NotReadyOnSameSizeRewrite",
	)
	if !ok {
		t.Errorf("the artifact cross-poll stability window is not honoured by artifactDetector.poll — "+
			"a deliverable is accepted while it is still being written (cycle-1198):\n%s", out)
	}
}

func TestC1258_002_ArtifactDebounceWiredIntoWaitLoop(t *testing.T) {
	const pattern = `^TestRunTmuxREPL_ArtifactDebounce(WiredIntoWaitLoop|HermeticUnderAmbientFleetEnv)$`
	out, ok := verboseRun(t, pattern)
	assertNamedTestsRan(t, out,
		"TestRunTmuxREPL_ArtifactDebounceWiredIntoWaitLoop",
		"TestRunTmuxREPL_ArtifactDebounceHermeticUnderAmbientFleetEnv",
	)
	if !ok {
		t.Errorf("the cross-poll debounce is NOT reached from the production wait loop "+
			"(runTmuxREPL), or the driver fixtures read the ambient process environment again "+
			"(the cycle-1252/1254 green-locally/red-in-gate defect):\n%s", out)
	}
}

func TestC1258_003_DebounceCostsNoFalseTimeouts(t *testing.T) {
	const pattern = `^(TestArtifactDetector_Poll|TestArtifactDetector_CtxCancelledShortCircuitsDebounce|TestArtifactStableTicks_IsAMeaningfulWindow)$`
	out, ok := verboseRun(t, pattern)
	assertNamedTestsRan(t, out,
		"TestArtifactDetector_Poll",
		"TestArtifactDetector_CtxCancelledShortCircuitsDebounce",
		"TestArtifactStableTicks_IsAMeaningfulWindow",
	)
	if !ok {
		t.Errorf("the debounce regressed the no-false-timeout contract, or artifactStableTicks is "+
			"no longer a real window (< 2 observations is not a debounce; > 3 underruns the "+
			"short-ArtifactTimeoutS fixtures at ~2s per tick):\n%s", out)
	}
}

var dialSubstrings = []string{
	"ARTIFACT_STABLE",
	"STABLE_TICKS",
	"STABLE_POLL",
	"ARTIFACT_DEBOUNCE",
	"DEBOUNCE",
	"ARTIFACT_SETTLE",
}

func TestC1258_004_StabilityWindowIsNotConfigurable(t *testing.T) {
	for _, f := range flagregistry.All {
		up := strings.ToUpper(f.Name)
		for _, frag := range dialSubstrings {
			if strings.Contains(up, frag) {
				t.Errorf("flag %q (status=%s) registers an artifact-stability dial — the cross-poll "+
					"window must stay a compiled constant (no_feature_flags; readGraceWindow's "+
					"\"deliberately NOT configurable\" convention)", f.Name, f.Status)
			}
		}
	}

	completion := filepath.Join(acsassert.RepoRoot(t), "go", "internal", "bridge", "completion.go")
	if !acsassert.FileMatchesRegex(t, completion, `(?m)^const artifactStableTicks = \d+$`) {
		t.Error("artifactStableTicks is no longer a package-level compiled const in completion.go — " +
			"a var or a Config field is a dial by another name")
	}
	if !acsassert.FileNotContains(t, completion, "os.Getenv") {
		t.Error("completion.go now reads os.Getenv — the completion contract must be selected by " +
			"newCompletionDetector's mode argument, never by ambient environment")
	}
}

type scoreCapEntry struct {
	Criterion    string `yaml:"criterion"`
	MaxIfMissing int    `yaml:"max_if_missing"`
	Evidence     string `yaml:"evidence"`
}

type evalFrontmatter struct {
	ScoreCap []scoreCapEntry `yaml:"score_cap"`
}

func parseFrontmatter(t *testing.T, raw string) evalFrontmatter {
	t.Helper()
	if !strings.HasPrefix(raw, "---\n") {
		t.Fatalf("%s does not open with a `---` YAML frontmatter block", evalRelPath)
	}
	end := strings.Index(raw[4:], "\n---")
	if end < 0 {
		t.Fatalf("%s has an unterminated YAML frontmatter block", evalRelPath)
	}
	var fm evalFrontmatter
	if err := yaml.Unmarshal([]byte(raw[4:4+end]), &fm); err != nil {
		t.Fatalf("%s frontmatter is not valid YAML: %v", evalRelPath, err)
	}
	return fm
}

func TestC1258_005_PermanentEvalEntryExistsAndItsEvidenceRuns(t *testing.T) {
	root := acsassert.RepoRoot(t)
	path := filepath.Join(root, evalRelPath)
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("permanent eval entry %s is absent: %v — the cross-poll debounce has been salvaged "+
			"across four cycles with no durable regression entry; nothing caps a future audit when "+
			"artifactDetector loses its stability window again", evalRelPath, err)
	}
	fm := parseFrontmatter(t, string(raw))
	if len(fm.ScoreCap) == 0 {
		t.Fatalf("%s declares no score_cap entries — an eval with no cap enforces nothing", evalRelPath)
	}

	for i, e := range fm.ScoreCap {
		if strings.TrimSpace(e.Criterion) == "" {
			t.Errorf("score_cap[%d] has an empty criterion", i)
		}
		if e.MaxIfMissing < 1 || e.MaxIfMissing > 10 {
			t.Errorf("score_cap[%d] max_if_missing = %d, want an integer in 1..10", i, e.MaxIfMissing)
		}
		if strings.TrimSpace(e.Evidence) == "" {
			t.Errorf("score_cap[%d] declares no evidence command", i)
			continue
		}

		ctx, cancel := context.WithTimeout(context.Background(), subprocessBudget)
		cmd := exec.CommandContext(ctx, "sh", "-c", e.Evidence)
		cmd.Dir = root
		cmd.WaitDelay = 10 * time.Second
		out, runErr := cmd.CombinedOutput()
		cancel()
		if runErr != nil {
			t.Errorf("score_cap[%d] evidence %q did not exit 0: %v — an eval whose evidence cannot "+
				"run caps nothing, forever:\n%s", i, e.Evidence, runErr, string(out))
		}
	}

	if !strings.Contains(string(raw), "1198") {
		t.Errorf("%s does not cite the source incident (cycle-1198, the truncated deliverable "+
			"accepted on first sight) — an eval without its provenance gets deleted by the next "+
			"person who tidies up", evalRelPath)
	}
}
