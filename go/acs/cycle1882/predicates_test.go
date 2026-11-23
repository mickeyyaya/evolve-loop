//go:build acs

package cycle1882

import (
	"encoding/json"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

var decisionSurfaceFiles = []string{"classifier.go", "latest.go", "complete.go", "lineage.go", "newestwins.go"}

func modelqueryDir(t *testing.T) string {
	t.Helper()
	return filepath.Join(acsassert.RepoRoot(t), "go", "internal", "modelquery")
}

func overlayOf(t *testing.T, dir, file string, mutate func(string) string) string {
	t.Helper()
	original := filepath.Join(dir, file)
	src, err := os.ReadFile(original)
	if err != nil {
		t.Fatalf("read %s: %v", original, err)
	}
	mutated := mutate(string(src))
	if mutated == string(src) {
		t.Fatalf("mutation left %s unchanged", file)
	}
	replacement := filepath.Join(t.TempDir(), file)
	if err := os.WriteFile(replacement, []byte(mutated), 0o644); err != nil {
		t.Fatal(err)
	}
	spec, err := json.Marshal(map[string]map[string]string{"Replace": {original: replacement}})
	if err != nil {
		t.Fatal(err)
	}
	overlay := filepath.Join(filepath.Dir(replacement), "overlay.json")
	if err := os.WriteFile(overlay, spec, 0o644); err != nil {
		t.Fatal(err)
	}
	return overlay
}

func bridgeDir(t *testing.T) string {
	t.Helper()
	return filepath.Join(acsassert.RepoRoot(t), "go", "internal", "bridge")
}

func runModelqueryTest(t *testing.T, run string, env []string, extra ...string) (string, bool) {
	t.Helper()
	return runPackageTest(t, "./internal/modelquery", run, env, extra...)
}

func runPackageTest(t *testing.T, pkg, run string, env []string, extra ...string) (string, bool) {
	t.Helper()
	args := append([]string{"test", "-count=1", "-run", run, "-v"}, extra...)
	args = append(args, pkg)
	cmd := exec.Command("go", args...)
	cmd.Dir = filepath.Join(acsassert.RepoRoot(t), "go")
	cmd.Env = append(os.Environ(), env...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		if _, isExit := err.(*exec.ExitError); !isExit {
			t.Fatalf("go test did not run: %v\n%s", err, out)
		}
	}
	return string(out), err == nil
}

var (
	pinBinaryOnce sync.Once
	pinBinary     string
	pinBinaryErr  error
	pinBinaryOut  []byte
)

func modelqueryTestBinary(t *testing.T) string {
	t.Helper()
	pinBinaryOnce.Do(func() {
		dir, err := os.MkdirTemp("", "modelquery-pin-acs-")
		if err != nil {
			pinBinaryErr = err
			return
		}
		pinBinary = filepath.Join(dir, "modelquery.test")
		cmd := exec.Command("go", "test", "-c", "-o", pinBinary, "./internal/modelquery")
		cmd.Dir = filepath.Join(acsassert.RepoRoot(t), "go")
		pinBinaryOut, pinBinaryErr = cmd.CombinedOutput()
	})
	if pinBinaryErr != nil {
		t.Fatalf("build modelquery test binary: %v\n%s", pinBinaryErr, pinBinaryOut)
	}
	return pinBinary
}

func runPinTestOnEditedCopy(t *testing.T, file string, mutate func(string) string) (string, bool) {
	t.Helper()
	binary := modelqueryTestBinary(t)
	copyDir := t.TempDir()
	for _, f := range decisionSurfaceFiles {
		src, err := os.ReadFile(filepath.Join(modelqueryDir(t), f))
		if err != nil {
			t.Fatalf("read %s: %v", f, err)
		}
		content := string(src)
		if f == file {
			content = mutate(content)
		}
		if err := os.WriteFile(filepath.Join(copyDir, f), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	cmd := exec.Command(binary, "-test.run", "^TestDecisionVersion_PinnedToAlgorithmSurface$", "-test.v", "-test.count=1")
	cmd.Dir = copyDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		if _, isExit := err.(*exec.ExitError); !isExit {
			t.Fatalf("pin test binary did not run: %v\n%s", err, out)
		}
	}
	if !strings.Contains(string(out), "TestDecisionVersion_PinnedToAlgorithmSurface") {
		t.Fatalf("pin test did not run:\n%s", out)
	}
	return string(out), err == nil
}

func appendComment(src string) string {
	return src + "\n// a comment-only edit must not move the decision-surface pin\n"
}

func addCodeDeclaration(src string) string {
	return src + "\nvar pinProbeCodeEdit = 0\n"
}

func TestC1882_001_CommentOnlyEditLeavesPinUnchanged(t *testing.T) {
	for _, file := range decisionSurfaceFiles {
		t.Run(file, func(t *testing.T) {
			out, passed := runPinTestOnEditedCopy(t, file, appendComment)
			if !passed {
				t.Errorf("a comment-only edit to %s moved the decision-surface pin:\n%s", file, out)
			}
		})
	}
}

func TestC1882_002_CodeEditChangesPin(t *testing.T) {
	for _, file := range decisionSurfaceFiles {
		t.Run(file, func(t *testing.T) {
			out, passed := runPinTestOnEditedCopy(t, file, addCodeDeclaration)
			if passed {
				t.Errorf("a code edit to %s left the decision-surface pin unchanged:\n%s", file, out)
			}
		})
	}
}

func TestC1882_003_UneditedSurfaceMatchesPin(t *testing.T) {
	out, passed := runModelqueryTest(t, "^TestDecisionVersion_", nil)
	if !passed {
		t.Errorf("decision-version tests fail on the unedited tree:\n%s", out)
	}
}

func TestC1882_004_DecisionSurfaceFilesCarryNoComments(t *testing.T) {
	for _, file := range decisionSurfaceFiles {
		path := filepath.Join(modelqueryDir(t), file)
		parsed, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ParseComments)
		if err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		for _, group := range parsed.Comments {
			if group == parsed.Doc {
				continue
			}
			t.Errorf("%s still carries a comment: %q", file, strings.TrimSpace(group.Text()))
			break
		}
	}
}

func TestC1882_005_CommentReadingOllamaTestRetired(t *testing.T) {
	cmd := exec.Command("go", "test", "-list", ".", "./internal/bridge")
	cmd.Dir = filepath.Join(acsassert.RepoRoot(t), "go")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go test -list: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), ollamaListTest) {
		t.Fatalf("behaviour pin %s is gone:\n%s", ollamaListTest, out)
	}
	if strings.Contains(string(out), "TestOllamaListMetadataExceptionDocumented") {
		t.Errorf("TestOllamaListMetadataExceptionDocumented still reads source text")
	}
}

const ollamaListTest = "TestModelLister_OllamaFixtureGivesTheModelIDs"

func dropListedModels(src string) string {
	return strings.Replace(src, "ids = append(ids, fields[0])", "_ = fields", 1)
}

func TestC1882_006_OllamaListTestCatchesLostListingWhenOllamaSucceeds(t *testing.T) {
	overlay := overlayOf(t, bridgeDir(t), "model_formats.go", dropListedModels)
	out, passed := runPackageTest(t, "./internal/bridge", "^"+ollamaListTest+"$", nil, "-overlay", overlay)
	if !strings.Contains(out, ollamaListTest) {
		t.Fatalf("ollama list test did not run:\n%s", out)
	}
	if passed {
		t.Errorf("%s passed while the parser dropped every listed model:\n%s", ollamaListTest, out)
	}
}

func TestC1882_007_OllamaListTestPassesOnTheHealthyParser(t *testing.T) {
	out, passed := runPackageTest(t, "./internal/bridge", "^"+ollamaListTest+"$", nil)
	if !passed {
		t.Errorf("ollama list test fails on the real parser:\n%s", out)
	}
}

func TestC1882_008_CatalogDocDropsAgyFromRecipePicker(t *testing.T) {
	// acs-predicate: config-check
	doc := filepath.Join(acsassert.RepoRoot(t), "docs", "architecture", "model-discovery-and-catalog.md")
	src, err := os.ReadFile(doc)
	if err != nil {
		t.Fatalf("read %s: %v", doc, err)
	}
	for _, line := range strings.Split(string(src), "\n") {
		if strings.Contains(line, "recipe engine") && strings.Contains(line, "agy") {
			t.Errorf("doc still lists agy under the tmux recipe picker: %q", line)
		}
	}
}

func TestC1882_009_ModelqueryPackagePasses(t *testing.T) {
	cmd := exec.Command("go", "test", "-count=1", "./internal/modelquery")
	cmd.Dir = filepath.Join(acsassert.RepoRoot(t), "go")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Errorf("go test ./internal/modelquery: %v\n%s", err, out)
	}
}
