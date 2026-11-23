package gc

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

var goCacheShardRe = regexp.MustCompile(`^[0-9a-f]{2}$`)
var goCacheEntryRe = regexp.MustCompile(`^[0-9a-f]+-[ad]$`)

var goCacheEntryInfo = func(e fs.DirEntry) (fs.FileInfo, error) { return e.Info() }

var errCacheRootNotExist = errors.New("cache root disappeared")

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
	isDir       bool
}

func (e goCacheEntry) path() string { return filepath.Join(e.shard, e.name) }

func TrimGoCache(dir string, b GoCacheBounds, remove func(string) error, opts ...TempOption) CacheTrimReport {
	var cfg tempConfig
	for _, o := range opts {
		o(&cfg)
	}
	entries, errs := listGoCache(dir)
	sortLeastRecentlyUsedFirst(entries)
	rep := CacheTrimReport{Errors: errs, RemainingBytes: totalSize(entries)}
	unused, rest := splitUnused(entries, b)
	overCap := selectOverCap(rest, b, cfg.held, &rep.Errors)
	rep.Files, rep.Bytes = rep.releaseVictims(unused, b.unusedBefore(), remove, cfg.held)
	rep.CapFiles, rep.CapBytes = rep.releaseVictims(overCap, b.Now.Add(-GoCacheInUseWindow), remove, nil)
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
		if !goCacheEntryRe.MatchString(e.Name()) {
			continue
		}
		if e.IsDir() {
			if !strings.HasSuffix(e.Name(), "-d") {
				continue
			}
			size, newest, ok, err := statCacheDir(filepath.Join(shard, e.Name()))
			if err != nil {
				errs = append(errs, fmt.Sprintf("stat %s: %v", filepath.Join(shard, e.Name()), err))
				continue
			}
			if ok {
				entries = append(entries, goCacheEntry{shard: shard, name: e.Name(), size: size, mtime: newest, isDir: true})
			}
			continue
		}
		info, err := goCacheEntryInfo(e)
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

func statCacheDir(root string) (size int64, newest time.Time, ok bool, err error) {
	var rootMod time.Time
	hasFile := false
	walkErr := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				if p == root {
					return errCacheRootNotExist
				}
				return nil
			}
			return err
		}
		if d.IsDir() {
			if p == root {
				info, infoErr := goCacheEntryInfo(d)
				if infoErr != nil {
					return infoErr
				}
				rootMod = info.ModTime()
			}
			return nil
		}
		info, err := goCacheEntryInfo(d)
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				return nil
			}
			return err
		}
		size += info.Size()
		if !hasFile || info.ModTime().After(newest) {
			newest = info.ModTime()
			hasFile = true
		}
		return nil
	})
	if walkErr != nil {
		if errors.Is(walkErr, errCacheRootNotExist) {
			return 0, time.Time{}, false, nil
		}
		return 0, time.Time{}, false, walkErr
	}
	if !hasFile {
		newest = rootMod
	}
	return size, newest, true, nil
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
	return selectOverCap(leastRecentFirst, b, nil, nil)
}

func sparedByHolder(path string, held func(string) (bool, error), errs *[]string) bool {
	if held == nil {
		return false
	}
	isHeld, err := held(path)
	if err != nil {
		*errs = append(*errs, fmt.Sprintf("spared %s: liveness unknown: %v", path, err))
		return true
	}
	return isHeld
}

func selectOverCap(leastRecentFirst []goCacheEntry, b GoCacheBounds, held func(string) (bool, error), errs *[]string) []goCacheEntry {
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
		if sparedByHolder(e.path(), held, errs) {
			continue
		}
		victims = append(victims, e)
		total -= e.size
	}
	return victims
}

func (r *CacheTrimReport) release(victims []goCacheEntry, unusedBefore time.Time, remove func(string) error) (files int, bytes int64) {
	return r.releaseVictims(victims, unusedBefore, remove, nil)
}

func (r *CacheTrimReport) releaseVictims(victims []goCacheEntry, unusedBefore time.Time, remove func(string) error, held func(string) (bool, error)) (files int, bytes int64) {
	for _, e := range victims {
		mtime, ok, err := entryMtime(e)
		if err != nil {
			r.Errors = append(r.Errors, err.Error())
			continue
		}
		if ok && !mtime.Before(unusedBefore) {
			continue
		}
		if ok && sparedByHolder(e.path(), held, &r.Errors) {
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

func entryMtime(e goCacheEntry) (time.Time, bool, error) {
	if e.isDir {
		_, newest, ok, err := statCacheDir(e.path())
		return newest, ok, err
	}
	info, err := os.Lstat(e.path())
	if errors.Is(err, fs.ErrNotExist) {
		return time.Time{}, false, nil
	}
	if err != nil {
		return time.Time{}, false, err
	}
	return info.ModTime(), true, nil
}

func totalSize(entries []goCacheEntry) int64 {
	var total int64
	for _, e := range entries {
		total += e.size
	}
	return total
}
