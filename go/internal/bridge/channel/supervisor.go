package channel

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridge/inbox"
)

var (
	ErrTransportNoInject = errors.New("channel: transport does not support live injection (not a *-tmux driver)")

	ErrResponseTimeout = errors.New("channel: timed out waiting for correlated response")
)

type SupervisorConfig struct {
	Workspace string
	Agent     string

	Transport string

	Timeout time.Duration

	PollEvery time.Duration

	Now func() time.Time

	NewID func() string
}

type Answer struct {
	Events []map[string]any
}

func (a Answer) Text() string {
	var b strings.Builder
	for _, ev := range a.Events {
		d, _ := ev["data"].(map[string]any)
		if d == nil {
			continue
		}
		if t, _ := d["text"].(string); t != "" {
			b.WriteString(t)
		}
	}
	return b.String()
}

type Supervisor struct{ cfg SupervisorConfig }

func NewSupervisor(cfg SupervisorConfig) *Supervisor {
	if cfg.Timeout <= 0 {
		cfg.Timeout = 120 * time.Second
	}
	if cfg.PollEvery <= 0 {
		cfg.PollEvery = 500 * time.Millisecond
	}
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	if cfg.NewID == nil {
		cfg.NewID = func() string {
			return fmt.Sprintf("sup-%d", cfg.Now().UnixNano())
		}
	}
	return &Supervisor{cfg: cfg}
}

func (s *Supervisor) Ask(ctx context.Context, question string) (Answer, error) {
	if !strings.HasSuffix(s.cfg.Transport, "-tmux") {
		return Answer{}, ErrTransportNoInject
	}

	id := s.cfg.NewID()
	env := inbox.Envelope{
		Kind:   inbox.KindCommand,
		Body:   question,
		CorrID: id,
		Source: "supervisor",
	}
	if err := inbox.Append(s.cfg.Workspace, s.cfg.Agent, env, s.cfg.Now); err != nil {
		return Answer{}, fmt.Errorf("channel: supervisor: append inbox: %w", err)
	}

	return s.awaitReply(ctx, id)
}

func (s *Supervisor) awaitReply(ctx context.Context, corrID string) (Answer, error) {
	deadline := s.cfg.Now().Add(s.cfg.Timeout)
	feedPath := FeedPath(s.cfg.Workspace, s.cfg.Agent)
	var offset int64

	ticker := time.NewTicker(s.cfg.PollEvery)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return Answer{}, ctx.Err()
		default:
		}

		if s.cfg.Now().After(deadline) {
			return Answer{}, ErrResponseTimeout
		}

		lines, newOffset := tailLines(feedPath, offset)
		offset = newOffset

		if lo, hi, ok := findResponseComplete(lines, corrID); ok {
			events := collectSpan(feedPath, lo, hi)
			return Answer{Events: events}, nil
		}

		select {
		case <-ctx.Done():
			return Answer{}, ctx.Err()
		case <-ticker.C:
		}

		if s.cfg.Now().After(deadline) {
			return Answer{}, ErrResponseTimeout
		}
	}
}

var tailHooks = struct {
	stat func(*os.File) (os.FileInfo, error)
	seek func(*os.File, int64, int) (int64, error)
}{
	stat: (*os.File).Stat,
	seek: (*os.File).Seek,
}

func tailLines(path string, off int64) ([]string, int64) {
	f, err := os.Open(path)
	if err != nil {
		return nil, off
	}
	defer func() { _ = f.Close() }()

	fi, err := tailHooks.stat(f)
	if err != nil {
		return nil, off
	}
	if fi.Size() < off {
		off = 0
	}
	if fi.Size() == off {
		return nil, off
	}

	if _, err := tailHooks.seek(f, off, io.SeekStart); err != nil {
		return nil, off
	}

	var lines []string
	var consumed int64
	r := bufio.NewReader(f)
	for {
		line, err := r.ReadBytes('\n')
		if err != nil {
			break
		}
		consumed += int64(len(line))
		lines = append(lines, strings.TrimRight(string(line), "\n"))
	}
	return lines, off + consumed
}

func findResponseComplete(lines []string, corrID string) (int64, int64, bool) {
	for _, line := range lines {
		var env map[string]any
		if err := json.Unmarshal([]byte(line), &env); err != nil {
			continue
		}
		kind, _ := env["kind"].(string)
		if kind != "correlation" {
			continue
		}
		data, _ := env["data"].(map[string]any)
		if data == nil {
			continue
		}
		if sub, _ := data["sub"].(string); sub != "response_complete" {
			continue
		}
		if cid, _ := data["corr_id"].(string); cid != corrID {
			continue
		}
		lo := toI64(data["start_seq"])
		hi := toI64(data["end_seq"])
		return lo, hi, true
	}
	return 0, 0, false
}

func collectSpan(path string, lo, hi int64) []map[string]any {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer func() { _ = f.Close() }()

	var result []map[string]any
	const (
		spanLineStartBuf = 1024 * 1024
		spanLineMaxBuf   = 10 * 1024 * 1024
	)
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, spanLineStartBuf), spanLineMaxBuf)
	for scanner.Scan() {
		var env map[string]any
		if err := json.Unmarshal(scanner.Bytes(), &env); err != nil {
			continue
		}
		kind, _ := env["kind"].(string)
		if kind == "correlation" {
			continue
		}
		seq := toI64(env["seq"])
		if seq >= lo && seq <= hi {
			result = append(result, env)
		}
	}
	if err := scanner.Err(); err != nil {
		fmt.Fprintf(os.Stderr, "[channel] WARN collectSpan scan %s: %v\n", path, err)
	}
	return result
}

func toI64(v any) int64 {
	switch n := v.(type) {
	case float64:
		return int64(n)
	case int64:
		return n
	case int:
		return int64(n)
	default:
		return 0
	}
}
