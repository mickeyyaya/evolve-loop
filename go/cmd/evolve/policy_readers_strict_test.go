package main

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/quotareset"
)

func strictPolicyRoot(t *testing.T, body string) string {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".evolve"), 0o755); err != nil {
		t.Fatal(err)
	}
	writePolicy(t, root, body)
	return root
}

func TestKBRootsAbs_KeepsAnAbsolutePolicyPathAndJoinsARelativeOne(t *testing.T) {
	t.Parallel()
	abs := filepath.Join(t.TempDir(), "kb")
	root := strictPolicyRoot(t, `{"paths": {"kb_search_paths": "`+abs+`:rel/kb"}}`)

	got := kbRootsAbs(root)

	if want := []string{abs, filepath.Join(root, "rel/kb")}; !reflect.DeepEqual(got, want) {
		t.Fatalf("kbRootsAbs = %v, want %v", got, want)
	}
}

func TestPolicyAllowedFamilies_ReadsTheCatalogBlock(t *testing.T) {
	t.Parallel()
	root := strictPolicyRoot(t, `{"catalog": {"allowed_families": {"agy": ["gemini"]}}}`)

	if got := policyAllowedFamilies(root); !reflect.DeepEqual(got, map[string][]string{"agy": {"gemini"}}) {
		t.Fatalf("allowed families = %v", got)
	}
}

func TestPolicyAllowedFamilies_ARefusedPolicyAllowsNoFamilyFilter(t *testing.T) {
	t.Parallel()
	root := strictPolicyRoot(t, `{"catalog": {"allowed_families": {"agy": ["gemini"]}, "alowed": 1}}`)

	if got := policyAllowedFamilies(root); got != nil {
		t.Fatalf("a refused policy applies no catalog block, got %v", got)
	}
}

func TestRunQuotaReset_ARefusedPolicyIsLoudAndTheEstimateUsesTheDefaults(t *testing.T) {
	root := strictPolicyRoot(t, `{"quota_reset": {"reset_at": "2099-01-15T00:00:00Z"}, "fleet": {"cuont": 3}}`)
	t.Setenv("EVOLVE_PROJECT_ROOT", root)
	var stdout, stderr bytes.Buffer

	rc := runQuotaReset([]string{t.TempDir()}, nil, &stdout, &stderr)

	if rc != 0 || !strings.Contains(stderr.String(), `"fleet.cuont"`) || strings.Contains(stdout.String(), "2099-01-15") {
		t.Fatalf("rc %d stdout %q stderr %q: want 0, a WARN naming the key, and no policy reset_at", rc, stdout.String(), stderr.String())
	}
}

func TestRunQuotaReset_UsesThePolicyResetAt(t *testing.T) {
	root := strictPolicyRoot(t, `{"quota_reset": {"reset_at": "2031-01-02T00:00:00Z"}}`)
	t.Setenv("EVOLVE_PROJECT_ROOT", root)
	var stdout, stderr bytes.Buffer

	rc := runQuotaReset([]string{t.TempDir(), "extra"}, nil, &stdout, &stderr)

	if rc != 0 || !strings.Contains(stdout.String(), "2031-01-02") || stderr.Len() != 0 {
		t.Fatalf("rc %d stdout %q stderr %q: want the policy reset_at", rc, stdout.String(), stderr.String())
	}
}

func TestRunQuotaReset_HelpAndAComputeFailure(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if rc := runQuotaReset([]string{"--help"}, nil, &stdout, &stderr); rc != 0 || !strings.Contains(stdout.String(), "Usage:") {
		t.Fatalf("--help: rc %d stdout %q", rc, stdout.String())
	}
	orig := computeQuotaResetFn
	t.Cleanup(func() { computeQuotaResetFn = orig })
	computeQuotaResetFn = func(string, quotareset.Options) (quotareset.Result, error) {
		return quotareset.Result{}, errors.New("no clock")
	}
	t.Setenv("EVOLVE_PROJECT_ROOT", t.TempDir())

	if rc := runQuotaReset(nil, nil, &stdout, &stderr); rc != 1 || !strings.Contains(stderr.String(), "FATAL: no clock") {
		t.Fatalf("a compute failure exits 1 with FATAL: rc %d stderr %q", rc, stderr.String())
	}
}

func TestRepairFixabilityStage_IsAStrictStageKeyWithTheDefaultShadow(t *testing.T) {
	t.Parallel()
	for body, want := range map[string]config.Stage{
		`{}`: config.StageShadow,
		`{"workflow": {"repair": {"fixability": {"stage": "enforce"}}}}`: config.StageEnforce,
		`{"workflow": {"repair": {"fixability": {"stage": "off"}}}}`:     config.StageOff,
	} {
		var warn strings.Builder
		got := policy.RepairFixabilityStageFor(strictPolicyRoot(t, body), "strict", &warn)
		if warn.Len() != 0 || got != want {
			t.Errorf("%s: stage = %v (%q); want %v", body, got, warn.String(), want)
		}
	}
	for _, body := range []string{
		`{"workflow": {"repair": {"fixability": {"stage": "advisory"}}}}`,
		`{"workflow": {"repair": {"fixability": {"stage": "shadow", "stgae": 1}}}}`,
	} {
		var warn strings.Builder
		if got := policy.RepairFixabilityStageFor(strictPolicyRoot(t, body), "strict", &warn); got != config.StageShadow || !strings.Contains(warn.String(), "workflow.repair.fixability") {
			t.Errorf("%s: stage %v, warn %q; want shadow and a refusal that names workflow.repair.fixability", body, got, warn.String())
		}
	}
}

func TestCycleSpend_EachKeyIsStrictInTheLoadedPolicy(t *testing.T) {
	t.Parallel()
	for _, body := range []string{
		`{"workflow": {"cycle_spend": {"max_llm_minutes": 601}}}`,
		`{"workflow": {"cycle_spend": {"max_audit_rounds": 0}}}`,
		`{"workflow": {"cycle_spend": {"stage": "advisory"}}}`,
		`{"workflow": {"cycle_spend": {"max_minutes": 90}}}`,
	} {
		root := strictPolicyRoot(t, body)
		if _, err := policy.Load(filepath.Join(root, ".evolve", "policy.json")); err == nil || !strings.Contains(err.Error(), "cycle_spend") {
			t.Errorf("%s: err = %v, want a refusal that names workflow.cycle_spend", body, err)
		}
	}
	p, err := policy.Load(filepath.Join(strictPolicyRoot(t, `{"workflow": {"cycle_spend": {"stage": "enforce", "max_llm_minutes": 90}}}`), ".evolve", "policy.json"))
	if err != nil || p.WorkflowConfig().CycleSpend.Stage != config.StageEnforce || p.WorkflowConfig().CycleSpend.Limits.MaxLLMMinutes != 90 {
		t.Errorf("load = %+v, %v", p.WorkflowConfig().CycleSpend, err)
	}
}
