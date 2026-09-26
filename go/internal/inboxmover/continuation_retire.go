package inboxmover

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/continuation"
)

// releasedContinuation embeds Continuation so the preserved pointer keeps the one continuation schema.
type releasedContinuation struct {
	continuation.Continuation
	ReleasedAt string `json:"released_at"`
	Reason     string `json:"reason"`
	// ReleasedBy names the authority behind the release; records older than the field omit it.
	ReleasedBy string `json:"released_by,omitempty"`
}

const retireAuthority = "runtime (inbox retirement)"

// releaseContinuationOnRetire releases taskID's binding as part of its retirement,
// loudly but never blocking. It preserves before it releases, so a crash cannot lose the pointer.
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
	// The preserved pointer rides the ship commit to a public remote, so host paths are redacted first.
	if perr := appendReleasedContinuation(itemPath, continuation.RedactHostPaths(c), reason, retireAuthority, opts.Now().UTC()); perr != nil {
		opts.logf("WARN: ", "retire '%s': preserved pointer (snapshot %s) NOT written to %s: %v — releasing the binding anyway", taskID, c.SnapshotSHA, itemPath, perr)
	}
	// Delete only if still this cycle's, so a sibling lane's fresh rebinding survives.
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

// appendReleasedContinuation appends c to path's released_continuations[]; a
// malformed existing value is replaced, because the entry being written is the one that matters.
func appendReleasedContinuation(path string, c continuation.Continuation, reason, releasedBy string, at time.Time) error {
	entry, err := json.Marshal(releasedContinuation{
		Continuation: c,
		ReleasedAt:   at.Format(time.RFC3339),
		Reason:       reason,
		ReleasedBy:   releasedBy,
	})
	if err != nil {
		return err
	}
	return updateItemJSON(path, func(m map[string]json.RawMessage) {
		var list []json.RawMessage
		if raw, ok := m["released_continuations"]; ok {
			_ = json.Unmarshal(raw, &list)
		}
		list = append(list, entry)
		if out, merr := json.Marshal(list); merr == nil {
			m["released_continuations"] = out
		}
	})
}

// scopeHasLiveItem reports whether scopeID sits at the inbox root or in a claim,
// the only places the batch loader can still pick it from.
func scopeHasLiveItem(opts Options, scopeID string) bool {
	if strings.TrimSpace(scopeID) == "" {
		return false
	}
	_, err := Locate(opts.InboxDir, scopeID)
	return err == nil
}

// scopeRetiredAt returns the retired copy's path and subtree, or ("", ""). Only
// positive evidence counts: carryover lane scopes never have an inbox file at all.
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
	return doc.ID
}
