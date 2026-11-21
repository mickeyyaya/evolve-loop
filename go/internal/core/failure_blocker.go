package core

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/cycleblocker"
	"github.com/mickeyyaya/evolve-loop/go/internal/cyclespend"
	"github.com/mickeyyaya/evolve-loop/go/internal/llmcalls"
	"github.com/mickeyyaya/evolve-loop/go/internal/paths"
)

var errBlockerRecord = errors.New("write the blocker record")

var blockerMarshal = json.Marshal

func writeBlockerRecord(workspace, class string, tags []string) error {
	evolveDir, cycle, ok := harnessRecordHome(workspace)
	if !ok {
		return nil
	}
	calls, err := ledgerCalls(workspace)
	if err != nil {
		return fmt.Errorf("%w: %w", errBlockerRecord, err)
	}
	verdict := cyclespend.VerdictOf(calls, time.Time{}, false)
	attribution := cycleblocker.Attribute(cycleblocker.Input{CheckTags: tags, Class: class, Verdict: verdict}, 0)
	b, err := blockerMarshal(cycleblocker.Record{Schema: cycleblocker.SchemaV1, Cycle: cycle, Blocker: attribution.Blocker, Minutes: attribution.Minutes})
	if err != nil {
		return fmt.Errorf("%w: marshal: %w", errBlockerRecord, err)
	}
	path := paths.BlockerRecordPath(evolveDir, cycle)
	if err := writeArtifactAtomically(path, b); err != nil {
		return fmt.Errorf("%w: %w", errBlockerRecord, err)
	}
	return nil
}

func ledgerCalls(workspace string) ([]llmcalls.Record, error) {
	res, err := llmcalls.ReadWorkspace(workspace)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	return res.Records, err
}

func blockerClass(phase, preClass string) string {
	if phase == "" || preClass == "" {
		return ""
	}
	return failureClassName(phase, preClass)
}
