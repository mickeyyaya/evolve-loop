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

type BatchRecord struct {
	Batch        int `json:"batch"`
	RC           int `json:"rc"`
	FleetCount   int `json:"fleet_count"`
	InboxPending int `json:"inbox_pending"`
}

type RefreshLogEntry struct {
	Batch           int    `json:"batch"`
	AuthorizedClass string `json:"authorized_class"`
	Timestamp       string `json:"timestamp"`
	OldSHA          string `json:"old_sha,omitempty"`
	NewSHA          string `json:"new_sha,omitempty"`
}

type attempt struct {
	RunningCommit string `json:"running_commit"`
	Batch         int    `json:"batch"`
	Timestamp     string `json:"timestamp"`
	PID           int    `json:"pid,omitempty"`
	WavesDone     int    `json:"waves_done,omitempty"`
}

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

func (r *Refresher) recordAttempt(runningCommit string, batch int) error {
	buf, err := json.Marshal(attempt{RunningCommit: runningCommit, Batch: batch, Timestamp: r.opts.now().UTC().Format(time.RFC3339), PID: r.opts.handoff.PID, WavesDone: r.opts.handoff.WavesDone})
	if err == nil {
		err = os.WriteFile(filepath.Join(r.roots.EvolveDir, r.opts.attemptFile), buf, 0o644)
	}
	if err != nil {
		return fmt.Errorf("write boundary-refresh attempt marker: %w", err)
	}
	return nil
}

type Claim struct {
	PID    int
	Commit string
	At     time.Time
}

func TakeHandoff(attemptPath string, c Claim) (wavesDone int, taken bool, err error) {
	raw, err := os.ReadFile(attemptPath)
	if err != nil {
		return 0, false, nil
	}
	var rec attempt
	if json.Unmarshal(raw, &rec) != nil || !rec.handsOffTo(c) {
		return 0, false, nil
	}
	buf, _ := json.Marshal(attempt{RunningCommit: rec.RunningCommit, Batch: rec.Batch, Timestamp: rec.Timestamp})
	if err := os.WriteFile(attemptPath, buf, 0o644); err != nil {
		return 0, false, fmt.Errorf("consume the boundary re-exec handoff: %w", err)
	}
	return rec.WavesDone, true, nil
}

func (a attempt) handsOffTo(c Claim) bool {
	armed, err := time.Parse(time.RFC3339, a.Timestamp)
	return err == nil && a.PID != 0 && a.PID == c.PID && a.RunningCommit != c.Commit && c.At.Sub(armed) <= handoffMaxAge
}

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
	err = json.NewEncoder(f).Encode(entry)
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	return wrapLogWrite(err)
}

func wrapLogWrite(err error) error {
	if err != nil {
		return fmt.Errorf("write boundary-refresh log: %w", err)
	}
	return nil
}

func (r *Refresher) logPath() string { return filepath.Join(r.roots.EvolveDir, r.opts.logFile) }

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

const handoffMaxAge = 5 * time.Minute
