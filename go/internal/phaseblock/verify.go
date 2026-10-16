package phaseblock

import (
	"errors"
	"fmt"
)

var ErrEmptyChain = errors.New("phaseblock: empty integrity chain")

type TamperError struct {
	Phase  string
	Reason string
}

func (e *TamperError) Error() string {
	return fmt.Sprintf("phaseblock: integrity violation at phase %q: %s", e.Phase, e.Reason)
}

type Provenance func(buildCommit string) bool

func Verify(chain []Digest, runningSHA, runningCommit string, prov Provenance) error {
	if len(chain) == 0 {
		return ErrEmptyChain
	}
	if runningSHA == "" {
		return &TamperError{Phase: "ship", Reason: "running binary sha could not be established"}
	}

	for i, d := range chain {
		if combine(d) != d.Combined {
			return &TamperError{Phase: d.Phase, Reason: "recomputed digest does not match recorded Combined"}
		}
		if i > 0 && d.PrevCombined != chain[i-1].Combined {
			return &TamperError{Phase: d.Phase, Reason: "broken chain link to previous phase"}
		}
	}

	if ranUnderBinary(chain, runningSHA) {
		return nil
	}

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

func ranUnderBinary(chain []Digest, sha string) bool {
	for _, d := range chain {
		if d.BinarySHA != sha {
			return false
		}
	}
	return true
}
