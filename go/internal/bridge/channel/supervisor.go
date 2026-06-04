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

// Sentinel errors returned by Supervisor.Ask.
var (
	// ErrTransportNoInject is returned when the configured transport is not a
	// tmux-REPL driver and therefore cannot receive live command injection.
	ErrTransportNoInject = errors.New("channel: transport does not support live injection (not a *-tmux driver)")

	// ErrResponseTimeout is returned when the correlated reply span does not
	// appear in the feed within the configured Timeout.
	ErrResponseTimeout = errors.New("channel: timed out waiting for correlated response")
)

// SupervisorConfig wires a Supervisor to one running phase agent.
type SupervisorConfig struct {
	Workspace string
	Agent     string

	// Transport is the driver name (e.g. "claude-tmux", "claude-p").
	// Only drivers whose name ends in "-tmux" support live injection.
	Transport string

	// Timeout is the maximum wall-time to wait for the correlated reply.
	// Defaults to 120 s.
	Timeout time.Duration

	// PollEvery is the feed-poll interval. Defaults to 500 ms.
	PollEvery time.Duration

	// Now is the time source. Defaults to time.Now. Injected for determinism.
	Now func() time.Time

	// NewID mints a fresh correlation ID. Defaults to a timestamp-based ID.
	// Injected for determinism in tests.
	NewID func() string
}

// Answer is the bracketed response span returned by Supervisor.Ask.
type Answer struct {
	// Events contains the raw decoded content envelopes (kind != "correlation")
	// whose top-level seq falls within the correlation span [start_seq, end_seq].
	Events []map[string]any
}

// Text concatenates the "text" field from all data objects in Events.
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

// Supervisor injects a correlated question via the ADR-0023 inbox and blocks
// until the agent's reply span appears in the feed, then returns the bracketed
// Answer. It is safe for concurrent use across separate Ask calls (each call
// mints its own correlation ID).
type Supervisor struct{ cfg SupervisorConfig }

// NewSupervisor constructs a Supervisor, applying defaults for optional fields.
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
		// Capture cfg.Now (set just above) rather than time.Now so an injected
		// clock fully governs determinism even when NewID is left default.
		cfg.NewID = func() string {
			return fmt.Sprintf("sup-%d", cfg.Now().UnixNano())
		}
	}
	return &Supervisor{cfg: cfg}
}

// Ask sends question to the agent via the inbox and blocks until the
// correlated response_complete span appears in the feed. It returns
// ErrTransportNoInject for non-tmux transports, ErrResponseTimeout on
// deadline, and ctx.Err() on context cancellation.
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

// awaitReply polls the feed until a correlation envelope with matching
// corr_id and sub=="response_complete" appears, then collects the content
// span and returns it.
func (s *Supervisor) awaitReply(ctx context.Context, corrID string) (Answer, error) {
	deadline := s.cfg.Now().Add(s.cfg.Timeout)
	feedPath := FeedPath(s.cfg.Workspace, s.cfg.Agent)
	var offset int64

	ticker := time.NewTicker(s.cfg.PollEvery)
	defer ticker.Stop()

	for {
		// Check context cancellation.
		select {
		case <-ctx.Done():
			return Answer{}, ctx.Err()
		default:
		}

		// Check Now-based timeout (deterministic; uses injected clock).
		if s.cfg.Now().After(deadline) {
			return Answer{}, ErrResponseTimeout
		}

		lines, newOffset := tailLines(feedPath, offset)
		offset = newOffset

		if lo, hi, ok := findResponseComplete(lines, corrID); ok {
			events := collectSpan(feedPath, lo, hi)
			return Answer{Events: events}, nil
		}

		// Wait for next poll tick or cancellation.
		select {
		case <-ctx.Done():
			return Answer{}, ctx.Err()
		case <-ticker.C:
		}

		// Re-check timeout after the wait.
		if s.cfg.Now().After(deadline) {
			return Answer{}, ErrResponseTimeout
		}
	}
}

// tailLines reads all complete newline-terminated lines from path starting at
// off, returns them and the new byte offset advanced past the last complete line.
func tailLines(path string, off int64) ([]string, int64) {
	f, err := os.Open(path)
	if err != nil {
		return nil, off
	}
	defer func() { _ = f.Close() }()

	fi, err := f.Stat()
	if err != nil {
		return nil, off
	}
	if fi.Size() < off {
		off = 0 // truncated/rotated
	}
	if fi.Size() == off {
		return nil, off
	}

	if _, err := f.Seek(off, io.SeekStart); err != nil {
		return nil, off
	}

	var lines []string
	var consumed int64
	r := bufio.NewReader(f)
	for {
		line, err := r.ReadBytes('\n')
		if err != nil {
			break // partial final line — do not consume
		}
		consumed += int64(len(line))
		lines = append(lines, strings.TrimRight(string(line), "\n"))
	}
	return lines, off + consumed
}

// findResponseComplete scans lines for a correlation envelope with sub ==
// "response_complete" and the matching corr_id. Returns (start_seq, end_seq,
// true) on success.
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

// collectSpan re-reads the entire feed from offset 0 and returns all content
// envelopes (kind != "correlation") whose top-level seq is in [lo, hi].
func collectSpan(path string, lo, hi int64) []map[string]any {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer func() { _ = f.Close() }()

	var result []map[string]any
	scanner := bufio.NewScanner(f)
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
	return result
}

// toI64 converts a JSON-decoded value to int64. JSON numbers decode as
// float64; native int64 values pass through. Other types return 0.
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
