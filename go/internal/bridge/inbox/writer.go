package inbox

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

const maxLine = 4096

func Append(workspace, agent string, env Envelope, now func() time.Time) error {
	if env.TS == "" {
		env.TS = now().UTC().Format(time.RFC3339)
	}
	b, _ := json.Marshal(env)
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
	defer func() { _ = f.Close() }()
	return writeLine(f, b)
}

func writeLine(w io.Writer, line []byte) error {
	if _, err := w.Write(line); err != nil {
		return fmt.Errorf("inbox: write: %w", err)
	}
	return nil
}
