package dossier

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

type ReadFailure struct {
	Path string
	Err  error
}

func (f ReadFailure) Error() string { return f.Path + ": " + f.Err.Error() }

type cycleBounds struct{ min, max int }

func (b cycleBounds) holds(n int) bool { return n >= b.min && (b.max == 0 || n <= b.max) }

type numberedFile struct {
	path  string
	cycle int
}

func ReadDirChecked(dir string, minCycle, maxCycle int) ([]*Dossier, []ReadFailure) {
	entries, err := os.ReadDir(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, []ReadFailure{{Path: dir, Err: err}}
	}
	out := make([]*Dossier, 0, len(entries))
	var failures []ReadFailure
	for _, f := range numberedFiles(dir, entries, cycleBounds{min: minCycle, max: maxCycle}) {
		d, err := readDossierFile(f.path)
		if err != nil {
			failures = append(failures, ReadFailure{Path: f.path, Err: err})
			continue
		}
		out = append(out, d)
	}
	return out, failures
}

func numberedFiles(dir string, entries []os.DirEntry, b cycleBounds) []numberedFile {
	var files []numberedFile
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".json" {
			continue
		}
		base := strings.TrimSuffix(e.Name(), ".json")
		n, err := strconv.Atoi(strings.TrimPrefix(base, "cycle-"))
		if err != nil || !b.holds(n) {
			continue
		}
		files = append(files, numberedFile{path: filepath.Join(dir, e.Name()), cycle: n})
	}
	sort.Slice(files, func(i, j int) bool { return files[i].cycle < files[j].cycle })
	return files
}

func readDossierFile(path string) (*Dossier, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return ParseJSON(data)
}
