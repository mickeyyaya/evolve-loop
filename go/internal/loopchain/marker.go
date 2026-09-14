package loopchain

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/phaseintegrity"
)

// BatchRecord is one chained batch's boundary observation. FleetCount is
// recorded per batch because fleet width is a hard operator commitment: a
// chain that silently narrows lanes from batch N to N+1 looks healthy in
// every other signal.
type BatchRecord struct {
	Batch        int `json:"batch"`
	RC           int `json:"rc"`
	FleetCount   int `json:"fleet_count"`
	InboxPending int `json:"inbox_pending"`
}

// RefreshLogEntry is one JSONL line of the boundary-refresh audit trail —
// the additive record that tells an automatic boundary re-pin apart from a
// manual `evolve reset-sha`.
type RefreshLogEntry struct {
	Batch           int    `json:"batch"`
	AuthorizedClass string `json:"authorized_class"`
	Timestamp       string `json:"timestamp"`
	OldSHA          string `json:"old_sha,omitempty"`
	NewSHA          string `json:"new_sha,omitempty"`
}

// attempt is the re-exec loop breaker's wire shape: the running build commit
// that triggered a refresh; a LATER attempt carrying the SAME commit means the
// previous re-exec came back on a binary that had not moved.
type attempt struct {
	RunningCommit string `json:"running_commit"`
	Batch         int    `json:"batch"`
	Timestamp     string `json:"timestamp"`
}

// alreadyAttempted reports whether a refresh was already performed for
// runningCommit. An absent/unreadable/corrupt marker means "no prior attempt"
// — the breaker must never be the thing that stops a legitimate first
// refresh.
func (r *Refresher) alreadyAttempted(runningCommit string) bool {
	if runningCommit == "" {
		return false
	}
	raw, err := os.ReadFile(filepath.Join(r.roots.EvolveDir, r.opts.attemptFile))
	if err != nil {
		return false
	}
	var rec attempt
	if err := json.Unmarshal(raw, &rec); err != nil {
		return false
	}
	return rec.RunningCommit == runningCommit
}

// recordAttempt persists the marker for runningCommit, arming the breaker
// against the next boundary.
func (r *Refresher) recordAttempt(runningCommit string, batch int) error {
	buf, err := json.Marshal(attempt{RunningCommit: runningCommit, Batch: batch, Timestamp: r.opts.now().UTC().Format(time.RFC3339)})
	if err == nil {
		err = os.WriteFile(filepath.Join(r.roots.EvolveDir, r.opts.attemptFile), buf, 0o644)
	}
	if err != nil {
		return fmt.Errorf("write boundary-refresh attempt marker: %w", err)
	}
	return nil
}

// appendLog appends one audit record. Best-effort: a failure here must not
// itself halt the chain (the pin already moved).
func (r *Refresher) appendLog(batch int, res phaseintegrity.RepinResult) error {
	entry := RefreshLogEntry{
		Batch:           batch,
		AuthorizedClass: AuthorizedClassBoundaryRefresh,
		Timestamp:       r.opts.now().UTC().Format(time.RFC3339),
		OldSHA:          res.OldSHA,
		NewSHA:          res.NewSHA,
	}
	f, err := os.OpenFile(r.logPath(), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("open boundary-refresh log: %w", err)
	}
	err = json.NewEncoder(f).Encode(entry) // json.Marshal's bytes plus the newline
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	return wrapLogWrite(err)
}

// wrapLogWrite names a failed audit append; nil passes through.
func wrapLogWrite(err error) error {
	if err != nil {
		return fmt.Errorf("write boundary-refresh log: %w", err)
	}
	return nil
}

func (r *Refresher) logPath() string { return filepath.Join(r.roots.EvolveDir, r.opts.logFile) }

// LastRefreshLogEntry reads the audit trail at logPath and returns the LAST
// (most-recently-appended) entry. Best-effort, nil-when-clean: a missing
// file, an empty file, or a read/parse error all resolve to (nil, nil) —
// surfacing a refresh event into the summary must never itself become a new
// failure mode for an already-successful boundary refresh (Q-C4).
func LastRefreshLogEntry(logPath string) (*RefreshLogEntry, error) {
	raw, err := os.ReadFile(logPath)
	if err != nil {
		return nil, nil
	}
	lines := strings.Split(strings.TrimRight(string(raw), "\n"), "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			continue
		}
		var entry RefreshLogEntry
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			return nil, nil
		}
		return &entry, nil
	}
	return nil, nil
}
