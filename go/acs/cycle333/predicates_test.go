//go:build acs

package cycle333

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/subagent"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

var subagentTargetFiles = []string{
	"go/internal/subagent/modeltier.go",
	"go/internal/subagent/ctxadvisory.go",
	"go/internal/subagent/dispatchparallel.go",
}

func prodPath(t *testing.T, rel string) string {
	t.Helper()
	return filepath.Join(acsassert.RepoRoot(t), rel)
}

func readProd(t *testing.T, rel string) string {
	t.Helper()
	b, err := os.ReadFile(prodPath(t, rel))
	if err != nil {
		t.Fatalf("RED: cannot read target file %s: %v", rel, err)
	}
	return string(b)
}

func assertNoDynamicCompile(t *testing.T, rel string) {
	t.Helper()
	if strings.Contains(readProd(t, rel), "regexp.MustCompile(fmt.Sprintf") {
		t.Errorf("RED: %s still calls regexp.MustCompile(fmt.Sprintf(...)) — hoist the pattern to a package-level var", rel)
	}
}

func TestC333_001_ResolveModelTierExtractsProfileStrings(t *testing.T) {
	resolve := func(profile string) (string, error) {
		return subagent.ResolveModelTier(
			subagent.ResolveModelTierRequest{ProfilePath: "in-memory"},
			subagent.ResolveModelTierOptions{
				ReadProfile: func(string) (string, error) { return profile, nil },
				ReadState:   func(string) (string, error) { return "", os.ErrNotExist },
			},
		)
	}

	if tier, err := resolve(`{"role":"auditor","model_tier_default":"sonnet"}`); err != nil || tier != "opus" {
		t.Errorf("role extraction: got (%q, %v), want (\"opus\", nil) — auditor role not extracted", tier, err)
	}

	if tier, err := resolve(`{"name":"auditor","model_tier_default":"sonnet"}`); err != nil || tier != "opus" {
		t.Errorf("name fallback: got (%q, %v), want (\"opus\", nil) — name not used when role absent", tier, err)
	}

	if tier, err := resolve(`{"role":"builder","model_tier_default":"haiku"}`); err != nil || tier != "haiku" {
		t.Errorf("model_tier_default extraction: got (%q, %v), want (\"haiku\", nil)", tier, err)
	}

	if tier, err := resolve(`{"role":"builder"}`); err == nil {
		t.Errorf("missing model_tier_default: got (%q, nil), want error", tier)
	}

	assertNoDynamicCompile(t, "go/internal/subagent/modeltier.go")
}

func TestC333_002_CheckCtxAdvisoryExtractsIntThreshold(t *testing.T) {
	write := func(body string) string {
		p := filepath.Join(t.TempDir(), "profile.json")
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatalf("write fixture: %v", err)
		}
		return p
	}

	if res, err := subagent.CheckCtxAdvisory(write(`{"context_clear_trigger_tokens":1000}`), 2000); err != nil || !res.Emit || res.Threshold != 1000 {
		t.Errorf("over threshold: got %+v err=%v, want Emit=true Threshold=1000", res, err)
	}

	if res, err := subagent.CheckCtxAdvisory(write(`{"context_clear_trigger_tokens":1000}`), 500); err != nil || res.Emit || res.Threshold != 1000 {
		t.Errorf("under threshold: got %+v err=%v, want Emit=false Threshold=1000", res, err)
	}

	if res, err := subagent.CheckCtxAdvisory(write(`{"role":"tester"}`), 2000); err != nil || res.Emit {
		t.Errorf("absent threshold: got %+v err=%v, want Emit=false", res, err)
	}

	assertNoDynamicCompile(t, "go/internal/subagent/ctxadvisory.go")
}

func TestC333_003_DispatchParallelExtractsBoolField(t *testing.T) {
	dispatch := func(profile string) error {
		_, err := subagent.DispatchParallel(
			context.Background(),
			subagent.DispatchParallelRequest{
				Agent:         "scout",
				Cycle:         0,
				WorkspacePath: t.TempDir(),
				ProfilesDir:   t.TempDir(),
			},
			subagent.DispatchParallelOptions{
				ReadProfile: func(string) (string, error) { return profile, nil },
			},
		)
		return err
	}

	if err := dispatch(`{"parallel_eligible":false}`); err == nil || !strings.Contains(err.Error(), "not parallel_eligible") {
		t.Errorf("parallel_eligible=false: got err=%v, want a \"not parallel_eligible\" refusal", err)
	}

	if err := dispatch(`{"cli":"claude"}`); err == nil || !strings.Contains(err.Error(), "not parallel_eligible") {
		t.Errorf("parallel_eligible absent: got err=%v, want a \"not parallel_eligible\" refusal", err)
	}

	if err := dispatch(`{"parallel_eligible":true}`); err == nil || !strings.Contains(err.Error(), "no parallel_subtasks") {
		t.Errorf("parallel_eligible=true: got err=%v, want it to clear eligibility and fail on \"no parallel_subtasks\"", err)
	}

	assertNoDynamicCompile(t, "go/internal/subagent/dispatchparallel.go")
}

// acs-predicate: config-check — this gate inherently asserts a SOURCE-STRUCTURE
func TestC333_004_SubagentRegexpHoisted(t *testing.T) {
	dynamicCompiles := 0
	staticVars := 0
	for _, rel := range subagentTargetFiles {
		body := readProd(t, rel)
		for _, line := range strings.Split(body, "\n") {
			if !strings.Contains(line, "regexp.MustCompile(") {
				continue
			}
			if strings.Contains(line, "fmt.Sprintf") {
				dynamicCompiles++
			} else {
				staticVars++
			}
		}
	}

	if dynamicCompiles != 0 {
		t.Errorf("RED: %d dynamic regexp.MustCompile(fmt.Sprintf(...)) calls remain across %v — want 0",
			dynamicCompiles, subagentTargetFiles)
	}

	const wantStaticAtLeast = 8
	if staticVars < wantStaticAtLeast {
		t.Errorf("RED: only %d static literal regexp.MustCompile vars across %v — want >= %d (the hoisted field patterns)",
			staticVars, subagentTargetFiles, wantStaticAtLeast)
	}
}
