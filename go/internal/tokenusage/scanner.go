// Package tokenusage recovers, best-effort, the token usage and context fill of one CLI launch
// from its transcript, events log or pane scrollback.
// See docs/architecture/packages/internal-tokenusage.md.
package tokenusage

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
)

// Source identifies how a Result's usage was obtained.
type Source string

const (
	// SourceNone means no source observed the launch; Usage is zero.
	SourceNone Source = "none"
	// SourceTranscript means usage was recovered from an attributed Claude Code transcript.
	SourceTranscript Source = "transcript"
)

// Window identifies the launch whose token usage is recovered, and carries each tier's input.
type Window struct {
	// Worktree is the exact-cwd fallback key, used only when ArtifactPath is empty.
	Worktree string
	// ArtifactPath is the primary attribution key, stamped into the launch's first user message.
	ArtifactPath  string
	EventsLogPath string
	// Scrollback is the captured pane content, not a pane id.
	Scrollback string
	// Driver is the launch's CLI identity, such as "claude-tmux" or "codex"; empty means claude.
	Driver string
	// Start and End bound, inclusively, the assistant turns that count.
	Start time.Time
	End   time.Time
}

// Result is a launch's recovered usage, the Source that produced it, and the derived fill reading.
type Result struct {
	// Usage is the launch's total spend, summed across its turns.
	Usage  cyclestate.TokenUsage
	Source Source
	// Warn names the driver when no source observed the launch, so unmeasured never reads as free.
	Warn string
	// FillPct is the percent of the driver's effective window in use, or FillPctUnmeasured.
	FillPct float64
	// PeakPromptTokens is the fullest single turn's prompt side: 0 when the tier reports no
	// turns, negative when turns were expected but none fell inside the Window.
	PeakPromptTokens int
	// PeakUsage holds the counters of that same fullest turn.
	PeakUsage cyclestate.TokenUsage
}

// transcriptLine is the subset of a Claude Code transcript JSONL record the scanner reads.
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
		// Content is a bare string or an array of text blocks, kept raw for contentText.
		Content json.RawMessage `json:"content"`
	} `json:"message"`
}

// ScanConfigRoot sums the usage of the transcripts under root/projects attributed to w; finding none is SourceNone, not an error.
func ScanConfigRoot(root string, w Window) (Result, error) {
	projects := filepath.Join(root, "projects")
	if _, err := os.Stat(projects); err != nil {
		return Result{Source: SourceNone}, nil
	}

	// Streamed deltas repeat a message id and are cumulative, so only the last one per id counts.
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
	// The sum is what the launch cost; the peak turn is how full its window got.
	// perMsg has no reliable turn order, so the peak stands in for the last turn.
	var total cyclestate.TokenUsage
	peak := promptTokensUnmeasured
	var peakUsage cyclestate.TokenUsage
	for _, u := range perMsg {
		total.Input += u.Input
		total.Output += u.Output
		total.CacheRead += u.CacheRead
		total.CacheWrite += u.CacheWrite
		if p := PromptTokens(u); p > peak {
			peak = p
			peakUsage = u
		}
	}
	return Result{Usage: total, Source: SourceTranscript, PeakPromptTokens: peak, PeakUsage: peakUsage}, nil
}

// readLines skips unparseable lines and returns nil for a file it cannot open.
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

// artifactMarker is the label both subagent prompt assemblers stamp before the deliverable path.
const artifactMarker = "Artifact path: "

// artifactAnchors are every label a prompt puts before its deliverable path; anchor+path keeps a prompt that only
// cites another launch's artifact from being billed to it. A dropped form silently degrades its launches to scrollback.
var artifactAnchors = []string{
	artifactMarker,
	phasecontract.FooterMarker + " ",
	"<artifact-path>",
}

// attributes matches the anchored ArtifactPath when set, else an exact cwd; cwd is lossy across the exec boundary.
// See ADR-0071.
func attributes(lines []transcriptLine, w Window) bool {
	if w.ArtifactPath != "" {
		text := firstUserText(lines)
		for _, anchor := range artifactAnchors {
			if strings.Contains(text, anchor+w.ArtifactPath) {
				return true
			}
		}
		return false
	}
	for _, ln := range lines {
		if ln.Cwd == w.Worktree {
			return true
		}
	}
	return false
}

func firstUserText(lines []transcriptLine) string {
	for _, ln := range lines {
		if ln.Type != "user" {
			continue
		}
		return contentText(ln.Message.Content)
	}
	return ""
}

// contentText decodes both content forms; real transcripts carry the phase prompt as a bare string.
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

// withinWindow admits a turn with no parseable timestamp, since its transcript is already attributed.
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
