package cliupdate

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/flock"
	"github.com/mickeyyaya/evolve-loop/go/internal/atomicwrite"
)

const RecordsFile = "cli-updates.json"

const maxRecords = 64

type RecordKind string

const (
	KindBaseline   RecordKind = "baseline"
	KindUpdate     RecordKind = "update"
	KindSelfUpdate RecordKind = "self-update"
)

type Record struct {
	Family string     `json:"family"`
	Kind   RecordKind `json:"kind"`
	Old    string     `json:"old,omitempty"`
	New    string     `json:"new"`
	At     time.Time  `json:"at"`
}

type Cause string

const (
	CauseUnrecorded     Cause = "unrecorded"
	CauseBoundaryUpdate Cause = "boundary-update"
	CauseSelfUpdate     Cause = "self-update"
)

func RecordsPath(evolveDir string) string {
	return filepath.Join(evolveDir, RecordsFile)
}

func LoadRecords(evolveDir string) ([]Record, error) {
	data, err := os.ReadFile(RecordsPath(evolveDir))
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var records []Record
	if err := json.Unmarshal(data, &records); err != nil {
		return nil, fmt.Errorf("%s: %w", RecordsPath(evolveDir), err)
	}
	return records, nil
}

func Remember(evolveDir string, rep Report, now time.Time) error {
	if !observedAnyVersion(rep) {
		return nil
	}
	if err := os.MkdirAll(evolveDir, 0o755); err != nil {
		return err
	}
	return flock.WithPathLock(RecordsPath(evolveDir), func() error {
		prior, err := LoadRecords(evolveDir)
		if err != nil {
			return err
		}
		added := acceptedRecords(rep, prior, now.UTC())
		if len(added) == 0 {
			return nil
		}
		all := append(prior, added...)
		if len(all) > maxRecords {
			all = all[len(all)-maxRecords:]
		}
		return atomicwrite.JSON(RecordsPath(evolveDir), all)
	})
}

func observedAnyVersion(rep Report) bool {
	for _, res := range rep.Results {
		if res.OldVersion != "" {
			return true
		}
	}
	return false
}

func acceptedRecords(rep Report, prior []Record, at time.Time) []Record {
	var out []Record
	for _, res := range rep.Results {
		out = append(out, accepted(res, lastSeen(prior, res.Family) != "", at)...)
	}
	return out
}

func accepted(res Result, known bool, at time.Time) []Record {
	if res.Status == StatusSmokeFailed || res.Status == StatusSkipped {
		return nil
	}
	var out []Record
	if res.SelfUpdatedFrom != "" {
		out = append(out, Record{Family: res.Family, Kind: KindSelfUpdate, Old: res.SelfUpdatedFrom, New: res.OldVersion, At: at})
	}
	if res.NewVersion != "" && res.NewVersion != res.OldVersion {
		out = append(out, Record{Family: res.Family, Kind: KindUpdate, Old: res.OldVersion, New: res.NewVersion, At: at})
	}
	if len(out) == 0 && !known && res.NewVersion != "" {
		out = append(out, Record{Family: res.Family, Kind: KindBaseline, New: res.NewVersion, At: at})
	}
	return out
}

func lastSeen(records []Record, family string) string {
	seen := ""
	for _, r := range records {
		if r.Family == family {
			seen = r.New
		}
	}
	return seen
}

func CauseOf(records []Record, family, from, to string) Cause {
	version, selfUpdated := from, false
	for _, r := range records {
		if r.Family == family && r.Old == version {
			version = r.New
			selfUpdated = selfUpdated || r.Kind == KindSelfUpdate
		}
	}
	switch {
	case from == to || version != to:
		return CauseUnrecorded
	case selfUpdated:
		return CauseSelfUpdate
	}
	return CauseBoundaryUpdate
}
