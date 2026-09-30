package commentaudit

import (
	"bytes"
	"errors"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
)

const (
	MinPackageDocWords    = 6
	packageDocLineColumns = 77
)

var (
	sentenceEnd   = regexp.MustCompile(`\.\s+\p{Lu}`)
	abbreviations = []string{"e.g", "i.e", "etc", "vs", "cf"}
)

type StrippedFile struct {
	Source            []byte
	Removed           int
	PackageDocToWrite bool
}

type StripReport struct {
	Files       int
	Removed     int
	Failed      []string
	DocsToWrite []string
}

type packageDocRule int

const (
	dropPackageDoc packageDocRule = iota
	keepPackageDoc
)

type goSource struct {
	src  []byte
	fset *token.FileSet
	file *ast.File
}

type textEdit struct {
	from, to int
	text     string
}

func StripComments(name string, src []byte) (StrippedFile, error) {
	return stripComments(name, src, dropPackageDoc)
}

func StripCommentsKeepingPackageDoc(name string, src []byte) (StrippedFile, error) {
	return stripComments(name, src, keepPackageDoc)
}

func stripComments(name string, src []byte, docRule packageDocRule) (StrippedFile, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, name, src, parser.ParseComments|parser.SkipObjectResolution)
	if err != nil {
		return StrippedFile{}, err
	}
	source := goSource{src: src, fset: fset, file: file}
	if source.generated() {
		return StrippedFile{Source: src}, nil
	}
	edits, removed, docToWrite := source.commentEdits(docRule)
	if len(edits) == 0 {
		return StrippedFile{Source: src, PackageDocToWrite: docToWrite}, nil
	}
	out, err := format.Source(applyEdits(src, edits))
	if err != nil {
		return StrippedFile{}, fmt.Errorf("%s: stripped source does not format: %w", name, err)
	}
	if same, why, err := Equivalent(name, src, out); err != nil || !same {
		return StrippedFile{}, fmt.Errorf("%s: stripping would change more than comments (%s, %v)", name, why, err)
	}
	return StrippedFile{Source: out, Removed: removed, PackageDocToWrite: docToWrite}, nil
}

func (s goSource) generated() bool {
	for _, group := range s.file.Comments {
		for _, c := range group.List {
			if extent, ok := markerExtentOf(c.Text); ok && extent == markerFile {
				return true
			}
		}
	}
	return false
}

func (s goSource) commentEdits(docRule packageDocRule) (edits []textEdit, removed int, docToWrite bool) {
	preamble := cgoPreambleGroup(s.file)
	for _, group := range s.file.Comments {
		switch {
		case group == preamble || carriesGroupMarker(group):
		case group == s.file.Doc && docRule == keepPackageDoc:
			edit, lines, ok := s.shortenedPackageDoc(group)
			if ok && lines > 0 {
				edits = append(edits, edit)
				removed += lines
			}
			docToWrite = !ok
		default:
			groupEdits, n := s.removableComments(group)
			edits = append(edits, groupEdits...)
			removed += n
		}
	}
	return edits, removed, docToWrite
}

func carriesGroupMarker(group *ast.CommentGroup) bool {
	return slices.ContainsFunc(group.List, func(c *ast.Comment) bool {
		extent, ok := markerExtentOf(c.Text)
		return ok && extent == markerGroup
	})
}

func (s goSource) removableComments(group *ast.CommentGroup) ([]textEdit, int) {
	var edits []textEdit
	removed := 0
	inNote := false
	for _, c := range group.List {
		extent, isMarker := markerExtentOf(c.Text)
		if isMarker && extent == markerRestOfGroup {
			break
		}
		inNote = (isMarker && extent == markerParagraph) || (inNote && strings.TrimSpace(c.Text) != "//")
		if inNote || isMarker {
			continue
		}
		edits = append(edits, removalOf(s.src, s.offset(c.Pos()), s.offset(c.End())))
		removed += strings.Count(c.Text, "\n") + 1
	}
	return edits, removed
}

func (s goSource) offset(pos token.Pos) int {
	return s.fset.Position(pos).Offset
}

func removalOf(src []byte, from, to int) textEdit {
	lineStart := bytes.LastIndexByte(src[:from], '\n') + 1
	lineEnd := len(src)
	if i := bytes.IndexByte(src[to:], '\n'); i >= 0 {
		lineEnd = to + i
	}
	before, after := src[lineStart:from], src[to:lineEnd]
	codeBefore, codeAfter := len(bytes.TrimSpace(before)) > 0, len(bytes.TrimSpace(after)) > 0
	switch {
	case !codeBefore && !codeAfter:
		return textEdit{from: lineStart, to: min(lineEnd+1, len(src))}
	case !codeAfter:
		return textEdit{from: lineStart + len(bytes.TrimRight(before, " \t")), to: to}
	default:
		return textEdit{from: from, to: to, text: separatorFor(src[from:to])}
	}
}

func separatorFor(comment []byte) string {
	if bytes.ContainsRune(comment, '\n') {
		return "\n"
	}
	return " "
}

func (s goSource) shortenedPackageDoc(group *ast.CommentGroup) (edit textEdit, removedLines int, ok bool) {
	text := group.Text()
	lines := strings.Count(strings.TrimRight(text, "\n"), "\n") + 1
	if lines <= maxPackageDocLines && len(strings.Fields(text)) >= MinPackageDocWords {
		return textEdit{}, 0, true
	}
	summary := leadingSentences(text, MinPackageDocWords)
	wrapped := wrapCommentLines(summary, packageDocLineColumns)
	if len(wrapped) > maxPackageDocLines || len(strings.Fields(summary)) < MinPackageDocWords {
		return textEdit{}, 0, false
	}
	edit = textEdit{from: s.offset(group.Pos()), to: s.offset(group.End()), text: strings.Join(wrapped, "\n")}
	return edit, lines - len(wrapped), true
}

func leadingSentences(text string, minWords int) string {
	firstParagraph := strings.Join(strings.Fields(strings.SplitN(text, "\n\n", 2)[0]), " ")
	for _, end := range sentenceEnd.FindAllStringIndex(firstParagraph, -1) {
		sentences := firstParagraph[:end[0]+1]
		if !endsWithAbbreviation(firstParagraph[:end[0]]) && len(strings.Fields(sentences)) >= minWords {
			return sentences
		}
	}
	if listIntro, ok := strings.CutSuffix(firstParagraph, ":"); ok {
		return listIntro + "."
	}
	return firstParagraph
}

func endsWithAbbreviation(text string) bool {
	words := strings.Fields(text)
	return len(words) > 0 && slices.Contains(abbreviations, strings.ToLower(words[len(words)-1]))
}

func wrapCommentLines(text string, columns int) []string {
	var lines []string
	line := "//"
	for _, word := range strings.Fields(text) {
		if line != "//" && len(line)+1+len(word) > columns {
			lines = append(lines, line)
			line = "//"
		}
		line += " " + word
	}
	return append(lines, line)
}

func applyEdits(src []byte, edits []textEdit) []byte {
	ordered := slices.SortedFunc(slices.Values(edits), func(a, b textEdit) int { return b.from - a.from })
	out := slices.Clone(src)
	for _, e := range ordered {
		out = slices.Concat(out[:e.from], []byte(e.text), out[e.to:])
	}
	return out
}

func StripDirs(dirs []string) (StripReport, error) {
	var report StripReport
	for _, dir := range dirs {
		packages, err := goFilesByDir(dir)
		if err != nil {
			return report, err
		}
		for _, pkgDir := range slices.Sorted(maps.Keys(packages)) {
			report = report.merged(stripPackage(pkgDir, packages[pkgDir]))
		}
	}
	return report, nil
}

func (r StripReport) merged(o StripReport) StripReport {
	return StripReport{
		Files:       r.Files + o.Files,
		Removed:     r.Removed + o.Removed,
		Failed:      slices.Concat(r.Failed, o.Failed),
		DocsToWrite: slices.Concat(r.DocsToWrite, o.DocsToWrite),
	}
}

func stripPackage(dir string, files []string) StripReport {
	var report StripReport
	docHolder := packageDocHolder(dir, files)
	for _, name := range files {
		path := filepath.Join(dir, name)
		strip := StripComments
		if name == docHolder {
			strip = StripCommentsKeepingPackageDoc
		}
		stripped, err := stripFile(path, strip)
		switch {
		case err != nil:
			report.Failed = append(report.Failed, err.Error())
		case stripped.Removed > 0:
			report.Files++
			report.Removed += stripped.Removed
		}
		if stripped.PackageDocToWrite {
			report.DocsToWrite = append(report.DocsToWrite, path)
		}
	}
	return report
}

func stripFile(path string, strip func(name string, src []byte) (StrippedFile, error)) (StrippedFile, error) {
	src, err := os.ReadFile(path)
	if err != nil {
		return StrippedFile{}, err
	}
	stripped, err := strip(path, src)
	if err != nil || stripped.Removed == 0 {
		return stripped, err
	}
	return stripped, replaceFile(path, stripped.Source)
}

func replaceFile(path string, content []byte) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".strip-*")
	if err != nil {
		return err
	}
	_, writeErr := tmp.Write(content)
	if err := errors.Join(writeErr, tmp.Chmod(info.Mode().Perm()), tmp.Close()); err != nil {
		return errors.Join(fmt.Errorf("%s: %w", path, err), os.Remove(tmp.Name()))
	}
	return os.Rename(tmp.Name(), path)
}

func packageDocHolder(dir string, files []string) string {
	candidates := []string{"doc.go", filepath.Base(dir) + ".go"}
	for _, name := range files {
		if !strings.HasSuffix(name, "_test.go") {
			candidates = append(candidates, name)
		}
	}
	for _, name := range candidates {
		if slices.Contains(files, name) && hasPackageDoc(filepath.Join(dir, name)) {
			return name
		}
	}
	return ""
}

func hasPackageDoc(path string) bool {
	file, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.PackageClauseOnly|parser.ParseComments)
	return err == nil && file.Doc != nil
}

func goFilesByDir(root string) (map[string][]string, error) {
	packages := map[string][]string{}
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if path != root && isSkippedDir(d.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(path, ".go") {
			packages[filepath.Dir(path)] = append(packages[filepath.Dir(path)], d.Name())
		}
		return nil
	})
	return packages, err
}
