// Package solutioncheck is the ONE deterministic, LLM-free judge of a document
// cycle's deliverable shape (ADR-0099 slice 2). A `document` cycle delivers
// solutions/<slug>/ — at least N candidate options, a recommendation and an
// assumptions-and-evidence file — and this engine is projected three ways: the
// build handoff floor (core), the `evolve solution check` CLI (the eval [code]
// grader and the agent's self-check) and the audit gate line. The contract
// itself is config (phase-registry.json:config.deliverable_kinds); judgment of
// QUALITY (rubrics, evidence strength) stays in the audit phase — never here.
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

// slugRE is the task-id vocabulary every id-consuming site in the tree enforces
// (triagecap, evalgate, phasespec); a slug outside it is never joined into a path.
var slugRE = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)

// Spec is the document deliverable contract — declared once in the registry
// (config.DeliverableKindSpec), aliased here so the engine and its consumers
// share one type.
type Spec = config.DeliverableKindSpec

// Failure is one deterministic contract violation: the repo-relative path it
// was found at and the reason, rendered "path: reason" by String.
type Failure struct {
	Path   string
	Reason string
}

// String renders the failure the way the build floor and the CLI print it.
func (f Failure) String() string { return f.Path + ": " + f.Reason }

// Check judges solutions/<slug>/ under tree against spec and returns every
// violation (empty = the deliverable is well-formed). An absent solution
// directory is one loud failure; every other violation is reported
// independently so a single re-dispatch can fix them all.
func Check(tree, slug string, spec Spec) []Failure {
	// The slug names ONE directory under the root: the inbox id vocabulary
	// (lowercase, digits, hyphens). Anything else — a path segment, `..`, an
	// uppercase or underscore id — is a contract violation here, at the single
	// engine, so no caller has to remember to confine it.
	if !slugRE.MatchString(slug) {
		return []Failure{{Path: filepath.ToSlash(filepath.Join(spec.Root, slug)), Reason: fmt.Sprintf("slug %q is not a task id (want ^[a-z0-9][a-z0-9-]*$) — the deliverable directory is named by the inbox item id", slug)}}
	}
	root := spec.Root // the registry is the SSOT; an empty root is a config load warning, never a runtime default
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
			evidence = "" // the evidence file cites nothing; it IS the source
		}
		out = append(out, checkDocument(tree, p, spec.RequiredSections[name], placeholders, evidence)...)
	}
	return out
}

// checkDocument applies the per-file rules: non-empty, required sections
// present, no placeholder left behind, and (when evidence is named) a citation
// of the evidence file so every number traces to an assumption or evidence entry.
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

// hasHeading reports whether md has a SECTION heading (level two or deeper —
// the H1 is the document's title, never a section) whose text equals title
// (case-insensitive, surrounding whitespace ignored). Lines inside fenced code
// blocks and indented code (four spaces or a tab — CommonMark's code block) are
// NOT headings: the words of a required section pasted inside a fence must
// not satisfy the structural check.
func hasHeading(md, title string) bool {
	var fenceChar byte // 0 = not in a fence
	fenceLen := 0
	for _, line := range strings.Split(md, "\n") {
		t := strings.TrimSpace(line)
		if c, n := fenceRun(t); n >= 3 {
			// CommonMark: a fence closes only on a run of the SAME character at
			// least as long as the opener; anything else is fence content.
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

// placeholderPatterns compiles each forbidden token as a whole-word,
// case-insensitive match.
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

// Describe renders the contract the way the Task Contract block hands it to
// the builder and the auditor — ONE renderer over the registry spec, so the
// words the builder is told and the shape the floor judges can never drift.
// A zero spec (no document contract declared) renders "".
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

// fenceRun reports the fence character ('`' or '~') and the length of the run
// that opens the trimmed line; n < 3 means the line is not a fence marker.
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
