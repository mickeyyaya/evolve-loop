package prompts

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

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

func TestBodyHasCompactMarker_BareHeading(t *testing.T) {
	body := "Content above.\n## Reference Index\nContent below.\n"
	if !bodyHasCompactMarker(body) {
		t.Error("bare '## Reference Index' heading not recognized by bodyHasCompactMarker (equality branch)")
	}
}

func TestBodyHasCompactMarker_TrailingSpaceOnly(t *testing.T) {
	body := "Content above.\n## Reference Index \nContent below.\n"
	if !bodyHasCompactMarker(body) {
		t.Error("'## Reference Index ' (trailing space only) not recognized by bodyHasCompactMarker (HasPrefix branch)")
	}
}

func TestStripOnDemandSections_BareCRLFHeading(t *testing.T) {
	body := "Body content.\r\n## Reference Index\r\nref link\r\n"
	stripped := StripOnDemandSections(body)
	if stripped == body {
		t.Error("bare '## Reference Index' with CRLF (\\r\\n) not stripped — " +
			"StripOnDemandSections may not handle \\r before \\n in bare heading lines; " +
			"bodyHasCompactMarker masks this via TrimRight")
	}
}

func TestStripOnDemandSections_LargeBody(t *testing.T) {
	above := strings.Repeat("Operational instruction line.\n", 600)
	tail := strings.Repeat("Reference entry line.\n", 200)
	body := above + "## Reference Index (Layer 3, on-demand)\n\n" + tail

	stripped := StripOnDemandSections(body)

	if len(stripped) >= len(body) {
		t.Errorf("large body: strip did not reduce size (before=%d after=%d)", len(body), len(stripped))
	}
	if stripped != above {
		t.Errorf("large body: stripped result differs from above-marker content (got len=%d, want len=%d)", len(stripped), len(above))
	}
}

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

	for _, anchor := range []string{
		"## Operating Principles",            // contains "Do NOT implement production code"
		"Mid-Trajectory Compaction Protocol", // contains "15-turn boundary"
	} {
		if !strings.Contains(stripped, anchor) {
			t.Errorf("tdd-engineer structural anchor %q absent from stripped body — may have been accidentally moved below ## Reference Index", anchor)
		}
	}
}

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
