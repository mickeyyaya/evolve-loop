package gotestname

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

var prefixes = []string{"Test", "Fuzz", "Benchmark"}

func Is(name string) bool {
	for _, prefix := range prefixes {
		if HasPrefix(name, prefix) {
			return true
		}
	}
	return false
}

func HasPrefix(name, prefix string) bool {
	rest, found := strings.CutPrefix(name, prefix)
	if !found {
		return false
	}
	next, _ := utf8.DecodeRuneInString(rest)
	return !unicode.IsLower(next)
}

func IgnoredDir(name string) bool {
	return name == "testdata" || name == "vendor" || strings.HasPrefix(name, ".") || strings.HasPrefix(name, "_")
}
