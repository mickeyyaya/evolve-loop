package commentaudit

import (
	"fmt"
	"io/fs"
	"strings"
)

type Added struct {
	File string
	Line string
}

type rule struct {
	added func(before, after []byte) []string
	keep  func(string) bool
}

var (
	commentsRule  = rule{added: AddedComments, keep: isPlain}
	narrativeRule = rule{added: AddedNarrative, keep: isNarrative}
)

func AddedAcrossDiff(files []string, before, after func(string) ([]byte, error)) ([]Added, error) {
	return commentsRule.acrossDiff(files, before, after)
}

func (r rule) acrossDiff(files []string, before, after func(string) ([]byte, error)) ([]Added, error) {
	var added []Added
	removed := map[string]int{}
	for _, f := range files {
		if !strings.HasSuffix(f, ".go") || isOutsideProjectCode(f) {
			continue
		}
		b, a, err := readBoth(before, after, f)
		if err != nil {
			return nil, err
		}
		for _, line := range r.added(b, a) {
			added = append(added, Added{File: f, Line: line})
		}
		for _, line := range subtract(commentLines(b, r.keep), commentLines(a, r.keep)) {
			removed[line]++
		}
	}
	return unmoved(added, removed), nil
}

func unmoved(added []Added, removed map[string]int) []Added {
	var out []Added
	for _, c := range added {
		if removed[c.Line] > 0 {
			removed[c.Line]--
			continue
		}
		out = append(out, c)
	}
	return out
}

func ReadAtBase(run func(args ...string) ([]byte, error), base string) func(string) ([]byte, error) {
	return func(path string) ([]byte, error) {
		if _, err := run("cat-file", "-e", base+":"+path); err == nil {
			return run("show", base+":"+path)
		}
		if _, err := run("cat-file", "-e", base+"^{commit}"); err != nil {
			return nil, fmt.Errorf("base %s is unreadable: %w", base, err)
		}
		return nil, fs.ErrNotExist
	}
}

func isOutsideProjectCode(path string) bool {
	return strings.Contains("/"+path, "/testdata/") || strings.Contains("/"+path, "/vendor/")
}
