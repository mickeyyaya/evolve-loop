package cyclespend

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const GateSpansFile = "gate-spans.ndjson"

var ErrGateSpans = errors.New("gate spans")

type GateSpan struct {
	Gate  string    `json:"gate"`
	Start time.Time `json:"start"`
	End   time.Time `json:"end"`
	Load  float64   `json:"load"`
}

func (s GateSpan) Minutes() float64 {
	if s.End.Before(s.Start) {
		return 0
	}
	return s.End.Sub(s.Start).Minutes()
}

func ReadGateSpans(workspace string) ([]GateSpan, error) {
	path := filepath.Join(workspace, GateSpansFile)
	raw, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("%w: %s: %w", ErrGateSpans, path, err)
	}

	var spans []GateSpan
	var firstErr error
	for i, line := range strings.Split(string(raw), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var sp GateSpan
		if err := json.Unmarshal([]byte(line), &sp); err != nil {
			if firstErr == nil {
				firstErr = fmt.Errorf("%w: %s: line %d: %w", ErrGateSpans, path, i+1, err)
			}
			continue
		}
		spans = append(spans, sp)
	}
	return spans, firstErr
}
