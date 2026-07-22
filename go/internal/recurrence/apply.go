package recurrence

// apply.go — S4 of the failure-disposition-router design (subsumes
// chronicle-s6-escalation-boundary): the BOUNDARY APPLIER that consumes the
// intents S3 (internal/dispositionrouter) staged and is the only writer allowed
// to mutate .evolve/inbox/ on the recurrence path.
//
// Why a separate boundary pass exists at all: staging happens mid-cycle, while
// fleet lanes are in flight. An inbox write at that moment races
// inboxmover.Claim's os.Rename and resurrects a claimed item into double work
// across lanes. ApplyBoundary therefore runs at the loop's per-iteration
// boundary (cmd_loop.go, after dispatchIteration returns with no lane running)
// and holds the inbox lock for the whole read-modify-write.
//
// Four safety properties, each pinned by a predicate in go/acs/cycle1062:
//   - idempotent per cycle (a stamp file, so a re-run never double-escalates);
//   - never LOWERS a weight (an already-hot item is left alone);
//   - CLAIMED items (under inbox/processing/) are skipped, never resurrected;
//   - shadow stage writes the report artifact and mutates nothing.
//
// The autofile path goes through internal/retrofile — the existing (until now
// caller-less) inbox filer — rather than a second hand-rolled writer.

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/flock"
	"github.com/mickeyyaya/evolve-loop/go/internal/atomicwrite"
	"github.com/mickeyyaya/evolve-loop/go/internal/dispositionrouter"
	"github.com/mickeyyaya/evolve-loop/go/internal/retrofile"
)

// appliedStampFile is the per-cycle idempotency stamp's basename, written
// beside the staging file so one escalations dir carries its own history.
const appliedStampFile = "applied-stamp.json"

// ApplyOptions is the boundary applier's injected configuration. Every path is
// caller-supplied so tests (and fleet lanes) stay off the live tree, and Now is
// injected so filed items are byte-stable.
type ApplyOptions struct {
	// InboxDir is the .evolve/inbox root: open items at the top level, claimed
	// items under processing/cycle-<N>/.
	InboxDir string
	// EscalationsPath is the staged pending-actions.jsonl file S3 appends to.
	EscalationsPath string
	// ReportPath receives the JSON apply report (written in every stage).
	ReportPath string
	// Cycle is the loop iteration this pass belongs to (the idempotency key).
	Cycle int
	// Shadow selects report-only mode: plan everything, mutate nothing.
	Shadow bool
	// Policy is the injected count->weight formula (never a Go literal here).
	Policy EscalationPolicy
	// Now stamps filed items.
	Now time.Time
}

// ApplyResult reports what the pass did, by inbox item id. In shadow the three
// action lists are empty and Planned carries what an enforce pass would do.
type ApplyResult struct {
	Bumped  []string `json:"bumped"`
	Filed   []string `json:"filed"`
	Skipped []string `json:"skipped"`
	Planned []string `json:"planned,omitempty"`
	Shadow  bool     `json:"shadow"`
	Cycle   int      `json:"cycle"`
}

// ApplyBoundary consumes the staged intents and applies them to the inbox,
// then writes the report to opts.ReportPath. An absent staging file is not an
// error (nothing was staged — the common case); the report is still written so
// the artifact's presence never depends on there being work.
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
		for _, in := range intents {
			key := stampKey(in)
			if stamp[key] == opts.Cycle {
				continue // already applied this cycle — the idempotency guard
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

// applyIntent applies one intent, appending its id to the matching result list.
// It reports whether the intent consumed its per-cycle stamp slot (a skip that
// can never succeed later — a claimed or absent item — does not).
func applyIntent(opts ApplyOptions, in dispositionrouter.Intent, res *ApplyResult) (bool, error) {
	id := intentID(in)
	switch in.Action {
	case dispositionrouter.ActionAutofile:
		filed, err := retrofile.FileActions(opts.InboxDir, opts.Cycle, []retrofile.PreventiveAction{{
			ID:         id,
			Title:      fmt.Sprintf("recurring failure %s (%d occurrences)", in.Pattern, in.Recurrence),
			WeightHint: in.Weight,
			Evidence:   in.Reason,
			Recurrence: in.Recurrence,
		}}, in.Weight, opts.Now)
		if err != nil {
			return false, err
		}
		if len(filed) == 0 {
			// retrofile deduplicated: the item is already open or already done.
			res.Skipped = append(res.Skipped, id)
			return true, nil
		}
		res.Filed = append(res.Filed, id)
		return true, nil

	case dispositionrouter.ActionEscalate:
		path, weight, state := findItem(opts.InboxDir, id)
		if state != itemOpen {
			// Claimed (in flight in another lane) or absent: never touch it.
			res.Skipped = append(res.Skipped, id)
			return false, nil
		}
		target := opts.Policy.Target(in.Weight, in.Recurrence)
		if target <= weight {
			// Escalation never lowers a weight.
			res.Skipped = append(res.Skipped, id)
			return true, nil
		}
		if err := setItemWeight(path, target); err != nil {
			return false, err
		}
		res.Bumped = append(res.Bumped, id)
		return true, nil

	default:
		res.Skipped = append(res.Skipped, id)
		return false, nil
	}
}

// itemState distinguishes a dispatchable item from one a lane already claimed.
type itemState int

const (
	itemAbsent itemState = iota
	itemOpen
	itemClaimed
)

// findItem locates the inbox item carrying id anywhere under inboxDir and
// reports its path, current weight and state. An item under processing/ has
// been claimed by a fleet lane (inboxmover.Claim) and is off-limits.
func findItem(inboxDir, id string) (path string, weight float64, state itemState) {
	_ = filepath.Walk(inboxDir, func(p string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || filepath.Ext(p) != ".json" || state != itemAbsent {
			return nil //nolint:nilerr // an absent tree is a legitimate "not found"
		}
		raw, rerr := os.ReadFile(p)
		if rerr != nil {
			return nil
		}
		var it struct {
			ID     string  `json:"id"`
			Weight float64 `json:"weight"`
		}
		if json.Unmarshal(raw, &it) != nil || it.ID != id {
			return nil
		}
		path, weight = p, it.Weight
		rel, relErr := filepath.Rel(inboxDir, p)
		if relErr == nil && strings.HasPrefix(rel, "processing"+string(filepath.Separator)) {
			state = itemClaimed
			return nil
		}
		state = itemOpen
		return nil
	})
	return path, weight, state
}

// setItemWeight rewrites just the "weight" field of the inbox item at path,
// preserving every other field the item carries.
func setItemWeight(path string, weight float64) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("recurrence: read inbox item %s: %w", path, err)
	}
	var item map[string]any
	if err := json.Unmarshal(raw, &item); err != nil {
		return fmt.Errorf("recurrence: parse inbox item %s: %w", path, err)
	}
	item["weight"] = weight
	return atomicwrite.JSON(path, item)
}

// intentID is the inbox id an intent addresses, falling back to its pattern
// when the router staged no explicit item id.
func intentID(in dispositionrouter.Intent) string {
	if in.ItemID != "" {
		return in.ItemID
	}
	return in.Pattern
}

// stampKey identifies one intent for the per-cycle idempotency stamp.
func stampKey(in dispositionrouter.Intent) string {
	return in.Action + "|" + intentID(in)
}

// loadStamp reads the applied-intent stamp map (missing file ⇒ empty map).
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

// saveStamp persists the applied-intent stamp map atomically.
func saveStamp(path string, stamp map[string]int) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("recurrence: create stamp dir: %w", err)
	}
	return atomicwrite.JSON(path, stamp)
}

// writeApplyReport persists the pass's report (skipped when no path is set).
func writeApplyReport(path string, res ApplyResult) error {
	if path == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("recurrence: create report dir: %w", err)
	}
	return atomicwrite.JSON(path, res)
}
