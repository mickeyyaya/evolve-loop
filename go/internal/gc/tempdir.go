package gc

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"time"
)

var pipelineTempRe = regexp.MustCompile(`^(go-build\d+|acs(-cycle)?\d+-.+|cycle\d+-.+|Test.+|release-pipeline-dryrun-.+\.json)$`)

type TempReapReport struct {
	Entries int      `json:"entries"`
	Bytes   int64    `json:"bytes"`
	Errors  []string `json:"errors,omitempty"`
}

func ReapPipelineTemp(dir string, cutoff time.Time, apply bool) TempReapReport {
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
		size := treeSize(p)
		if apply {
			if err := os.RemoveAll(p); err != nil {
				rep.Errors = append(rep.Errors, err.Error())
				continue
			}
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
