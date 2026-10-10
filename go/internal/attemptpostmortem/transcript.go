package attemptpostmortem

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var ErrNoTranscript = errors.New("attemptpostmortem: the CLI writes no transcript that the bridge can read")

type Trace struct {
	Path           string
	Commands       []Command
	LastActivityAt time.Time
	SkippedLines   int
}

type Transcript func() (Trace, error)

func TranscriptFor(cli, path string) Transcript {
	if path != "" && WritesTranscript(cli) {
		return ClaudeTranscript(path)
	}
	return func() (Trace, error) { return Trace{}, fmt.Errorf("%w (cli %s)", ErrNoTranscript, cli) }
}

func WritesTranscript(cli string) bool {
	return cli == "claude" || strings.HasPrefix(cli, "claude-")
}

func ClaudeTranscript(path string) Transcript {
	return func() (Trace, error) {
		entries, skipped, err := readEntries(path)
		if err != nil {
			return Trace{}, err
		}
		trace := traceOf(path, entries)
		trace.SkippedLines = skipped
		return trace, nil
	}
}

const maxTranscriptLineBytes = 8 * 1024 * 1024

type transcriptEntry struct {
	Type      string `json:"type"`
	Timestamp string `json:"timestamp"`
	Message   struct {
		Content json.RawMessage `json:"content"`
	} `json:"message"`
}

type contentBlock struct {
	Type      string                   `json:"type"`
	ID        string                   `json:"id"`
	Name      string                   `json:"name"`
	Input     struct{ Command string } `json:"input"`
	ToolUseID string                   `json:"tool_use_id"`
	IsError   bool                     `json:"is_error"`
	Content   json.RawMessage          `json:"content"`
}

func readEntries(path string) ([]transcriptEntry, int, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, 0, err
	}
	defer func() { _ = f.Close() }()
	r := bufio.NewReaderSize(f, 64*1024)
	var entries []transcriptEntry
	skipped := 0
	for {
		line, tooLong, err := readLine(r)
		var e transcriptEntry
		switch {
		case tooLong:
			skipped++
		case json.Unmarshal(line, &e) == nil:
			entries = append(entries, e)
		}
		if errors.Is(err, io.EOF) {
			return entries, skipped, nil
		}
		if err != nil {
			return nil, 0, fmt.Errorf("read transcript %s: %w", path, err)
		}
	}
}

func readLine(r *bufio.Reader) ([]byte, bool, error) {
	var line []byte
	tooLong := false
	for {
		chunk, err := r.ReadSlice('\n')
		if !tooLong {
			line = append(line, chunk...)
			tooLong = len(bytes.TrimSuffix(line, []byte("\n"))) > maxTranscriptLineBytes
		}
		if !errors.Is(err, bufio.ErrBufferFull) {
			if tooLong {
				line = nil
			}
			return line, tooLong, err
		}
	}
}

func traceOf(path string, entries []transcriptEntry) Trace {
	trace := Trace{Path: path}
	index := map[string]int{}
	for _, e := range entries {
		at, err := time.Parse(time.RFC3339Nano, e.Timestamp)
		if err == nil && at.After(trace.LastActivityAt) {
			trace.LastActivityAt = at
		}
		for _, b := range blocksOf(e.Message.Content) {
			switch {
			case b.Type == "tool_use" && b.Name == "Bash":
				index[b.ID] = len(trace.Commands)
				trace.Commands = append(trace.Commands, Command{Text: b.Input.Command, StartedAt: at, Status: StatusNoResult})
			case b.Type == "tool_result":
				if i, ok := index[b.ToolUseID]; ok {
					trace.Commands[i].Status, trace.Commands[i].ExitCode = resultStatus(b)
				}
			}
		}
	}
	return trace
}

func blocksOf(raw json.RawMessage) []contentBlock {
	var blocks []contentBlock
	if json.Unmarshal(raw, &blocks) != nil {
		return nil
	}
	return blocks
}

var exitCodeLine = regexp.MustCompile(`^Exit code (\d{1,3})(?:\n|$)`)

const signalExitFloor = 128

func resultStatus(b contentBlock) (Status, int) {
	if !b.IsError {
		return StatusOK, 0
	}
	m := exitCodeLine.FindStringSubmatch(resultText(b.Content))
	if m == nil {
		return StatusError, 0
	}
	code, _ := strconv.Atoi(m[1])
	if code > signalExitFloor {
		return StatusSignal, code
	}
	return StatusError, code
}

func resultText(raw json.RawMessage) string {
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return s
	}
	var parts []struct {
		Text string `json:"text"`
	}
	if json.Unmarshal(raw, &parts) != nil {
		return ""
	}
	var b strings.Builder
	for _, p := range parts {
		b.WriteString(p.Text)
	}
	return b.String()
}
