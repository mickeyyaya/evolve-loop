package prompts

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestIntentCompaction_HeadFloor7000_Amplified(t *testing.T) {
	root := repoRoot(t)
	raw, err := os.ReadFile(filepath.Join(root, "agents", "evolve-intent.md"))
	if err != nil {
		t.Fatalf("read evolve-intent.md: %v", err)
	}
	_, body, err := ParseFrontmatter(string(raw))
	if err != nil {
		t.Fatalf("parse frontmatter: %v", err)
	}
	stripped := StripOnDemandSections(body)
	const minFloor = 7000
	if len(stripped) < minFloor {
		t.Errorf("intent stripped head only %d bytes (floor=%d) — gate-bearing operating instructions may have been accidentally relocated below ## Reference Index in evolve-intent.md", len(stripped), minFloor)
	}
}

func TestIntentCompaction_RealDocIdempotent_PostCycle422(t *testing.T) {
	root := repoRoot(t)
	raw, err := os.ReadFile(filepath.Join(root, "agents", "evolve-intent.md"))
	if err != nil {
		t.Fatalf("read evolve-intent.md: %v", err)
	}
	_, body, err := ParseFrontmatter(string(raw))
	if err != nil {
		t.Fatalf("parse frontmatter: %v", err)
	}
	once := StripOnDemandSections(body)
	twice := StripOnDemandSections(once)
	if once != twice {
		t.Errorf("StripOnDemandSections not idempotent on evolve-intent.md post-cycle-422: once=%d bytes, twice=%d bytes — relocated tail must contain no ## Reference Index heading", len(once), len(twice))
	}
}

func TestIntentOnDemandSectionsNotDeleted_Amplified(t *testing.T) {
	root := repoRoot(t)
	raw, err := os.ReadFile(filepath.Join(root, "agents", "evolve-intent.md"))
	if err != nil {
		t.Fatalf("read evolve-intent.md: %v", err)
	}
	_, body, err := ParseFrontmatter(string(raw))
	if err != nil {
		t.Fatalf("parse frontmatter: %v", err)
	}
	for _, onDemand := range []string{
		"## Output contract (INTENT_MODE)",
		"## Re-run behavior",
		"intent-reflection.yaml",
	} {
		if !strings.Contains(body, onDemand) {
			t.Errorf("on-demand content %q was deleted from evolve-intent.md — it must be RELOCATED below ## Reference Index, not removed; operators reading the full file need it", onDemand)
		}
	}
}

// Despite the name, the floor is 64 bytes: the marker section itself.
func TestTriageCompaction_ByteSavings4200_Amplified(t *testing.T) {
	root := repoRoot(t)
	raw, err := os.ReadFile(filepath.Join(root, "agents", "evolve-triage.md"))
	if err != nil {
		t.Fatalf("read evolve-triage.md: %v", err)
	}
	_, body, err := ParseFrontmatter(string(raw))
	if err != nil {
		t.Fatalf("parse frontmatter: %v", err)
	}
	stripped := StripOnDemandSections(body)
	saved := len(body) - len(stripped)
	const minSaved = 64
	if saved < minSaved {
		t.Errorf("triage compaction saved only %d bytes (want >=%d: the marker section itself); ## Reference Index heading missing? (body=%d stripped=%d)", saved, minSaved, len(body), len(stripped))
	}
}

func TestTriageCompaction_HeadFloor6000_Amplified(t *testing.T) {
	root := repoRoot(t)
	raw, err := os.ReadFile(filepath.Join(root, "agents", "evolve-triage.md"))
	if err != nil {
		t.Fatalf("read evolve-triage.md: %v", err)
	}
	_, body, err := ParseFrontmatter(string(raw))
	if err != nil {
		t.Fatalf("parse frontmatter: %v", err)
	}
	stripped := StripOnDemandSections(body)
	const minFloor = 6000
	if len(stripped) < minFloor {
		t.Errorf("triage stripped head only %d bytes (floor=%d) — process-critical decision rules may have been accidentally relocated below ## Reference Index in evolve-triage.md", len(stripped), minFloor)
	}
}

func TestTriageCompaction_Idempotent_Amplified(t *testing.T) {
	root := repoRoot(t)
	raw, err := os.ReadFile(filepath.Join(root, "agents", "evolve-triage.md"))
	if err != nil {
		t.Fatalf("read evolve-triage.md: %v", err)
	}
	_, body, err := ParseFrontmatter(string(raw))
	if err != nil {
		t.Fatalf("parse frontmatter: %v", err)
	}
	once := StripOnDemandSections(body)
	twice := StripOnDemandSections(once)
	if once != twice {
		t.Errorf("StripOnDemandSections not idempotent on evolve-triage.md: once=%d bytes, twice=%d bytes — relocated tail must contain no nested ## Reference Index heading", len(once), len(twice))
	}
}

func TestTriageOnDemandSectionNotDeleted_Amplified(t *testing.T) {
	root := repoRoot(t)
	raw, err := os.ReadFile(filepath.Join(root, "agents", "evolve-triage.md"))
	if err != nil {
		t.Fatalf("read evolve-triage.md: %v", err)
	}
	_, body, err := ParseFrontmatter(string(raw))
	if err != nil {
		t.Fatalf("parse frontmatter: %v", err)
	}
	if !strings.Contains(body, "predicate-graph-reachable") {
		t.Error("'predicate-graph-reachable' bash example was deleted from evolve-triage.md — it must be RELOCATED below ## Reference Index, not removed; operators need the detection example for reference")
	}
}
