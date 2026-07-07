// Package tokenusage scans Claude Code transcript JSONL files to recover the
// token usage a launch actually consumed (token-telemetry campaign S1;
// docs/plans/token-telemetry-2026-07.md). It is best-effort instrumentation:
// a missing or unreadable transcript yields zero usage and SourceNone rather
// than an error, so telemetry never blocks a cycle.
//
// The scanner is deliberately conservative about attribution. A transcript is
// counted only when its recorded cwd matches the launch Window's Worktree
// exactly (the on-disk session-directory slug is a lossy sanitisation of cwd
// and must never be trusted as a key), and — when concurrent same-directory
// sessions are possible — only when the launch's unique ArtifactPath appears in
// the first user message. Streamed usage deltas that repeat one message id are
// deduplicated to the last (highest-cumulative) delta, never summed.
package tokenusage

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
)

// Source identifies how a Result's usage was obtained.
type Source string

const (
	// SourceNone means no transcript was found for the Window; Usage is zero.
	SourceNone Source = "none"
	// SourceTranscript means usage was recovered from a matched transcript.
	SourceTranscript Source = "transcript"
)

// Window describes the launch whose token usage is being recovered. Worktree
// is the exact cwd a matching transcript must record. ArtifactPath, when set,
// is the launch's unique artifact reference that must appear in a session's
// first user message to disambiguate concurrent same-directory sessions.
// Start and End bound the assistant turns that count toward the launch.
type Window struct {
	Worktree     string
	ArtifactPath string
	Start        time.Time
	End          time.Time
}

// Result is the outcome of a scan: the summed token usage and the Source that
// produced it.
type Result struct {
	Usage  cyclestate.TokenUsage
	Source Source
}

// transcriptLine is the subset of a Claude Code transcript JSONL record the
// scanner reads. One JSON object per line.
type transcriptLine struct {
	Type      string `json:"type"`
	Cwd       string `json:"cwd"`
	Timestamp string `json:"timestamp"`
	Message   struct {
		ID    string `json:"id"`
		Usage *struct {
			Input      int `json:"input_tokens"`
			Output     int `json:"output_tokens"`
			CacheRead  int `json:"cache_read_input_tokens"`
			CacheWrite int `json:"cache_creation_input_tokens"`
		} `json:"usage"`
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	} `json:"message"`
}

// ScanConfigRoot walks <root>/projects for transcript JSONL files, attributes
// the ones belonging to the launch described by w, and returns their summed
// token usage. A missing projects directory (or no matching transcript) yields
// a zero Usage with SourceNone and no error — token telemetry is best-effort.
func ScanConfigRoot(root string, w Window) (Result, error) {
	projects := filepath.Join(root, "projects")
	if _, err := os.Stat(projects); err != nil {
		return Result{Source: SourceNone}, nil
	}

	// Last usage per message id (dedup streamed deltas), accumulated across
	// every attributed transcript.
	perMsg := map[string]cyclestate.TokenUsage{}
	matched := false

	err := filepath.WalkDir(projects, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil || d.IsDir() || !strings.HasSuffix(path, ".jsonl") {
			return nil
		}
		lines := readLines(path)
		if !attributes(lines, w) {
			return nil
		}
		matched = true
		for _, ln := range lines {
			if ln.Type != "assistant" || ln.Message.Usage == nil || ln.Message.ID == "" {
				continue
			}
			if !withinWindow(ln.Timestamp, w) {
				continue
			}
			u := ln.Message.Usage
			perMsg[ln.Message.ID] = cyclestate.TokenUsage{
				Input:      u.Input,
				Output:     u.Output,
				CacheRead:  u.CacheRead,
				CacheWrite: u.CacheWrite,
			}
		}
		return nil
	})
	if err != nil {
		return Result{Source: SourceNone}, nil
	}

	if !matched {
		return Result{Source: SourceNone}, nil
	}
	var total cyclestate.TokenUsage
	for _, u := range perMsg {
		total.Input += u.Input
		total.Output += u.Output
		total.CacheRead += u.CacheRead
		total.CacheWrite += u.CacheWrite
	}
	return Result{Usage: total, Source: SourceTranscript}, nil
}

// readLines parses a transcript file into its records, silently skipping
// unparseable lines (best-effort). A file it cannot open yields nil.
func readLines(path string) []transcriptLine {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()

	var out []transcriptLine
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 8*1024*1024)
	for sc.Scan() {
		var ln transcriptLine
		if json.Unmarshal(sc.Bytes(), &ln) != nil {
			continue
		}
		out = append(out, ln)
	}
	return out
}

// attributes reports whether a transcript belongs to the launch: its recorded
// cwd must equal w.Worktree exactly, and — when w.ArtifactPath is set — that
// path must appear in the first user message (content-verification for
// concurrent same-directory sessions).
func attributes(lines []transcriptLine, w Window) bool {
	cwdMatch := false
	for _, ln := range lines {
		if ln.Cwd == w.Worktree {
			cwdMatch = true
			break
		}
	}
	if !cwdMatch {
		return false
	}
	if w.ArtifactPath == "" {
		return true
	}
	return strings.Contains(firstUserText(lines), w.ArtifactPath)
}

// firstUserText returns the concatenated text of the first user message's
// content blocks, or "" if there is none.
func firstUserText(lines []transcriptLine) string {
	for _, ln := range lines {
		if ln.Type != "user" {
			continue
		}
		var b strings.Builder
		for _, c := range ln.Message.Content {
			b.WriteString(c.Text)
		}
		return b.String()
	}
	return ""
}

// withinWindow reports whether an assistant turn's timestamp falls inside the
// launch window. An absent or unparseable timestamp is included (best-effort:
// a real turn missing a timestamp should still be attributed once its
// transcript is content-verified).
func withinWindow(ts string, w Window) bool {
	if ts == "" {
		return true
	}
	t, err := time.Parse(time.RFC3339, ts)
	if err != nil {
		return true
	}
	return !t.Before(w.Start) && !t.After(w.End)
}
