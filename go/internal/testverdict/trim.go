package testverdict

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"time"
)

type TrimReport struct {
	Files  int
	Bytes  int64
	Errors []string
}

type trimEntry struct {
	path  string
	size  int64
	mtime time.Time
}

func Trim(root string, now time.Time, maxAge time.Duration, maxBytes int64, remove func(string) error) TrimReport {
	var rep TrimReport
	files, dirs, err := listStore(root)
	if err != nil {
		rep.Errors = append(rep.Errors, err.Error())
		return rep
	}
	sort.Slice(files, func(i, j int) bool {
		if !files[i].mtime.Equal(files[j].mtime) {
			return files[i].mtime.Before(files[j].mtime)
		}
		return files[i].path < files[j].path
	})
	total := int64(0)
	for _, f := range files {
		total += f.size
	}
	cutoff := now.Add(-maxAge)
	for _, f := range files {
		if (maxAge > 0 && f.mtime.Before(cutoff)) || (maxBytes > 0 && total > maxBytes) {
			if rep.attempt(remove, f.path) {
				rep.Files++
				rep.Bytes += f.size
				total -= f.size
			}
		}
	}
	for _, d := range dirs {
		if entries, err := os.ReadDir(d); err == nil && len(entries) == 0 {
			rep.attempt(remove, d)
		}
	}
	return rep
}

func (r *TrimReport) attempt(remove func(string) error, path string) bool {
	if err := remove(path); err != nil && !errors.Is(err, fs.ErrNotExist) {
		r.Errors = append(r.Errors, err.Error())
		return false
	}
	return true
}

func listStore(root string) (files []trimEntry, dirs []string, err error) {
	err = filepath.Walk(root, func(p string, info fs.FileInfo, walkErr error) error {
		switch {
		case walkErr != nil:
			return walkErr
		case info.IsDir() && p != root:
			dirs = append(dirs, p)
		case !info.IsDir():
			files = append(files, trimEntry{path: p, size: info.Size(), mtime: info.ModTime()})
		}
		return nil
	})
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil, nil
	}
	return files, dirs, err
}
