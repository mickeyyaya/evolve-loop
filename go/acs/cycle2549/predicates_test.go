//go:build acs

// Package cycle2549 holds the predicates of the harness removal test design.
package cycle2549

import (
	"go/parser"
	"go/token"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	designRel   = "docs/plans/harness-removal-test-2026-11.md"
	journeyFile = "cmd/evolve/cmd_docs_removal_design_test.go"
)

func removalDesignTests(t *testing.T, names ...string) {
	t.Helper()
	acsassert.GoTests(t, acsassert.GoTestSpec{
		Dir:     acsassert.GoModuleDir(t),
		Package: "./cmd/evolve",
		Pattern: "^(" + strings.Join(names, "|") + ")$",
		Names:   names,
	})
}

func TestC2549_001_TheDesignNamesTheComponentTheWindowTheMeasureAndTheStopRule(t *testing.T) {
	removalDesignTests(t,
		"TestRemovalDesign_TheCheckerAcceptsAFullDesign",
		"TestRemovalDesign_TheCheckerNamesEachMissingSection",
		"TestRemovalDesign_TheCheckerRefusesAnEmptySectionAndMissingParts",
		"TestRemovalDesign_TheCheckerReadsNumberedHeadings",
		"TestRemovalDesign_TheDesignNamesTheComponentTheWindowTheMeasureAndTheStopRule",
		"TestRemovalDesign_ThePlanLinksItsR9RowToTheDesign",
	)
}

func TestC2549_002_TheDesignWaitsForTheM2BaselineOfADR0133(t *testing.T) {
	removalDesignTests(t,
		"TestRemovalDesign_TheCheckerRefusesAnEmptySectionAndMissingParts",
		"TestRemovalDesign_TheDesignWaitsForTheM2BaselineOfADR0133",
	)
}

func TestC2549_003_TheDesignPassesSTELintThroughTheRealEntryPoint(t *testing.T) {
	root := acsassert.RepoRoot(t)
	design := filepath.Join(root, filepath.FromSlash(designRel))
	if !acsassert.FileExists(t, design) {
		t.Fatalf("RED: %s missing on disk", designRel)
	}
	if _, _, code, _ := acsassert.SubprocessOutput("git", "-C", root, "check-ignore", "-q", designRel); code == 0 {
		t.Errorf("RED: %s is gitignored and would be dropped at ship", designRel)
	}
	stdout, stderr, code, err := acsassert.SubprocessOutput("go", "-C", acsassert.GoModuleDir(t), "run", "./cmd/evolve",
		"docs", "ste-lint", "--strict", "--project-root", root, design)
	if code != 0 {
		t.Fatalf("RED: evolve docs ste-lint --strict exit %d (%v)\nstdout:\n%s\nstderr:\n%s", code, err, stdout, acsassert.OutputTail(stderr, 20))
	}
	if !strings.Contains(stdout, "1 file(s) checked") {
		t.Errorf("RED: the lint did not check the design: %q", stdout)
	}
}

func TestC2549_004_TheJourneyTestRunsThroughTheDocsEntryPointAndHoldsNoComment(t *testing.T) {
	removalDesignTests(t, "TestRemovalDesign_TheDocsEntryPointLintsTheDesignInSTE")
	journey := filepath.Join(acsassert.GoModuleDir(t), filepath.FromSlash(journeyFile))
	parsed, err := parser.ParseFile(token.NewFileSet(), journey, nil, parser.ParseComments)
	if err != nil {
		t.Fatalf("RED: parse %s: %v", journeyFile, err)
	}
	if len(parsed.Comments) != 0 {
		t.Errorf("RED: %s holds %d comment group(s); code has no comments", journeyFile, len(parsed.Comments))
	}
}
