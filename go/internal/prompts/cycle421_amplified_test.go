package prompts

// cycle421_amplified_test.go — Adversarial amplification for cycle-421 tasks.
//
// Probes gaps NOT covered by predicates_test.go (C421_001–C421_010) or
// retro_compaction_test.go:
//
//   - Retro doc head floor (≥8000B remains after strip) — C421_001 verifies ≥1500B SAVED
//     but no floor exists on what REMAINS; this is the complementary anti-over-strip guard.
//   - Strip idempotency on real retro doc — cycle-416 added idempotency for intent; retro uncovered.
//   - Strip idempotency on real orchestrator doc — existing tests verify savings/anchors, not stability.
//   - Versioned sections NOT deleted from retro doc — C421_003 verifies sections absent from STRIPPED
//     body; this verifies they still exist in the full raw body (moved, not removed).
//   - On-demand sections NOT deleted from orchestrator doc — same relocation-vs-deletion guard.
//   - Retro doc has line-anchored ## Reference Index marker — compaction_coverage_test.go covers
//     the 7 always-on agents but NOT evolve-retrospective (conditional phase).
//   - Orchestrator byte-savings floor tied to cycle-421 spec (>=2000B vs realdoc's pre-cycle 512B).

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestRetroCompaction_HeadFloor_Amplified asserts that the stripped evolve-retrospective.md
// body retains at least 8000 bytes after StripOnDemandSections.
//
// Amplification angle: C421_001 verifies >=1500B SAVED (upper bound on stripping).
// This test adds a complementary FLOOR on what REMAINS, modeled after C421_008's 9000B
// guard for the orchestrator. The retro body before cycle-421 was ~11732B; after stripping
// ~1970B the head should be ~9762B — well above the 8000B floor.
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

// TestRetroCompaction_Idempotent asserts that applying StripOnDemandSections twice to
// the real evolve-retrospective.md body produces the same result as applying it once.
//
// Idempotency is the key safety property: once the on-demand tail is stripped, the
// remaining head no longer contains the ## Reference Index marker, so a second strip
// must be a complete no-op — not corrupt or truncate the head content.
//
// Amplification angle: cycle-416 adds real-doc idempotency for evolve-intent;
// no equivalent guard exists for evolve-retrospective.
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

// TestOrchestratorCompaction_Idempotent asserts that applying StripOnDemandSections twice
// to the real evolve-orchestrator.md body produces the same result as applying it once.
//
// Amplification angle: cycle-421 moved >=2965B of sections below the marker in orchestrator.
// Idempotency guards against a scenario where the moved sections contain another
// ## Reference Index line that would cause a second strip to further truncate the tail.
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

// TestRetroVersionedSectionsNotDeleted asserts that versioned sections relocated below
// ## Reference Index in evolve-retrospective.md are still present in the raw file body.
//
// Amplification angle: C421_003 verifies the sections are ABSENT from the STRIPPED body
// (below the marker). This test guards the complementary invariant: the sections must
// still exist in the full document — relocated, not deleted. A builder who deleted these
// sections would pass C421_003 while losing the on-demand reference content.
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

// TestOrchestratorOnDemandSectionsNotDeleted asserts that on-demand sections relocated
// below ## Reference Index in evolve-orchestrator.md are still present in the raw file body.
//
// Amplification angle: C421_010 verifies sections are ABSENT from the STRIPPED body.
// This test guards the complementary invariant: sections were moved, not deleted.
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

// TestRetroDocHasLineAnchoredMarker asserts that evolve-retrospective.md contains a
// line-anchored ## Reference Index heading — the same gate StripOnDemandSections uses
// to detect the cut point.
//
// Amplification angle: compaction_coverage_test.go's TestAllPerCycleAgentsStrictlyCompact
// covers 7 always-on per-cycle agents but NOT evolve-retrospective (conditional phase:
// runs only on FAIL/WARN). Without this guard, a future edit that accidentally removes
// the marker from the retro doc would silently disable compaction (StripOnDemandSections
// no-ops on markerless bodies).
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

// TestOrchestratorCompaction_ByteSavingsFloorAmplified asserts that StripOnDemandSections
// applied to the real evolve-orchestrator.md saves >=2000 bytes — the cycle-421 floor.
//
// Amplification angle: realdoc_strip_test.go (written before cycle-421) uses 512B as the
// orchestrator floor; C421_007 raises it to 2000B. This test explicitly names the 2000B
// floor in the prompts-package test suite so any future edit that reduces the on-demand
// tail back below 2000B is caught here as well as in the ACS suite.
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
