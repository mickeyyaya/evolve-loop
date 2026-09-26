package prompts

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRetroCompaction_HeadFloor_Amplified(t *testing.T) {
	root := repoRoot(t)
	raw, err := os.ReadFile(filepath.Join(root, "agents", "evolve-retrospective.md"))
	if err != nil {
		t.Fatalf("read evolve-retrospective.md: %v", err)
	}
	_, body, err := ParseFrontmatter(string(raw))
	if err != nil {
		t.Fatalf("parse frontmatter: %v", err)
	}
	stripped := StripOnDemandSections(body)
	const minFloor = 8000
	if len(stripped) < minFloor {
		t.Errorf("retro stripped head only %d bytes (floor=%d) — gate-bearing process sections may have been accidentally relocated below ## Reference Index in evolve-retrospective.md", len(stripped), minFloor)
	}
}

func TestRetroCompaction_Idempotent(t *testing.T) {
	root := repoRoot(t)
	raw, err := os.ReadFile(filepath.Join(root, "agents", "evolve-retrospective.md"))
	if err != nil {
		t.Fatalf("read evolve-retrospective.md: %v", err)
	}
	_, body, err := ParseFrontmatter(string(raw))
	if err != nil {
		t.Fatalf("parse frontmatter: %v", err)
	}
	once := StripOnDemandSections(body)
	twice := StripOnDemandSections(once)
	if once != twice {
		t.Errorf("StripOnDemandSections is not idempotent on evolve-retrospective.md: once=%d bytes, twice=%d bytes — stripped head must contain no ## Reference Index marker", len(once), len(twice))
	}
}

func TestOrchestratorCompaction_Idempotent(t *testing.T) {
	root := repoRoot(t)
	raw, err := os.ReadFile(filepath.Join(root, "agents", "evolve-orchestrator.md"))
	if err != nil {
		t.Fatalf("read evolve-orchestrator.md: %v", err)
	}
	_, body, err := ParseFrontmatter(string(raw))
	if err != nil {
		t.Fatalf("parse frontmatter: %v", err)
	}
	once := StripOnDemandSections(body)
	twice := StripOnDemandSections(once)
	if once != twice {
		t.Errorf("StripOnDemandSections is not idempotent on evolve-orchestrator.md: once=%d bytes, twice=%d bytes", len(once), len(twice))
	}
}

func TestRetroVersionedSectionsNotDeleted(t *testing.T) {
	root := repoRoot(t)
	raw, err := os.ReadFile(filepath.Join(root, "agents", "evolve-retrospective.md"))
	if err != nil {
		t.Fatalf("read evolve-retrospective.md: %v", err)
	}
	_, body, err := ParseFrontmatter(string(raw))
	if err != nil {
		t.Fatalf("parse frontmatter: %v", err)
	}
	for _, versioned := range []string{
		"### 1.5 Read abnormal-events.jsonl (v46+)",
		"### 1.7 Read reflector synthesis (v10.20.0+)",
	} {
		if !strings.Contains(body, versioned) {
			t.Errorf("versioned section %q was deleted from evolve-retrospective.md — it must be RELOCATED below ## Reference Index, not removed; operators reading the full file need it", versioned)
		}
	}
}

func TestOrchestratorOnDemandSectionsNotDeleted(t *testing.T) {
	root := repoRoot(t)
	raw, err := os.ReadFile(filepath.Join(root, "agents", "evolve-orchestrator.md"))
	if err != nil {
		t.Fatalf("read evolve-orchestrator.md: %v", err)
	}
	_, body, err := ParseFrontmatter(string(raw))
	if err != nil {
		t.Fatalf("parse frontmatter: %v", err)
	}
	for _, onDemand := range []string{
		"## Path conventions",
		"## Worktree contract",
		"## Closure-Mode Detection",
	} {
		if !strings.Contains(body, onDemand) {
			t.Errorf("on-demand section %q was deleted from evolve-orchestrator.md — it must be RELOCATED below ## Reference Index, not removed; operators need it for reference", onDemand)
		}
	}
}

func TestRetroDocHasLineAnchoredMarker(t *testing.T) {
	root := repoRoot(t)
	raw, err := os.ReadFile(filepath.Join(root, "agents", "evolve-retrospective.md"))
	if err != nil {
		t.Fatalf("read evolve-retrospective.md: %v", err)
	}
	_, body, err := ParseFrontmatter(string(raw))
	if err != nil {
		t.Fatalf("parse frontmatter: %v", err)
	}
	var hasMarker bool
	for _, line := range strings.Split(body, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "## Reference Index" || strings.HasPrefix(trimmed, "## Reference Index ") {
			hasMarker = true
			break
		}
	}
	if !hasMarker {
		t.Errorf("evolve-retrospective.md has no line-anchored '## Reference Index' heading — StripOnDemandSections will not fire on retro dispatches, silently disabling compaction for this phase")
	}
}

func TestOrchestratorCompaction_ByteSavingsFloorAmplified(t *testing.T) {
	root := repoRoot(t)
	raw, err := os.ReadFile(filepath.Join(root, "agents", "evolve-orchestrator.md"))
	if err != nil {
		t.Fatalf("read evolve-orchestrator.md: %v", err)
	}
	_, body, err := ParseFrontmatter(string(raw))
	if err != nil {
		t.Fatalf("parse frontmatter: %v", err)
	}
	stripped := StripOnDemandSections(body)
	saved := len(body) - len(stripped)
	const minSaved = 2000
	if saved < minSaved {
		t.Errorf("orchestrator compaction saved only %d bytes (want >=%d post-cycle-421); on-demand sections may have been promoted above ## Reference Index (body=%d stripped=%d)", saved, minSaved, len(body), len(stripped))
	}
}
