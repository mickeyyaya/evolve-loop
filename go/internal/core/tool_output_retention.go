package core

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/mickeyyaya/evolve-loop/go/internal/gcpolicy"
)

func pruneToolOutputOnPass(workspace, verdict string) error {
	if verdict != VerdictPASS || workspace == "" {
		return nil
	}
	var errs []error
	for _, name := range gcpolicy.ToolOutputFiles() {
		path := filepath.Join(workspace, name)
		if err := os.Remove(path); err != nil && !errors.Is(err, fs.ErrNotExist) {
			errs = append(errs, fmt.Errorf("delete raw tool output %s: %w", path, err))
		}
	}
	return errors.Join(errs...)
}
