package apicover

import (
	"bufio"
	"io"
	"path/filepath"
	"strconv"
	"strings"
)

// CoverEntry is one row of `go tool cover -func` output.
type CoverEntry struct {
	Path string // import-path-qualified file, e.g. "github.com/x/y/foo.go"
	File string // basename, e.g. "foo.go"
	Line int    // the func's line
	Func string // bare func/method name as cover prints it (no receiver)
	Pct  float64
}

// ParseCoverFunc parses `go tool cover -func` output. Each data row is
// "path:line:\tFuncName\tNN.N%"; the trailing "total:" summary line is skipped.
// The func name and percentage are taken as the last two whitespace fields and
// everything before them is the location, so a path containing spaces (common
// on developer machines) parses correctly. Malformed lines are skipped rather
// than failing the whole report.
func ParseCoverFunc(r io.Reader) ([]CoverEntry, error) {
	var out []CoverEntry
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		fields := strings.Fields(sc.Text())
		if len(fields) < 3 || fields[0] == "total:" {
			continue
		}
		pctStr := fields[len(fields)-1]
		name := fields[len(fields)-2]
		loc := strings.TrimSuffix(strings.Join(fields[:len(fields)-2], " "), ":")
		colon := strings.LastIndex(loc, ":")
		if colon < 0 {
			continue
		}
		lineNo, err := strconv.Atoi(loc[colon+1:])
		if err != nil {
			continue
		}
		pct, err := strconv.ParseFloat(strings.TrimSuffix(pctStr, "%"), 64)
		if err != nil {
			continue
		}
		path := loc[:colon]
		out = append(out, CoverEntry{
			Path: path,
			File: filepath.Base(path),
			Line: lineNo,
			Func: name,
			Pct:  pct,
		})
	}
	return out, sc.Err()
}
