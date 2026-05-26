// Package logfilter writes a human-readable <phase>-stdout.clean.txt
// companion alongside each raw <phase>-stdout.log captured by the bridge.
// The raw file is never touched.
//
// Each line is classified as stream-json (claude-p/codex/agy) or plain
// text (claude-tmux) by attempting JSON parse first; a single entry point
// covers all drivers without format-mode flags.
package logfilter

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// maxScannerBufBytes mirrors cyclecost's 16 MB cap so stream-json events
// with deeply nested usage stats don't blow up the line scanner.
const maxScannerBufBytes = 1 << 24

// Process reads <workspace>/<phase>-stdout.log and writes
// <workspace>/<phase>-stdout.clean.txt atomically.
// Returns nil when the raw file is missing (phase never reached the bridge).
func Process(workspace, phase string) error {
	rawPath := filepath.Join(workspace, phase+"-stdout.log")
	cleanPath := filepath.Join(workspace, phase+"-stdout.clean.txt")

	raw, err := os.Open(rawPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("open raw: %w", err)
	}
	defer func() { _ = raw.Close() }()

	tmp, err := os.CreateTemp(workspace, "."+phase+"-stdout.clean.tmp.*")
	if err != nil {
		return fmt.Errorf("create temp: %w", err)
	}
	tmpPath := tmp.Name()
	cleanup := func() {
		_ = tmp.Close()
		_ = os.Remove(tmpPath)
	}

	if err := filterStream(raw, tmp); err != nil {
		cleanup()
		return fmt.Errorf("filter: %w", err)
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("close temp: %w", err)
	}
	if err := os.Rename(tmpPath, cleanPath); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("rename: %w", err)
	}
	return nil
}

// filterStream drives the per-line classifier. Exported for tests.
// The trailing `return out.Flush()` is the sole flush — every error
// path returns early before reaching it, leaving no buffered bytes
// behind because partial output on error is undesirable anyway.
func filterStream(r io.Reader, w io.Writer) error {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 1<<10), maxScannerBufBytes)
	out := bufio.NewWriter(w)

	pt := newPlainTextState()
	for scanner.Scan() {
		line := scanner.Bytes()
		if handled, formatted := classifyJSON(line); handled {
			if flushed := pt.flush(); flushed != "" {
				if _, err := out.WriteString(flushed); err != nil {
					return err
				}
			}
			if formatted != "" {
				if _, err := out.WriteString(formatted); err != nil {
					return err
				}
				if err := out.WriteByte('\n'); err != nil {
					return err
				}
			}
			continue
		}
		if formatted, emit := pt.next(string(line)); emit {
			if _, err := out.WriteString(formatted); err != nil {
				return err
			}
			if err := out.WriteByte('\n'); err != nil {
				return err
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	if flushed := pt.flush(); flushed != "" {
		if _, err := out.WriteString(flushed); err != nil {
			return err
		}
	}
	return out.Flush()
}
