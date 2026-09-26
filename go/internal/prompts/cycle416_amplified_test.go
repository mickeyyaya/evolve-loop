package prompts

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestIntentStrippedBodyFloor(t *testing.T) {
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
	const minFloor = 5000
	if len(stripped) < minFloor {
		t.Errorf("intent stripped body only %d bytes (floor=%d) — required operating instructions may have been accidentally moved below ## Reference Index", len(stripped), minFloor)
	}
}

func TestIntentCompaction_ReflectionAuthoringNotDeleted(t *testing.T) {
	root := repoRoot(t)
	raw, err := os.ReadFile(filepath.Join(root, "agents", "evolve-intent.md"))
	if err != nil {
		t.Fatalf("read evolve-intent.md: %v", err)
	}
	_, body, err := ParseFrontmatter(string(raw))
	if err != nil {
		t.Fatalf("parse frontmatter: %v", err)
	}
	if !strings.Contains(body, "Reflection Authoring") {
		t.Error("'Reflection Authoring' section was deleted from evolve-intent.md — it must be RELOCATED below ## Reference Index, not removed; operators need it for reference")
	}
}

func TestIntentCompaction_AskWhenNeededAboveMarker(t *testing.T) {
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
	if !strings.Contains(stripped, "Ask-when-Needed") {
		t.Error("required anchor 'Ask-when-Needed' (AwN classifier) lost below ## Reference Index — must remain above marker in evolve-intent.md (eval spec: 'AwN classifier must survive')")
	}
}

func TestIntentCompaction_ReferenceSectionAbsentAfterStrip(t *testing.T) {
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
	for _, line := range strings.Split(stripped, "\n") {
		l := strings.TrimSpace(line)
		if strings.HasPrefix(l, "## Reference") && !strings.HasPrefix(l, "## Reference Index") {
			t.Errorf("reference section heading %q still appears in stripped body — must be relocated below ## Reference Index marker in evolve-intent.md", line)
		}
	}
}

func TestIntentHasCompactMarkerViaGate(t *testing.T) {
	root := repoRoot(t)
	raw, err := os.ReadFile(filepath.Join(root, "agents", "evolve-intent.md"))
	if err != nil {
		t.Fatalf("read evolve-intent.md: %v", err)
	}
	_, body, err := ParseFrontmatter(string(raw))
	if err != nil {
		t.Fatalf("parse frontmatter: %v", err)
	}
	if !bodyHasCompactMarker(body) {
		t.Error("evolve-intent.md body not recognized by bodyHasCompactMarker — line-anchored ## Reference Index heading is missing or malformed")
	}
}

func TestIntentCompaction_TighterBytesSavingThreshold(t *testing.T) {
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
	saved := len(body) - len(stripped)
	if saved < 600 {
		t.Errorf("intent compaction saved only %d bytes (want ≥600); build-report states ~650B from relocating BOTH ## Composition and ## Reference below the marker (body=%d stripped=%d)", saved, len(body), len(stripped))
	}
}

func TestIntentCompaction_RealDocIdempotent(t *testing.T) {
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
		t.Errorf("StripOnDemandSections not idempotent on real evolve-intent.md: first=%d bytes, second=%d bytes", len(once), len(twice))
	}
}

func TestIntentAgentFileExists(t *testing.T) {
	root := repoRoot(t)
	path := filepath.Join(root, "agents", "evolve-intent.md")
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("evolve-intent.md: %v", err)
	}
	if info.Size() == 0 {
		t.Error("evolve-intent.md is empty — must be a non-trivial agent document")
	}
}
