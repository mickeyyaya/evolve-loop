package core

// console_lease.go — ADR-0080 S4, defense in depth for the tree-diff guard.
// The plane separation (S1) makes operator edits in the runtime tree
// structurally absent; this lease covers the residual case where an operator
// MUST touch the runtime tree while lanes run. It is the loud, bounded
// variant of the "operator allowlist" the ADR rejected as a primary design:
// exact paths only, hard expiry required, malformed = armed guard, and every
// waived path WARNs so a waiver is never silent.
//
// Written by `evolve console-lease` (cli/opscmd); ADOPTED once at cycle
// start (cyclerun.go) and consulted by the guard's classifier chain in
// cyclerun_review.go beside the mint/eval exemptions.
//
// LOCATION (review BLOCK): the lease lives in the git COMMON dir (the hub,
// resolved via plane.CommonGitDir) — outside every worktree. A lane phase
// writing ".evolve/console-lease.json" inside ANY checkout writes a file no
// reader honors. Adoption at
// cycle start means a mid-cycle write — even to the hub — cannot waive the
// cycle that wrote it.

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

// consoleLease is the on-disk shape of .evolve/console-lease.json.
type consoleLease struct {
	Paths     []string `json:"paths"`
	ExpiresAt string   `json:"expires_at"` // RFC3339; REQUIRED — no expiry, no lease
	Reason    string   `json:"reason,omitempty"`
}

// consoleLeasePath resolves the hub-resident lease location for a project
// root. Empty on any resolution failure — no hub, no lease, guard armed.
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

// readConsoleLease returns the exact paths an ACTIVE lease waives, keyed for
// O(1) classifier lookup. Absent, malformed, expiry-less, or expired leases
// all yield an empty set — the guard stays armed in every degraded shape.
func readConsoleLease(projectRoot string, now time.Time) map[string]bool {
	leased, _ := readConsoleLeaseRaw(projectRoot, now)
	return leased
}

// readConsoleLeaseRaw additionally returns the exact bytes the adopted set
// was parsed from — the digest in the adoption log must describe what was
// ADOPTED, not a second read of a possibly-changed file (review LOW).
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

// adoptConsoleLease reads the hub lease ONCE at cycle start and logs its
// identity — the digest in the cycle log is what makes a waiver auditable
// back to the exact lease that granted it.
func adoptConsoleLease(projectRoot string, now time.Time, warn io.Writer) map[string]bool {
	leased, raw := readConsoleLeaseRaw(projectRoot, now)
	if len(leased) == 0 {
		return nil
	}
	sum := sha256.Sum256(raw)
	fmt.Fprintf(warn, "[orchestrator] console-lease adopted for this cycle: %d path(s), digest %s (ADR-0080 S4 — every waiver will WARN)\n", len(leased), hex.EncodeToString(sum[:])[:12])
	return leased
}
