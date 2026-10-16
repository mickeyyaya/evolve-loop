package ship

import (
	"bytes"
	"encoding/json"
	"strings"
)

const maxTapLine = 1 << 20

type pkgResult struct {
	Package string
	Passed  bool
	Tags    []string
	Flags   []string
}

type pkgResultTap struct {
	tags    []string
	flags   []string
	partial []byte
	dropped int
	results []pkgResult
}

func newPkgResultTap(args []string) *pkgResultTap {
	flags, tags := splitTestArgs(args)
	return &pkgResultTap{flags: flags, tags: tags}
}

func (t *pkgResultTap) Write(p []byte) (int, error) {
	t.partial = append(t.partial, p...)
	for {
		i := bytes.IndexByte(t.partial, '\n')
		if i < 0 {
			if len(t.partial) > maxTapLine {
				t.partial = nil
				t.dropped++
			}
			return len(p), nil
		}
		t.note(t.partial[:i])
		t.partial = t.partial[i+1:]
	}
}

func (t *pkgResultTap) note(line []byte) {
	var ev packEvent
	if json.Unmarshal(line, &ev) != nil || ev.Test != "" || ev.Package == "" {
		return
	}
	if ev.Action == "pass" || ev.Action == "fail" {
		t.results = append(t.results, pkgResult{Package: ev.Package, Passed: ev.Action == "pass", Tags: t.tags, Flags: t.flags})
	}
}

var valuedTestFlags = map[string]bool{"-timeout": true, "-run": true}

func splitTestArgs(args []string) (flags, tags []string) {
	for i := 1; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "-tags" && i+1 < len(args):
			tags = strings.Split(args[i+1], ",")
			i++
		case valuedTestFlags[a] && i+1 < len(args):
			flags = append(flags, a, args[i+1])
			i++
		case strings.HasPrefix(a, "-"):
			flags = append(flags, a)
		}
	}
	return flags, tags
}
