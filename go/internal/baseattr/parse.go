package baseattr

import (
	"regexp"
	"slices"
	"strings"
)

var (
	failedTest    = regexp.MustCompile(`^\s*--- FAIL: (\S+)`)
	failedPackage = regexp.MustCompile(`^FAIL\t(\S+)`)
	panicLine     = regexp.MustCompile(`^(panic: |fatal error: )`)
)

func ParseGoTest(out string) Reds {
	var r Reds
	var pending []string
	for _, line := range strings.Split(out, "\n") {
		switch {
		case panicLine.MatchString(line):
			r.Unnamed = true
		case failedTest.MatchString(line):
			pending = append(pending, topLevel(failedTest.FindStringSubmatch(line)[1]))
		case failedPackage.MatchString(line):
			pkg := failedPackage.FindStringSubmatch(line)[1]
			r.Unnamed = r.Unnamed || len(pending) == 0
			for _, test := range pending {
				r.Keys = appendKey(r.Keys, Key{Package: pkg, Test: test})
			}
			pending = nil
		}
	}
	r.Unnamed = r.Unnamed || len(pending) > 0
	return r
}

func topLevel(name string) string {
	top, _, _ := strings.Cut(name, "/")
	return top
}

func FailedTests(out string) []string {
	var names []string
	for _, line := range strings.Split(out, "\n") {
		if m := failedTest.FindStringSubmatch(line); m != nil {
			if name := topLevel(m[1]); !slices.Contains(names, name) {
				names = append(names, name)
			}
		}
	}
	return names
}
