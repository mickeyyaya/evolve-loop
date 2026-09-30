//go:build acs

package cycle387

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

func TestC387_001_LineCountReduced(t *testing.T) {
	root := acsassert.RepoRoot(t)
	path := filepath.Join(root, "agents", "evolve-tdd-engineer.md")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("cannot read agents/evolve-tdd-engineer.md: %v", err)
	}
	lineCount := strings.Count(string(raw), "\n")
	const maxLines = 405
	if lineCount > maxLines {
		t.Errorf("RED: agents/evolve-tdd-engineer.md has %d lines — must be ≤%d.\n"+
			"Builder must remove the retired-bash repetitions and the fallback shell-test\n"+
			"example (lines 103–128 per scout-report.md) to achieve the ≥27-line reduction.\n"+
			"Baseline: 432 lines. Target: ≤405.",
			lineCount, maxLines)
	}
}

// acs-predicate: config-check
func TestC387_002_AllSectionHeadersPresent(t *testing.T) {
	root := acsassert.RepoRoot(t)
	path := filepath.Join(root, "agents", "evolve-tdd-engineer.md")
	required := []string{
		"## Inputs",
		"## Pipeline Position",
		"## Workflow",
		"## Operating Principles",
		"## AC-Materialization Contract",
		"## Predicate Quality Requirements",
		"## Failure Modes",
		"## Output",
		"## Reflection Authoring",
	}
	for _, header := range required {
		if !acsassert.FileContains(t, path, header) {
			t.Errorf("section %q was dropped — Builder must only remove redundant prose, not real sections",
				header)
		}
	}
}

// acs-predicate: config-check
func TestC387_003_FrontmatterAndKeywordsPreserved(t *testing.T) {
	root := acsassert.RepoRoot(t)
	path := filepath.Join(root, "agents", "evolve-tdd-engineer.md")
	required := []string{
		"perspective:",
		"output-format:",
		"//go:build acs",
		"acsassert",
		"FORBIDDEN",
		"RED",
		".evolve/evals/",
	}
	for _, kw := range required {
		if !acsassert.FileContains(t, path, kw) {
			t.Errorf("keyword/field %q is missing after trim — Builder must preserve all behavioral keywords and frontmatter fields",
				kw)
		}
	}
}

func TestC387_004_FileIsGitTracked(t *testing.T) {
	root := acsassert.RepoRoot(t)
	relPath := filepath.Join("agents", "evolve-tdd-engineer.md")
	_, _, code, err := acsassert.SubprocessOutput("git", "-C", root, "ls-files", "--error-unmatch", relPath)
	if err != nil || code != 0 {
		t.Errorf("RED: agents/evolve-tdd-engineer.md is not tracked by git (exit=%d: %v).\n"+
			"Builder's edit must remain within the git-tracked file — do not move or gitignore it.",
			code, err)
	}
}
