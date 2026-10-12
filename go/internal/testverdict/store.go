package testverdict

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/atomicwrite"
	"github.com/mickeyyaya/evolve-loop/go/internal/testkey"
)

func (s *Store) Put(r Record) error {
	if r.Outcome != Pass {
		return ErrNotGreen
	}
	if !validKey(r.Key) || !r.Key.Valid() {
		return ErrBadKey
	}
	if !r.Key.Consistent(r.Log) {
		return ErrInconsistent
	}
	r.StoredAt = s.now().UTC()
	data := sealRecord(r)
	if err := atomicwrite.Bytes(entryPath(s.root, r.Key.File, r.Key.Full), data); err != nil {
		return err
	}
	return atomicwrite.Bytes(indexPath(s.root, r.Package), data)
}

func (s *Store) Lookup(file string, env map[string]string) (Record, bool) {
	if !digestPattern.MatchString(file) {
		return Record{}, false
	}
	dir := filepath.Join(s.root, file)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return Record{}, false
	}
	for _, e := range entries {
		r, ok := open(filepath.Join(dir, e.Name()))
		if ok && entryPath(s.root, r.Key.File, r.Key.Full) == filepath.Join(dir, e.Name()) && envMatches(r.Key.Env, env) {
			return r, true
		}
	}
	return Record{}, false
}

func (s *Store) LastLog(pkg string) (testkey.Log, bool) {
	r, ok := open(indexPath(s.root, pkg))
	if !ok || r.Package != pkg {
		return testkey.Log{}, false
	}
	return r.Log, true
}

func (s *Store) GC(maxAge time.Duration) (int, error) {
	if _, err := os.ReadDir(s.root); errors.Is(err, fs.ErrNotExist) {
		return 0, nil
	} else if err != nil {
		return 0, err
	}
	cutoff := s.now().Add(-maxAge)
	removed := 0
	var dirs []string
	err := filepath.WalkDir(s.root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			dirs = append(dirs, p)
			return nil
		}
		if info, err := d.Info(); err != nil || !info.ModTime().Before(cutoff) {
			return nil
		}
		gone, err := removeIfPresent(p)
		if gone {
			removed++
		}
		return err
	})
	if err != nil {
		return removed, err
	}
	return removed, removeEmpty(s.root, dirs)
}

func removeEmpty(root string, dirs []string) error {
	for _, d := range dirs {
		if d == root || strings.HasSuffix(d, string(filepath.Separator)+indexDir) {
			continue
		}
		if _, err := removeIfPresent(d); err != nil && !errors.Is(err, syscall.ENOTEMPTY) {
			return err
		}
	}
	return nil
}

var removePath = os.Remove

func removeIfPresent(p string) (bool, error) {
	err := removePath(p)
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}
	return err == nil, err
}
