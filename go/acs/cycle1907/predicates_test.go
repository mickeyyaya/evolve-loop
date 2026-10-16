//go:build acs

package cycle1907

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const conventionDoc = "docs/conventions/markdown-structure.md"

func readConvention(t *testing.T) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(acsassert.RepoRoot(t), conventionDoc))
	if err != nil {
		t.Fatalf("RED: %s unreadable: %v", conventionDoc, err)
	}
	return string(b)
}

func bodyAfterBanner(doc string) string {
	if idx := strings.Index(doc, "\n## "); idx >= 0 {
		return doc[idx:]
	}
	return ""
}

func sectionOf(doc, heading string) string {
	idx := strings.Index(doc, "\n"+heading+"\n")
	if idx < 0 {
		return ""
	}
	section := doc[idx+len(heading)+2:]
	if next := strings.Index(section, "\n## "); next >= 0 {
		section = section[:next]
	}
	return section
}

func TestC1907_001_BodyHasNoUnscopedFrontmatterOrTLDRMandate(t *testing.T) {
	body := strings.ToLower(bodyAfterBanner(readConvention(t)))
	if body == "" {
		t.Fatalf("RED: %s has no body sections", conventionDoc)
	}
	for _, contradiction := range []string{
		"frontmatter is mandatory",
		"all markdown files must follow this schema",
		"every markdown file must start with a yaml frontmatter",
		"every file requires a tldr",
		"every file requires it",
	} {
		if strings.Contains(body, contradiction) {
			t.Errorf("RED: body still states the unscoped mandate %q that the ADR-0132 banner revokes", contradiction)
		}
	}
}

func TestC1907_002_FrontmatterAndTLDRScopedToSkillsAndAgents(t *testing.T) {
	doc := readConvention(t)
	if !strings.Contains(doc, "0132-documents-are-written-through-one-skill.md") {
		t.Errorf("RED: the ADR-0132 link was lost")
	}
	for _, heading := range []string{"## Frontmatter Block", "## TLDR Section"} {
		section := strings.ToLower(sectionOf(doc, heading))
		if section == "" {
			t.Errorf("RED: section %q is missing", heading)
			continue
		}
		if !strings.Contains(section, "skill") || !strings.Contains(section, "agent") {
			t.Errorf("RED: section %q does not scope its rule to skill and agent files", heading)
		}
		if !strings.Contains(section, "docs/") {
			t.Errorf("RED: section %q does not state the rule for documents under docs/", heading)
		}
	}
}

func TestC1907_003_DocsUnderDocsStartWithLinksAndStatusLine(t *testing.T) {
	body := strings.ToLower(bodyAfterBanner(readConvention(t)))
	if !strings.Contains(body, "status line") {
		t.Errorf("RED: body does not state that a docs/ document starts with a status line")
	}
	template := strings.ToLower(sectionOf(readConvention(t), "## Full Template"))
	if template == "" {
		t.Fatalf("RED: section ## Full Template is missing")
	}
	if !strings.Contains(template, "status") {
		t.Errorf("RED: the Full Template gives no status line for a docs/ document")
	}
}

func TestC1907_004_TOCRuleOver100LinesIsKept(t *testing.T) {
	toc := strings.ToLower(sectionOf(readConvention(t), "## Table of Contents Rule"))
	if toc == "" {
		t.Fatalf("section ## Table of Contents Rule was removed")
	}
	if !strings.Contains(toc, "100 lines") {
		t.Errorf("the table-of-contents threshold of 100 lines was lost")
	}
}

func TestC1907_005_ConventionPassesSteLint(t *testing.T) {
	root := acsassert.RepoRoot(t)
	out, errOut, code, err := acsassert.SubprocessOutput("go", "-C", filepath.Join(root, "go"), "run", "./cmd/evolve",
		"docs", "ste-lint", "--strict", "--project-root", root, conventionDoc)
	if err != nil {
		t.Fatalf("ste-lint did not run: %v", err)
	}
	if code != 0 || !strings.Contains(out, "ste-lint: 0 finding(s)") {
		t.Errorf("ste-lint rc=%d out=%q stderr=%q", code, out, errOut)
	}
}
