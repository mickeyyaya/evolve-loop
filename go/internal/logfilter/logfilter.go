// Package logfilter writes a human-readable clean companion next to each raw phase stdout log.
// See docs/architecture/packages/internal-logfilter.md.
package logfilter

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

const maxScannerBufBytes = 1 << 24

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
