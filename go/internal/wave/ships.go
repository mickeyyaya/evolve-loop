package wave

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/integration"
	"github.com/mickeyyaya/evolve-loop/go/internal/paths"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/ship/landing"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/shipjournal"
)

const (
	wavesDirName      = "waves"
	waveDirPrefix     = "integration-"
	deliverablePrefix = "deliverable-"
)

type shipWindow struct {
	from, to time.Time
	open     bool
}

func newShipWindow(from, to time.Time) shipWindow {
	return shipWindow{from: from.Truncate(time.Second), to: to.Truncate(time.Second), open: to.IsZero()}
}

func (w shipWindow) holds(at time.Time) bool {
	at = at.Truncate(time.Second)
	return !at.Before(w.from) && (w.open || at.Before(w.to))
}

func ShipsIn(root, stage string, from, to time.Time) ([]int, []ReadFailure) {
	w := newShipWindow(from, to)
	evolveDir := paths.EvolveDirOf(root)
	var ids map[int]bool
	var failures []ReadFailure
	if stage == policy.FleetIntegrationEnforce {
		ids, failures = deliveredIn(evolveDir, w)
	} else {
		ids, failures = landedIn(evolveDir, w)
	}
	return sortedKeys(ids), failures
}

func landedIn(evolveDir string, w shipWindow) (map[int]bool, []ReadFailure) {
	ids := map[int]bool{}
	path := shipjournal.Path(evolveDir)
	j, err := shipjournal.Read(path)
	if err != nil {
		return ids, []ReadFailure{{Path: path, Err: err}}
	}
	var failures []ReadFailure
	failures = appendFailure(failures, path, j.Strict())
	for _, e := range j.Entries {
		if e.Class != shipjournal.ClassCycle || e.Cycle <= 0 {
			continue
		}
		at, bad := landedAt(evolveDir, e)
		failures = append(failures, bad...)
		if !at.IsZero() && w.holds(at) {
			ids[e.Cycle] = true
		}
	}
	return ids, failures
}

func landedAt(evolveDir string, e shipjournal.Entry) (time.Time, []ReadFailure) {
	path := shipjournal.Path(evolveDir)
	journaled, ok := e.Time()
	if !ok {
		return time.Time{}, []ReadFailure{{Path: path, Err: fmt.Errorf("entry %s of cycle %d has the stamp %q", e.SHA, e.Cycle, e.TS)}}
	}
	intentPath := landing.IntentPath(evolveDir, e.Cycle)
	in, found, err := landing.ReadIntent(intentPath)
	if err != nil {
		return time.Time{}, []ReadFailure{{Path: intentPath, Err: err}}
	}
	if !found || in.Status != landing.IntentComplete || in.CommitSHA != e.SHA {
		return time.Time{}, nil
	}
	if in.CompletedAt == "" {
		return journaled, nil
	}
	completed, ok := in.Completed()
	if !ok {
		return time.Time{}, []ReadFailure{{Path: intentPath, Err: fmt.Errorf("completed_at %q is not a time", in.CompletedAt)}}
	}
	return completed, nil
}

func deliveredIn(evolveDir string, w shipWindow) (map[int]bool, []ReadFailure) {
	ids := map[int]bool{}
	wavesDir := filepath.Join(evolveDir, wavesDirName)
	waves, err := os.ReadDir(wavesDir)
	if errors.Is(err, fs.ErrNotExist) {
		return ids, nil
	}
	if err != nil {
		return ids, []ReadFailure{{Path: wavesDir, Err: err}}
	}
	var failures []ReadFailure
	for _, d := range waves {
		if !d.IsDir() || !strings.HasPrefix(d.Name(), waveDirPrefix) {
			continue
		}
		failures = append(failures, deliveredInWave(filepath.Join(wavesDir, d.Name()), w, ids)...)
	}
	return ids, failures
}

func deliveredInWave(dir string, w shipWindow, ids map[int]bool) []ReadFailure {
	files, err := os.ReadDir(dir)
	if err != nil {
		return []ReadFailure{{Path: dir, Err: err}}
	}
	var failures []ReadFailure
	for _, f := range files {
		if !strings.HasPrefix(f.Name(), deliverablePrefix) || filepath.Ext(f.Name()) != ".json" {
			continue
		}
		path := filepath.Join(dir, f.Name())
		r, err := integration.ReadRecord(path)
		if err != nil {
			failures = append(failures, ReadFailure{Path: path, Err: err})
			continue
		}
		if !r.Console() && w.holds(r.DeliveredAt) {
			ids[r.Cycle] = true
		}
	}
	return failures
}
