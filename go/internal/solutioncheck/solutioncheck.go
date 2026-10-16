// Package solutioncheck is the ONE deterministic, LLM-free judge of a
// document cycle's deliverable shape.
package solutioncheck

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
)

var slugRE = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)

type Spec = config.DeliverableKindSpec

type Failure struct {
	Path   string
	Reason string
}

func (f Failure) String() string { return f.Path + ": " + f.Reason }

func Check(tree, slug string, spec Spec) []Failure {
	if !slugRE.MatchString(slug) {
		return []Failure{{Path: filepath.ToSlash(filepath.Join(spec.Root, slug)), Reason: fmt.Sprintf("slug %q is not a task id (want ^[a-z0-9][a-z0-9-]*$) — the deliverable directory is named by the inbox item id", slug)}}
	}
	root := spec.Root
	rel := filepath.ToSlash(filepath.Join(root, slug))
	dir := filepath.Join(tree, root, slug)
	if info, err := os.Stat(dir); err != nil || !info.IsDir() {
		return []Failure{{Path: rel, Reason: fmt.Sprintf("solution directory %s is missing — a document cycle delivers its options and recommendation there", rel)}}
	}
	var out []Failure
	placeholders := placeholderPatterns(spec.ForbidPlaceholders)

	options, _ := filepath.Glob(filepath.Join(dir, "options", "*.md"))
	sort.Strings(options)
	if len(options) < spec.MinOptions {
		out = append(out, Failure{Path: rel + "/options", Reason: fmt.Sprintf("found %d option file(s), need at least %d — each candidate strategy is its own options/<n>-<name>.md", len(options), spec.MinOptions)})
	}
	for _, p := range options {
		out = append(out, checkDocument(tree, p, nil, placeholders, spec.EvidenceFile)...)
	}
	for _, name := range spec.RequiredFiles {
		p := filepath.Join(dir, name)
		if _, err := os.Stat(p); err != nil {
			out = append(out, Failure{Path: rel + "/" + name, Reason: "required file missing"})
			continue
		}
		evidence := spec.EvidenceFile
		if name == spec.EvidenceFile {
			evidence = ""
		}
		out = append(out, checkDocument(tree, p, spec.RequiredSections[name], placeholders, evidence)...)
	}
	return out
}

func checkDocument(tree, path string, sections []string, placeholders []*regexp.Regexp, evidence string) []Failure {
	rel, _ := filepath.Rel(tree, path)
	rel = filepath.ToSlash(rel)
	raw, err := os.ReadFile(path)
	if err != nil {
		return []Failure{{Path: rel, Reason: "unreadable: " + err.Error()}}
	}
	md := string(raw)
	if strings.TrimSpace(md) == "" {
		return []Failure{{Path: rel, Reason: "empty file"}}
	}
	var out []Failure
	for _, s := range sections {
		if !hasHeading(md, s) {
			out = append(out, Failure{Path: rel, Reason: fmt.Sprintf("required section %q missing (a markdown heading)", s)})
		}
	}
	for _, re := range placeholders {
		if loc := re.FindStringIndex(md); loc != nil {
			out = append(out, Failure{Path: rel, Reason: fmt.Sprintf("placeholder %q left behind", md[loc[0]:loc[1]])})
			break
		}
	}
	if evidence != "" && !strings.Contains(md, evidence) {
		out = append(out, Failure{Path: rel, Reason: fmt.Sprintf("does not cite %s — every number must trace to an assumption or evidence entry", evidence)})
	}
	return out
}

func hasHeading(md, title string) bool {
	var fenceChar byte
	fenceLen := 0
	for _, line := range strings.Split(md, "\n") {
		t := strings.TrimSpace(line)
		if c, n := fenceRun(t); n >= 3 {
			switch {
			case fenceChar == 0:
				fenceChar, fenceLen = c, n
			case c == fenceChar && n >= fenceLen:
				fenceChar, fenceLen = 0, 0
			}
			continue
		}
		if fenceChar != 0 || strings.HasPrefix(line, "    ") || strings.HasPrefix(line, "\t") {
			continue
		}
		if !strings.HasPrefix(t, "##") {
			continue
		}
		if strings.EqualFold(strings.TrimSpace(strings.TrimLeft(t, "#")), strings.TrimSpace(title)) {
			return true
		}
	}
	return false
}

func placeholderPatterns(tokens []string) []*regexp.Regexp {
	out := make([]*regexp.Regexp, 0, len(tokens))
	for _, tok := range tokens {
		if strings.TrimSpace(tok) == "" {
			continue
		}
		out = append(out, regexp.MustCompile(`(?i)\b`+regexp.QuoteMeta(strings.TrimSpace(tok))+`\b`))
	}
	return out
}

func Describe(spec Spec) string {
	if spec.Root == "" && len(spec.RequiredFiles) == 0 && spec.MinOptions == 0 {
		return ""
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%s/<slug>/ with at least %d candidate strategies under options/<n>-<name>.md", spec.Root, spec.MinOptions)
	if len(spec.RequiredFiles) > 0 {
		b.WriteString("; required files: ")
		for i, f := range spec.RequiredFiles {
			if i > 0 {
				b.WriteString(", ")
			}
			b.WriteString(f)
			if secs := spec.RequiredSections[f]; len(secs) > 0 {
				fmt.Fprintf(&b, " (sections: %s)", strings.Join(secs, ", "))
			}
		}
	}
	if spec.EvidenceFile != "" {
		fmt.Fprintf(&b, "; every option and the recommendation must cite %s", spec.EvidenceFile)
	}
	if len(spec.ForbidPlaceholders) > 0 {
		fmt.Fprintf(&b, "; placeholders forbidden: %s", strings.Join(spec.ForbidPlaceholders, ", "))
	}
	fmt.Fprintf(&b, ". Self-check: `evolve solution check %s/<slug>`.", spec.Root)
	return b.String()
}

func fenceRun(t string) (byte, int) {
	if t == "" || (t[0] != '`' && t[0] != '~') {
		return 0, 0
	}
	c := t[0]
	n := 0
	for n < len(t) && t[n] == c {
		n++
	}
	return c, n
}
