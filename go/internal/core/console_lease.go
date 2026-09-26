package core

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/plane"
)

// consoleLease lives in the git common dir, outside every worktree, so no lane
// phase can author or shadow it.
// See ADR-0080.
type consoleLease struct {
	Paths     []string `json:"paths"`
	ExpiresAt string   `json:"expires_at"` // RFC3339; REQUIRED — no expiry, no lease
	Reason    string   `json:"reason,omitempty"`
}

// consoleLeasePath is empty when the hub cannot be resolved, which leaves the guard armed.
func consoleLeasePath(projectRoot string) string {
	info, err := plane.Classify(projectRoot)
	if err != nil {
		return ""
	}
	common, err := plane.CommonGitDir(info)
	if err != nil {
		return ""
	}
	return filepath.Join(common, plane.ConsoleLeaseFileName)
}

// readConsoleLease returns an empty set for an absent, malformed, expiry-less
// or expired lease, so the guard stays armed in every degraded shape.
func readConsoleLease(projectRoot string, now time.Time) map[string]bool {
	leased, _ := readConsoleLeaseRaw(projectRoot, now)
	return leased
}

// readConsoleLeaseRaw also returns the parsed bytes, so the adoption digest
// describes what was adopted rather than a second read of the file.
func readConsoleLeaseRaw(projectRoot string, now time.Time) (map[string]bool, []byte) {
	path := consoleLeasePath(projectRoot)
	if path == "" {
		return nil, nil
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, nil
	}
	var lease consoleLease
	if json.Unmarshal(raw, &lease) != nil {
		return nil, nil
	}
	expires, perr := time.Parse(time.RFC3339, lease.ExpiresAt)
	if perr != nil || now.After(expires) {
		return nil, nil
	}
	leased := make(map[string]bool, len(lease.Paths))
	for _, p := range lease.Paths {
		if p != "" {
			leased[filepath.ToSlash(p)] = true
		}
	}
	return leased, raw
}

// adoptConsoleLease runs once at cycle start, so a mid-cycle write cannot waive
// its own cycle; the logged digest ties each waiver to the lease that granted it.
func adoptConsoleLease(projectRoot string, now time.Time, warn io.Writer) map[string]bool {
	leased, raw := readConsoleLeaseRaw(projectRoot, now)
	if len(leased) == 0 {
		return nil
	}
	sum := sha256.Sum256(raw)
	fmt.Fprintf(warn, "[orchestrator] console-lease adopted for this cycle: %d path(s), digest %s (ADR-0080 S4 — every waiver will WARN)\n", len(leased), hex.EncodeToString(sum[:])[:12])
	return leased
}
