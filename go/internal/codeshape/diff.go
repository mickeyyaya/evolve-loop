package codeshape

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

const (
	goPrefix   = "go/"
	gitTimeout = time.Minute
)

var hunkHeader = regexp.MustCompile(`^@@ -\d+(?:,\d+)? \+(\d+)(?:,(\d+))? @@`)

type lineRange struct{ from, to int }

type ShipDiff struct {
	Ranges map[string][]lineRange
	Added  [][]string
}

func gitOut(root string, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), gitTimeout)
	defer cancel()
	out, err := sysexec.Command(ctx, "git", append([]string{"-C", root}, args...)...).Output()
	if err != nil {
		return nil, fmt.Errorf("git %s: %w", strings.Join(args, " "), err)
	}
	return out, nil
}

func DiffOf(root, commit string) (ShipDiff, error) {
	out, err := gitOut(root, "-c", "core.quotepath=off", "show", "-U0", "--no-renames", "--diff-merges=first-parent", "--format=", "--end-of-options", commit, "--", "*.go")
	if err != nil {
		return ShipDiff{}, err
	}
	return parseDiff(string(out)), nil
}

func parseDiff(text string) ShipDiff {
	d := ShipDiff{Ranges: map[string][]lineRange{}}
	path := ""
	var hunk []string
	flush := func() {
		if len(hunk) > 0 {
			d.Added = append(d.Added, hunk)
		}
		hunk = nil
	}
	for _, line := range strings.Split(text, "\n") {
		switch {
		case strings.HasPrefix(line, "+++ "):
			flush()
			path = strings.TrimPrefix(strings.TrimPrefix(line, "+++ "), "b/")
		case strings.HasPrefix(line, "@@"):
			flush()
			if r, ok := rangeOf(line); ok && productionPath(path) {
				d.Ranges[path] = append(d.Ranges[path], r)
			}
		case strings.HasPrefix(line, "+") && productionPath(path):
			hunk = append(hunk, line[1:])
		}
	}
	flush()
	return d
}

func rangeOf(header string) (lineRange, bool) {
	m := hunkHeader.FindStringSubmatch(header)
	if m == nil {
		return lineRange{}, false
	}
	start, _ := strconv.Atoi(m[1])
	count := 1
	if m[2] != "" {
		count, _ = strconv.Atoi(m[2])
	}
	return lineRange{from: start, to: start + count - 1}, count > 0
}

func productionPath(path string) bool {
	return strings.HasPrefix(path, goPrefix) && Production(strings.TrimPrefix(path, goPrefix))
}

func ChangedFuncs(root, commit string) ([]Func, error) {
	d, err := DiffOf(root, commit)
	if err != nil {
		return nil, err
	}
	var out []Func
	for path, ranges := range d.Ranges {
		src, err := gitOut(root, "show", "--end-of-options", commit+":"+path)
		if err != nil {
			return nil, err
		}
		funcs, err := ParseFuncs(path, src)
		if err != nil {
			return nil, err
		}
		out = append(out, touched(funcs, ranges)...)
	}
	return out, nil
}

func touched(funcs []Func, ranges []lineRange) []Func {
	var out []Func
	for _, f := range funcs {
		for _, r := range ranges {
			if r.from <= f.End && r.to >= f.Start {
				out = append(out, f)
				break
			}
		}
	}
	return out
}
