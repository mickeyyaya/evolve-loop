// Package phaseblock models each pipeline phase as a content-addressed,
// independently verifiable "agent block" (ADR-0065). A phase's Digest covers
// the binary that ran it, that binary's build-commit (provenance), the agent
// profile/prompt, the phase's report artifact, and the worktree tree — chained
// to the previous phase's Combined. A chain of digests replaces the single
// pipeline-level binary pin as the integrity record.
//
// This package is a leaf: it depends only on the stdlib (plus the injected
// DigestSource / Provenance seams), never on core/ship/git, so it stays
// trivially unit-testable and cycle-free.
package phaseblock

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
)

// Digest is the per-phase content-addressed integrity record. Combined is the
// content address — a stable hash of the integrity-relevant fields and the
// previous phase's Combined (the chain link). RunID/CompletedAt are metadata
// and are intentionally NOT part of Combined, so regenerating a phase with
// identical content is idempotent.
type Digest struct {
	Phase        string `json:"phase"`
	BinarySHA    string `json:"binarySha"`
	BinaryCommit string `json:"binaryCommit,omitempty"`
	ProfileSHA   string `json:"profileSha,omitempty"`
	ReportSHA    string `json:"reportSha,omitempty"`
	TreeSHA      string `json:"treeSha,omitempty"`
	PrevCombined string `json:"prevCombined,omitempty"`
	Combined     string `json:"combined"`
	RunID        string `json:"runId,omitempty"`
	CompletedAt  string `json:"completedAt,omitempty"`
}

// DigestSource supplies the per-phase pre-image SHAs. It is the DI seam that
// keeps phaseblock free of IO/git: real implementations read the binary, the
// profile+prompt, the report artifact, and `git write-tree`; tests inject
// canned values.
type DigestSource interface {
	BinarySHA() (string, error)
	BinaryCommit() string
	ProfileSHA() (string, error)
	ReportSHA() (string, error) // "" (nil err) when the phase has no report
	TreeSHA() (string, error)   // "" (nil err) when the phase has no worktree
}

// Compute builds the digest for one phase from its source, chaining
// prevCombined (the previous phase's Combined; "" for the first phase).
func Compute(phase, runID, completedAt, prevCombined string, src DigestSource) (Digest, error) {
	binSHA, err := src.BinarySHA()
	if err != nil {
		return Digest{}, fmt.Errorf("phaseblock: binary sha for %s: %w", phase, err)
	}
	profSHA, err := src.ProfileSHA()
	if err != nil {
		return Digest{}, fmt.Errorf("phaseblock: profile sha for %s: %w", phase, err)
	}
	reportSHA, err := src.ReportSHA()
	if err != nil {
		return Digest{}, fmt.Errorf("phaseblock: report sha for %s: %w", phase, err)
	}
	treeSHA, err := src.TreeSHA()
	if err != nil {
		return Digest{}, fmt.Errorf("phaseblock: tree sha for %s: %w", phase, err)
	}
	d := Digest{
		Phase:        phase,
		BinarySHA:    binSHA,
		BinaryCommit: src.BinaryCommit(),
		ProfileSHA:   profSHA,
		ReportSHA:    reportSHA,
		TreeSHA:      treeSHA,
		PrevCombined: prevCombined,
		RunID:        runID,
		CompletedAt:  completedAt,
	}
	d.Combined = combine(d)
	return d, nil
}

// combine is the content-addressing function: a stable sha256 over the
// integrity-relevant fields (and the chain link), excluding run metadata. The
// NUL separators make the field concatenation unambiguous.
func combine(d Digest) string {
	canonical := d.Phase + "\x00" + d.BinarySHA + "\x00" + d.BinaryCommit + "\x00" +
		d.ProfileSHA + "\x00" + d.ReportSHA + "\x00" + d.TreeSHA + "\x00" + d.PrevCombined
	sum := sha256.Sum256([]byte(canonical))
	return hex.EncodeToString(sum[:])
}
