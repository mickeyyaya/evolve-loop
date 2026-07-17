// Package tokenusage scans Claude Code transcript JSONL files to recover the
// token usage a launch actually consumed (token-telemetry campaign S1;
// docs/plans/token-telemetry-2026-07.md). It is best-effort instrumentation:
// a missing or unreadable transcript yields zero usage and SourceNone rather
// than an error, so telemetry never blocks a cycle.
//
// The scanner attributes a transcript to a launch by the launch's unique
// ArtifactPath appearing in the transcript's first user message — the only key
// that is both stable and unique across the exec boundary. Neither the on-disk
// session-directory slug (a lossy sanitisation of cwd) nor the recorded cwd
// itself can be trusted: cwd degrades when WORKTREE_PATH falls back to the repo
// root and shifts when the agent cd's into a subdir. ArtifactPath-less Windows
// (legacy launches, unit fixtures) fall back to an exact cwd==Worktree match.
// Streamed usage deltas that repeat one message id are deduplicated to the last
// (highest-cumulative) delta, never summed.
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

// Window describes the launch whose token usage is being recovered.
// ArtifactPath is the launch's unique deliverable reference and the PRIMARY
// attribution key: it appears verbatim in the transcript's first user message.
// Worktree is the fallback key — the exact cwd a matching transcript must
// record — consulted only for ArtifactPath-less Windows.
// Start and End bound the assistant turns that count toward the launch.
// EventsLogPath and Scrollback carry the lower fallback tiers' inputs:
// the launch's *-events.ndjson path (tier 2) and the captured pane
// scrollback content — not a pane id — (tier 3). Either may be empty; an
// empty input simply leaves that tier with no data. Driver is the launch's
// CLI/driver identity (req.CLI, e.g. "claude-tmux", "codex", "agy") so the
// resolver can dispatch the fidelity chain per driver; empty means claude
// (backward compatible).
type Window struct {
	Worktree      string
	ArtifactPath  string
	EventsLogPath string
	Scrollback    string
	Driver        string
	Start         time.Time
	End           time.Time
}

// Result is the outcome of a scan: the summed token usage and the Source that
// produced it. Warn carries an explicit per-driver coverage warning when no
// tier could observe the launch's usage (Source == SourceNone) — the signal
// that distinguishes "unmeasured" from "measured zero" so uncovered drivers
// never masquerade as free (the 2026-07-13 all-zeros baseline defect).
type Result struct {
	Usage  cyclestate.TokenUsage
	Source Source
	Warn   string
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
		// Content is a JSON union (bare string OR array of typed blocks); kept
		// raw so contentText can decode either form.
		Content json.RawMessage `json:"content"`
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
	defer func() { _ = f.Close() }()

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

// attributes reports whether a transcript belongs to the launch. When
// w.ArtifactPath is set (all production launches) it alone attributes: the
// deliverable path is stamped verbatim into this launch's first user message
// (the subagent assembler's "Artifact path: <path>" line) and is cycle+phase
// unique. Cwd is unreliable across the exec boundary (see the package doc), so
// it is only a fallback for ArtifactPath-less Windows (legacy launches, unit
// fixtures), which require an exact cwd == w.Worktree match.
func attributes(lines []transcriptLine, w Window) bool {
	if w.ArtifactPath != "" {
		return strings.Contains(firstUserText(lines), w.ArtifactPath)
	}
	for _, ln := range lines {
		if ln.Cwd == w.Worktree {
			return true
		}
	}
	return false
}

// firstUserText returns the text of the first user message, or "" if there is
// none. The Claude Code transcript encodes message content in two forms and the
// scanner must read both (the first user message — the phase prompt carrying the
// ArtifactPath attribution key — is commonly the bare-string form).
func firstUserText(lines []transcriptLine) string {
	for _, ln := range lines {
		if ln.Type != "user" {
			continue
		}
		return contentText(ln.Message.Content)
	}
	return ""
}

// contentText decodes a transcript message's content, which is EITHER a bare
// JSON string OR an array of typed blocks ({"type":"text","text":...}). It
// returns the string form directly, or the concatenated block text; unknown or
// empty content yields "".
func contentText(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return s
	}
	var blocks []struct {
		Text string `json:"text"`
	}
	if json.Unmarshal(raw, &blocks) == nil {
		var b strings.Builder
		for _, blk := range blocks {
			b.WriteString(blk.Text)
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
