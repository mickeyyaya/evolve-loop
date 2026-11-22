package dispatchevents

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

const maxEventLineBytes = 1 << 20

func ReadEvents(workspace string) ([]Event, error) {
	path := filepath.Join(workspace, EventsFile)
	f, err := os.Open(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("dispatchevents read: %w", err)
	}
	defer func() { _ = f.Close() }()
	var out []Event
	sc := bufio.NewScanner(f)
	sc.Buffer(nil, maxEventLineBytes)
	for line := 1; sc.Scan(); line++ {
		text := strings.TrimSpace(sc.Text())
		if text == "" {
			continue
		}
		var e Event
		if err := json.Unmarshal([]byte(text), &e); err != nil {
			return nil, fmt.Errorf("dispatchevents read %s line %d: %w", path, line, err)
		}
		out = append(out, e)
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("dispatchevents read %s: %w", path, err)
	}
	return out, nil
}
