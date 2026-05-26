package inbox

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// maxLine is the single-write atomicity ceiling. POSIX guarantees a write()
// of <= PIPE_BUF (4096 on darwin/linux) to an O_APPEND file is atomic, so
// concurrent senders never interleave lines. An envelope serializing larger
// than this is rejected loudly rather than risking a corrupt log.
const maxLine = 4096

// Append atomically appends one envelope as a single NDJSON line to the
// agent's inbox. Safe for multiple concurrent writers (CLI + observer):
// O_APPEND guarantees each write starts at EOF, and a sub-maxLine payload is
// written in a single syscall. now mints the TS when env.TS is empty.
func Append(workspace, agent string, env Envelope, now func() time.Time) error {
	if env.TS == "" {
		env.TS = now().UTC().Format(time.RFC3339)
	}
	b, err := json.Marshal(env)
	if err != nil {
		return fmt.Errorf("inbox: marshal: %w", err)
	}
	b = append(b, '\n')
	if len(b) > maxLine {
		return fmt.Errorf("inbox: envelope %d bytes exceeds atomic-append limit %d", len(b), maxLine)
	}
	p := Path(workspace, agent)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return fmt.Errorf("inbox: mkdir: %w", err)
	}
	f, err := os.OpenFile(p, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("inbox: open: %w", err)
	}
	defer f.Close()
	if _, err := f.Write(b); err != nil { // single write → atomic line
		return fmt.Errorf("inbox: write: %w", err)
	}
	return nil
}
