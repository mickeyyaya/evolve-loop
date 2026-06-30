package prompts

// cycle422_amplified_test.go — Adversarial amplification for cycle-422 tasks.
//
// Probes gaps NOT covered by predicates_test.go (C422_001–C422_010) or the
// direct test files updated for cycle-422:
//
//   - Intent head floor (≥7000B remains after strip) — C422_001 verifies ≥2200B SAVED
//     but has no floor on what REMAINS; complementary anti-over-strip guard.
//   - Intent idempotency on real doc — cycle-416 added real-doc idempotency; re-verified
//     after cycle-422 expands the below-marker tail.
//   - Intent on-demand sections not deleted — C422_002/003/004 verify sections absent from
//     STRIPPED body; this verifies they still exist in the raw body (relocated, not removed).
//   - Triage byte-savings floor ≥4200B in prompts package (mirrors C422_007 in ACS suite).
//   - Triage head floor (≥6000B remains after strip) — anti-over-strip guard for triage.
//   - Triage idempotency on real doc.
//   - Triage on-demand section not deleted — bash example must exist in raw body after relocation.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestIntentCompaction_HeadFloor7000_Amplified asserts that the stripped evolve-intent.md
// body (above-marker head) retains at least 7000 bytes after StripOnDemandSections.
//
// Amplification angle: C422_001 verifies ≥2200B SAVED. This adds a complementary FLOOR
// on what REMAINS, preventing over-relocation that would strip gate-bearing operating
// instructions. Current intent head is ~10232B; moving ~1500B below leaves ~8700B >> 7000B.
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

// TestIntentCompaction_RealDocIdempotent_PostCycle422 asserts that applying
// StripOnDemandSections twice to the real evolve-intent.md body (after cycle-422 expansion)
// produces the same result as applying it once.
//
// Amplification angle: cycle-416 added real-doc idempotency. Cycle-422 expands the
// below-marker tail by ~1500B; this re-verifies the moved content contains no nested
// ## Reference Index heading that would cause a second strip to further truncate.
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

// TestIntentOnDemandSectionsNotDeleted_Amplified asserts that on-demand sections relocated
// below ## Reference Index in evolve-intent.md are still present in the raw file body.
//
// Amplification angle: C422_002/003/004 verify sections are ABSENT from the STRIPPED body.
// This guards the complementary invariant: sections were moved, not deleted.
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

// TestTriageCompaction_ByteSavings4200_Amplified asserts that StripOnDemandSections applied
// to the real evolve-triage.md body saves ≥4200 bytes — the cycle-422 floor.
//
// Amplification angle: compact_marker_gate_test.go raises the triage threshold to ≥4200B
// for the ACS-gated test; this mirrors it in the always-on CI prompts package.
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
	const minSaved = 4200
	if saved < minSaved {
		t.Errorf("triage compaction saved only %d bytes (want >=%d post-cycle-422); on-demand sections may have been promoted above ## Reference Index (body=%d stripped=%d)", saved, minSaved, len(body), len(stripped))
	}
}

// TestTriageCompaction_HeadFloor6000_Amplified asserts that the stripped evolve-triage.md
// body retains at least 6000 bytes after StripOnDemandSections.
//
// Amplification angle: C422_007 verifies ≥4200B SAVED. This adds a complementary FLOOR
// on what REMAINS, preventing over-relocation that would strip process-critical triage
// decision rules. Current triage head (above marker) is ~11963B; moving ~1011B more
// below leaves ~10952B >> 6000B.
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

// TestTriageCompaction_Idempotent_Amplified asserts that applying StripOnDemandSections
// twice to the real evolve-triage.md body produces the same result as applying it once.
//
// Idempotency is the key safety property: the moved content must contain no nested
// ## Reference Index heading that would cause a second strip to further truncate the head.
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

// TestTriageOnDemandSectionNotDeleted_Amplified asserts that the step-3b bash detection
// example relocated below ## Reference Index in evolve-triage.md is still present in
// the raw file body.
//
// Amplification angle: C422_010 verifies "predicate-graph-reachable" is ABSENT from the
// STRIPPED body. This guards the complementary invariant: the bash example was moved, not
// deleted — operators reading the full agent doc still need it for reference.
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
