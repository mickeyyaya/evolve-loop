package attemptpostmortem

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"strings"
	"time"
)

type Input struct {
	Attempt        Attempt
	Transcript     Transcript
	ScrollbackPath string
	WorktreeDelta  string
}

func Collect(in Input, cfg Config) (Record, error) {
	if err := cfg.Validate(); err != nil {
		return Record{}, err
	}
	pane, panePath, err := readPane(in.ScrollbackPath, cfg.MaxPaneCaptureBytes)
	if err != nil {
		return Record{}, err
	}
	rec := Record{
		Schema:        SchemaVersion,
		Attempt:       in.Attempt,
		CommandSource: SourceTranscript,
		PaneTail:      capTail(stripControls(pane), cfg.MaxPaneTailRunes),
		WorktreeDelta: capHead(stripControls(in.WorktreeDelta), cfg.MaxDeltaRunes),
	}
	trace, err := readTrace(in.Transcript)
	if err != nil {
		rec.CommandSource, rec.SourceError = fallbackSource(rec.PaneTail), err.Error()
	}
	rec.LastActivityAt, rec.SkippedTranscriptLines = trace.LastActivityAt, trace.SkippedLines
	rec.Suspect = findSuspect(trace.Commands, sessionEnd(trace, in.Attempt), cfg.SuspectWindow)
	rec.Commands = lastCommands(trace.Commands, cfg)
	if rec.Suspect != nil {
		rec.Suspect.Command.Text = capHead(rec.Suspect.Command.Text, cfg.MaxCommandRunes)
	}
	rec.EvidencePaths = evidencePaths(trace.Path, panePath)
	return rec, rec.Validate()
}

func fallbackSource(paneTail string) Source {
	if paneTail == "" {
		return SourceNone
	}
	return SourcePaneTail
}

func readTrace(t Transcript) (Trace, error) {
	if t == nil {
		return Trace{}, ErrNoTranscript
	}
	return t()
}

func readPane(path string, maxBytes int) (string, string, error) {
	info, err := os.Stat(path)
	if errors.Is(err, fs.ErrNotExist) {
		return "", "", nil
	}
	data, err := readTail(path, info, err, int64(maxBytes))
	if err != nil {
		return "", "", fmt.Errorf("attemptpostmortem: read the pane scrollback %s: %w", path, err)
	}
	return strings.TrimRight(string(data), " \t\r\n"), path, nil
}

func readTail(path string, info fs.FileInfo, statErr error, maxBytes int64) ([]byte, error) {
	if statErr != nil {
		return nil, statErr
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	offset := max(0, info.Size()-maxBytes)
	return io.ReadAll(io.NewSectionReader(f, offset, info.Size()-offset))
}

func sessionEnd(trace Trace, a Attempt) time.Time {
	if trace.LastActivityAt.IsZero() {
		return a.EndedAt
	}
	return trace.LastActivityAt
}

func lastCommands(all []Command, cfg Config) []Command {
	kept := lastN(all, cfg.MaxCommands)
	out := make([]Command, len(kept))
	for i, c := range kept {
		c.Text = capHead(c.Text, cfg.MaxCommandRunes)
		out[i] = c
	}
	return out
}

func evidencePaths(paths ...string) []string {
	out := []string{}
	for _, p := range paths {
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}
