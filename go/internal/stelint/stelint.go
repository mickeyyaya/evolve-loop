// Package stelint checks documents and Go log and error text against the ASD-STE100 house rules.
package stelint

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	RuleSentence         = "STE-SENTENCE"
	RuleParagraph        = "STE-PARAGRAPH"
	RuleWord             = "STE-WORD"
	RuleSkippedGenerated = "STE-SKIPPED-GENERATED"
	StandardPath         = "docs/conventions/ste100-writing.md"
	WordTableHeading     = "House list of words to replace"
)

type Substitution struct {
	Phrase      string
	Replacement string
}

type Options struct {
	Words      []Substitution
	IsStandard bool
}

type Finding struct {
	Line    int    `json:"line"`
	Rule    string `json:"rule"`
	Message string `json:"message"`
	Excerpt string `json:"excerpt"`
}

func (f Finding) IsSkipNotice() bool {
	return f.Rule == RuleSkippedGenerated
}

var rootDocs = setOf([]string{"README.md", "CLAUDE.md", "AGENTS.md", "CHANGELOG.md"})

func normalize(rel string) string {
	return strings.TrimPrefix(filepath.ToSlash(strings.TrimSpace(rel)), "./")
}

func InDocsScope(rel string) bool {
	p := normalize(rel)
	return rootDocs[p] || (strings.HasPrefix(p, "docs/") && strings.HasSuffix(p, ".md"))
}

func InGoScope(rel string) bool {
	p := normalize(rel)
	return strings.HasPrefix(p, "go/") && strings.HasSuffix(p, ".go") && !strings.HasSuffix(p, "_test.go") &&
		!strings.HasPrefix(p, "go/acs/") && !strings.Contains(p, "/testdata/")
}

func LoadStandard(root string) ([]Substitution, error) {
	raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(StandardPath)))
	if err != nil {
		return nil, fmt.Errorf("read the standard: %w", err)
	}
	words, err := ParseWordTable(raw)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", StandardPath, err)
	}
	return words, nil
}

func LintFile(path string, opt Options) ([]Finding, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if strings.HasSuffix(path, ".go") {
		findings, err := CheckGo(raw, opt)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", path, err)
		}
		return findings, nil
	}
	opt.IsStandard = opt.IsStandard || isStandardPath(path)
	return Check(raw, opt), nil
}

func isStandardPath(path string) bool {
	p := filepath.ToSlash(path)
	return p == StandardPath || strings.HasSuffix(p, "/"+StandardPath)
}
