package sizeratchet

import (
	"errors"
	"fmt"
	"go/token"
	"io/fs"
	"path"
	"strings"
)

func measured(rel string) bool {
	if !strings.HasSuffix(rel, ".go") || strings.HasSuffix(rel, "_test.go") {
		return false
	}
	for _, seg := range strings.Split(path.Dir(rel), "/") {
		if seg != "." && skipDir(seg) {
			return false
		}
	}
	return true
}

func readSpans(read func(string) ([]byte, error), prefix string, paths []string) ([]FuncSpan, error) {
	fset := token.NewFileSet()
	var spans []FuncSpan
	for _, p := range paths {
		rel, inModule := strings.CutPrefix(p, prefix)
		if !inModule || !measured(rel) {
			continue
		}
		src, err := read(p)
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("sizeratchet: read %s: %w", p, err)
		}
		fileSpans, err := parseSpans(fset, path.Dir(rel), p, src)
		if err != nil {
			return nil, err
		}
		spans = append(spans, fileSpans...)
	}
	return spans, nil
}

func CheckAgainstBase(prefix string, paths []string, atBase, onDisk func(string) ([]byte, error)) (string, error) {
	listing, err := atBase(prefix + OffendersRelPath)
	if errors.Is(err, fs.ErrNotExist) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	offenders, err := parseOffenders(listing, OffendersRelPath+" at the base")
	if err != nil {
		return "", err
	}
	before, err := readSpans(atBase, prefix, paths)
	if err != nil {
		return "", err
	}
	after, err := readSpans(onDisk, prefix, paths)
	if err != nil {
		return "", err
	}
	if err := Check(after, Tighten(before, offenders)); err != nil {
		return err.Error(), nil
	}
	return "", nil
}
