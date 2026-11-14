package launchrecord

import (
	"cmp"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/cloudlaunch"
)

const Subdir = "agent-runs"

type Record struct {
	RunID          string               `json:"run_id"`
	Profile        string               `json:"profile"`
	CLI            string               `json:"cli"`
	Family         string               `json:"family"`
	Model          string               `json:"model"`
	Verdict        string               `json:"verdict"`
	WritesSource   bool                 `json:"writes_source,omitempty"`
	TreeBefore     string               `json:"tree_state_sha_before"`
	TreeAfter      string               `json:"tree_state_sha_after"`
	ArtifactSHA256 string               `json:"artifact_sha256,omitempty"`
	Worktree       string               `json:"worktree,omitempty"`
	TS             string               `json:"ts"`
	Cloud          *cloudlaunch.Session `json:"cloud,omitempty"`
	Violations     []Violation          `json:"violations,omitempty"`
}

type Violation struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

const deliverableSuffix = ".deliverable.md"

func DeliverablePath(evolveDir, runID string) (string, error) {
	path, err := Path(evolveDir, runID)
	if err != nil {
		return "", err
	}
	return strings.TrimSuffix(path, ".json") + deliverableSuffix, nil
}

func KeepDeliverable(evolveDir, runID, body string, keep int) error {
	path, err := DeliverablePath(evolveDir, runID)
	if err != nil {
		return err
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("launch record %s: %w", runID, err)
	}
	staging := path + ".tmp"
	if err := os.WriteFile(staging, []byte(body), 0o644); err != nil {
		return fmt.Errorf("launch record %s: %w", runID, err)
	}
	if err := os.Rename(staging, path); err != nil {
		return errors.Join(fmt.Errorf("launch record %s: %w", runID, err), os.Remove(staging))
	}
	return pruneDeliverables(dir, keep)
}

func pruneDeliverables(dir string, keep int) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("launch records: %w", err)
	}
	stale, err := staleDeliverables(entries, keep)
	var errs []error
	for _, name := range stale {
		errs = append(errs, os.Remove(filepath.Join(dir, name)))
	}
	return errors.Join(append(errs, err)...)
}

func staleDeliverables(entries []fs.DirEntry, keep int) ([]string, error) {
	type kept struct {
		name string
		at   int64
	}
	var files []kept
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), deliverableSuffix) || e.IsDir() {
			continue
		}
		info, err := e.Info()
		if err != nil {
			return nil, fmt.Errorf("launch records: %w", err)
		}
		files = append(files, kept{e.Name(), info.ModTime().UnixNano()})
	}
	slices.SortFunc(files, func(a, b kept) int {
		return cmp.Or(cmp.Compare(b.at, a.at), strings.Compare(b.name, a.name))
	})
	var stale []string
	for _, f := range files[min(keep, len(files)):] {
		stale = append(stale, f.name)
	}
	return stale, nil
}

func (r Record) BoundTo(treeStateSHA string) bool {
	return treeStateSHA != "" && r.TreeBefore == treeStateSHA && r.TreeAfter == treeStateSHA
}

var runIDPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)

func Path(evolveDir, runID string) (string, error) {
	if !runIDPattern.MatchString(runID) {
		return "", fmt.Errorf("launch record: run id %q is not one name of letters, digits, '.', '_' and '-'", runID)
	}
	return filepath.Join(evolveDir, Subdir, runID+".json"), nil
}

func Write(evolveDir string, r Record) (string, error) {
	path, err := Path(evolveDir, r.RunID)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", fmt.Errorf("launch record %s: %w", r.RunID, err)
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return "", fmt.Errorf("launch record %s: %w", r.RunID, err)
	}
	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	return path, errors.Join(enc.Encode(r), f.Close())
}

func Read(evolveDir, runID string) (Record, error) {
	path, err := Path(evolveDir, runID)
	if err != nil {
		return Record{}, err
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return Record{}, fmt.Errorf("launch record %s: %w", runID, err)
	}
	var r Record
	if err := json.Unmarshal(raw, &r); err != nil {
		return Record{}, fmt.Errorf("launch record %s: %w", runID, err)
	}
	if r.RunID != runID {
		return Record{}, fmt.Errorf("launch record %s: the file holds the run id %q", runID, r.RunID)
	}
	return r, nil
}

func List(evolveDir string) ([]Record, error) {
	entries, err := os.ReadDir(filepath.Join(evolveDir, Subdir))
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("launch records: %w", err)
	}
	var out []Record
	for _, e := range entries {
		runID, isRecord := strings.CutSuffix(e.Name(), ".json")
		if !isRecord || e.IsDir() {
			continue
		}
		r, err := Read(evolveDir, runID)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	slices.SortStableFunc(out, func(a, b Record) int { return strings.Compare(a.TS, b.TS) })
	return out, nil
}
