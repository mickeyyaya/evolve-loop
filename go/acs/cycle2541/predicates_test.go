//go:build acs

package cycle2541

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/testhost"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	workflowRel   = ".github/workflows/go.yml"
	declaredTools = testhost.RequiredToolsEnv + ":"
	testJobCount  = 3
	missingTool   = "git"
	keptTool      = "tmux"
)

type recordingT struct {
	skipped bool
	fatal   bool
}

func (r *recordingT) Helper()      {}
func (r *recordingT) Skip(...any)  { r.skipped = true }
func (r *recordingT) Fatal(...any) { r.fatal = true }

func declaredToolLists(t *testing.T) []string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(acsassert.RepoRoot(t), workflowRel))
	if err != nil {
		t.Fatal(err)
	}
	var values []string
	for _, line := range strings.Split(string(raw), "\n") {
		trimmed := strings.TrimSpace(line)
		if value, ok := strings.CutPrefix(trimmed, declaredTools); ok {
			values = append(values, strings.Trim(strings.TrimSpace(value), `"'`))
		}
	}
	return values
}

func hostDeclaring(value string) testhost.Host {
	return testhost.Host{
		GOOS: "linux",
		Getenv: func(key string) string {
			if key == testhost.RequiredToolsEnv {
				return value
			}
			return ""
		},
		LookPath: func(string) (string, error) { return "", errors.New("not found") },
	}
}

func TestC2541_001_EachTestJobDeclaresGitRequired(t *testing.T) {
	lists := declaredToolLists(t)
	if len(lists) != testJobCount {
		t.Fatalf("%s carries %d %s lines, want %d", workflowRel, len(lists), testhost.RequiredToolsEnv, testJobCount)
	}
	for i, value := range lists {
		probe := &recordingT{}
		hostDeclaring(value).Tool(probe, missingTool)
		if probe.skipped || !probe.fatal {
			t.Errorf("line %d declares %q: a runner without %s skipped=%v fatal=%v, want a fatal failure", i+1, value, missingTool, probe.skipped, probe.fatal)
		}
	}
}

func TestC2541_002_EachDeclarationStaysWellFormedAndKeepsTmux(t *testing.T) {
	lists := declaredToolLists(t)
	if len(lists) != testJobCount {
		t.Fatalf("%s carries %d %s lines, want %d", workflowRel, len(lists), testhost.RequiredToolsEnv, testJobCount)
	}
	for i, value := range lists {
		var seen []string
		for _, entry := range strings.Split(value, ",") {
			tool := strings.TrimSpace(entry)
			if tool == "" {
				t.Errorf("line %d declares %q: an empty entry", i+1, value)
			}
			if slices.Contains(seen, tool) {
				t.Errorf("line %d declares %q: %s twice", i+1, value, tool)
			}
			seen = append(seen, tool)
		}
		if !slices.Contains(seen, keptTool) {
			t.Errorf("line %d declares %q: %s is no longer required", i+1, value, keptTool)
		}
		if !slices.Contains(seen, missingTool) {
			t.Errorf("line %d declares %q: %s is not required", i+1, value, missingTool)
		}
	}
}

func TestC2541_003_TheTmuxTierClaimOfTheWorkflowStillHolds(t *testing.T) {
	out, _, code, err := acsassert.SubprocessOutput("go", "-C", acsassert.GoModuleDir(t), "test", "-count=1", "-run", "TestEnvExclusive_TheCIJobsTheBridgeBackstopNamesRunTheTmuxTier", "./internal/ciparity")
	if err != nil || code != 0 {
		t.Fatalf("ciparity tier-scope claim exit=%d err=%v\n%s", code, err, acsassert.OutputTail(out, 20))
	}
}
