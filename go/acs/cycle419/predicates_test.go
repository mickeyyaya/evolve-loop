//go:build acs

package cycle419

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/prompts"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

func scoutContent(t *testing.T) (raw []byte, fm map[string]any, body string) {
	t.Helper()
	root := acsassert.RepoRoot(t)
	p := filepath.Join(root, "agents", "evolve-scout.md")
	raw, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("read %s: %v", p, err)
	}
	fm, body, err = prompts.ParseFrontmatter(string(raw))
	if err != nil {
		t.Fatalf("ParseFrontmatter(agents/evolve-scout.md): %v", err)
	}
	return raw, fm, body
}

func goDir(t *testing.T) string {
	t.Helper()
	return filepath.Join(acsassert.RepoRoot(t), "go")
}

func TestC419_001_ScoutLineCountAtFloor(t *testing.T) {
	raw, _, _ := scoutContent(t)
	lines := strings.Split(string(raw), "\n")
	n := len(lines)
	if n > 0 && lines[n-1] == "" {
		n--
	}
	const maxLines = 204
	if n > maxLines {
		t.Errorf("RED: agents/evolve-scout.md has %d lines (want ≤%d).\n"+
			"Builder must remove ≥%d lines by collapsing the §9 eval-materialization\n"+
			"run-on paragraph and the challenge-token multi-cycle war-story. (current: %d → target: ≤%d)",
			n, maxLines, n-maxLines, n, maxLines)
	}
}

func TestC419_002_ScoutByteCountAtFloor(t *testing.T) {
	raw, _, _ := scoutContent(t)
	const maxBytes = 14941
	if len(raw) >= maxBytes {
		t.Errorf("RED: agents/evolve-scout.md is %d bytes (want <%d).\n"+
			"Builder must shrink below the 14941-byte baseline.\n"+
			"Current: %d bytes (need to remove ≥1 byte of redundant prose).",
			len(raw), maxBytes, len(raw))
	}
}

func TestC419_003_ScoutFrontmatterPreserved(t *testing.T) {
	_, fm, _ := scoutContent(t)
	if fm == nil {
		t.Fatalf("ParseFrontmatter returned nil map — YAML frontmatter fence is broken")
	}
	for _, key := range []string{"name", "model", "description", "tools"} {
		v, ok := fm[key]
		if !ok {
			t.Errorf("frontmatter missing key %q — trim corrupted the YAML block", key)
			continue
		}
		switch val := v.(type) {
		case string:
			if strings.TrimSpace(val) == "" {
				t.Errorf("frontmatter[%q] is present but empty", key)
			}
		case []string:
			if len(val) == 0 {
				t.Errorf("frontmatter[%q] is an empty list", key)
			}
		case nil:
			t.Errorf("frontmatter[%q] has nil value", key)
		}
	}
}

func TestC419_004_ScoutOutputSectionsPreserved(t *testing.T) {
	_, _, body := scoutContent(t)
	for _, section := range []string{
		"Discovery Summary",
		"Key Findings",
		"Research",
		"Research → Implementation Map",
		"Hypotheses",
		"Beyond-the-Ask Hypotheses",
		"Selected Tasks",
		"Acceptance Criteria Summary",
		"Carryover Decisions",
		"Deferred",
		"Decision Trace",
	} {
		if !strings.Contains(body, section) {
			t.Errorf("required output-section name %q absent from parsed body — trim over-deleted", section)
		}
	}
}

func TestC419_005_ScoutGatesAndTokenPreserved(t *testing.T) {
	_, _, body := scoutContent(t)
	for _, gate := range []string{
		"system-health-complete",
		"inbox-audit-complete",
		"backlog-complete",
		"build-plan-written",
		"research-cache-section",
		"evals-materialized",
	} {
		if !strings.Contains(body, gate) {
			t.Errorf("STOP-CRITERION gate %q absent from parsed body — trim must not delete gate rows", gate)
		}
	}
	if !strings.Contains(body, "challenge-token") {
		t.Errorf("challenge-token requirement absent from parsed body — trim must preserve the challenge-token mandate")
	}
}

func TestC419_006_ScoutAntiGamingFloor_Negative(t *testing.T) {
	raw, _, body := scoutContent(t)
	lines := strings.Split(string(raw), "\n")
	n := len(lines)
	if n > 0 && lines[n-1] == "" {
		n--
	}
	const minLines = 150
	if n < minLines {
		t.Errorf("NEGATIVE: agents/evolve-scout.md has only %d lines (want ≥%d).\n"+
			"Builder over-deleted — trim dedup prose only, not behavioral rules.", n, minLines)
	}
	if !strings.Contains(body, "eval materialization") {
		t.Errorf("NEGATIVE: 'eval materialization' absent from body — Builder stripped the §9 mandate\n" +
			"(must only collapse the redundant repetition that duplicates gate #6, not the mandate itself)")
	}
}

func TestC419_007_ScoutNoDuplicateHeadings(t *testing.T) {
	_, _, body := scoutContent(t)
	seen := make(map[string]int)
	for _, line := range strings.Split(body, "\n") {
		trimmed := strings.TrimRight(line, "\r")
		if strings.HasPrefix(trimmed, "## ") {
			seen[trimmed]++
		}
	}
	for heading, count := range seen {
		if count > 1 {
			t.Errorf("duplicate ## heading %q appears %d times — trim must not introduce duplicate headings", heading, count)
		}
	}
}

func TestC419_008_ScoutPromptsAndPhaseSuiteGreen(t *testing.T) {
	dir := goDir(t)
	_, stderr, code, err := acsassert.SubprocessOutput(
		"go", "test", "-C", dir, "-count=1",
		"./internal/prompts/...", "./internal/phases/scout/...")
	if err != nil || code != 0 {
		t.Errorf("RED/REGRESSION: prompts+scout test suite failed (exit=%d, err=%v):\n%s",
			code, err, stderr)
	}
}
