//go:build acs

package cycle1145

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/evalgate"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
	"github.com/mickeyyaya/evolve-loop/go/internal/subagent"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const scoutArtifactLiteral = `"scout-report.md"`

const registryDeclSite = "internal/phasecontract/contract_registry.go"

func TestC1145_001_ScoutReportNameDeclaredOnlyInRegistry(t *testing.T) {
	root := acsassert.RepoRoot(t)
	internalDir := filepath.Join(root, "go", "internal")

	var offenders []string
	err := filepath.Walk(internalDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		rel, relErr := filepath.Rel(filepath.Join(root, "go"), path)
		if relErr != nil {
			return relErr
		}
		if filepath.ToSlash(rel) == registryDeclSite {
			return nil
		}
		body, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		if strings.Contains(string(body), scoutArtifactLiteral) {
			offenders = append(offenders, filepath.ToSlash(rel))
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walking %s: %v", internalDir, err)
	}

	if len(offenders) > 0 {
		t.Errorf("the scout artifact filename %s is independently declared in %d production file(s) outside the phasecontract SSOT: %s — route each through phasecontract.For(\"scout\").ArtifactName",
			scoutArtifactLiteral, len(offenders), strings.Join(offenders, ", "))
	}
}

func TestC1145_002_EvalGateResolvesScoutReportByRegistryName(t *testing.T) {
	contract, ok := phasecontract.For("scout")
	if !ok {
		t.Fatal("phasecontract has no registered contract for phase \"scout\"")
	}

	const unmaterialized = "cycle1145-deliberately-unmaterialized-slug"
	report := "# Scout Report\n\n## Selected Tasks\n\n### Task 1: " + unmaterialized +
		"\n- **Slug:** " + unmaterialized + "\n- **Complexity:** S\n"

	review := func(t *testing.T, filename string) core.ReviewResult {
		t.Helper()
		workspace := t.TempDir()
		projectRoot := t.TempDir()
		if err := os.WriteFile(filepath.Join(workspace, filename), []byte(report), 0o644); err != nil {
			t.Fatalf("writing %s: %v", filename, err)
		}
		return evalgate.NewReviewer(config.StageEnforce).Review(context.Background(), core.ReviewInput{
			Phase:       string(core.PhaseScout),
			Workspace:   workspace,
			ProjectRoot: projectRoot,
		})
	}

	got := review(t, contract.ArtifactName)
	if got.Approve {
		t.Errorf("evalgate approved a scout phase whose report (%s, the registry ArtifactName) selects unmaterialized slug %q — the gate did not read the registry-named artifact",
			contract.ArtifactName, unmaterialized)
	} else if !strings.Contains(got.Reason, unmaterialized) {
		t.Errorf("evalgate blocked but its reason does not name the offending slug %q: %q", unmaterialized, got.Reason)
	}

	if other := review(t, "scout-report-cycle1145-not-the-contract.md"); !other.Approve {
		t.Errorf("evalgate blocked on a report that is NOT at the contracted artifact name (reason=%q) — the gate is matching something other than the registry filename",
			other.Reason)
	}
}

func TestC1145_003_ScoutContractDeclaresRuntimeTruthName(t *testing.T) {
	contract, ok := phasecontract.For("scout")
	if !ok {
		t.Fatal("phasecontract has no registered contract for phase \"scout\"")
	}
	if contract.ArtifactName != "scout-report.md" {
		t.Errorf("phasecontract.For(\"scout\").ArtifactName = %q, want \"scout-report.md\" (the filename the scout phase actually writes)", contract.ArtifactName)
	}

	var found bool
	for _, a := range phasecontract.RequiredArtifacts() {
		if a == contract.ArtifactName {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("phasecontract.RequiredArtifacts() = %v does not include the scout artifact %q — the completeness half of the SSOT was dropped",
			phasecontract.RequiredArtifacts(), contract.ArtifactName)
	}
}

func rejectedAsUnknownAgent(t *testing.T, role string) bool {
	t.Helper()
	_, err := subagent.DispatchParallel(context.Background(), subagent.DispatchParallelRequest{
		Agent: role,
		Cycle: -1,
	}, subagent.DispatchParallelOptions{})
	if err == nil {
		t.Fatalf("DispatchParallel(%q, cycle=-1) returned no error; expected at least the cycle-range rejection", role)
	}
	return strings.Contains(err.Error(), "unknown agent")
}

func TestC1145_004_DispatchAllowListCoversEveryRegisteredAgent(t *testing.T) {
	var missing []string
	for _, c := range phasecontract.Contracts() {
		if c.NoArtifact || c.AgentName == "" {
			continue
		}
		if rejectedAsUnknownAgent(t, c.AgentName) {
			missing = append(missing, c.Phase+"→"+c.AgentName)
		}
	}
	if len(missing) > 0 {
		t.Errorf("subagent dispatch rejects %d agent(s) the phasecontract registry declares: %s — derive the allow-list from the registry instead of re-typing it",
			len(missing), strings.Join(missing, ", "))
	}
}

func TestC1145_005_DispatchAllowListRetainsNonRegistryRoles(t *testing.T) {
	nonRegistry := []string{"inspirer", "evaluator", "plan-reviewer", "memo", "tester"}
	for _, role := range nonRegistry {
		if rejectedAsUnknownAgent(t, role) {
			t.Errorf("dispatch role %q is no longer accepted — it has a profile and no phasecontract entry, so the registry-derived allow-list dropped it", role)
		}
	}
}

func TestC1145_006_DispatchRejectsNonDispatchableAndUnknownRoles(t *testing.T) {
	root := acsassert.RepoRoot(t)
	shipProfile := filepath.Join(root, ".evolve", "profiles", "ship.json")
	if _, err := os.Stat(shipProfile); err == nil {
		t.Fatalf("premise broken: %s now exists, so \"ship\" may legitimately be dispatchable — revisit this predicate", shipProfile)
	}

	for _, role := range []string{"ship", "Scout", "not-a-real-role", ""} {
		if !rejectedAsUnknownAgent(t, role) {
			t.Errorf("subagent dispatch accepted role %q — the allow-list over-reached beyond profile-backed, dispatchable roles", role)
		}
	}
}
