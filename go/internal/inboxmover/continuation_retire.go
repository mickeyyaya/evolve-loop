package inboxmover

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/continuation"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxbatch"
)

type releasedContinuation struct {
	continuation.Continuation
	ReleasedAt string `json:"released_at"`
	Reason     string `json:"reason"`
	ReleasedBy string `json:"released_by,omitempty"`
}

const retireAuthority = "runtime (inbox retirement)"

func releaseContinuationOnRetire(opts Options, itemPath, taskID, reason string) {
	if taskID == "" || taskID == "unknown" || opts.ProjectRoot == "" {
		return
	}
	c, ok, err := continuation.ReadRegistryEntry(opts.ProjectRoot, taskID)
	if err != nil {
		opts.logf("WARN: ", "continuation registry unreadable while retiring '%s' (%v) — binding NOT released, it will be refused at the next scope read", taskID, err)
		return
	}
	if !ok {
		return
	}
	if perr := appendReleasedContinuation(itemPath, continuation.RedactHostPaths(c), reason, retireAuthority, opts.Now().UTC()); perr != nil {
		opts.logf("WARN: ", "retire '%s': preserved pointer (snapshot %s) NOT written to %s: %v — releasing the binding anyway", taskID, c.SnapshotSHA, itemPath, perr)
	}
	released, derr := continuation.DeleteRegistryEntryIfCycle(opts.ProjectRoot, taskID, c.Cycle)
	switch {
	case derr != nil:
		opts.logf("WARN: ", "retire '%s': continuation binding release failed: %v", taskID, derr)
	case !released:
		opts.logf("WARN: ", "retire '%s': continuation binding was rebound by another lane between read and release — left intact (cycle %d no longer owns it)", taskID, c.Cycle)
	default:
		opts.logf("", "retire '%s': released continuation binding (snapshot %s, cycle %d) — pointer preserved in %s", taskID, c.SnapshotSHA, c.Cycle, filepath.Base(itemPath))
	}
}

var marshalReleasedContinuation = func(v releasedContinuation) ([]byte, error) { return json.Marshal(v) }

func appendReleasedContinuation(path string, c continuation.Continuation, reason, releasedBy string, at time.Time) error {
	entry, err := marshalReleasedContinuation(releasedContinuation{Continuation: c, ReleasedAt: at.Format(time.RFC3339), Reason: reason, ReleasedBy: releasedBy})
	if err != nil {
		return err
	}
	return updateItemJSON(path, func(m map[string]json.RawMessage) { releasePointer(m, entry, c.SnapshotSHA) })
}

func releasePointer(m map[string]json.RawMessage, entry []byte, snapshot string) {
	var live struct {
		SnapshotSHA string `json:"snapshot_sha"`
	}
	if snapshot != "" && json.Unmarshal(m[inboxbatch.ContinuationField], &live) == nil && live.SnapshotSHA == snapshot {
		delete(m, inboxbatch.ContinuationField)
	}
	appendReleasedEntry(m, entry)
}

func appendReleasedEntry(m map[string]json.RawMessage, entry []byte) {
	var list []json.RawMessage
	if raw, ok := m[inboxbatch.ReleasedContinuationsField]; ok {
		_ = json.Unmarshal(raw, &list)
	}
	list = append(list, entry)
	if out, merr := json.Marshal(list); merr == nil {
		m[inboxbatch.ReleasedContinuationsField] = out
	}
}

func scopeHasLiveItem(opts Options, scopeID string) bool {
	if strings.TrimSpace(scopeID) == "" {
		return false
	}
	_, err := Locate(opts.InboxDir, scopeID)
	return err == nil
}

func scopeRetiredAt(opts Options, scopeID string) (string, string) {
	if strings.TrimSpace(scopeID) == "" {
		return "", ""
	}
	for _, sub := range retirementStates {
		root := filepath.Join(opts.InboxDir, sub)
		found := ""
		_ = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
			if err != nil || found != "" {
				return nil
			}
			if d.IsDir() || !strings.HasSuffix(d.Name(), ".json") {
				return nil
			}
			if itemIDAt(path) == scopeID {
				found = path
			}
			return nil
		})
		if found != "" {
			return found, sub
		}
	}
	return "", ""
}

func itemIDAt(path string) string {
	body, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	var doc struct {
		ID string `json:"id"`
	}
	if json.Unmarshal(body, &doc) != nil {
		return ""
	}
	return inboxbatch.ItemID(doc.ID, path)
}
