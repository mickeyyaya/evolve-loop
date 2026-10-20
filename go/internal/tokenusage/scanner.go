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

type Source string

const (
	SourceNone       Source = "none"
	SourceTranscript Source = "transcript"
)

type Window struct {
	Worktree       string
	ArtifactPath   string
	EventsLogPath  string
	Scrollback     string
	Driver         string
	TokenLineRegex string
	Start          time.Time
	End            time.Time
}

type Result struct {
	Usage            cyclestate.TokenUsage
	Source           Source
	Warn             string
	FillPct          float64
	PeakPromptTokens int
	PeakUsage        cyclestate.TokenUsage
	ServedModel      string
}

type transcriptLine struct {
	Type      string `json:"type"`
	Cwd       string `json:"cwd"`
	Timestamp string `json:"timestamp"`
	Message   struct {
		ID    string `json:"id"`
		Model string `json:"model"`
		Usage *struct {
			Input      int `json:"input_tokens"`
			Output     int `json:"output_tokens"`
			CacheRead  int `json:"cache_read_input_tokens"`
			CacheWrite int `json:"cache_creation_input_tokens"`
		} `json:"usage"`
		Content json.RawMessage `json:"content"`
	} `json:"message"`
}

var walkDir = filepath.WalkDir

var readTranscript = readLines

const transcriptMtimeSlack = time.Minute

func predatesWindow(d os.DirEntry, w Window) bool {
	info, err := d.Info()
	if err != nil {
		return false
	}
	return info.ModTime().Before(w.Start.Add(-transcriptMtimeSlack))
}

func ScanConfigRoot(root string, w Window) (Result, error) {
	projects := filepath.Join(root, "projects")
	if _, err := os.Stat(projects); err != nil {
		return Result{Source: SourceNone}, nil
	}

	perMsg := map[string]cyclestate.TokenUsage{}
	matched := false
	warn := ""
	var served servedReply

	err := walkDir(projects, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil || d.IsDir() || !strings.HasSuffix(path, ".jsonl") {
			return nil
		}
		if predatesWindow(d, w) {
			return nil
		}
		lines, readErr := readTranscript(path)
		if readErr != nil && warn == "" {
			warn = "transcript " + path + " was read only in part, so its usage may be under-counted: " + readErr.Error()
		}
		if !attributes(lines, w) {
			return nil
		}
		matched = true
		recordAssistantUsage(perMsg, lines, w)
		served = latestServedReply(served, lines, w)
		return nil
	})
	if err != nil {
		return Result{Source: SourceNone}, nil
	}

	if !matched {
		return Result{Source: SourceNone}, nil
	}
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
	return Result{Usage: total, Source: SourceTranscript, PeakPromptTokens: peak, PeakUsage: peakUsage, Warn: warn, ServedModel: served.model}, nil
}

const syntheticModel = "<synthetic>"

type servedReply struct {
	model string
	at    time.Time
}

func latestServedReply(latest servedReply, lines []transcriptLine, w Window) servedReply {
	for _, ln := range lines {
		model := ln.Message.Model
		if ln.Type != "assistant" || model == "" || model == syntheticModel || !withinWindow(ln.Timestamp, w) {
			continue
		}
		at, _ := time.Parse(time.RFC3339, ln.Timestamp)
		if latest.model == "" || !at.Before(latest.at) {
			latest = servedReply{model: model, at: at}
		}
	}
	return latest
}

func recordAssistantUsage(perMsg map[string]cyclestate.TokenUsage, lines []transcriptLine, w Window) {
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
}

func readLines(path string) ([]transcriptLine, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, nil
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
	return out, sc.Err()
}

const artifactMarker = "Artifact path: "

var artifactAnchors = []string{
	artifactMarker,
	phasecontract.FooterMarker + " ",
	"<artifact-path>",
}

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
