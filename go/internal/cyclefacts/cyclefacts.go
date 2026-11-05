package cyclefacts

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/mickeyyaya/evolve-loop/go/internal/paths"
)

type State string

const (
	StatePresent    State = "present"
	StateAbsent     State = "absent"
	StateCorrupt    State = "corrupt"
	StateUnreadable State = "unreadable"
)

const (
	DecisionFile    = "failure-decision.json"
	DispositionFile = "disposition.json"
	FailReasonFile  = "audit-fail-reason.json"
)

type Read[T any] struct {
	Value T
	State State
	Path  string
	Err   error
}

type Decision struct {
	Category string `json:"category"`
	Level    string `json:"level"`
	Action   string `json:"action"`
	FixType  string `json:"fix_type"`
}

type Salvage struct {
	WorktreeHasValue bool   `json:"worktree_has_value"`
	Pointer          string `json:"pointer"`
}

type RootCause struct {
	Layer   string `json:"layer"`
	Summary string `json:"summary"`
}

type Disposition struct {
	Fingerprint string    `json:"fingerprint"`
	Recurrence  int       `json:"recurrence"`
	Legitimacy  string    `json:"legitimacy"`
	Urgency     string    `json:"urgency"`
	Routing     string    `json:"routing"`
	RootCause   RootCause `json:"root_cause"`
	Salvage     Salvage   `json:"salvage"`
}

type FailReason struct {
	Phase   string   `json:"phase"`
	Reasons []string `json:"reasons"`
}

type DigestRecord struct {
	Fingerprint      string   `json:"fingerprint"`
	PreClass         string   `json:"pre_class"`
	Checks           []string `json:"checks"`
	ClassFingerprint string   `json:"class_fingerprint"`
	ClassLayer       string   `json:"class_layer"`
	ClassRecurrence  int      `json:"class_recurrence"`
	ClassRoute       string   `json:"class_route"`
	ClassRouteReason string   `json:"class_route_reason"`
}

func ReadDecision(workspace string) Read[Decision] {
	return readJSON[Decision](filepath.Join(workspace, DecisionFile))
}

func ReadDisposition(workspace string) Read[Disposition] {
	return readJSON[Disposition](filepath.Join(workspace, DispositionFile))
}

func ReadFailReason(workspace string) Read[FailReason] {
	return readJSON[FailReason](filepath.Join(workspace, FailReasonFile))
}

func ReadDigestRecord(evolveDir string, cycle int) Read[DigestRecord] {
	return readJSON[DigestRecord](paths.FailureDigestRecordPath(evolveDir, cycle))
}

func readJSON[T any](path string) Read[T] {
	r := Read[T]{Path: path}
	raw, err := os.ReadFile(path)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		r.State = StateAbsent
	case err != nil:
		r.State, r.Err = StateUnreadable, err
	default:
		if err := json.Unmarshal(raw, &r.Value); err != nil {
			r.Value, r.State, r.Err = *new(T), StateCorrupt, fmt.Errorf("%s: %w", filepath.Base(path), err)
		} else {
			r.State = StatePresent
		}
	}
	return r
}
