package codeindex

import (
	"cmp"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
)

var ErrTruthDisagrees = errors.New("the type-checked truth and the grep check disagree")

func GrepCheck(root string, sym BenchSymbol) error {
	call, definition := callPatterns(sym.Name)
	found := map[string]bool{}
	for _, file := range sym.GrepFiles {
		raw, err := os.ReadFile(filepath.Join(root, file))
		if err != nil {
			return fmt.Errorf("grep check %s: %w", sym.Name, err)
		}
		for i, line := range strings.Split(string(raw), "\n") {
			if call.MatchString(line) && !definition.MatchString(line) {
				found[siteKey(file, i+1)] = true
			}
		}
	}
	claimed := map[string]bool{}
	for _, site := range sym.Sites {
		claimed[siteKey(site.File, site.Line)] = true
	}
	missed, extra := difference(found, claimed), difference(claimed, found)
	if len(missed) == 0 && len(extra) == 0 {
		return nil
	}
	return fmt.Errorf("%w: %s: grep sees calls the truth lacks %v; the truth has sites with no call %v", ErrTruthDisagrees, sym.Name, missed, extra)
}

func callPatterns(name string) (call, definition *regexp.Regexp) {
	bare := regexp.QuoteMeta(name[strings.LastIndex(name, ".")+1:])
	return regexp.MustCompile(`\b` + bare + `\(`), regexp.MustCompile(`^\s*func\s+(\([^)]*\)\s*)?` + bare + `\(`)
}

func siteKey(file string, line int) string { return fmt.Sprintf("%s:%d", file, line) }

func difference(a, b map[string]bool) []string {
	var out []string
	for key := range a {
		if !b[key] {
			out = append(out, key)
		}
	}
	slices.SortFunc(out, cmp.Compare[string])
	return out
}
