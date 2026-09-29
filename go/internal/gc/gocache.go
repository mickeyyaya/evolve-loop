package gc

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"time"
)

var goCacheShardRe = regexp.MustCompile(`^[0-9a-f]{2}$`)
var goCacheEntryRe = regexp.MustCompile(`^[0-9a-f]+-[ad]$`)

type CacheTrimReport struct {
	Files  int      `json:"files"`
	Bytes  int64    `json:"bytes"`
	Errors []string `json:"errors,omitempty"`
}

func TrimGoCache(dir string, cutoff time.Time, apply bool) CacheTrimReport {
	var rep CacheTrimReport
	shards, err := os.ReadDir(dir)
	if err != nil {
		rep.Errors = append(rep.Errors, fmt.Sprintf("read go build cache %s: %v", dir, err))
		return rep
	}
	for _, s := range shards {
		if s.IsDir() && goCacheShardRe.MatchString(s.Name()) {
			trimShard(filepath.Join(dir, s.Name()), cutoff, apply, &rep)
		}
	}
	return rep
}

func trimShard(shard string, cutoff time.Time, apply bool, rep *CacheTrimReport) {
	entries, err := os.ReadDir(shard)
	if err != nil {
		rep.Errors = append(rep.Errors, fmt.Sprintf("read %s: %v", shard, err))
		return
	}
	for _, e := range entries {
		if e.IsDir() || !goCacheEntryRe.MatchString(e.Name()) {
			continue
		}
		info, err := e.Info()
		if err != nil || !info.ModTime().Before(cutoff) {
			continue
		}
		if apply {
			if err := os.Remove(filepath.Join(shard, e.Name())); err != nil && !os.IsNotExist(err) {
				rep.Errors = append(rep.Errors, err.Error())
				continue
			}
		}
		rep.Files++
		rep.Bytes += info.Size()
	}
}
