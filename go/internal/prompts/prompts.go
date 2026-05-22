// Package prompts loads agent and skill markdown with YAML frontmatter.
//
// The loader is fs.FS-backed so it can serve from three sources
// without API churn:
//
//  1. fstest.MapFS — unit tests
//  2. os.DirFS    — dev override at $EVOLVE_PROMPTS_DIR
//  3. embed.FS    — Phase 3 vendored copy of agents/ + skills/
//
// Plan §1 decision #13 wires the embed path; this Phase 2 layer
// commits to the fs.FS surface so the Phase 3 swap is one line at the
// orchestrator wire-up site.
//
// The frontmatter parser is intentionally minimal — it handles only
// the shapes observed in agents/*.md and skills/*/SKILL.md (flat
// key-value, inline bracketed arrays, quoted strings). Loading any of
// the existing 25 agent files must succeed; adding a full YAML
// dependency would inflate the binary for no gain.
package prompts

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"sort"
	"strings"
)

// Loader resolves agent/skill names to parsed Prompt values.
//
// A zero loader is valid; every read returns fs.ErrNotExist. Construct
// via NewFromFS (any filesystem) or NewFromDir (a directory path; empty
// path yields the zero loader).
type Loader struct {
	fs fs.FS
}

// Prompt is the parsed form of an agent or skill .md file.
type Prompt struct {
	Name        string
	Frontmatter map[string]any
	Body        string
	Raw         string
}

// NewFromFS constructs a Loader backed by any fs.FS. Pass nil to get
// the zero loader (every read returns fs.ErrNotExist).
func NewFromFS(fsys fs.FS) *Loader { return &Loader{fs: fsys} }

// NewFromDir constructs a Loader rooted at the given directory. Empty
// path returns the zero loader — caller is responsible for combining
// with an embed.FS fallback in Phase 3.
func NewFromDir(dir string) *Loader {
	if dir == "" {
		return &Loader{}
	}
	return &Loader{fs: os.DirFS(dir)}
}

// Agent reads agents/<name>.md and parses its frontmatter.
func (l *Loader) Agent(name string) (Prompt, error) {
	return l.load(path.Join("agents", name+".md"), name)
}

// Skill reads skills/<name>/SKILL.md and parses its frontmatter.
func (l *Loader) Skill(name string) (Prompt, error) {
	return l.load(path.Join("skills", name, "SKILL.md"), name)
}

// Agents enumerates agent file names (without .md extension), sorted.
func (l *Loader) Agents() ([]string, error) {
	if l.fs == nil {
		return nil, nil
	}
	entries, err := fs.ReadDir(l.fs, "agents")
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(entries))
	for _, e := range entries {
		n := e.Name()
		if e.IsDir() || !strings.HasSuffix(n, ".md") {
			continue
		}
		out = append(out, strings.TrimSuffix(n, ".md"))
	}
	sort.Strings(out)
	return out, nil
}

// Skills enumerates skill directory names that contain a SKILL.md,
// sorted. Directories without SKILL.md are omitted.
func (l *Loader) Skills() ([]string, error) {
	if l.fs == nil {
		return nil, nil
	}
	entries, err := fs.ReadDir(l.fs, "skills")
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(entries))
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		// Only include directories that contain SKILL.md.
		if _, err := fs.Stat(l.fs, path.Join("skills", e.Name(), "SKILL.md")); err != nil {
			continue
		}
		out = append(out, e.Name())
	}
	sort.Strings(out)
	return out, nil
}

// load reads, parses, and packages a single prompt.
func (l *Loader) load(p, name string) (Prompt, error) {
	if l.fs == nil {
		return Prompt{}, fmt.Errorf("prompts: %w (no source configured)", fs.ErrNotExist)
	}
	raw, err := fs.ReadFile(l.fs, p)
	if err != nil {
		return Prompt{}, fmt.Errorf("prompts: read %s: %w", p, err)
	}
	fm, body, err := ParseFrontmatter(string(raw))
	if err != nil {
		return Prompt{}, fmt.Errorf("prompts: parse %s: %w", p, err)
	}
	return Prompt{
		Name:        name,
		Frontmatter: fm,
		Body:        body,
		Raw:         string(raw),
	}, nil
}

// ParseFrontmatter splits a raw .md file into (frontmatter map, body).
//
// Behavior:
//
//   - No leading "---\n"            → (nil, full content, nil)
//   - "---\n...\n---\n"             → (parsed map, content after fence, nil)
//   - Opening fence with no close   → (nil, "", error)
//
// The parser handles three line shapes inside the block:
//
//   - "key: value"
//   - "key: \"quoted value\"" / "key: 'quoted'"
//   - "key: [a, b, \"c\"]"  (inline array → []string)
//
// Blank lines and lines starting with '#' are skipped. The first ':'
// on each line splits the key from the value; remaining colons are
// preserved (so "description: Phase 2: do Y" parses correctly).
func ParseFrontmatter(raw string) (map[string]any, string, error) {
	if !strings.HasPrefix(raw, "---\n") && !strings.HasPrefix(raw, "---\r\n") {
		return nil, raw, nil
	}
	// Skip the opening fence.
	rest := strings.TrimPrefix(raw, "---\n")
	rest = strings.TrimPrefix(rest, "---\r\n")
	// Find the closing fence: a line that is exactly "---".
	end := -1
	lines := strings.Split(rest, "\n")
	for i, line := range lines {
		l := strings.TrimRight(line, "\r")
		if l == "---" {
			end = i
			break
		}
	}
	if end == -1 {
		return nil, "", errors.New("unterminated frontmatter block")
	}
	fm := make(map[string]any)
	for _, line := range lines[:end] {
		line = strings.TrimRight(line, "\r")
		trim := strings.TrimSpace(line)
		if trim == "" || strings.HasPrefix(trim, "#") {
			continue
		}
		colon := strings.Index(line, ":")
		if colon < 0 {
			continue
		}
		key := strings.TrimSpace(line[:colon])
		val := strings.TrimSpace(line[colon+1:])
		if key == "" {
			continue
		}
		fm[key] = parseValue(val)
	}
	body := strings.Join(lines[end+1:], "\n")
	return fm, body, nil
}

// parseValue interprets a single frontmatter value.
func parseValue(v string) any {
	if v == "" {
		return ""
	}
	// Inline array.
	if strings.HasPrefix(v, "[") && strings.HasSuffix(v, "]") {
		inner := strings.TrimSpace(v[1 : len(v)-1])
		if inner == "" {
			return []string{}
		}
		parts := splitArray(inner)
		out := make([]string, 0, len(parts))
		for _, p := range parts {
			out = append(out, unquote(strings.TrimSpace(p)))
		}
		return out
	}
	return unquote(v)
}

// unquote strips matched outer quotes (single or double).
func unquote(s string) string {
	if len(s) >= 2 {
		first, last := s[0], s[len(s)-1]
		if (first == '"' && last == '"') || (first == '\'' && last == '\'') {
			return s[1 : len(s)-1]
		}
	}
	return s
}

// splitArray splits an inline-array body on commas, respecting quoted
// elements. Minimal — covers the patterns in the existing agent files;
// not a full CSV parser.
func splitArray(s string) []string {
	out := []string{}
	cur := strings.Builder{}
	inQuote := byte(0)
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case inQuote != 0:
			cur.WriteByte(c)
			if c == inQuote {
				inQuote = 0
			}
		case c == '"' || c == '\'':
			cur.WriteByte(c)
			inQuote = c
		case c == ',':
			out = append(out, cur.String())
			cur.Reset()
		default:
			cur.WriteByte(c)
		}
	}
	if cur.Len() > 0 {
		out = append(out, cur.String())
	}
	return out
}
