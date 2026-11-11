package covchanged

import (
	"fmt"
	"go/parser"
	"go/token"
	"slices"
	"sort"
	"strconv"
)

func FuncPercents(profile Profile, read func(rel string) ([]byte, error)) ([]Func, error) {
	files := make([]string, 0, len(profile))
	for rel := range profile {
		files = append(files, rel)
	}
	sort.Strings(files)
	var out []Func
	for _, rel := range files {
		funcs, err := funcsOfFile(rel, profile[rel], read)
		if err != nil {
			return nil, err
		}
		out = slices.Concat(out, funcs)
	}
	return out, nil
}

func funcsOfFile(rel string, blocks []Block, read func(string) ([]byte, error)) ([]Func, error) {
	data, err := read(rel)
	if err != nil {
		return nil, fmt.Errorf("covchanged: read %s: %w", rel, err)
	}
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, rel, data, parser.SkipObjectResolution)
	if err != nil {
		return nil, fmt.Errorf("covchanged: parse %s: %w", rel, err)
	}
	return funcsOf(fset, file, rel, blocks, func(int, int) bool { return true }), nil
}

func (f Func) CoverPercent() float64 {
	if f.Statements == 0 {
		return 0
	}
	percent, _ := strconv.ParseFloat(strconv.FormatFloat(100*float64(f.Covered)/float64(f.Statements), 'f', 1, 64), 64)
	return percent
}
