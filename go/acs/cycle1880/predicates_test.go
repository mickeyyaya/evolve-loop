//go:build acs

package cycle1880

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/commentaudit"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	preCycleHead        = "26838170cd0a90a9de528b45d9ae6bfe10a4b71b"
	stallPromptRegex    = `Do you want to trust this workspace\?`
	stallResponseKeys   = "1,Enter"
	healthyOutputLine   = "Reading go/cmd/evolve/cmd_bridge.go (312 lines)"
	healthyPromptLine   = "Do you want to trust this workspace? (1) Yes (2) No"
	tooShortRegex       = `trust\?`
	unknownFlagMarker   = "--no-such-flag=1"
	promotedStageShadow = "stage: shadow"
)

var evolveBinary string

func TestMain(m *testing.M) {
	code := runWithBinary(m)
	os.Exit(code)
}

func runWithBinary(m *testing.M) int {
	dir, err := os.MkdirTemp("", "acs-cycle1880-bin-")
	if err != nil {
		os.Stderr.WriteString("mkdtemp: " + err.Error() + "\n")
		return 1
	}
	defer os.RemoveAll(dir)
	wd, err := os.Getwd()
	if err != nil {
		os.Stderr.WriteString("getwd: " + err.Error() + "\n")
		return 1
	}
	goModuleRoot := filepath.Clean(filepath.Join(wd, "..", ".."))
	evolveBinary = filepath.Join(dir, "evolve")
	build := exec.Command("go", "build", "-o", evolveBinary, "./cmd/evolve")
	build.Dir = goModuleRoot
	if out, err := build.CombinedOutput(); err != nil {
		os.Stderr.WriteString("build evolve: " + err.Error() + "\n" + string(out))
		return 1
	}
	return m.Run()
}

func writeCorpus(t *testing.T, lines ...string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "healthy-corpus.txt")
	if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func promoteRule(t *testing.T, args ...string) (string, int) {
	t.Helper()
	full := append([]string{"bridge", "promote-rule"}, args...)
	out, errOut, code, err := acsassert.SubprocessOutput(evolveBinary, full...)
	if err != nil && code < 0 {
		t.Fatalf("run evolve bridge promote-rule: %v", err)
	}
	return out + errOut, code
}

func yamlFiles(t *testing.T, dir string) []string {
	t.Helper()
	matches, err := filepath.Glob(filepath.Join(dir, "*.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	return matches
}

func assertSingleShadowRule(t *testing.T, rulesDir, output string) {
	t.Helper()
	files := yamlFiles(t, rulesDir)
	if len(files) != 1 {
		t.Fatalf("RED: want exactly one promoted rule file in %s, got %v\noutput:\n%s", rulesDir, files, output)
	}
	ruleFile := files[0]
	id := strings.TrimSuffix(filepath.Base(ruleFile), ".yaml")
	if !strings.Contains(output, id) {
		t.Errorf("RED: output does not name the promoted rule id %q:\n%s", id, output)
	}
	if !strings.Contains(output, ruleFile) {
		t.Errorf("RED: output does not name the promoted rule path %q:\n%s", ruleFile, output)
	}
	raw, err := os.ReadFile(ruleFile)
	if err != nil {
		t.Fatal(err)
	}
	body := string(raw)
	if !strings.Contains(body, promotedStageShadow) {
		t.Errorf("RED: promoted rule is not at stage shadow:\n%s", body)
	}
	if !strings.Contains(body, `Do you want to trust this workspace\\?`) {
		t.Errorf("RED: promoted rule does not carry the requested regex:\n%s", body)
	}
}

func TestC1880_001_PromoteRuleWritesShadowRuleWhenCorpusDoesNotMatch(t *testing.T) {
	rulesDir := t.TempDir()
	corpus := writeCorpus(t, healthyOutputLine)
	out, code := promoteRule(t,
		"--regex="+stallPromptRegex,
		"--response-keys="+stallResponseKeys,
		"--corpus="+corpus,
		"--note=learned from a trust-prompt stall",
		"--dir="+rulesDir,
	)
	if code != 0 {
		t.Fatalf("RED: promote-rule exit=%d, want 0:\n%s", code, out)
	}
	assertSingleShadowRule(t, rulesDir, out)
}

func TestC1880_002_PromoteRuleDefaultsToProjectInteractionRulesDir(t *testing.T) {
	projectRoot := t.TempDir()
	corpus := writeCorpus(t, healthyOutputLine)
	out, code := promoteRule(t,
		"--regex="+stallPromptRegex,
		"--response-keys="+stallResponseKeys,
		"--corpus="+corpus,
		"--project-root="+projectRoot,
	)
	if code != 0 {
		t.Fatalf("RED: promote-rule exit=%d, want 0:\n%s", code, out)
	}
	assertSingleShadowRule(t, filepath.Join(projectRoot, ".evolve", "instincts", "interaction-rules"), out)
}

func TestC1880_003_PromoteRuleRefusedByCorpusNamesTheLineAndWritesNothing(t *testing.T) {
	rulesDir := t.TempDir()
	corpus := writeCorpus(t, healthyOutputLine, healthyPromptLine)
	out, code := promoteRule(t,
		"--regex="+stallPromptRegex,
		"--response-keys="+stallResponseKeys,
		"--corpus="+corpus,
		"--dir="+rulesDir,
	)
	if code != 1 {
		t.Errorf("RED: corpus-matching regex exit=%d, want 1:\n%s", code, out)
	}
	if !strings.Contains(out, healthyPromptLine) {
		t.Errorf("RED: refusal does not name the refuting corpus line %q:\n%s", healthyPromptLine, out)
	}
	if files := yamlFiles(t, rulesDir); len(files) != 0 {
		t.Errorf("RED: a refused rule was written: %v", files)
	}
}

func TestC1880_004_PromoteRuleCorpusSkipsCommentAndBlankLines(t *testing.T) {
	rulesDir := t.TempDir()
	corpus := writeCorpus(t, "# "+healthyPromptLine, "", healthyOutputLine)
	out, code := promoteRule(t,
		"--regex="+stallPromptRegex,
		"--response-keys="+stallResponseKeys,
		"--corpus="+corpus,
		"--dir="+rulesDir,
	)
	if code != 0 {
		t.Fatalf("RED: a commented-out corpus line refused the rule, exit=%d, want 0:\n%s", code, out)
	}
	assertSingleShadowRule(t, rulesDir, out)
}

func TestC1880_005_PromoteRuleUsageErrorsExit10(t *testing.T) {
	corpus := writeCorpus(t, healthyOutputLine)
	cases := []struct {
		name string
		args []string
	}{
		{"no flags", nil},
		{"missing regex", []string{"--response-keys=" + stallResponseKeys, "--corpus=" + corpus}},
		{"missing response keys", []string{"--regex=" + stallPromptRegex, "--corpus=" + corpus}},
		{"missing corpus", []string{"--regex=" + stallPromptRegex, "--response-keys=" + stallResponseKeys}},
		{"unknown flag", []string{"--regex=" + stallPromptRegex, "--response-keys=" + stallResponseKeys, "--corpus=" + corpus, unknownFlagMarker}},
		{"pattern under the minimum length", []string{"--regex=" + tooShortRegex, "--response-keys=" + stallResponseKeys, "--corpus=" + corpus}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rulesDir := t.TempDir()
			out, code := promoteRule(t, append(tc.args, "--dir="+rulesDir)...)
			if code != 10 {
				t.Errorf("RED: exit=%d, want 10:\n%s", code, out)
			}
			if files := yamlFiles(t, rulesDir); len(files) != 0 {
				t.Errorf("RED: an invalid invocation wrote a rule: %v", files)
			}
		})
	}
}

func TestC1880_006_PromoteRuleListedInBridgeHelpAndDocumented(t *testing.T) {
	out, errOut, code, err := acsassert.SubprocessOutput(evolveBinary, "bridge", "help")
	if err != nil && code < 0 {
		t.Fatalf("run evolve bridge help: %v", err)
	}
	if code != 0 || !strings.Contains(out+errOut, "promote-rule") {
		t.Errorf("RED: evolve bridge help does not list promote-rule (exit=%d):\n%s%s", code, out, errOut)
	}
	root := acsassert.RepoRoot(t)
	reference := filepath.Join(root, "docs", "operations", "runtime-reference.md")
	acsassert.FileContains(t, reference, "evolve bridge promote-rule")
	acsassert.FileContains(t, reference, "evolve bridge add-rule")
}

func TestC1880_007_ChangedGoFilesAddNoComments(t *testing.T) {
	root := acsassert.RepoRoot(t)
	for _, rel := range []string{"go/cmd/evolve/cmd_bridge.go", "go/internal/bridge/interaction_rules.go"} {
		before, errOut, code, err := acsassert.SubprocessOutput("git", "-C", root, "show", preCycleHead+":"+rel)
		if err != nil || code != 0 {
			t.Fatalf("git show %s:%s exit=%d: %v %s", preCycleHead, rel, code, err, errOut)
		}
		after, err := os.ReadFile(filepath.Join(root, rel))
		if err != nil {
			t.Fatal(err)
		}
		if added := commentaudit.AddedComments([]byte(before), after); len(added) != 0 {
			t.Errorf("RED: %s adds comments: %q", rel, added)
		}
	}
}
