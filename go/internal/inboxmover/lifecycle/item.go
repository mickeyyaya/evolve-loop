package lifecycle

// item.go — the processed-record primitives (inboxmover.go:787-893, :966-993
// and claimstate.go:21-53 on the base): the one id→file resolver over one dir
// (FindFileByTaskID) and over the claim layout (Locate), the one *.json
// iterator, the one atomic item rewrite and the failure counter's one reader
// and one writer.

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

// Location is where an inbox item currently lives. Cycle is 0 while the item
// is pending at the inbox root and the claiming cycle once it sits under
// processing/cycle-<Cycle>/.
type Location struct {
	Path  string
	Cycle int
}

// Locate resolves an item id to its file. Liveness order is Promote's: a
// processing claim first (a lane holding the item outranks a stale root copy
// of the same id), then the pending root. An id with no file in either place —
// including a project with no inbox at all — is ErrNotFound; only a read fault
// on an existing directory is returned as itself.
//
// The two reads are not one atomic snapshot: a rename of this very id landing
// between them (a sibling lane claiming it mid-scan) can read as ErrNotFound
// once. Accepted: every caller re-reads on its next step (the gate's
// correction ladder re-verifies, Promote re-resolves), and a false "absent"
// never fails anything closed.
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

// FindFileByTaskID resolves a task id to its file within one inbox directory
// (ids live INSIDE the JSON; filenames carry timestamps). The ReadDir error is
// returned as itself; an unreadable or malformed file is skipped; no match is
// the bare ErrNotFound.
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

// jsonEntries is the ONE spelling of "iterate <dir>/*.json, skip directories";
// the ReadDir error is the caller's to swallow or return.
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

// readID reads the JSON .id of a file; ok is false when the file cannot be
// read or parsed (an item without an id reads as "", true).
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

// readTaskIDOrUnknown returns the JSON .id of a file, or "unknown" on failure.
func readTaskIDOrUnknown(path string) string {
	if id, ok := readID(path); ok && id != "" {
		return id
	}
	return "unknown"
}

// ReadFailureCount resolves taskID across the inbox root and processing/
// cycle-* dirs and returns its durable failure_count (written by the drain's
// bump on FAIL release). (0,false) = item not found; (0,true) = item present,
// never failed. Read-only; malformed JSON reads as not-found (the tolerant-
// reader convention). quarantine/ and retry/ are never walked.
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

// readFailureCountAt reads one record's failure_count; ok is false when the
// record cannot be read or parsed.
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

// BumpFailureCount increments the durable "failure_count" on an inbox item
// (the single source of truth for ADR-0072 S5 task-level failure memory) and
// stamps the latest failure reason, preserving every other field. Returns the
// new count. Atomic (write-tmp + rename) so a crash never leaves a
// half-written item. Any parse/IO error is returned so the caller can fail
// open. It never sheds the continuation stamp (the drain's bumpWith does).
func BumpFailureCount(path, reason string) (int, error) {
	return bumpWith(path, reason, nil)
}

// bumpWith is the ONE atomic rewrite of the failure counter: the count and
// the reason land in one write, and when shedAt reports the new count reached
// the ceiling the item's continuation stamp is shed in the SAME bytes —
// quarantine is terminal parking, so an operator revival starts fresh
// (ADR-0076 slice C). One rename instead of the two the old bump-then-shed
// performed; identical final bytes.
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

// UpdateItemJSON reads an inbox item, applies mutate to its top-level field
// map (preserving every field the loop does not touch), and writes it back
// atomically (write-tmp + rename; json.Marshal ⇒ sorted keys, no indent — the
// first touch normalises an item's key order). Any parse/IO error is returned
// so callers can fail open. mutate must not retain the map after returning.
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

// commitTmp renames the written tmp over path, removing the tmp (best-effort)
// when the rename fails.
func commitTmp(tmp, path string) error {
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}
