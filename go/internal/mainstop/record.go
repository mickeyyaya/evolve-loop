package mainstop

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/flock"
	"github.com/mickeyyaya/evolve-loop/go/internal/atomicwrite"
)

const recordName = "main-red-stop.json"

func Path(evolveDir string) string {
	return filepath.Join(evolveDir, recordName)
}

func Read(evolveDir string) (Stop, bool, error) {
	data, err := os.ReadFile(Path(evolveDir))
	if errors.Is(err, fs.ErrNotExist) {
		return Stop{}, false, nil
	}
	if err != nil {
		return Stop{}, false, fmt.Errorf("mainstop: read: %w", err)
	}
	var s Stop
	if err := json.Unmarshal(data, &s); err != nil {
		return Stop{}, false, fmt.Errorf("mainstop: parse %s: %w", Path(evolveDir), err)
	}
	if s.Schema != Schema {
		return Stop{}, false, fmt.Errorf("mainstop: %s has schema %q, want %q", Path(evolveDir), s.Schema, Schema)
	}
	return s, true, nil
}

func Update(evolveDir string, fn func(Stop, bool) (Stop, error)) (Stop, error) {
	release, err := flock.PathLock(Path(evolveDir))
	if err != nil {
		return Stop{}, fmt.Errorf("mainstop: %w", err)
	}
	defer release()
	current, found, err := Read(evolveDir)
	if err != nil {
		return Stop{}, err
	}
	next, err := fn(current, found)
	if err != nil {
		return current, err
	}
	next.Schema = Schema
	if err := atomicwrite.JSON(Path(evolveDir), next); err != nil {
		return current, fmt.Errorf("mainstop: write: %w", err)
	}
	return next, nil
}

const judgedName = "main-red-judged.json"

type judgedRecord struct {
	SHA string `json:"sha"`
}

func JudgedPath(evolveDir string) string {
	return filepath.Join(evolveDir, judgedName)
}

func Judged(evolveDir, sha string) bool {
	return readJudged(evolveDir) == sha
}

func readJudged(evolveDir string) string {
	data, err := os.ReadFile(JudgedPath(evolveDir))
	if err != nil {
		return ""
	}
	var r judgedRecord
	if json.Unmarshal(data, &r) != nil {
		return ""
	}
	return r.SHA
}

func ClaimJudged(evolveDir, sha string) (bool, error) {
	release, err := flock.PathLock(Path(evolveDir))
	if err != nil {
		return false, fmt.Errorf("mainstop: %w", err)
	}
	defer release()
	if readJudged(evolveDir) == sha {
		return false, nil
	}
	if err := atomicwrite.JSON(JudgedPath(evolveDir), judgedRecord{SHA: sha}); err != nil {
		return false, fmt.Errorf("mainstop: write the judged record: %w", err)
	}
	return true, nil
}

func ReleaseJudged(evolveDir, sha string) error {
	release, err := flock.PathLock(Path(evolveDir))
	if err != nil {
		return fmt.Errorf("mainstop: %w", err)
	}
	defer release()
	if readJudged(evolveDir) != sha {
		return nil
	}
	if err := os.Remove(JudgedPath(evolveDir)); err != nil {
		return fmt.Errorf("mainstop: release the judged record: %w", err)
	}
	return nil
}
