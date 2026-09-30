//go:build acs

package cycle1101

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

const personaBudgetTestName = "TestPersonaLineBudgetFixture"

const promptsPkgMarker = "internal/prompts"

func fixtureRepo(t *testing.T, promptsRed bool) (root, baseSHA string) {
	t.Helper()
	root = t.TempDir()

	body := "func " + personaBudgetTestName + "(t *testing.T) {\n"
	if promptsRed {
		body += "\tt.Errorf(\"combined persona line count = 812, want < 751 (pre-dedupe baseline)\")\n"
	}
	body += "}\n"

	files := map[string]string{
		"agents/evolve-scout.md":             "# Evolve Scout\n\n## STOP CRITERION\n\nbaseline persona doc.\n",
		"docs/notes.md":                      "# Notes\n\nnon-persona documentation.\n",
		"go/go.mod":                          "module fixture\n\ngo 1.22\n",
		"go/internal/prompts/budget_test.go": "package prompts\n\nimport \"testing\"\n\n" + body,
	}
	for rel, content := range files {
		abs := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
			t.Fatalf("mkdir for %s: %v", rel, err)
		}
		if err := os.WriteFile(abs, []byte(content), 0o644); err != nil {
			t.Fatalf("write %s: %v", rel, err)
		}
	}

	git(t, root, "init", "-q")
	git(t, root, "add", "-A")
	git(t, root, "-c", "user.email=acs@evolve.local", "-c", "user.name=acs", "commit", "-q", "-m", "fixture base")
	baseSHA = strings.TrimSpace(gitOut(t, root, "rev-parse", "HEAD"))
	if baseSHA == "" {
		t.Fatalf("fixture base SHA is empty")
	}
	return root, baseSHA
}

func git(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
}

func gitOut(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git %s: %v", strings.Join(args, " "), err)
	}
	return string(out)
}

func dirty(t *testing.T, root, rel, extra string) {
	t.Helper()
	abs := filepath.Join(root, rel)
	data, err := os.ReadFile(abs)
	if err != nil {
		t.Fatalf("read %s: %v", rel, err)
	}
	if err := os.WriteFile(abs, append(data, []byte(extra)...), 0o644); err != nil {
		t.Fatalf("write %s: %v", rel, err)
	}
}

func runFloor(t *testing.T, root, baseSHA string) []string {
	t.Helper()
	return core.DefaultBuildFloorChecks(context.Background(), core.ReviewInput{
		Phase:           "build",
		Worktree:        root,
		WorktreeBaseSHA: baseSHA,
	})
}

func TestC1101_001_PersonaBudgetBreachRejectedAtHandoff(t *testing.T) {
	root, base := fixtureRepo(t, true)
	dirty(t, root, "agents/evolve-scout.md", "\nextra persona lines that blow the budget.\n")

	failures := runFloor(t, root, base)
	if len(failures) == 0 {
		t.Fatalf("DefaultBuildFloorChecks returned NO failures for a lane touching agents/evolve-scout.md with go/internal/prompts RED — the persona line budget breach reaches main's CI unblocked (the cycle-1101 in-lane gate gap)")
	}
	joined := strings.Join(failures, "\n")
	if !strings.Contains(joined, promptsPkgMarker) {
		t.Errorf("build-floor rejected but no failure names %q — the operator cannot tell which gate fired; got:\n%s", promptsPkgMarker, joined)
	}
}

func TestC1101_002_NonPersonaLaneUnaffected(t *testing.T) {
	root, base := fixtureRepo(t, true)
	dirty(t, root, "docs/notes.md", "\nan unrelated documentation edit.\n")

	failures := runFloor(t, root, base)
	if len(failures) != 0 {
		t.Errorf("DefaultBuildFloorChecks returned %d failure(s) for a lane touching only docs/notes.md — the persona gate must not fire for lanes that never touch agents/evolve-*.md (fail-open floor policy); got:\n%s", len(failures), strings.Join(failures, "\n"))
	}
}

func TestC1101_003_PersonaTouchWithGreenBudgetApproves(t *testing.T) {
	root, base := fixtureRepo(t, false)
	dirty(t, root, "agents/evolve-scout.md", "\na small, within-budget persona edit.\n")

	failures := runFloor(t, root, base)
	if len(failures) != 0 {
		t.Errorf("DefaultBuildFloorChecks returned %d failure(s) for a persona edit whose go/internal/prompts budget test is GREEN — the gate must assert on the TEST OUTCOME, not on the mere presence of an agents/evolve-*.md path; got:\n%s", len(failures), strings.Join(failures, "\n"))
	}
}
