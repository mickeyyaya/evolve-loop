package recurrence

import (
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/flock"
	"github.com/mickeyyaya/evolve-loop/go/internal/atomicwrite"
	"github.com/mickeyyaya/evolve-loop/go/internal/dispositionrouter"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxbatch"
	"github.com/mickeyyaya/evolve-loop/go/internal/retrofile"
)

const appliedStampFile = "applied-stamp.json"

type ApplyOptions struct {
	InboxDir        string
	EscalationsPath string
	ReportPath      string
	Cycle           int
	Shadow          bool
	Now             time.Time
}

type ApplyResult struct {
	Escalated []string `json:"escalated"`
	Filed     []string `json:"filed"`
	Skipped   []string `json:"skipped"`
	Refused   []string `json:"refused,omitempty"`
	Planned   []string `json:"planned,omitempty"`
	Shadow    bool     `json:"shadow"`
	Cycle     int      `json:"cycle"`
}

func ApplyBoundary(opts ApplyOptions) (ApplyResult, error) {
	res := ApplyResult{Shadow: opts.Shadow, Cycle: opts.Cycle}
	intents, err := dispositionrouter.LoadIntents(opts.EscalationsPath)
	if err != nil {
		return res, err
	}

	if opts.Shadow {
		for _, in := range intents {
			res.Planned = append(res.Planned, fmt.Sprintf("%s:%s", in.Action, intentID(in)))
		}
		return res, writeApplyReport(opts.ReportPath, res)
	}

	stampPath := filepath.Join(filepath.Dir(opts.EscalationsPath), appliedStampFile)
	err = flock.WithPathLock(opts.InboxDir, func() error {
		stamp, serr := loadStamp(stampPath)
		if serr != nil {
			return serr
		}
		priorStamp := maps.Clone(stamp)
		for _, in := range intents {
			key := stampKey(in)
			if prev, ok := priorStamp[key]; ok && prev == opts.Cycle {
				continue
			}
			applied, aerr := applyIntent(opts, in, &res)
			if aerr != nil {
				return aerr
			}
			if applied {
				stamp[key] = opts.Cycle
			}
		}
		return saveStamp(stampPath, stamp)
	})
	if err != nil {
		return res, err
	}
	return res, writeApplyReport(opts.ReportPath, res)
}

func applyIntent(opts ApplyOptions, in dispositionrouter.Intent, res *ApplyResult) (bool, error) {
	id := intentID(in)
	switch in.Action {
	case dispositionrouter.ActionAutofile:
		return autofileIntent(opts, in, res)

	case dispositionrouter.ActionEscalate:
		if findItem(opts.InboxDir, id) != itemOpen {
			res.Skipped = append(res.Skipped, id)
			return false, nil
		}
		res.Escalated = append(res.Escalated, id)
		return true, nil

	default:
		res.Skipped = append(res.Skipped, id)
		return false, nil
	}
}

func autofileIntent(opts ApplyOptions, in dispositionrouter.Intent, res *ApplyResult) (bool, error) {
	id := intentID(in)
	title := in.Title
	if title == "" {
		title = fmt.Sprintf("recurring failure %s (%d occurrences)", in.Pattern, in.Recurrence)
	}
	filed, err := retrofile.FileActions(opts.InboxDir, opts.Cycle, []retrofile.PreventiveAction{{
		ID:            id,
		Title:         title,
		Summary:       in.Summary,
		Acceptance:    in.Acceptance,
		WeightHint:    in.Weight,
		Evidence:      in.Reason,
		Recurrence:    in.Recurrence,
		PriorityClass: in.PriorityClass,
	}}, in.Weight, opts.Now)
	if errors.Is(err, inboxbatch.ErrNoPriorityClass) {
		res.Refused = append(res.Refused, id)
		return true, nil
	}
	if err != nil {
		return false, err
	}
	if alreadyOpenOrDone := len(filed) == 0; alreadyOpenOrDone {
		res.Skipped = append(res.Skipped, id)
		return true, nil
	}
	res.Filed = append(res.Filed, id)
	return true, nil
}

type itemState int

const (
	itemAbsent itemState = iota
	itemOpen
	itemClaimed
)

func findItem(inboxDir, id string) (state itemState) {
	_ = filepath.Walk(inboxDir, func(p string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || filepath.Ext(p) != ".json" || state != itemAbsent {
			return nil //nolint:nilerr // an absent tree is a legitimate "not found"
		}
		raw, rerr := os.ReadFile(p)
		if rerr != nil {
			return nil
		}
		var it struct {
			ID string `json:"id"`
		}
		if json.Unmarshal(raw, &it) != nil || inboxbatch.ItemID(it.ID, p) != id {
			return nil
		}
		rel, relErr := filepath.Rel(inboxDir, p)
		if relErr == nil && strings.HasPrefix(rel, "processing"+string(filepath.Separator)) {
			state = itemClaimed
			return nil
		}
		state = itemOpen
		return nil
	})
	return state
}

func intentID(in dispositionrouter.Intent) string {
	if in.ItemID != "" {
		return in.ItemID
	}
	return in.Pattern
}

func stampKey(in dispositionrouter.Intent) string {
	return in.Action + "|" + intentID(in)
}

func loadStamp(path string) (map[string]int, error) {
	out := map[string]int{}
	raw, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return out, nil
	}
	if err != nil {
		return nil, fmt.Errorf("recurrence: read apply stamp %s: %w", path, err)
	}
	if len(raw) == 0 {
		return out, nil
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("recurrence: parse apply stamp %s: %w", path, err)
	}
	return out, nil
}

func saveStamp(path string, stamp map[string]int) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("recurrence: create stamp dir: %w", err)
	}
	return atomicwrite.JSON(path, stamp)
}

func writeApplyReport(path string, res ApplyResult) error {
	if path == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("recurrence: create report dir: %w", err)
	}
	return atomicwrite.JSON(path, res)
}
