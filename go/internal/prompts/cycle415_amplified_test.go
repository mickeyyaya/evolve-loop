package prompts

// cycle415_amplified_test.go — Adversarial amplification for cycle-415 tasks.
//
// Probes gaps NOT covered by:
//   strip_test.go (8 synthetic cases for StripOnDemandSections),
//   strip_amplified_test.go (adversarial boundary cases + idempotent + no-heading-in-output),
//   compact_marker_gate_test.go (real-doc compaction + gate + inline-mention),
//   realdoc_strip_test.go (6 real docs mustStrip with minSave).
//
// New adversarial angles:
//   - Stripped body minimum floor (prevents over-stripping that deletes required instructions)
//   - Reference stub file existence (cycle-415 created evolve-tdd-engineer-reference.md)
//   - CRLF bare-heading detection gap (bodyHasCompactMarker trims \r; StripOnDemandSections may not)
//   - bodyHasCompactMarker edge forms (bare heading, trailing-space-only heading)
//   - Large synthetic body correctness with exact boundary check
//   - Newline termination preservation (off-by-one in line reconstruction)
//   - Additional tdd-engineer behavior-anchors not in TestTddEngineerCompaction
//   - Canonical always-on doc file existence (guards against silent renames/deletions)

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestTddEngineerStrippedBodyFloor asserts the stripped body of evolve-tdd-engineer.md
// retains at least 5,000 bytes — guarding against over-stripping that would delete required
// operating instructions by placing the ## Reference Index marker too early.
func TestTddEngineerStrippedBodyFloor(t *testing.T) {
	root := repoRoot(t)
	raw, err := os.ReadFile(filepath.Join(root, "agents", "evolve-tdd-engineer.md"))
	if err != nil {
		t.Fatalf("read evolve-tdd-engineer.md: %v", err)
	}
	_, body, err := ParseFrontmatter(string(raw))
	if err != nil {
		t.Fatalf("parse frontmatter: %v", err)
	}
	stripped := StripOnDemandSections(body)
	const minFloor = 5000
	if len(stripped) < minFloor {
		t.Errorf("tdd-engineer stripped body only %d bytes (floor=%d) — required operating instructions may have been accidentally moved below ## Reference Index", len(stripped), minFloor)
	}
}

// TestTriageStrippedBodyFloor asserts the stripped body of evolve-triage.md retains at
// least 3,000 bytes — guarding against the required output sections being accidentally
// relocated below the ## Reference Index marker.
func TestTriageStrippedBodyFloor(t *testing.T) {
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
	const minFloor = 3000
	if len(stripped) < minFloor {
		t.Errorf("triage stripped body only %d bytes (floor=%d) — required sections may have been moved below ## Reference Index", len(stripped), minFloor)
	}
}

// TestTddEngineerReferenceStubExists verifies the evolve-tdd-engineer-reference.md file
// created in cycle-415 is non-empty. An empty or missing stub breaks the Layer 3 on-demand
// lookup contract for operators who fetch it via the reference index.
func TestTddEngineerReferenceStubExists(t *testing.T) {
	root := repoRoot(t)
	path := filepath.Join(root, "agents", "evolve-tdd-engineer-reference.md")
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("evolve-tdd-engineer-reference.md: %v", err)
	}
	if info.Size() == 0 {
		t.Error("evolve-tdd-engineer-reference.md is empty — must be a non-trivial reference stub")
	}
}

// TestBodyHasCompactMarker_BareHeading verifies that a bare ## Reference Index heading
// (no suffix, no trailing space) is recognized by bodyHasCompactMarker.
// Exercises the exact-equality branch: trimmed == "## Reference Index".
func TestBodyHasCompactMarker_BareHeading(t *testing.T) {
	body := "Content above.\n## Reference Index\nContent below.\n"
	if !bodyHasCompactMarker(body) {
		t.Error("bare '## Reference Index' heading not recognized by bodyHasCompactMarker (equality branch)")
	}
}

// TestBodyHasCompactMarker_TrailingSpaceOnly verifies that "## Reference Index " with a
// trailing space (and no further suffix) is accepted by bodyHasCompactMarker.
// Exercises the HasPrefix branch at its minimal suffix boundary.
func TestBodyHasCompactMarker_TrailingSpaceOnly(t *testing.T) {
	body := "Content above.\n## Reference Index \nContent below.\n"
	if !bodyHasCompactMarker(body) {
		t.Error("'## Reference Index ' (trailing space only) not recognized by bodyHasCompactMarker (HasPrefix branch)")
	}
}

// TestStripOnDemandSections_BareCRLFHeading probes a potential gap between the gate
// (bodyHasCompactMarker trims \r before comparison) and the strip function.
//
// bodyHasCompactMarker explicitly trims \r: "## Reference Index\r" → "## Reference Index" (equality match).
// StripOnDemandSections may use strings.HasPrefix without trimming \r: "## Reference Index\r"
// would NOT match prefix "## Reference Index " (next char is \r, not space).
//
// If this test fails, it confirms a CRLF gap: the gate passes but the strip silently skips.
// The suffixed production heading "## Reference Index (Layer 3, on-demand)\r" is unaffected
// (space before suffix still matches), making this a bare-heading-only regression risk.
func TestStripOnDemandSections_BareCRLFHeading(t *testing.T) {
	// CRLF body with bare ## Reference Index heading.
	body := "Body content.\r\n## Reference Index\r\nref link\r\n"
	stripped := StripOnDemandSections(body)
	if stripped == body {
		t.Error("bare '## Reference Index' with CRLF (\\r\\n) not stripped — " +
			"StripOnDemandSections may not handle \\r before \\n in bare heading lines; " +
			"bodyHasCompactMarker masks this via TrimRight")
	}
}

// TestStripOnDemandSections_LargeBody ensures correct stripping and exact boundary
// preservation for a large (~22 KB) synthetic body. Guards against off-by-one or
// buffer-related issues at realistic agent-doc scale.
func TestStripOnDemandSections_LargeBody(t *testing.T) {
	above := strings.Repeat("Operational instruction line.\n", 600) // ~18 KB
	tail := strings.Repeat("Reference entry line.\n", 200)          // ~4 KB
	body := above + "## Reference Index (Layer 3, on-demand)\n\n" + tail

	stripped := StripOnDemandSections(body)

	if len(stripped) >= len(body) {
		t.Errorf("large body: strip did not reduce size (before=%d after=%d)", len(body), len(stripped))
	}
	if stripped != above {
		t.Errorf("large body: stripped result differs from above-marker content (got len=%d, want len=%d)", len(stripped), len(above))
	}
}

// TestStripOnDemandSections_NewlineTermination verifies that stripping preserves the exact
// trailing newline of the above-marker content. Guards against off-by-one in line
// reconstruction that could silently drop the final \n or add an extra one.
func TestStripOnDemandSections_NewlineTermination(t *testing.T) {
	cases := []struct {
		name string
		body string
		want string
	}{
		{
			name: "trailing-newline-before-heading-preserved",
			body: "Line one.\nLine two.\n## Reference Index (Layer 3, on-demand)\n- ref\n",
			want: "Line one.\nLine two.\n",
		},
		{
			name: "content-glued-to-heading-on-same-line-not-stripped",
			// "Line two.## Reference Index" does NOT start a line with "## Reference Index"
			body: "Line one.\nLine two.## Reference Index (Layer 3, on-demand)\n- ref\n",
			want: "Line one.\nLine two.## Reference Index (Layer 3, on-demand)\n- ref\n",
		},
		{
			name: "multiple-blank-lines-before-heading-all-preserved",
			body: "Content.\n\n\n## Reference Index (Layer 3, on-demand)\n- ref\n",
			want: "Content.\n\n\n",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := StripOnDemandSections(c.body)
			if got != c.want {
				t.Fatalf("StripOnDemandSections(%q)\n  got  %q\n  want %q", c.body, got, c.want)
			}
		})
	}
}

// TestTddEngineerAdditionalAnchors verifies structural section anchors beyond the 4 phrases
// in TestTddEngineerCompaction. Confirmed above the marker by the cycle-415 build-report
// "Behavior Anchor Verification" section.
func TestTddEngineerAdditionalAnchors(t *testing.T) {
	root := repoRoot(t)
	raw, err := os.ReadFile(filepath.Join(root, "agents", "evolve-tdd-engineer.md"))
	if err != nil {
		t.Fatalf("read evolve-tdd-engineer.md: %v", err)
	}
	_, body, err := ParseFrontmatter(string(raw))
	if err != nil {
		t.Fatalf("parse frontmatter: %v", err)
	}
	stripped := StripOnDemandSections(body)

	// Section names confirmed present above the marker by the cycle-415 build-report.
	for _, anchor := range []string{
		"## Operating Principles",            // contains "Do NOT implement production code"
		"Mid-Trajectory Compaction Protocol", // contains "15-turn boundary"
	} {
		if !strings.Contains(stripped, anchor) {
			t.Errorf("tdd-engineer structural anchor %q absent from stripped body — may have been accidentally moved below ## Reference Index", anchor)
		}
	}
}

// TestAlwaysOnDocFilesExist verifies that all 6 canonical always-on phase doc files
// actually exist on disk. Guards against silent renames or deletions that would cause
// TestAlwaysOnPhaseDocsHaveCompactMarker to error-skip rather than fail.
func TestAlwaysOnDocFilesExist(t *testing.T) {
	root := repoRoot(t)
	canonical := []string{
		"evolve-tdd-engineer",
		"evolve-triage",
		"evolve-orchestrator",
		"evolve-auditor",
		"evolve-builder",
		"evolve-scout",
	}
	for _, name := range canonical {
		path := filepath.Join(root, "agents", name+".md")
		if _, err := os.Stat(path); err != nil {
			t.Errorf("canonical always-on phase doc %s.md missing: %v", name, err)
		}
	}
}
