package phaseblock

import (
	"errors"
	"fmt"
)

// ErrEmptyChain signals that no per-phase integrity was recorded (a pre-field
// checkpoint). Callers treat it as "fall back to the legacy pipeline check",
// not as tamper.
var ErrEmptyChain = errors.New("phaseblock: empty integrity chain")

// TamperError is returned when a chain fails verification: a broken link, a
// recomputed-Combined mismatch, or a binary whose provenance is unverifiable.
type TamperError struct {
	Phase  string
	Reason string
}

func (e *TamperError) Error() string {
	return fmt.Sprintf("phaseblock: integrity violation at phase %q: %s", e.Phase, e.Reason)
}

// Provenance reports whether a build-commit is verifiable — typically "is this
// commit an ancestor of HEAD?". Injected so phaseblock stays a git-free leaf.
type Provenance func(buildCommit string) bool

// Verify checks a recorded phase chain against the running binary's identity.
//
// It returns:
//   - ErrEmptyChain when the chain is empty (caller falls back to legacy).
//   - nil when the chain links are intact AND every phase's binary is either
//     the running binary or a provenance-verified one (and, when more than one
//     binary appears, the running binary's own commit is provenance-verified).
//   - *TamperError otherwise.
//
// The two checks are independent: chain integrity (links + recomputed
// Combined) catches post-hoc record tampering; provenance catches an
// unverifiable binary swap. A single-binary cycle is the common clean path.
func Verify(chain []Digest, runningSHA, runningCommit string, prov Provenance) error {
	if len(chain) == 0 {
		return ErrEmptyChain
	}
	// An unestablished running-binary identity cannot be verified — guard the
	// single-binary fast-path so an all-empty-sha chain never slips through.
	if runningSHA == "" {
		return &TamperError{Phase: "ship", Reason: "running binary sha could not be established"}
	}

	// 1. Chain integrity: recomputed Combined + back-pointer link.
	for i, d := range chain {
		if combine(d) != d.Combined {
			return &TamperError{Phase: d.Phase, Reason: "recomputed digest does not match recorded Combined"}
		}
		if i > 0 && d.PrevCombined != chain[i-1].Combined {
			return &TamperError{Phase: d.Phase, Reason: "broken chain link to previous phase"}
		}
	}

	// 2. Binary provenance. Fast path: every phase ran under the running
	// binary — a normal single-binary cycle, no provenance needed.
	allSame := true
	for _, d := range chain {
		if d.BinarySHA != runningSHA {
			allSame = false
			break
		}
	}
	if allSame {
		return nil
	}

	// More than one binary participated (e.g. a resume under a rebuild). The
	// running binary AND every differing phase binary must be provenance-verified.
	if runningCommit == "" || !prov(runningCommit) {
		return &TamperError{Phase: "ship", Reason: "running binary build-commit is unverifiable (not an ancestor of HEAD)"}
	}
	for _, d := range chain {
		if d.BinarySHA == runningSHA {
			continue
		}
		if d.BinaryCommit == "" || !prov(d.BinaryCommit) {
			return &TamperError{Phase: d.Phase, Reason: "phase binary build-commit is unverifiable (not an ancestor of HEAD)"}
		}
	}
	return nil
}
