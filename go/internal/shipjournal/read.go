package shipjournal

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
)

type LineError struct {
	Line int
	Err  error
}

type Journal struct {
	Path      string
	Entries   []Entry
	Malformed []LineError
}

func Read(path string) (Journal, error) {
	j := Journal{Path: path}
	raw, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return j, nil
	}
	if err != nil {
		return j, fmt.Errorf("%s: %w", path, err)
	}
	for i, line := range bytes.Split(raw, []byte("\n")) {
		if len(bytes.TrimSpace(line)) == 0 {
			continue
		}
		var e Entry
		if err := json.Unmarshal(line, &e); err != nil {
			j.Malformed = append(j.Malformed, LineError{Line: i + 1, Err: err})
			continue
		}
		j.Entries = append(j.Entries, e)
	}
	return j, nil
}

func (j Journal) Strict() error {
	if len(j.Malformed) == 0 {
		return nil
	}
	first := j.Malformed[0]
	return fmt.Errorf("%s line %d: %w", j.Path, first.Line, first.Err)
}
