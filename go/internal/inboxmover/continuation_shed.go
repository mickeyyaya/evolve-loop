package inboxmover

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/continuation"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxbatch"
)

const declineAuthority = "runtime (adoption declined)"

type shedRecord struct {
	snapshot string
	reason   string
	at       time.Time
}

func ShedDeclinedContinuation(opts Options, cycle int, declined continuation.Continuation) ([]string, error) {
	opts.resolveOpts()
	if declined.SnapshotSHA == "" {
		return nil, nil
	}
	dir := inboxbatch.ProcessingCycleDir(opts.InboxDir, cycle)
	entries, err := os.ReadDir(dir)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("inboxmover: shed declined continuation: read %s: %w", dir, err)
	}
	rec := shedRecord{snapshot: declined.SnapshotSHA, reason: fmt.Sprintf("adoption declined in cycle %d", cycle), at: opts.Now().UTC()}
	var shed []string
	var errs []error
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		id, ok, serr := shedPointer(filepath.Join(dir, e.Name()), rec)
		if serr != nil {
			errs = append(errs, fmt.Errorf("%s: %w", e.Name(), serr))
		}
		if ok {
			shed = append(shed, id)
		}
	}
	return shed, errors.Join(errs...)
}

func shedPointer(path string, rec shedRecord) (string, bool, error) {
	var it struct {
		ID           string                     `json:"id"`
		Continuation *continuation.Continuation `json:"continuation"`
	}
	body, err := os.ReadFile(path)
	if err != nil || json.Unmarshal(body, &it) != nil || it.Continuation == nil || it.Continuation.SnapshotSHA != rec.snapshot {
		return "", false, err
	}
	err = appendReleasedContinuation(path, continuation.RedactHostPaths(*it.Continuation), rec.reason, declineAuthority, rec.at)
	return inboxbatch.ItemID(it.ID, filepath.Base(path)), err == nil, err
}
