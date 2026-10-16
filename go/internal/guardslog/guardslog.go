// Package guardslog appends one timestamped line to a guard log.
// It returns the close error rather than discarding it.
package guardslog

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

var openAppend = func(path string) (io.WriteCloser, error) {
	return os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
}

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
