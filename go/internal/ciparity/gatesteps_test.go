package ciparity

import (
	"bytes"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

const (
	wantJobUbuntu = "go / build + test (Go 1.23, ubuntu, every package and every gate)"
	wantJobDarwin = "go / build + test (Go 1.23, macOS, the darwin package set)"
)

func wantGateSteps() []GateStep {
	return []GateStep{
		{"make cover-changed", wantJobUbuntu, "changed functions at 100% (cover-changed; hard-fail)", 200},
		{"make apicover-check", wantJobUbuntu, "api-coverage enforce (apicover; per-package hard-fail, Phase 5)", 2000},
		{"make cover-strict", wantJobUbuntu, "line-coverage strict (go/.cover-strict; per-package hard-fail, ADR-0101)", 1200},
		{"make cover-changed-darwin", wantJobDarwin, "changed functions of the darwin set at 100% (cover-changed; hard-fail)", 200},
		{"make cover-strict-darwin", wantJobDarwin, "line-coverage strict of the darwin set (go/.cover-strict floors of go/.darwin-set packages; hard-fail)", 1200},
	}
}

func TestGateSteps_HoldTheFiveGatesWithTheirBounds(t *testing.T) {
	t.Parallel()
	want := wantGateSteps()
	if len(GateSteps) != len(want) {
		t.Fatalf("GateSteps has %d rows, want %d", len(GateSteps), len(want))
	}
	for i, w := range want {
		if GateSteps[i] != w {
			t.Errorf("GateSteps[%d] = %+v, want %+v", i, GateSteps[i], w)
		}
	}
}

func goCallerID(t *testing.T) string {
	t.Helper()
	for id, job := range readWorkflowContract(t, RequiredWorkflow).Jobs {
		if job.Uses == "./.github/workflows/go.yml" {
			return id
		}
	}
	t.Fatal("required.yml has no job that calls go.yml")
	return ""
}

func TestGateSteps_EachRowNamesARealJobAndStepOfGoYml(t *testing.T) {
	t.Parallel()
	caller := goCallerID(t)
	jobs := readWorkflowContract(t, "go.yml").Jobs
	for _, row := range GateSteps {
		found := false
		for _, job := range jobs {
			if caller+" / "+job.Name != row.Job {
				continue
			}
			for _, step := range job.Steps {
				if step.Name != row.Step {
					continue
				}
				found = true
				if !strings.Contains(step.Run, row.Gate) {
					t.Errorf("step %q runs %q, want it to hold %q", step.Name, step.Run, row.Gate)
				}
				if step.If != "" || step.ContinueOnError != "" {
					t.Errorf("step %q has if=%q continue-on-error=%q; a gate step must run only after the test step passed", step.Name, step.If, step.ContinueOnError)
				}
			}
		}
		if !found {
			t.Errorf("no step %q in a job displayed as %q", row.Step, row.Job)
		}
	}
}

func TestGateSteps_ApicoverBoundHoldsTheEnforcedPackageList(t *testing.T) {
	t.Parallel()
	raw, err := os.ReadFile(filepath.Join(workflowRepoRoot(t), "go", ".apicover-enforce"))
	if err != nil {
		t.Fatal(err)
	}
	pkgs := 0
	for _, line := range bytes.Split(raw, []byte("\n")) {
		if trimmed := bytes.TrimSpace(line); len(trimmed) > 0 && trimmed[0] != '#' {
			pkgs++
		}
	}
	for _, row := range GateSteps {
		if row.Gate != "make apicover-check" {
			continue
		}
		if need := 3*pkgs + 60; need > row.Bound {
			t.Errorf("apicover bound %d < %d (3 * %d packages + 60); raise it with the list", row.Bound, need, pkgs)
		}
		return
	}
	t.Fatal("no apicover-check row")
}

func TestGateSteps_NoRowDuplicatesAGateOrAStepWithinAJob(t *testing.T) {
	t.Parallel()
	seen := map[string]bool{}
	for _, row := range GateSteps {
		for _, key := range []string{"gate\x00" + row.Gate, "step\x00" + row.Job + "\x00" + row.Step} {
			if seen[key] {
				t.Errorf("duplicate %q", strconv.Quote(key))
			}
			seen[key] = true
		}
	}
}

func TestLookupGateStep_IsExactOnJobAndStep(t *testing.T) {
	t.Parallel()
	for _, w := range wantGateSteps() {
		if got, ok := LookupGateStep(w.Job, w.Step); !ok || got != w {
			t.Errorf("LookupGateStep(%q, %q) = %+v, %v", w.Job, w.Step, got, ok)
		}
	}
	w := wantGateSteps()[0]
	for _, miss := range [][2]string{{"release / " + w.Job[5:], w.Step}, {w.Job, "build"}, {"", ""}} {
		if got, ok := LookupGateStep(miss[0], miss[1]); ok {
			t.Errorf("LookupGateStep(%q, %q) = %+v, want a miss", miss[0], miss[1], got)
		}
	}
}
