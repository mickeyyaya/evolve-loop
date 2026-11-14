package gc

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"time"
)

var pipelineTempRe = regexp.MustCompile(`^(go-build\d+|acs(-cycle)?\d+-.+|cycle\d+-.+|release-pipeline-dryrun-.+\.json|Test[\w.-]*\d{6,}|gittest\d{6,})$`)

type TempReapReport struct {
	Entries int      `json:"entries"`
	Held    int      `json:"held,omitempty"`
	Bytes   int64    `json:"bytes"`
	Errors  []string `json:"errors,omitempty"`
}

type TempOption func(*tempConfig)

type tempConfig struct {
	held func(path string) (bool, error)
}

func SparingHeld(held func(path string) (bool, error)) TempOption {
	return func(c *tempConfig) { c.held = held }
}

func ReapPipelineTemp(dir string, cutoff time.Time, removeAll func(string) error, opts ...TempOption) TempReapReport {
	var cfg tempConfig
	for _, o := range opts {
		o(&cfg)
	}
	var rep TempReapReport
	entries, err := os.ReadDir(dir)
	if err != nil {
		rep.Errors = append(rep.Errors, fmt.Sprintf("read temp dir %s: %v", dir, err))
		return rep
	}
	for _, e := range entries {
		if !pipelineTempRe.MatchString(e.Name()) {
			continue
		}
		info, err := e.Info()
		if err != nil || !info.ModTime().Before(cutoff) {
			continue
		}
		p := filepath.Join(dir, e.Name())
		if cfg.held != nil {
			held, err := cfg.held(p)
			if err != nil {
				rep.Errors = append(rep.Errors, fmt.Sprintf("spared %s: liveness unknown: %v", p, err))
				continue
			}
			if held {
				rep.Held++
				continue
			}
		}
		size := treeSize(p)
		if err := removeAll(p); err != nil {
			rep.Errors = append(rep.Errors, err.Error())
			continue
		}
		rep.Entries++
		rep.Bytes += size
	}
	return rep
}

func treeSize(root string) int64 {
	var n int64
	_ = filepath.WalkDir(root, func(_ string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		if info, err := d.Info(); err == nil {
			n += info.Size()
		}
		return nil
	})
	return n
}
