package inbox

import (
	"bufio"
	"encoding/json"
	"errors"
	"io"
	"os"
)

// Cursor tracks a byte offset into one agent inbox so a draining driver
// delivers only NEW envelopes each poll tick. It is byte-offset based (not
// seq based) so a partial final line written by a concurrent sender is never
// half-parsed: Drain only advances past the last complete '\n'-terminated
// line.
//
// Concurrency contract: many writers (Append), exactly one reader (Drain)
// per agent. The reader never writes the file; it only mutates its in-memory
// offset.
type Cursor struct {
	path   string
	offset int64
}

// NewCursor returns a cursor positioned at the start of the agent inbox.
func NewCursor(workspace, agent string) *Cursor {
	return &Cursor{path: Path(workspace, agent)}
}

// Offset returns the current read offset (bytes consumed).
func (c *Cursor) Offset() int64 { return c.offset }

// SetOffset moves the cursor. The driver seeks to EOF on entry so a resumed
// named session (or a stale ephemeral file) never replays a pre-launch
// backlog — live injection only ever delivers envelopes appended after the
// agent is running.
func (c *Cursor) SetOffset(n int64) { c.offset = n }

// Drain reads complete lines appended since the last call, parses each into
// an Envelope, advances the offset past the last complete line, and returns
// the parsed envelopes. A missing file yields (nil, nil). A file shorter than
// the offset (truncation/rotation) resets the offset to 0 and re-reads.
// Malformed lines are skipped (the caller logs).
//
// Caveat: an in-place truncate followed by a rewrite to the EXACT same byte
// length between two Drain calls is undetectable by offset alone and is not
// handled — the inbox is append-only in practice, drained every couple of
// seconds, so this would at worst miss one injection on a pathological input.
func (c *Cursor) Drain() ([]Envelope, error) {
	fi, err := os.Stat(c.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	if fi.Size() < c.offset {
		c.offset = 0 // truncated/rotated
	}
	if fi.Size() == c.offset {
		return nil, nil
	}
	f, err := os.Open(c.path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	if _, err := f.Seek(c.offset, io.SeekStart); err != nil {
		return nil, err
	}

	var envs []Envelope
	var consumed int64
	r := bufio.NewReader(f)
	for {
		line, err := r.ReadBytes('\n')
		if err != nil {
			// No trailing newline → partial line; do not consume or parse it.
			break
		}
		consumed += int64(len(line))
		var env Envelope
		if jerr := json.Unmarshal(line[:len(line)-1], &env); jerr != nil {
			continue // skip malformed; offset still advances past it
		}
		envs = append(envs, env)
	}
	c.offset += consumed
	return envs, nil
}
