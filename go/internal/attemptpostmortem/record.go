package attemptpostmortem

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
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
	SourceNone       Source = "none"
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
	LastActivityAt         time.Time `json:"last_activity_at"`
	CommandSource          Source    `json:"command_source"`
	SourceError            string    `json:"source_error,omitempty"`
	Commands               []Command `json:"commands"`
	Suspect                *Suspect  `json:"suspect,omitempty"`
	PaneTail               string    `json:"pane_tail"`
	WorktreeDelta          string    `json:"worktree_delta"`
	EvidencePaths          []string  `json:"evidence_paths"`
	SkippedTranscriptLines int       `json:"skipped_transcript_lines,omitempty"`
}

func (a Attempt) Abnormal() bool { return a.CauseCode != "" || a.ExitCode != 0 }

func (r Record) Validate() error {
	checks := []struct {
		field string
		ok    bool
	}{
		{"schema", r.Schema == SchemaVersion},
		{"phase", ValidPhase(r.Phase)},
		{"cycle", r.Cycle > 0},
		{"attempt", r.Number > 0},
		{"ended_at", !r.EndedAt.Before(r.StartedAt)},
		{"command_source", knownSource(r.CommandSource)},
		{"suspect", r.Suspect == nil || knownReason(r.Suspect.Reason)},
	}
	for _, c := range checks {
		if !c.ok {
			return fmt.Errorf("attemptpostmortem: invalid record field %s (phase %q, record %d)", c.field, r.Phase, r.Number)
		}
	}
	return nil
}

var bareName = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]*$`)

func ValidPhase(phase string) bool { return bareName.MatchString(phase) }

func knownSource(s Source) bool {
	return s == SourceTranscript || s == SourcePaneTail || s == SourceNone
}

func knownReason(r SuspectReason) bool {
	return r == ReasonNoResult || r == ReasonSignalExit || r == ReasonEndWindow
}

func Path(workspace, phase string, number int) string {
	return filepath.Join(workspace, fmt.Sprintf("%s-attempt-%d-postmortem.json", phase, number))
}

func NextNumber(workspace, phase string) int {
	n := 1
	for {
		if _, err := os.Stat(Path(workspace, phase, n)); err != nil {
			return n
		}
		n++
	}
}

type BadRecord struct {
	Path string
	Err  error
}

func (b BadRecord) Error() string { return b.Err.Error() }

var writeRecordBody = func(f *os.File, data []byte) error {
	_, err := f.Write(data)
	return err
}

func Write(workspace string, r Record) (Record, error) {
	for {
		err := Create(workspace, r)
		if !errors.Is(err, fs.ErrExist) {
			return r, err
		}
		r.Number++
	}
}

func Create(workspace string, r Record) error {
	data, err := encode(r)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(workspace, 0o755); err != nil {
		return fmt.Errorf("attemptpostmortem: ensure the run directory %s: %w", workspace, err)
	}
	path := Path(workspace, r.Phase, r.Number)
	tmp, err := writeTemp(workspace, filepath.Base(path), data)
	if err != nil {
		return fmt.Errorf("attemptpostmortem: write %s: %w", path, err)
	}
	defer func() { _ = os.Remove(tmp) }()
	if err := os.Link(tmp, path); err != nil {
		return fmt.Errorf("attemptpostmortem: create %s: %w", path, err)
	}
	return nil
}

func writeTemp(dir, base string, data []byte) (string, error) {
	f, err := os.CreateTemp(dir, "."+base+".*.tmp")
	if err != nil {
		return "", err
	}
	err = writeRecordBody(f, data)
	if closeErr := f.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		_ = os.Remove(f.Name())
		return "", err
	}
	return f.Name(), nil
}

func Replace(workspace string, r Record) error {
	data, err := encode(r)
	if err != nil {
		return err
	}
	path := Path(workspace, r.Phase, r.Number)
	if err := atomicwrite.Bytes(path, data); err != nil {
		return fmt.Errorf("attemptpostmortem: replace %s: %w", path, err)
	}
	return nil
}

func encode(r Record) ([]byte, error) {
	if err := r.Validate(); err != nil {
		return nil, err
	}
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("attemptpostmortem: encode %s record %d: %w", r.Phase, r.Number, err)
	}
	return data, nil
}

func ReadAll(workspace, phase string) ([]Record, []BadRecord, error) {
	if !ValidPhase(phase) {
		return nil, nil, fmt.Errorf("attemptpostmortem: phase %q is not a bare name", phase)
	}
	paths, err := filepath.Glob(filepath.Join(workspace, phase+"-attempt-*-postmortem.json"))
	if err != nil {
		return nil, nil, fmt.Errorf("attemptpostmortem: list the records of %q: %w", phase, err)
	}
	records := make([]Record, 0, len(paths))
	var bad []BadRecord
	for _, path := range paths {
		r, err := readRecord(path)
		if err != nil {
			bad = append(bad, BadRecord{Path: path, Err: err})
			continue
		}
		records = append(records, r)
	}
	sort.Slice(records, func(i, j int) bool { return records[i].Number < records[j].Number })
	return records, bad, nil
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
