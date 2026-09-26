package lifecycle

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/inboxbatch"
)

// Location is where an item lives; Cycle is 0 at the inbox root, else the cycle whose claim holds it.
type Location struct {
	Path  string
	Cycle int
}

// Locate finds an id's file, a processing claim before the inbox root; a missing id or inbox is ErrNotFound.
func Locate(inboxDir, id string) (Location, error) {
	for _, dir := range inboxbatch.ProcessingCycleDirs(inboxDir) {
		if path, ferr := FindFileByTaskID(dir, id); ferr == nil {
			cycle, _ := inboxbatch.ParseProcessingCycle(filepath.Base(dir))
			return Location{Path: path, Cycle: cycle}, nil
		}
	}
	path, err := FindFileByTaskID(inboxDir, id)
	switch {
	case err == nil:
		return Location{Path: path}, nil
	case errors.Is(err, ErrNotFound) || os.IsNotExist(err):
		return Location{}, ErrNotFound
	default:
		return Location{}, err
	}
}

// FindFileByTaskID finds the file in dir whose JSON id is taskID (filenames carry timestamps); no match is ErrNotFound.
func FindFileByTaskID(dir, taskID string) (string, error) {
	entries, err := jsonEntries(dir)
	if err != nil {
		return "", err
	}
	for _, e := range entries {
		path := filepath.Join(dir, e.Name())
		if id, ok := readID(path); ok && id == taskID {
			return path, nil
		}
	}
	return "", ErrNotFound
}

func jsonEntries(dir string) ([]os.DirEntry, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var out []os.DirEntry
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		out = append(out, e)
	}
	return out, nil
}

// readID reads a file's JSON id; ok is false only when the file cannot be read or parsed.
func readID(path string) (string, bool) {
	body, err := os.ReadFile(path)
	if err != nil {
		return "", false
	}
	var doc struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(body, &doc); err != nil {
		return "", false
	}
	return doc.ID, true
}

func readTaskIDOrUnknown(path string) string {
	if id, ok := readID(path); ok && id != "" {
		return id
	}
	return "unknown"
}

// ReadFailureCount returns taskID's failure_count from the inbox root or a processing claim; ok is false when absent.
func (m *Mover) ReadFailureCount(taskID string) (int, bool) {
	dirs := append([]string{m.inboxDir}, inboxbatch.ProcessingCycleDirs(m.inboxDir)...)
	for _, d := range dirs {
		path, err := FindFileByTaskID(d, taskID)
		if err != nil {
			continue
		}
		if n, ok := readFailureCountAt(path); ok {
			return n, true
		}
	}
	return 0, false
}

func readFailureCountAt(path string) (int, bool) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return 0, false
	}
	var doc struct {
		FailureCount int `json:"failure_count"`
	}
	if json.Unmarshal(raw, &doc) != nil {
		return 0, false
	}
	return doc.FailureCount, true
}

// BumpFailureCount atomically increments an item's failure_count, records reason and returns the new count.
func BumpFailureCount(path, reason string) (int, error) {
	return bumpWith(path, reason, nil)
}

// bumpWith sheds the continuation stamp in the same write once shedAt(count) holds:
// quarantine is terminal, so a revived item starts fresh.
func bumpWith(path, reason string, shedAt func(count int) bool) (int, error) {
	count := 0
	err := UpdateItemJSON(path, func(item map[string]json.RawMessage) {
		if raw, ok := item["failure_count"]; ok {
			_ = json.Unmarshal(raw, &count) // a non-numeric counter reads as 0 (tolerant)
		}
		count++
		item["failure_count"] = json.RawMessage(strconv.Itoa(count))
		if reason != "" {
			rb, _ := json.Marshal(reason) // a string never fails to marshal
			item["last_failure_reason"] = rb
		}
		if shedAt != nil && shedAt(count) {
			delete(item, "continuation")
		}
	})
	if err != nil {
		return 0, err
	}
	return count, nil
}

// UpdateItemJSON atomically rewrites an item with mutate applied to its top-level fields; mutate must not retain the map.
func UpdateItemJSON(path string, mutate func(m map[string]json.RawMessage)) error {
	body, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var item map[string]json.RawMessage
	if err := json.Unmarshal(body, &item); err != nil {
		return err
	}
	mutate(item)
	out, err := json.Marshal(item)
	if err != nil {
		return err
	}
	tmp := fmt.Sprintf("%s.tmp.%d", path, os.Getpid())
	if err := os.WriteFile(tmp, out, 0o644); err != nil {
		return err
	}
	return commitTmp(tmp, path)
}

func commitTmp(tmp, path string) error {
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}
