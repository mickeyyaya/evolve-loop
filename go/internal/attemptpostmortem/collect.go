package attemptpostmortem

import (
	"errors"
	"fmt"
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
	pane, panePath, err := readPane(in.ScrollbackPath)
	if err != nil {
		return Record{}, err
	}
	rec := Record{
		Schema:        SchemaVersion,
		Attempt:       in.Attempt,
		CommandSource: SourceTranscript,
		PaneTail:      capTail(pane, cfg.MaxPaneTailRunes),
		WorktreeDelta: capHead(in.WorktreeDelta, cfg.MaxDeltaRunes),
	}
	trace, err := in.Transcript()
	if err != nil {
		rec.CommandSource, rec.SourceError = SourcePaneTail, err.Error()
	}
	rec.LastActivityAt = trace.LastActivityAt
	rec.Suspect = findSuspect(trace.Commands, sessionEnd(trace, in.Attempt), cfg.SuspectWindow)
	rec.Commands = lastCommands(trace.Commands, cfg)
	if rec.Suspect != nil {
		rec.Suspect.Command.Text = capHead(rec.Suspect.Command.Text, cfg.MaxCommandRunes)
	}
	rec.EvidencePaths = evidencePaths(trace.Path, panePath)
	return rec, rec.Validate()
}

func readPane(path string) (string, string, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return "", "", nil
	}
	if err != nil {
		return "", "", fmt.Errorf("attemptpostmortem: read the pane scrollback %s: %w", path, err)
	}
	return strings.TrimRight(string(data), " \t\r\n"), path, nil
}

func sessionEnd(trace Trace, a Attempt) time.Time {
	if trace.LastActivityAt.IsZero() {
		return a.EndedAt
	}
	return trace.LastActivityAt
}

func lastCommands(all []Command, cfg Config) []Command {
	if len(all) > cfg.MaxCommands {
		all = all[len(all)-cfg.MaxCommands:]
	}
	out := make([]Command, len(all))
	for i, c := range all {
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
