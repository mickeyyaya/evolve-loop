package checkpoint

import (
	"cmp"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/flock"
)

var errNoCheckpoint = errors.New("checkpoint: the state has no checkpoint block")

const autoResumeSpentAction = "the auto-resume budget is spent: check the usage of the walled CLIs, then run `evolve loop --resume` by hand"

func RecordResume(path string, auto bool) (Checkpoint, bool, error) {
	var cp Checkpoint
	admitted := false
	err := flock.WithPathLock(path, func() error {
		existing, err := readBlockAt(path)
		if err != nil {
			return err
		}
		cp, admitted = nextResume(existing, auto)
		return applyWithHooks(defaultHooks(), path, cp)
	})
	if err != nil {
		return Checkpoint{}, false, err
	}
	return cp, admitted, nil
}

func nextResume(cp Checkpoint, auto bool) (Checkpoint, bool) {
	switch {
	case !auto:
		cp.AutoResumeAttempts, cp.OperatorAction = 0, ""
	case cp.AutoResumeAttempts >= cp.AutoResumeMaxAttempts:
		return withAutoResumeBudget(cp, cp.AutoResumeAttempts), false
	default:
		cp.AutoResumeAttempts++
	}
	return cp, true
}

func withAutoResumeBudget(cp Checkpoint, attempts int) Checkpoint {
	cp.AutoResumeAttempts = attempts
	if attempts >= cp.AutoResumeMaxAttempts {
		cp.OperatorAction = cmp.Or(cp.OperatorAction, autoResumeSpentAction)
	}
	return cp
}

func carriedAttempts(path string) (int, error) {
	cp, err := readBlockAt(path)
	switch {
	case errors.Is(err, errNoCheckpoint):
		return 0, nil
	case err != nil:
		return 0, err
	}
	return cp.AutoResumeAttempts, nil
}

func applyQuotaPause(path string, cp Checkpoint) error {
	attempts, carryErr := carriedAttempts(path)
	if carryErr != nil {
		attempts = cp.AutoResumeMaxAttempts
	}
	if err := applyWithHooks(defaultHooks(), path, withAutoResumeBudget(cp, attempts)); err != nil {
		return err
	}
	if carryErr != nil {
		return fmt.Errorf("checkpoint: the auto-resume count is unreadable, so this pause spends the budget: %w", carryErr)
	}
	return nil
}

func readBlockAt(path string) (Checkpoint, error) {
	b, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return Checkpoint{}, fmt.Errorf("%w: %w", errNoCheckpoint, err)
	}
	if err != nil {
		return Checkpoint{}, fmt.Errorf("checkpoint: read state: %w", err)
	}
	var state struct {
		Checkpoint *Checkpoint `json:"checkpoint"`
	}
	if err := json.Unmarshal(b, &state); err != nil {
		return Checkpoint{}, fmt.Errorf("checkpoint: parse state: %w", err)
	}
	if state.Checkpoint == nil {
		return Checkpoint{}, fmt.Errorf("%w: %s", errNoCheckpoint, path)
	}
	return *state.Checkpoint, nil
}
