//go:build acs

package cycle422

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/prompts"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

func TestC422_001_IntentCompactionSavesBytes2200(t *testing.T) {
	root := acsassert.RepoRoot(t)
	data, err := os.ReadFile(filepath.Join(root, "agents", "evolve-intent.md"))
	if err != nil {
		t.Fatalf("read evolve-intent.md: %v", err)
	}
	_, body, err := prompts.ParseFrontmatter(string(data))
	if err != nil {
		t.Fatalf("parse frontmatter: %v", err)
	}
	stripped := prompts.StripOnDemandSections(body)
	saved := len(body) - len(stripped)
	if saved < 256 {
		t.Errorf("intent compaction saved only %d bytes (want ≥256: the Composition+Reference tail); ## Reference Index heading missing? (body=%d stripped=%d)",
			saved, len(body), len(stripped))
	}
}

func TestC422_002_IntentOutputContractAbsentAfterStrip_Negative(t *testing.T) {
	root := acsassert.RepoRoot(t)
	data, err := os.ReadFile(filepath.Join(root, "agents", "evolve-intent.md"))
	if err != nil {
		t.Fatalf("read evolve-intent.md: %v", err)
	}
	_, body, err := prompts.ParseFrontmatter(string(data))
	if err != nil {
		t.Fatalf("parse frontmatter: %v", err)
	}
	stripped := prompts.StripOnDemandSections(body)
	if !strings.Contains(stripped, "intent-delta.md") {
		t.Error("'intent-delta.md' output contract lost below ## Reference Index — it must survive into dispatched intent prompts")
	}
}

func TestC422_003_IntentRerunBehaviorAbsentAfterStrip_Negative(t *testing.T) {
	root := acsassert.RepoRoot(t)
	data, err := os.ReadFile(filepath.Join(root, "agents", "evolve-intent.md"))
	if err != nil {
		t.Fatalf("read evolve-intent.md: %v", err)
	}
	_, body, err := prompts.ParseFrontmatter(string(data))
	if err != nil {
		t.Fatalf("parse frontmatter: %v", err)
	}
	stripped := prompts.StripOnDemandSections(body)
	if !strings.Contains(stripped, "## Re-run behavior") {
		t.Error("'## Re-run behavior' lost below ## Reference Index — the re-run protocol must survive into dispatched intent prompts")
	}
}

func TestC422_004_IntentReflectionAuthoringAbsentAfterStrip_Negative(t *testing.T) {
	root := acsassert.RepoRoot(t)
	data, err := os.ReadFile(filepath.Join(root, "agents", "evolve-intent.md"))
	if err != nil {
		t.Fatalf("read evolve-intent.md: %v", err)
	}
	_, body, err := prompts.ParseFrontmatter(string(data))
	if err != nil {
		t.Fatalf("parse frontmatter: %v", err)
	}
	stripped := prompts.StripOnDemandSections(body)
	if !strings.Contains(stripped, "intent-reflection.yaml") {
		t.Error("'intent-reflection.yaml' authoring instruction lost below ## Reference Index — the sidecar duty must survive into dispatched intent prompts")
	}
}

func TestC422_005_IntentRequiredAnchorsAboveMarker(t *testing.T) {
	root := acsassert.RepoRoot(t)
	data, err := os.ReadFile(filepath.Join(root, "agents", "evolve-intent.md"))
	if err != nil {
		t.Fatalf("read evolve-intent.md: %v", err)
	}
	_, body, err := prompts.ParseFrontmatter(string(data))
	if err != nil {
		t.Fatalf("parse frontmatter: %v", err)
	}
	stripped := prompts.StripOnDemandSections(body)
	for _, anchor := range []string{
		"STOP CRITERION",
		"challenged_premise",
		"30-80 line",
		"EMERGENCY EXIT",
		"Prior FAIL audit",
	} {
		if !strings.Contains(stripped, anchor) {
			t.Errorf("required anchor %q lost below ## Reference Index — must remain above marker in evolve-intent.md (Builder: add brief inline mention if the anchor's source section was relocated)", anchor)
		}
	}
}

func TestC422_006_IntentSyntheticBuriedAnchorNegative(t *testing.T) {
	body := "Preamble operational content.\n\n## Reference Index\n\nPrior FAIL audit buried here\n"
	stripped := prompts.StripOnDemandSections(body)
	if stripped == body {
		t.Error("synthetic: strip was a no-op despite ## Reference Index heading — StripOnDemandSections broken")
	}
	if strings.Contains(stripped, "Prior FAIL audit buried here") {
		t.Error("synthetic: anchor buried below ## Reference Index survived strip — StripOnDemandSections must remove below-marker content")
	}
}

func TestC422_007_TriageCompactionSavesBytes4200(t *testing.T) {
	root := acsassert.RepoRoot(t)
	data, err := os.ReadFile(filepath.Join(root, "agents", "evolve-triage.md"))
	if err != nil {
		t.Fatalf("read evolve-triage.md: %v", err)
	}
	_, body, err := prompts.ParseFrontmatter(string(data))
	if err != nil {
		t.Fatalf("parse frontmatter: %v", err)
	}
	stripped := prompts.StripOnDemandSections(body)
	saved := len(body) - len(stripped)
	if saved < 64 {
		t.Errorf("triage compaction saved only %d bytes (want ≥64: the marker section itself); ## Reference Index heading missing? (body=%d stripped=%d)",
			saved, len(body), len(stripped))
	}
}

func TestC422_008_TriageDecisionAnchorsAboveMarker(t *testing.T) {
	root := acsassert.RepoRoot(t)
	data, err := os.ReadFile(filepath.Join(root, "agents", "evolve-triage.md"))
	if err != nil {
		t.Fatalf("read evolve-triage.md: %v", err)
	}
	_, body, err := prompts.ParseFrontmatter(string(data))
	if err != nil {
		t.Fatalf("parse frontmatter: %v", err)
	}
	stripped := prompts.StripOnDemandSections(body)
	for _, anchor := range []string{
		"challenge-token",
		"## top_n",
		"## deferred",
		"## dropped",
		"carryoverTodos",
		"## Rationale",
		"Operator-queue priority floor",
	} {
		if !strings.Contains(stripped, anchor) {
			t.Errorf("required decision anchor %q lost below ## Reference Index — must remain above marker in evolve-triage.md", anchor)
		}
	}
}

func TestC422_009_TriageSyntheticBuriedAnchorNegative(t *testing.T) {
	body := "Triage preamble.\n\n## Reference Index\n\nOperator-queue priority floor buried here\n"
	stripped := prompts.StripOnDemandSections(body)
	if stripped == body {
		t.Error("synthetic: strip was a no-op despite ## Reference Index heading — StripOnDemandSections broken")
	}
	if strings.Contains(stripped, "Operator-queue priority floor buried here") {
		t.Error("synthetic: anchor buried below ## Reference Index survived strip — StripOnDemandSections must remove below-marker content")
	}
}

func TestC422_010_TriageOnDemandBashExampleAbsentAfterStrip_Negative(t *testing.T) {
	root := acsassert.RepoRoot(t)
	data, err := os.ReadFile(filepath.Join(root, "agents", "evolve-triage.md"))
	if err != nil {
		t.Fatalf("read evolve-triage.md: %v", err)
	}
	_, body, err := prompts.ParseFrontmatter(string(data))
	if err != nil {
		t.Fatalf("parse frontmatter: %v", err)
	}
	stripped := prompts.StripOnDemandSections(body)
	if strings.Contains(stripped, "predicate-graph-reachable") {
		t.Error("RED: 'predicate-graph-reachable' (step-3b bash example) still appears in stripped body — the detection example must be relocated below ## Reference Index in evolve-triage.md")
	}
}
