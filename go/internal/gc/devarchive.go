package gc

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	devArchiveFile     = "dev-worktrees.jsonl"
	checkpointRefStart = "refs/checkpoints/"
)

type DevArchiveRecord struct {
	Path       string    `json:"path"`
	Branch     string    `json:"branch"`
	Head       string    `json:"head"`
	Checkpoint string    `json:"checkpoint,omitempty"`
	IdleDays   float64   `json:"idle_days"`
	ArchivedAt time.Time `json:"archived_at"`
}

func DevArchivePath(evolveDir string) string {
	return filepath.Join(evolveDir, "archive", devArchiveFile)
}

func AppendDevArchive(evolveDir string, rec DevArchiveRecord) error {
	line, err := json.Marshal(rec)
	if err != nil {
		return fmt.Errorf("gc: encode archive record for %s: %w", rec.Path, err)
	}
	path := DevArchivePath(evolveDir)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("gc: create archive dir for %s: %w", path, err)
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("gc: open %s: %w", path, err)
	}
	_, werr := f.Write(append(line, '\n'))
	return errors.Join(werr, f.Close())
}

func ReadDevArchive(evolveDir string) ([]DevArchiveRecord, error) {
	path := DevArchivePath(evolveDir)
	f, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("gc: open %s: %w", path, err)
	}
	defer func() { _ = f.Close() }()
	var recs []DevArchiveRecord
	sc := bufio.NewScanner(f)
	sc.Buffer(nil, 1<<20)
	for n := 1; sc.Scan(); n++ {
		if strings.TrimSpace(sc.Text()) == "" {
			continue
		}
		var rec DevArchiveRecord
		if err := json.Unmarshal(sc.Bytes(), &rec); err != nil {
			return nil, fmt.Errorf("gc: %s line %d: %w", path, n, err)
		}
		recs = append(recs, rec)
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("gc: read %s: %w", path, err)
	}
	return recs, nil
}

func (r DevArchiveRecord) RestoreCommands() []string {
	cmds := []string{fmt.Sprintf("git worktree add %s %s", r.Path, r.Branch)}
	if r.Checkpoint != "" {
		short := strings.TrimPrefix(r.Checkpoint, checkpointRefStart)
		cmds = append(cmds, fmt.Sprintf("evolve checkpoint restore %s --into %s", short, r.Path))
	}
	return cmds
}
