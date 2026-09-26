package commentaudit

import (
	"io/fs"
	"path"
	"regexp"
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
	historyMarker = regexp.MustCompile(`(?i)\bcycles?[- ]\d+|\bwave[- ]\d+|\bbatch[- ]\d+|\b20\d\d-\d\d-\d\d\b`)
	caseMarker    = regexp.MustCompile(`\bF\d{2,3}\b|\b[Ii]ncident`)
	adrMention    = regexp.MustCompile(`\bADR-\d{4}\b`)
	adrPointer    = regexp.MustCompile(`^//\s*See ADR-\d{4}(, ADR-\d{4})*\.?$`)
)

// goReferenceDate is the layout Go formats dates with, not a date in history.
const goReferenceDate = "2006-01-02"

// isNarrative reports whether a comment line carries project history; a bare
// `See ADR-NNNN.` pointer is allowed by the convention and is not history.
func isNarrative(line string) bool {
	return historyMarker.MatchString(strings.ReplaceAll(line, goReferenceDate, "")) || caseMarker.MatchString(line) ||
		(adrMention.MatchString(line) && !adrPointer.MatchString(line))
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
	held := map[string]int{}
	for _, line := range narrativeLines(before) {
		held[line]++
	}
	var added []string
	for _, line := range narrativeLines(after) {
		if held[line] > 0 {
			held[line]--
			continue
		}
		added = append(added, line)
	}
	return added
}

func narrativeLines(src []byte) []string {
	var found []string
	forEachLine(src, func(line string, isComment bool) {
		if isComment && isNarrative(line) {
			found = append(found, line)
		}
	})
	return found
}

func forEachLine(src []byte, visit func(line string, isComment bool)) {
	inBlock := false
	for _, line := range strings.Split(string(src), "\n") {
		line = strings.TrimSpace(line)
		isComment := inBlock || strings.HasPrefix(line, "//") || strings.HasPrefix(line, "/*")
		if strings.HasPrefix(line, "/*") {
			inBlock = true
		}
		if inBlock && strings.Contains(line, "*/") {
			inBlock = false
		}
		if line != "" {
			visit(line, isComment)
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
			if p != "." && (d.Name() == "testdata" || d.Name() == "vendor" || strings.HasPrefix(d.Name(), ".")) {
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
