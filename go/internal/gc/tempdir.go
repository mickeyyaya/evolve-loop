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

type tempAction string

const (
	tempActionRemove       tempAction = "remove"
	tempActionKeepFresh    tempAction = "keep_fresh"
	tempActionSpareHeld    tempAction = "spare_held"
	tempActionSpareUnknown tempAction = "spare_unknown"
)

type tempClassification struct {
	path     string
	action   tempAction
	rule     string
	evidence string
}

func classifyTempDir(dir string, cutoff time.Time, cfg tempConfig) ([]tempClassification, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read temp dir %s: %w", dir, err)
	}
	var items []tempClassification
	for _, e := range entries {
		if !pipelineTempRe.MatchString(e.Name()) {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		items = append(items, classifyTempEntry(filepath.Join(dir, e.Name()), info.ModTime(), cutoff, cfg.held))
	}
	return items, nil
}

func classifyTempEntry(path string, modTime, cutoff time.Time, heldFn func(string) (bool, error)) tempClassification {
	if !modTime.Before(cutoff) {
		return tempClassification{
			path:     path,
			action:   tempActionKeepFresh,
			rule:     "temp.fresh",
			evidence: fmt.Sprintf("mtime %s is at or after cutoff %s", modTime.UTC().Format(time.RFC3339), cutoff.UTC().Format(time.RFC3339)),
		}
	}
	if heldFn != nil {
		held, err := heldFn(path)
		if err != nil {
			return tempClassification{
				path:     path,
				action:   tempActionSpareUnknown,
				rule:     "temp.holder_unknown",
				evidence: fmt.Sprintf("spared %s: liveness unknown: %v", path, err),
			}
		}
		if held {
			return tempClassification{
				path:     path,
				action:   tempActionSpareHeld,
				rule:     "temp.held",
				evidence: "held by a live process",
			}
		}
	}
	return tempClassification{
		path:     path,
		action:   tempActionRemove,
		rule:     "temp.ttl_hours",
		evidence: fmt.Sprintf("mtime %s is before cutoff %s; no live holder", modTime.UTC().Format(time.RFC3339), cutoff.UTC().Format(time.RFC3339)),
	}
}

func ReapPipelineTemp(dir string, cutoff time.Time, removeAll func(string) error, opts ...TempOption) TempReapReport {
	var cfg tempConfig
	for _, o := range opts {
		o(&cfg)
	}
	var rep TempReapReport
	items, err := classifyTempDir(dir, cutoff, cfg)
	if err != nil {
		rep.Errors = append(rep.Errors, err.Error())
		return rep
	}
	for _, it := range items {
		switch it.action {
		case tempActionKeepFresh:
			continue
		case tempActionSpareUnknown:
			rep.Errors = append(rep.Errors, it.evidence)
		case tempActionSpareHeld:
			rep.Held++
		case tempActionRemove:
			size := treeSize(it.path)
			if err := removeAll(it.path); err != nil {
				rep.Errors = append(rep.Errors, err.Error())
				continue
			}
			rep.Entries++
			rep.Bytes += size
		}
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
