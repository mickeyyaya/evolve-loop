package gc

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"time"
)

var goCacheShardRe = regexp.MustCompile(`^[0-9a-f]{2}$`)
var goCacheEntryRe = regexp.MustCompile(`^[0-9a-f]+-[ad]$`)

const goMtimeRefresh = time.Hour

const GoCacheInUseWindow = 2 * goMtimeRefresh

type GoCacheBounds struct {
	Now       time.Time
	UnusedFor time.Duration
	MaxBytes  int64
}

func (b GoCacheBounds) unusedBefore() time.Time {
	return b.Now.Add(-max(b.UnusedFor, GoCacheInUseWindow))
}

type CacheTrimReport struct {
	Files          int      `json:"files"`
	Bytes          int64    `json:"bytes"`
	CapFiles       int      `json:"cap_files"`
	CapBytes       int64    `json:"cap_bytes"`
	RemainingBytes int64    `json:"remaining_bytes"`
	Errors         []string `json:"errors,omitempty"`
}

type goCacheEntry struct {
	shard, name string
	size        int64
	mtime       time.Time
}

func (e goCacheEntry) path() string { return filepath.Join(e.shard, e.name) }

func TrimGoCache(dir string, b GoCacheBounds, remove func(string) error) CacheTrimReport {
	entries, errs := listGoCache(dir)
	sortLeastRecentlyUsedFirst(entries)
	rep := CacheTrimReport{Errors: errs, RemainingBytes: totalSize(entries)}
	unused, rest := splitUnused(entries, b)
	overCap := leastRecentlyUsedOverCap(rest, b)
	rep.Files, rep.Bytes = rep.release(unused, b.unusedBefore(), remove)
	rep.CapFiles, rep.CapBytes = rep.release(overCap, b.Now.Add(-GoCacheInUseWindow), remove)
	rep.RemainingBytes -= rep.Bytes + rep.CapBytes
	return rep
}

func listGoCache(dir string) ([]goCacheEntry, []string) {
	shards, err := os.ReadDir(dir)
	if err != nil {
		return nil, []string{fmt.Sprintf("read go build cache %s: %v", dir, err)}
	}
	var entries []goCacheEntry
	var errs []string
	for _, s := range shards {
		if !s.IsDir() || !goCacheShardRe.MatchString(s.Name()) {
			continue
		}
		found, shardErrs := listShard(filepath.Join(dir, s.Name()))
		entries = append(entries, found...)
		errs = append(errs, shardErrs...)
	}
	return entries, errs
}

func listShard(shard string) ([]goCacheEntry, []string) {
	names, err := os.ReadDir(shard)
	if err != nil {
		return nil, []string{fmt.Sprintf("read %s: %v", shard, err)}
	}
	var entries []goCacheEntry
	var errs []string
	for _, e := range names {
		if e.IsDir() || !goCacheEntryRe.MatchString(e.Name()) {
			continue
		}
		info, err := e.Info()
		switch {
		case errors.Is(err, fs.ErrNotExist):
			continue
		case err != nil:
			errs = append(errs, fmt.Sprintf("stat %s: %v", filepath.Join(shard, e.Name()), err))
			continue
		}
		entries = append(entries, goCacheEntry{shard: shard, name: e.Name(), size: info.Size(), mtime: info.ModTime()})
	}
	return entries, errs
}

func splitUnused(entries []goCacheEntry, b GoCacheBounds) (unused, rest []goCacheEntry) {
	if b.UnusedFor <= 0 {
		return nil, entries
	}
	cutoff := b.unusedBefore()
	for _, e := range entries {
		if e.mtime.Before(cutoff) {
			unused = append(unused, e)
		} else {
			rest = append(rest, e)
		}
	}
	return unused, rest
}

func sortLeastRecentlyUsedFirst(entries []goCacheEntry) {
	sort.Slice(entries, func(i, j int) bool {
		if !entries[i].mtime.Equal(entries[j].mtime) {
			return entries[i].mtime.Before(entries[j].mtime)
		}
		return entries[i].path() < entries[j].path()
	})
}

func leastRecentlyUsedOverCap(leastRecentFirst []goCacheEntry, b GoCacheBounds) []goCacheEntry {
	total := totalSize(leastRecentFirst)
	if b.MaxBytes <= 0 || total <= b.MaxBytes {
		return nil
	}
	inUseSince := b.Now.Add(-GoCacheInUseWindow)
	var victims []goCacheEntry
	for _, e := range leastRecentFirst {
		if total <= b.MaxBytes || !e.mtime.Before(inUseSince) {
			break
		}
		victims = append(victims, e)
		total -= e.size
	}
	return victims
}

func (r *CacheTrimReport) release(victims []goCacheEntry, unusedBefore time.Time, remove func(string) error) (files int, bytes int64) {
	for _, e := range victims {
		info, err := os.Lstat(e.path())
		switch {
		case err != nil && !errors.Is(err, fs.ErrNotExist):
			r.Errors = append(r.Errors, err.Error())
			continue
		case err == nil && !info.ModTime().Before(unusedBefore):
			continue
		}
		if err := remove(e.path()); err != nil && !errors.Is(err, fs.ErrNotExist) {
			r.Errors = append(r.Errors, err.Error())
			continue
		}
		files++
		bytes += e.size
	}
	return files, bytes
}

func totalSize(entries []goCacheEntry) int64 {
	var total int64
	for _, e := range entries {
		total += e.size
	}
	return total
}
