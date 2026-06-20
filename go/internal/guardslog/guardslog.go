// Package guardslog centralizes the best-effort "guards log" NDJSON-ish append
// shared by the commit-prefix and post-edit validation guards. Both used to
// open/write/close inline with the close error silently discarded; this single
// home makes the close error a first-class, testable return value.
package guardslog

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

// openAppend opens path for create/append/write. A package var so tests can
// inject a WriteCloser whose Close fails (the close-error path is otherwise
// hard to trigger deterministically).
var openAppend = func(path string) (io.WriteCloser, error) {
	return os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
}

// Append writes one line "[ts] [label] msg\n" to path (creating parent dirs)
// and returns the FIRST error encountered — the write error if any, else the
// close error (which a buffered/networked sink can surface only at Close).
// Returns nil for an empty path. Best-effort callers discard the result
// explicitly; centralizing the open/write/close here is what makes the close
// error observable and testable.
func Append(path, label, msg string, ts time.Time) error {
	if path == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	wc, err := openAppend(path)
	if err != nil {
		return err
	}
	_, werr := fmt.Fprintf(wc, "[%s] [%s] %s\n", ts.UTC().Format(time.RFC3339), label, msg)
	cerr := wc.Close()
	if werr != nil {
		return werr
	}
	return cerr
}
