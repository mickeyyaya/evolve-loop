package attemptpostmortem

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/atomicwrite"
)

const SchemaVersion = "attempt-postmortem/1.0"

type Status string

const (
	StatusOK       Status = "ok"
	StatusError    Status = "error"
	StatusSignal   Status = "signal"
	StatusNoResult Status = "no_result"
)

type Command struct {
	Text      string    `json:"text"`
	StartedAt time.Time `json:"started_at"`
	Status    Status    `json:"status"`
	ExitCode  int       `json:"exit_code,omitempty"`
}

type SuspectReason string

const (
	ReasonNoResult   SuspectReason = "no_result"
	ReasonSignalExit SuspectReason = "signal_exit"
	ReasonEndWindow  SuspectReason = "end_window"
)

type Suspect struct {
	Command Command       `json:"command"`
	Reason  SuspectReason `json:"reason"`
}

type Source string

const (
	SourceTranscript Source = "transcript"
	SourcePaneTail   Source = "pane_tail"
)

type Attempt struct {
	Phase      string    `json:"phase"`
	Cycle      int       `json:"cycle"`
	Number     int       `json:"attempt"`
	CLI        string    `json:"cli"`
	Session    string    `json:"session"`
	DispatchID string    `json:"dispatch_id"`
	StartedAt  time.Time `json:"started_at"`
	EndedAt    time.Time `json:"ended_at"`
	CauseCode  string    `json:"cause_code"`
	ExitCode   int       `json:"exit_code"`
}

type Record struct {
	Schema string `json:"schema"`
	Attempt
	LastActivityAt time.Time `json:"last_activity_at"`
	CommandSource  Source    `json:"command_source"`
	SourceError    string    `json:"source_error,omitempty"`
	Commands       []Command `json:"commands"`
	Suspect        *Suspect  `json:"suspect,omitempty"`
	PaneTail       string    `json:"pane_tail"`
	WorktreeDelta  string    `json:"worktree_delta"`
	EvidencePaths  []string  `json:"evidence_paths"`
}

func (a Attempt) Abnormal() bool { return a.CauseCode != "" || a.ExitCode != 0 }

func (r Record) Validate() error {
	checks := []struct {
		field string
		ok    bool
	}{
		{"schema", r.Schema == SchemaVersion},
		{"phase", r.Phase != ""},
		{"cycle", r.Cycle > 0},
		{"attempt", r.Number > 0},
		{"ended_at", !r.EndedAt.Before(r.StartedAt)},
		{"command_source", r.CommandSource == SourceTranscript || r.CommandSource == SourcePaneTail},
		{"suspect", r.Suspect == nil || knownReason(r.Suspect.Reason)},
	}
	for _, c := range checks {
		if !c.ok {
			return fmt.Errorf("attemptpostmortem: invalid record field %s (phase %q, record %d)", c.field, r.Phase, r.Number)
		}
	}
	return nil
}

func knownReason(r SuspectReason) bool {
	return r == ReasonNoResult || r == ReasonSignalExit || r == ReasonEndWindow
}

func Path(workspace, phase string, number int) string {
	return filepath.Join(workspace, fmt.Sprintf("%s-attempt-%d-postmortem.json", phase, number))
}

func Write(workspace string, r Record) error {
	if err := r.Validate(); err != nil {
		return err
	}
	if err := atomicwrite.JSON(Path(workspace, r.Phase, r.Number), r); err != nil {
		return fmt.Errorf("attemptpostmortem: write %s record %d: %w", r.Phase, r.Number, err)
	}
	return nil
}

func ReadAll(workspace, phase string) ([]Record, error) {
	paths, err := filepath.Glob(filepath.Join(workspace, phase+"-attempt-*-postmortem.json"))
	if err != nil {
		return nil, fmt.Errorf("attemptpostmortem: list the records of %q: %w", phase, err)
	}
	records := make([]Record, 0, len(paths))
	for _, path := range paths {
		r, err := readRecord(path)
		if err != nil {
			return nil, err
		}
		records = append(records, r)
	}
	sort.Slice(records, func(i, j int) bool { return records[i].Number < records[j].Number })
	return records, nil
}

func readRecord(path string) (Record, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Record{}, fmt.Errorf("attemptpostmortem: read %s: %w", path, err)
	}
	var r Record
	if err := json.Unmarshal(data, &r); err != nil {
		return Record{}, fmt.Errorf("attemptpostmortem: decode %s: %w", path, err)
	}
	if err := r.Validate(); err != nil {
		return Record{}, fmt.Errorf("%s: %w", path, err)
	}
	return r, nil
}
