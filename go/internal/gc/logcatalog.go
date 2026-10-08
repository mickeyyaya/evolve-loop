package gc

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/gcpolicy"
	"github.com/mickeyyaya/evolve-loop/go/internal/runlease"
)

type logEntry struct {
	path   string
	newest time.Time
	size   int64
	pinned bool
	marker string
}

type logPlanner struct {
	now  time.Time
	add  func(string, Action, string)
	warn func(string)
}

func (lp logPlanner) planCatalog(evolveDir string, cats []gcpolicy.LogCategory) {
	current := currentLogRunDir(evolveDir)
	for _, cat := range cats {
		lp.planCategory(cat, lp.collect(evolveDir, cat.Homes, current))
	}
}

func (lp logPlanner) planCategory(cat gcpolicy.LogCategory, entries []logEntry) {
	sort.Slice(entries, func(i, j int) bool { return entries[i].newest.After(entries[j].newest) })
	var total, pinned int64
	for _, e := range entries {
		if e.pinned {
			total += e.size
			pinned += e.size
		}
	}
	capRule := "logs." + cat.Name + ".max_total_mb"
	drop := func(e logEntry, rule string) {
		lp.add(e.path, ActionDelete, rule)
		if e.marker != "" {
			lp.add(e.marker, ActionDelete, rule)
		}
	}
	for _, e := range entries {
		switch {
		case e.pinned:
		case ageDays(lp.now, e.newest) > float64(cat.TTLDays):
			drop(e, "logs."+cat.Name+".ttl_days")
		case total+e.size > cat.MaxTotalBytes:
			drop(e, capRule)
		default:
			total += e.size
		}
	}
	if total > cat.MaxTotalBytes {
		lp.warn(fmt.Sprintf("%s: %d bytes stay over the cap of %d bytes; the current or live logs hold %d bytes and are never deleted", capRule, total, cat.MaxTotalBytes, pinned))
	}
}

func (lp logPlanner) collect(evolveDir string, homes []gcpolicy.LogHome, current string) []logEntry {
	var out []logEntry
	for _, h := range homes {
		matches, err := filepath.Glob(filepath.Join(evolveDir, h.Dir, h.Glob))
		if err != nil {
			continue
		}
		for _, p := range matches {
			if e, ok := logEntryAt(p, h.IsDir); ok {
				e.pinned = p == current || logWriterLive(p, h.IsDir)
				e.marker = fileMarker(p, h.IsDir)
				out = append(out, e)
			}
		}
	}
	return out
}

func fileMarker(path string, isDir bool) string {
	marker := path + gcpolicy.LogWriterPIDSuffix
	if _, err := os.Lstat(marker); isDir || err != nil {
		return ""
	}
	return marker
}

func logWriterLive(path string, isDir bool) bool {
	markers := []string{path + gcpolicy.LogWriterPIDSuffix}
	if isDir {
		markers, _ = filepath.Glob(filepath.Join(path, "*"+gcpolicy.LogWriterPIDSuffix))
	}
	for _, m := range markers {
		raw, err := os.ReadFile(m)
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		pid, perr := strconv.Atoi(strings.TrimSpace(string(raw)))
		if err != nil || perr != nil || runlease.PIDAlive(pid) {
			return true
		}
	}
	return false
}

func logEntryAt(path string, isDir bool) (logEntry, bool) {
	info, err := os.Lstat(path)
	if err != nil {
		return logEntry{}, false
	}
	switch {
	case isDir && info.IsDir():
		newest, size := treeNewestAndSize(path, info.ModTime())
		return logEntry{path: path, newest: newest, size: size}, true
	case !isDir && info.Mode().IsRegular():
		return logEntry{path: path, newest: info.ModTime(), size: info.Size()}, true
	}
	return logEntry{}, false
}

func treeNewestAndSize(root string, dirMod time.Time) (time.Time, int64) {
	newest, size := dirMod, int64(0)
	_ = filepath.WalkDir(root, func(_ string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		if info, err := d.Info(); err == nil {
			size += info.Size()
			if info.ModTime().After(newest) {
				newest = info.ModTime()
			}
		}
		return nil
	})
	return newest, size
}

func currentLogRunDir(evolveDir string) string {
	logsDir := filepath.Join(evolveDir, gcpolicy.LogsDir)
	target, err := os.Readlink(filepath.Join(logsDir, gcpolicy.LogsCurrentLink))
	if err != nil {
		return ""
	}
	if !filepath.IsAbs(target) {
		target = filepath.Join(logsDir, target)
	}
	return filepath.Clean(target)
}
