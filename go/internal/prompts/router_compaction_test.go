package prompts

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// router_compaction_test.go — RED contract for cycle-417 task router-catalog-prose-compaction.
//
// RED state (before builder):
//   - evolve-router.md "## Phase Catalog — Core Values" section is ~10507B (want <8000B)
//   - StripOnDemandSections is NOT used for the router (catalog is the working menu);
//     compaction is in-place prose trimming, not marker-based removal.

// TestRouterCompaction_CoreValuesSectionUnder8000Bytes asserts that the
// "## Phase Catalog — Core Values" section in evolve-router.md is <8000 bytes.
// RED: current section is ~10507B — builder must compact the per-row justification
// prose to a tight one-clause trigger per row while keeping all 66 names.
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

	// Measure from the heading to the next top-level ## heading or EOF.
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

// TestRouterCompaction_66RowsRetained asserts that the router catalog retains exactly
// 66 phase rows after prose compaction (no row may be deleted during the trim).
// Pre-existing GREEN: catalog currently has 66 rows.
// Anti-gaming sentinel: catches any builder who shrinks the byte count by dropping rows.
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
	// Find end of catalog section (next ## heading or EOF)
	nextSection := strings.Index(section[len(heading):], "\n## ")
	if nextSection >= 0 {
		section = section[:len(heading)+nextSection]
	}

	// Count data rows: lines that start with "| `" (backtick phase names)
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

// TestRouterCompaction_NoEmptyTriggerRows_Negative asserts that no phase row in the
// catalog has an empty or whitespace-only trigger (i.e., second column).
// Pre-existing GREEN: all rows currently have substantive trigger text.
// Anti-gaming sentinel: catches over-trimming that leaves bare "| `name` | |" entries.
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
		// Split on " | " to extract columns
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
