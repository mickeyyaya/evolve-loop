// Package prompts loads agent and skill markdown docs, parses their
// frontmatter, and strips their on-demand reference tails.
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

type Loader struct {
	fs fs.FS
}

type Prompt struct {
	Name        string
	Frontmatter map[string]any
	Body        string
	Raw         string
}

func NewFromFS(fsys fs.FS) *Loader { return &Loader{fs: fsys} }

func NewFromDir(dir string) *Loader {
	if dir == "" {
		return &Loader{}
	}
	return &Loader{fs: os.DirFS(dir)}
}

func NewForProject(projectRoot string) *Loader {
	return NewFromDir(projectRoot)
}

func (l *Loader) Agent(name string) (Prompt, error) {
	return l.load(path.Join("agents", name+".md"), name)
}

func (l *Loader) Skill(name string) (Prompt, error) {
	return l.load(path.Join("skills", name, "SKILL.md"), name)
}

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
		if _, err := fs.Stat(l.fs, path.Join("skills", e.Name(), "SKILL.md")); err != nil {
			continue
		}
		out = append(out, e.Name())
	}
	sort.Strings(out)
	return out, nil
}

var ErrNoSource = errors.New("prompts: no source configured")

func (l *Loader) load(p, name string) (Prompt, error) {
	if l.fs == nil {
		return Prompt{}, fmt.Errorf("prompts: %w (%w)", fs.ErrNotExist, ErrNoSource)
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

func StripOnDemandSections(body string) string {
	offset := 0
	for _, line := range strings.SplitAfter(body, "\n") {
		trimmed := strings.TrimRight(line, "\r\n")
		if trimmed == "## Reference Index" || strings.HasPrefix(trimmed, "## Reference Index ") {
			return body[:offset]
		}
		offset += len(line)
	}
	return body
}

func ParseFrontmatter(raw string) (map[string]any, string, error) {
	if !strings.HasPrefix(raw, "---\n") && !strings.HasPrefix(raw, "---\r\n") {
		return nil, raw, nil
	}
	rest := strings.TrimPrefix(raw, "---\n")
	rest = strings.TrimPrefix(rest, "---\r\n")
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

func parseValue(v string) any {
	if v == "" {
		return ""
	}
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

func unquote(s string) string {
	if len(s) >= 2 {
		first, last := s[0], s[len(s)-1]
		if (first == '"' && last == '"') || (first == '\'' && last == '\'') {
			return s[1 : len(s)-1]
		}
	}
	return s
}

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
