package gc

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/gcpolicy"
)

const (
	guardsLogName              = "guards.log"
	ActionRotate        Action = "rotate"
	guardsLogArchiveDir        = "dispatch-logs"
)

func planGuardsLog(evolveDir string, pol Policy, add func(string, Action, string)) {
	path := filepath.Join(evolveDir, guardsLogName)
	if info, err := os.Stat(path); err == nil && info.Mode().IsRegular() && info.Size() > pol.GuardsLogMaxBytes() {
		add(path, ActionRotate, "guards_log_max_mb")
	}
}

func rotateItem(evolveDir, path string, now time.Time) error {
	dir := filepath.Join(evolveDir, guardsLogArchiveDir)
	stem := "guards-" + gcpolicy.LogRunID(now)
	dst := filepath.Join(dir, stem+".log")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("gc: rotate %s: %w", path, err)
	}
	for n := 1; ; n++ {
		if _, err := os.Lstat(dst); err != nil {
			break
		}
		dst = filepath.Join(dir, stem+"."+strconv.Itoa(n)+".log")
	}
	if err := os.Rename(path, dst); err != nil {
		return fmt.Errorf("gc: rotate %s → %s: %w", path, dst, err)
	}
	return nil
}
