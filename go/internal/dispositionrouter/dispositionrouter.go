// Package dispositionrouter decides whether a classified failure goes to the operator console or the lane queue.
// It also stages the resulting intents for the boundary applier.
// See docs/architecture/packages/internal-dispositionrouter.md.
package dispositionrouter

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/flock"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxbatch"
)

const (
	RouteConsole = "console"
	RouteQueue   = "queue"
)

const GuardAbortClass = "guard-abort"

const (
	LayerTaskCode     = "task-code"
	LayerPipelineCode = "pipeline-code"
	LayerHarness      = "harness"
	LayerInfra        = "infra"
	LayerEvalContract = "eval-contract"
)

const RecurrenceConsoleFloor = 3

const PendingActionsFile = "pending-actions.jsonl"

type Decision struct {
	Route  string
	Forced bool
	Reason string
}

func Decide(preClass string, recurrence int, llmRoute string) Decision {
	if preClass == GuardAbortClass {
		return Decision{
			Route:  RouteConsole,
			Forced: true,
			Reason: "floor: guard-abort pre-class is operator-owned (pipeline machinery, not a lane task)",
		}
	}
	if recurrence >= RecurrenceConsoleFloor {
		return Decision{
			Route:  RouteConsole,
			Forced: true,
			Reason: fmt.Sprintf("floor: recurrence %d >= %d — the defect survived prior fixes", recurrence, RecurrenceConsoleFloor),
		}
	}
	if llmRoute == RouteConsole {
		return Decision{Route: RouteConsole, Reason: "advisory raise: queue -> console"}
	}
	return Decision{Route: RouteQueue}
}

type Intent struct {
	Cycle         int     `json:"cycle"`
	Pattern       string  `json:"pattern"`
	ItemID        string  `json:"item_id,omitempty"`
	Action        string  `json:"action"`
	Route         string  `json:"route"`
	Recurrence    int     `json:"recurrence"`
	Weight        float64 `json:"weight"`
	Reason        string  `json:"reason,omitempty"`
	PriorityClass string  `json:"priority_class,omitempty"`
}

const (
	ActionEscalate = "escalate"
	ActionAutofile = "autofile"
)

func PendingActionsPath(escalationsDir string) string {
	return filepath.Join(escalationsDir, PendingActionsFile)
}

var stagingWrite = func(f *os.File, b []byte) (int, error) { return f.Write(b) }

func StageIntent(escalationsDir string, in Intent) (string, error) {
	if err := requireAutofileClass(in); err != nil {
		return "", err
	}
	path := PendingActionsPath(escalationsDir)
	if err := os.MkdirAll(escalationsDir, 0o755); err != nil {
		return "", fmt.Errorf("dispositionrouter: create escalations dir: %w", err)
	}
	line, err := json.Marshal(in)
	if err != nil {
		return "", fmt.Errorf("dispositionrouter: encode intent: %w", err)
	}
	err = flock.WithPathLock(path, func() error {
		f, oerr := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
		if oerr != nil {
			return fmt.Errorf("dispositionrouter: open staging file: %w", oerr)
		}
		defer func() { _ = f.Close() }()
		if _, werr := stagingWrite(f, append(line, '\n')); werr != nil {
			return fmt.Errorf("dispositionrouter: append intent: %w", werr)
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	return path, nil
}

func requireAutofileClass(in Intent) error {
	if in.Action != ActionAutofile {
		return nil
	}
	if err := inboxbatch.RequirePriorityClass(in.PriorityClass); err != nil {
		return fmt.Errorf("dispositionrouter: autofile intent %q: %w", in.ItemID, err)
	}
	return nil
}

func LoadIntents(path string) ([]Intent, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("dispositionrouter: read %s: %w", path, err)
	}
	var out []Intent
	dec := json.NewDecoder(bytes.NewReader(data))
	for dec.More() {
		var in Intent
		if derr := dec.Decode(&in); derr != nil {
			return nil, fmt.Errorf("dispositionrouter: parse %s: %w", path, derr)
		}
		out = append(out, in)
	}
	return out, nil
}
