package prompts

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRouterCompaction_CoreValuesSectionUnder8000Bytes(t *testing.T) {
	root := repoRoot(t)
	raw, err := os.ReadFile(filepath.Join(root, "agents", "evolve-router.md"))
	if err != nil {
		t.Fatalf("read evolve-router.md: %v", err)
	}
	body := string(raw)

	const heading = "## Phase Catalog — Core Values"
	idx := strings.Index(body, heading)
	if idx < 0 {
		t.Fatalf("evolve-router.md missing '## Phase Catalog — Core Values' section")
	}

	rest := body[idx+len(heading):]
	nextSection := strings.Index(rest, "\n## ")
	var sectionBytes int
	if nextSection < 0 {
		sectionBytes = len(heading) + len(rest)
	} else {
		sectionBytes = len(heading) + nextSection
	}

	const maxBytes = 8000
	if sectionBytes >= maxBytes {
		t.Errorf("RED: '## Phase Catalog — Core Values' section is %d bytes (want <%d).\n"+
			"Builder must compact per-row justification prose to a tight one-clause trigger.\n"+
			"Keep all 66 rows. Target: <%d bytes.", sectionBytes, maxBytes, maxBytes)
	}
}

func TestRouterCompaction_66RowsRetained(t *testing.T) {
	root := repoRoot(t)
	raw, err := os.ReadFile(filepath.Join(root, "agents", "evolve-router.md"))
	if err != nil {
		t.Fatalf("read evolve-router.md: %v", err)
	}
	body := string(raw)

	const heading = "## Phase Catalog — Core Values"
	idx := strings.Index(body, heading)
	if idx < 0 {
		t.Fatalf("evolve-router.md missing '## Phase Catalog — Core Values' section")
	}
	section := body[idx:]
	nextSection := strings.Index(section[len(heading):], "\n## ")
	if nextSection >= 0 {
		section = section[:len(heading)+nextSection]
	}

	count := 0
	for _, line := range strings.Split(section, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "| `") && strings.Contains(trimmed, "` |") {
			count++
		}
	}

	const want = 66
	if count != want {
		t.Errorf("router catalog has %d phase rows (want %d) — prose compaction MUST NOT delete any row;\n"+
			"every phase name must survive verbatim in the catalog", count, want)
	}
}

func TestRouterCompaction_NoEmptyTriggerRows_Negative(t *testing.T) {
	root := repoRoot(t)
	raw, err := os.ReadFile(filepath.Join(root, "agents", "evolve-router.md"))
	if err != nil {
		t.Fatalf("read evolve-router.md: %v", err)
	}
	body := string(raw)

	const heading = "## Phase Catalog — Core Values"
	idx := strings.Index(body, heading)
	if idx < 0 {
		t.Fatalf("evolve-router.md missing '## Phase Catalog — Core Values' section")
	}
	section := body[idx:]
	nextSection := strings.Index(section[len(heading):], "\n## ")
	if nextSection >= 0 {
		section = section[:len(heading)+nextSection]
	}

	for _, line := range strings.Split(section, "\n") {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "| `") || !strings.Contains(trimmed, "` |") {
			continue
		}
		parts := strings.SplitN(trimmed, "` | ", 2)
		if len(parts) < 2 {
			t.Errorf("row has no second column: %q", trimmed)
			continue
		}
		trigger := strings.TrimSuffix(strings.TrimSpace(parts[1]), " |")
		trigger = strings.TrimSpace(trigger)
		if trigger == "" {
			t.Errorf("row has empty trigger (second column blank): %q", trimmed)
		}
	}
}
