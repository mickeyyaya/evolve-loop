package practicewatch

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/mickeyyaya/evolve-loop/go/internal/atomicwrite"
)

type snapshot struct {
	Hash      string `json:"hash"`
	FetchedAt string `json:"fetched_at"`
	ETag      string `json:"etag,omitempty"`
	Excerpt   string `json:"excerpt"`
}

func snapshotPath(root, id string) (string, error) {
	if !sourceIDPattern.MatchString(id) {
		return "", fmt.Errorf("source id %q must be kebab-case: it names a snapshot file", id)
	}
	return filepath.Join(root, ".evolve", "practice", id+".json"), nil
}

func loadSnapshot(root, id string) (snapshot, bool, error) {
	path, err := snapshotPath(root, id)
	if err != nil {
		return snapshot{}, false, err
	}
	raw, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return snapshot{}, false, nil
	}
	if err != nil {
		return snapshot{}, false, fmt.Errorf("read the snapshot %s: %w", path, err)
	}
	var snap snapshot
	if err := json.Unmarshal(raw, &snap); err != nil || snap.Hash == "" {
		return snapshot{}, false, fmt.Errorf("the snapshot %s is not valid (delete it to record again): %v", path, err)
	}
	return snap, true, nil
}

func saveSnapshot(root, id string, snap snapshot) error {
	path, err := snapshotPath(root, id)
	if err != nil {
		return err
	}
	if err := atomicwrite.JSON(path, snap); err != nil {
		return fmt.Errorf("write the snapshot %s: %w", path, err)
	}
	return nil
}
