package commentaudit

import (
	"go/parser"
	"go/scanner"
	"go/token"
	"io/fs"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"
)

// FileStats counts non-blank lines: Code lines, whole-line Comment lines, and
// the comment lines that carry project history (Narrative).
type FileStats struct {
	Code, Comment, Narrative int
}

func (s FileStats) add(o FileStats) FileStats {
	return FileStats{Code: s.Code + o.Code, Comment: s.Comment + o.Comment, Narrative: s.Narrative + o.Narrative}
}

var (
	historyMarker = regexp.MustCompile(`(?i)\bcycles?[- ]\d+|\bwave[- ]\d+|\bbatch[- ]\d+|\bround[- ]\d+|\b20\d\d-\d\d-\d\d\b|#\d{2,}\b|\bv\d+\.\d+\.\d+\b`)
	hexWord       = regexp.MustCompile(`\b[0-9a-f]{7,40}\b`)
	caseMarker    = regexp.MustCompile(`\bF\d{2,3}\b|\b[Ii]ncident`)
	adrMention    = regexp.MustCompile(`\bADR-\d{4}\b`)
	adrPointer    = regexp.MustCompile(`^//\s*See ADR-\d{4}(, ADR-\d{4})*\.?$`)
)

// goReferenceDate is the layout Go formats dates with, not a date in history.
const goReferenceDate = "2006-01-02"

func isNarrative(line string) bool {
	return historyMarker.MatchString(strings.ReplaceAll(line, goReferenceDate, "")) || caseMarker.MatchString(line) ||
		(adrMention.MatchString(line) && !adrPointer.MatchString(line)) || hasCommitSHA(line)
}

func hasCommitSHA(line string) bool {
	return slices.ContainsFunc(hexWord.FindAllString(line, -1), func(w string) bool {
		return strings.ContainsAny(w, "abcdef") && strings.ContainsAny(w, "0123456789")
	})
}

func isSkippedDir(name string) bool {
	return name == "testdata" || name == "vendor" || (strings.HasPrefix(name, ".") && name != ".")
}

const documentationRoot = "docs/"

func isDocumentation(file string) bool {
	return strings.HasPrefix(filepath.ToSlash(file), documentationRoot)
}

func isOutsideProjectCode(file string) bool {
	return slices.ContainsFunc(strings.Split(path.Dir(filepath.ToSlash(file)), "/"), isSkippedDir)
}

// Stats counts the lines of one Go source file.
func Stats(src []byte) FileStats {
	var s FileStats
	forEachLine(src, func(line string, isComment bool) {
		switch {
		case !isComment:
			s.Code++
		case isNarrative(line):
			s.Comment++
			s.Narrative++
		default:
			s.Comment++
		}
	})
	return s
}

// AddedNarrative returns the whole-line narrative comments in after that
// before does not already hold; a moved line is not added.
func AddedNarrative(before, after []byte) []string {
	return subtract(commentLines(after, isNarrative), commentLines(before, isNarrative))
}

const maxPackageDocLines = 3

func AddedComments(before, after []byte) []string {
	held := commentLines(before, isPlain)
	if sparesPackageDoc(before, after) {
		held = append(held, packageDoc(after)...)
	}
	return subtract(commentLines(after, isPlain), held)
}

func sparesPackageDoc(before, after []byte) bool {
	limit := maxPackageDocLines
	if before != nil {
		existing := len(packageDoc(before))
		if existing == 0 {
			return false
		}
		limit = max(limit, existing)
	}
	return len(packageDoc(after)) <= limit
}

func isPlain(line string) bool {
	return !directive.MatchString(line)
}

func subtract(lines, held []string) []string {
	count := map[string]int{}
	for _, line := range held {
		count[line]++
	}
	var added []string
	for _, line := range lines {
		if count[line] > 0 {
			count[line]--
			continue
		}
		added = append(added, line)
	}
	return added
}

func commentLines(src []byte, keep func(string) bool) []string {
	var found []string
	forEachLine(src, func(line string, isComment bool) {
		if isComment && keep(line) {
			found = append(found, line)
		}
	})
	return found
}

// A file whose package clause does not parse has no package doc to spare.
func packageDoc(src []byte) []string {
	file, err := parser.ParseFile(token.NewFileSet(), "", src, parser.PackageClauseOnly|parser.ParseComments)
	if err != nil || file.Doc == nil {
		return nil
	}
	var lines []string
	for _, c := range file.Doc.List {
		for _, line := range strings.Split(c.Text, "\n") {
			lines = append(lines, strings.TrimSpace(line))
		}
	}
	return lines
}

func forEachLine(src []byte, visit func(line string, isComment bool)) {
	isCommentLine := wholeLineCommentLines(src)
	for i, line := range strings.Split(string(src), "\n") {
		if line = strings.TrimSpace(line); line != "" {
			visit(line, isCommentLine[i+1])
		}
	}
}

func wholeLineCommentLines(src []byte) map[int]bool {
	fset := token.NewFileSet()
	file := fset.AddFile("", fset.Base(), len(src))
	var s scanner.Scanner
	s.Init(file, src, nil, scanner.ScanComments)
	commentLines := map[int]bool{}
	lastCodeLine := 0
	for {
		pos, tok, lit := s.Scan()
		switch {
		case tok == token.EOF:
			return commentLines
		case tok == token.COMMENT && file.PositionFor(pos, false).Line != lastCodeLine:
			first := file.PositionFor(pos, false).Line
			for line := first; line <= first+strings.Count(lit, "\n"); line++ {
				commentLines[line] = true
			}
		case tok == token.STRING:
			lastCodeLine = file.PositionFor(pos, false).Line + strings.Count(lit, "\n")
		case tok != token.COMMENT:
			lastCodeLine = file.PositionFor(pos, false).Line
		}
	}
}

// PackageStats sums the Go files of one directory.
type PackageStats struct {
	Dir   string
	Files int
	Stats FileStats
}

// Rank returns every directory holding Go files, most narrative first.
func Rank(fsys fs.FS) ([]PackageStats, error) {
	byDir := map[string]*PackageStats{}
	err := fs.WalkDir(fsys, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if isSkippedDir(d.Name()) {
				return fs.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(p, ".go") {
			return nil
		}
		src, err := fs.ReadFile(fsys, p)
		if err != nil {
			return err
		}
		dir := path.Dir(p)
		if byDir[dir] == nil {
			byDir[dir] = &PackageStats{Dir: dir}
		}
		byDir[dir].Files++
		byDir[dir].Stats = byDir[dir].Stats.add(Stats(src))
		return nil
	})
	if err != nil {
		return nil, err
	}
	ranked := make([]PackageStats, 0, len(byDir))
	for _, p := range byDir {
		ranked = append(ranked, *p)
	}
	sort.Slice(ranked, func(i, j int) bool {
		a, b := ranked[i].Stats, ranked[j].Stats
		if a.Narrative != b.Narrative {
			return a.Narrative > b.Narrative
		}
		if a.Comment != b.Comment {
			return a.Comment > b.Comment
		}
		return ranked[i].Dir < ranked[j].Dir
	})
	return ranked, nil
}
