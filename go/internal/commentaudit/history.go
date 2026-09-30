package commentaudit

import (
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"maps"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
)

type HistoryEntry struct {
	File   string
	Line   int
	Anchor string
	Lines  []string
}

const (
	maxAnchorRunes    = 120
	historyArchiveDir = "docs/history/code-comments"
	historyIndex      = "README.md"
	pageTitlePrefix   = "# Comment history: `"
	pageHeader        = pageTitlePrefix + "%s`\n\nThe history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).\n"
	indexHeader       = "# Comment history archive\n\nThe history the code's comments carried, by the rule `commentaudit check` uses, kept when a change removes those comments. `commentaudit history -base <ref> -label <change> -out " + historyArchiveDir + "` appends one section per change to each package's page and rewrites this index from the pages, so nothing here is edited by hand. See [the code-comments convention](../../conventions/code-comments.md).\n\n| Package | Entries | Page |\n|---|---|---|\n"
)

func RemovedHistoryAcrossDiff(files []string, before, after func(string) ([]byte, error)) ([]HistoryEntry, error) {
	var removed []HistoryEntry
	addedElsewhere := map[string]int{}
	for _, f := range files {
		if !strings.HasSuffix(f, ".go") || isOutsideProjectCode(f) {
			continue
		}
		gone, arrived, err := unmatchedHistory(f, before, after)
		if err != nil {
			return nil, err
		}
		removed = append(removed, gone...)
		for _, g := range arrived {
			addedElsewhere[textKey(g)]++
		}
	}
	var entries []HistoryEntry
	for _, g := range removed {
		if addedElsewhere[textKey(g)] > 0 {
			addedElsewhere[textKey(g)]--
			continue
		}
		entries = append(entries, g)
	}
	return entries, nil
}

func unmatchedHistory(name string, before, after func(string) ([]byte, error)) (gone, arrived []HistoryEntry, err error) {
	b, a, err := readBoth(before, after, name)
	if err != nil {
		return nil, nil, err
	}
	was, err := historyGroups(name, b)
	if err != nil {
		return nil, nil, err
	}
	now, err := historyGroups(name, a)
	if err != nil {
		return nil, nil, err
	}
	was, now = unmatched(was, now, func(g HistoryEntry) string { return textKey(g) + "\x00" + g.Anchor })
	was, now = unmatched(was, now, textKey)
	return was, now, nil
}

func unmatched(was, now []HistoryEntry, key func(HistoryEntry) string) (restWas, restNow []HistoryEntry) {
	return without(was, now, key), without(now, was, key)
}

func without(entries, others []HistoryEntry, key func(HistoryEntry) string) []HistoryEntry {
	pending := map[string]int{}
	for _, g := range others {
		pending[key(g)]++
	}
	var rest []HistoryEntry
	for _, g := range entries {
		if pending[key(g)] > 0 {
			pending[key(g)]--
			continue
		}
		rest = append(rest, g)
	}
	return rest
}

func textKey(g HistoryEntry) string {
	return strings.Join(g.Lines, "\n")
}

type parsedSource struct {
	lines         []string
	wholeComment  map[int]bool
	trailingStart map[int]int
}

func historyGroups(name string, src []byte) ([]HistoryEntry, error) {
	if src == nil {
		return nil, nil
	}
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, name, src, parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", name, err)
	}
	source := newParsedSource(string(src), fset, file)
	var groups []HistoryEntry
	for _, group := range file.Comments {
		lines := groupLines(group)
		if !slices.ContainsFunc(lines, isNarrative) {
			continue
		}
		start := fset.Position(group.Pos())
		groups = append(groups, HistoryEntry{File: name, Line: start.Line, Anchor: source.anchor(start, fset.Position(group.End()).Line), Lines: lines})
	}
	return groups, nil
}

func groupLines(group *ast.CommentGroup) []string {
	var lines []string
	for _, c := range group.List {
		for _, line := range strings.Split(c.Text, "\n") {
			lines = append(lines, strings.TrimSpace(line))
		}
	}
	return lines
}

func newParsedSource(src string, fset *token.FileSet, file *ast.File) parsedSource {
	s := parsedSource{lines: strings.Split(src, "\n"), wholeComment: map[int]bool{}, trailingStart: map[int]int{}}
	for _, group := range file.Comments {
		for _, c := range group.List {
			start, end := fset.Position(c.Pos()), fset.Position(c.End())
			if strings.TrimSpace(s.lines[start.Line-1][:start.Column-1]) != "" {
				s.trailingStart[start.Line] = start.Column
			} else {
				s.wholeComment[start.Line] = true
			}
			for line := start.Line + 1; line <= end.Line; line++ {
				s.wholeComment[line] = true
			}
		}
	}
	return s
}

func (s parsedSource) anchor(start token.Position, endLine int) string {
	if code := strings.TrimSpace(s.lines[start.Line-1][:start.Column-1]); code != "" {
		return clip(code)
	}
	for i := endLine; i < len(s.lines); i++ {
		if strings.TrimSpace(s.lines[i]) != "" && !s.wholeComment[i+1] {
			return clip(s.code(i + 1))
		}
	}
	return ""
}

func (s parsedSource) code(line int) string {
	text := s.lines[line-1]
	if col, ok := s.trailingStart[line]; ok {
		text = text[:col-1]
	}
	return strings.TrimSpace(text)
}

func clip(s string) string {
	runes := []rune(s)
	if len(runes) <= maxAnchorRunes {
		return s
	}
	return string(runes[:maxAnchorRunes]) + "…"
}

func writeHistoryArchive(dir, label string, entries []HistoryEntry) ([]string, error) {
	if strings.ContainsAny(label, "\r\n") {
		return nil, fmt.Errorf("history label %q spans lines; a section title is one line", label)
	}
	byPage := map[string][]HistoryEntry{}
	for _, e := range entries {
		page := filepath.Join(dir, strings.ReplaceAll(packageOf(e.File), "/", "-")+".md")
		byPage[page] = append(byPage[page], e)
	}
	pages := slices.Sorted(maps.Keys(byPage))
	if err := refuseRecordedLabel(pages, label); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("history archive %s: %w", dir, err)
	}
	var written []string
	for _, page := range pages {
		if err := appendHistorySection(page, label, byPage[page]); err != nil {
			return written, errors.Join(err, writeHistoryIndex(dir))
		}
		written = append(written, page)
	}
	return written, writeHistoryIndex(dir)
}

func packageOf(file string) string {
	dir := path.Dir(filepath.ToSlash(file))
	if dir == "." || dir == "go" {
		return "root"
	}
	return strings.TrimPrefix(dir, "go/")
}

func refuseRecordedLabel(pages []string, label string) error {
	for _, page := range pages {
		body, err := os.ReadFile(page)
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			return fmt.Errorf("history archive %s: %w", page, err)
		}
		if strings.Contains(string(body), "\n## "+label+"\n") {
			return fmt.Errorf("history archive %s already has a section %q; a change is recorded once", page, label)
		}
	}
	return nil
}

func appendHistorySection(page, label string, entries []HistoryEntry) error {
	var body strings.Builder
	if _, err := os.Stat(page); errors.Is(err, fs.ErrNotExist) {
		fmt.Fprintf(&body, pageHeader, packageOf(entries[0].File))
	}
	body.WriteString(RenderHistorySection(label, entries))
	if err := appendFile(page, body.String()); err != nil {
		return fmt.Errorf("history archive %s: %w", page, err)
	}
	return nil
}

func appendFile(name, text string) error {
	f, err := os.OpenFile(name, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	if _, err := f.WriteString(text); err != nil {
		_ = f.Close()
		return err
	}
	return f.Close()
}

func writeHistoryIndex(dir string) error {
	pages, err := filepath.Glob(filepath.Join(dir, "*.md"))
	if err != nil {
		return err
	}
	var index strings.Builder
	index.WriteString(indexHeader)
	for _, page := range pages {
		name := filepath.Base(page)
		body, err := os.ReadFile(page)
		if err != nil {
			return fmt.Errorf("history index %s: %w", page, err)
		}
		title, isPage := pageTitle(string(body))
		if !isPage {
			continue
		}
		fmt.Fprintf(&index, "| `%s` | %d | [%s](%s) |\n", title, strings.Count(string(body), "\n### `"), name, name)
	}
	if err := os.WriteFile(filepath.Join(dir, historyIndex), []byte(index.String()), 0o644); err != nil {
		return fmt.Errorf("history index %s: %w", dir, err)
	}
	return nil
}

func pageTitle(body string) (string, bool) {
	firstLine, _, _ := strings.Cut(body, "\n")
	rest, isPage := strings.CutPrefix(firstLine, pageTitlePrefix)
	title, _, _ := strings.Cut(rest, "`")
	return title, isPage
}

func RenderHistorySection(label string, entries []HistoryEntry) string {
	var out strings.Builder
	fmt.Fprintf(&out, "\n## %s\n", label)
	for _, e := range entries {
		fmt.Fprintf(&out, "\n### `%s:%d`", e.File, e.Line)
		if e.Anchor != "" {
			fmt.Fprintf(&out, " — above `%s`", strings.ReplaceAll(e.Anchor, "`", "'"))
		}
		fence := strings.Repeat("`", max(3, longestBacktickRun(e.Lines)+1))
		fmt.Fprintf(&out, "\n\n%stext\n%s\n%s\n", fence, strings.Join(e.Lines, "\n"), fence)
	}
	return out.String()
}

func longestBacktickRun(lines []string) int {
	longest := 0
	for _, line := range lines {
		run := 0
		for _, r := range line {
			if r != '`' {
				run = 0
				continue
			}
			run++
			longest = max(longest, run)
		}
	}
	return longest
}
