package apicover

import (
	"bufio"
	"io"
	"path/filepath"
	"strconv"
	"strings"
)

type CoverEntry struct {
	Path string
	File string
	Line int
	Func string
	Pct  float64
}

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
