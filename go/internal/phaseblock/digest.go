// Package phaseblock models each pipeline phase as a content-addressed, independently verifiable block.
// See docs/architecture/packages/internal-phaseblock.md.
package phaseblock

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
)

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

type DigestSource interface {
	BinarySHA() (string, error)
	BinaryCommit() string
	ProfileSHA() (string, error)
	ReportSHA() (string, error)
	TreeSHA() (string, error)
}

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

func combine(d Digest) string {
	canonical := d.Phase + "\x00" + d.BinarySHA + "\x00" + d.BinaryCommit + "\x00" +
		d.ProfileSHA + "\x00" + d.ReportSHA + "\x00" + d.TreeSHA + "\x00" + d.PrevCombined
	sum := sha256.Sum256([]byte(canonical))
	return hex.EncodeToString(sum[:])
}
