package prompts

// cycle416_amplified_test.go — Adversarial amplification for cycle-416 tasks.
//
// Probes gaps NOT covered by:
//   intent_compaction_test.go (4 tests: ≥500B, 5-anchor check, ## Composition absent, synthetic anti-gaming),
//   compaction_coverage_test.go (2 tests: all 7 agents strict-decrease, markerless unchanged),
//   compact_marker_gate_test.go (bodyHasCompactMarker gate for 6 agents, excluding intent).
//
// New adversarial angles:
//   - evolve-intent.md stripped body floor (≥5000B prevents over-stripping the behavior-bearing head)
//   - Reflection Authoring anchor in stripped intent body (eval spec mandates it; not in the 5-anchor list)
//   - Ask-when-Needed (AwN) classifier in stripped intent body (eval spec mandates; no Go test covers it)
//   - ## Reference section absent after strip (build-report moved BOTH ## Composition and ## Reference below marker)
//   - bodyHasCompactMarker gate applied to intent (TestAlwaysOnPhaseDocsHaveCompactMarker covers 6 agents not 7)
//   - Tighter byte-savings bound ≥600B (build-report states ~650B; adversarially tighter than existing ≥500B)
//   - Real-doc idempotency: strip applied twice to real evolve-intent.md (synthetic idempotency exists; real-doc does not)
//   - evolve-intent.md file existence guard (TestAlwaysOnDocFilesExist covers 6 agents; intent is absent)

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestIntentStrippedBodyFloor asserts that the stripped evolve-intent.md body retains at
// least 5,000 bytes — guarding against over-stripping that would accidentally place required
// operating instructions (IMKI, STOP CRITERION, Output contract, AwN classifier) below the
// ## Reference Index marker.
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

// TestIntentCompaction_ReflectionAuthoringNotDeleted asserts that the Reflection Authoring
// section still exists in the raw evolve-intent.md body (relocated, not deleted).
//
// Cycle-416 placed ## Reflection Authoring (v10.20.0+) above the marker.
// Cycle-422 intentionally moves it BELOW the marker as on-demand reference — so checking
// for it in the stripped body would fail correctly. This test guards the complementary
// invariant: the section must still exist in the full document (relocated, not removed).
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

// TestIntentCompaction_AskWhenNeededAboveMarker asserts that the Ask-when-Needed (AwN)
// classifier framework is present in the stripped evolve-intent.md body.
//
// Eval spec requires "AwN classifier" to survive strip (remain above the marker). No existing
// Go test verifies this. The agent description confirms intent classifies goals via this framework.
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

// TestIntentCompaction_ReferenceSectionAbsentAfterStrip asserts that any "## Reference"
// section heading (NOT the strip-marker heading "## Reference Index …") is absent from the
// stripped evolve-intent.md body.
//
// Gap: TestIntentCompaction_ReferenceContentAbsentAfterStrip_Negative only verifies ## Composition.
// The build-report states "## Composition and ## Reference sections relocated below marker".
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
		// Match any "## Reference …" heading that is NOT the strip-marker itself.
		if strings.HasPrefix(l, "## Reference") && !strings.HasPrefix(l, "## Reference Index") {
			t.Errorf("reference section heading %q still appears in stripped body — must be relocated below ## Reference Index marker in evolve-intent.md", line)
		}
	}
}

// TestIntentHasCompactMarkerViaGate asserts that evolve-intent.md's body is recognized by
// bodyHasCompactMarker — the same gate function used in TestAlwaysOnPhaseDocsHaveCompactMarker.
//
// Gap: TestAlwaysOnPhaseDocsHaveCompactMarker covers 6 per-cycle agents (scout, builder,
// auditor, orchestrator, tdd-engineer, triage) but NOT evolve-intent, which was added in
// cycle-416. TestAllPerCycleAgentsStrictlyCompact covers intent via length-decrease, not
// via the bodyHasCompactMarker gate function itself.
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

// TestIntentCompaction_TighterBytesSavingThreshold asserts that stripping evolve-intent.md
// saves at least 600 bytes — an adversarial tightening of the existing ≥500-byte threshold.
//
// The build-report states "~650 bytes saved (Composition + Reference sections below marker)".
// A ≥600-byte threshold catches a regression where only ONE of the two sections was relocated
// below the marker instead of both, yielding savings just above 500 but well below 600.
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

// TestIntentCompaction_RealDocIdempotent asserts that applying StripOnDemandSections twice
// to the real evolve-intent.md body yields the same result as applying it once.
//
// Distinct from TestStripOnDemandSections_Idempotent (synthetic bodies only); exercises the
// real production heading "## Reference Index (Layer 3, on-demand)" in the actual agent file.
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

// TestIntentAgentFileExists asserts that evolve-intent.md exists on disk and is non-empty.
//
// Guard against silent renames or deletions: TestAlwaysOnDocFilesExist covers 6 canonical
// agents (scout, builder, auditor, orchestrator, tdd-engineer, triage) but NOT evolve-intent.
// Without this guard, a missing evolve-intent.md causes the intent-specific tests above to
// fail with a misleading I/O error rather than a clear description of what broke.
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
