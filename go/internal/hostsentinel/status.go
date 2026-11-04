package hostsentinel

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/atomicwrite"
)

const StatusFile = "host-sentinel.json"

type Status struct {
	Stage     Stage     `json:"stage"`
	Sample    Sample    `json:"sample"`
	Breaches  []Breach  `json:"breaches,omitempty"`
	Releases  []Release `json:"releases,omitempty"`
	HoldLanes bool      `json:"hold_lanes"`
}

func (s Status) Summary() string {
	parts := make([]string, 0, len(checks))
	for _, k := range checks {
		r, ok := s.Sample.Readings[k.metric]
		switch {
		case !ok:
			continue
		case r.Err != "":
			parts = append(parts, k.label+" failed")
		default:
			parts = append(parts, k.label+" "+k.unit(r.Value))
		}
	}
	text := fmt.Sprintf("sampled %s, stage %s: %s", s.Sample.At.UTC().Format(time.RFC3339), s.Stage, strings.Join(parts, ", "))
	if len(s.Breaches) > 0 {
		codes := make([]string, len(s.Breaches))
		for i, b := range s.Breaches {
			codes[i] = string(b.Code)
		}
		text += "; breaches " + strings.Join(codes, ",")
	}
	if s.HoldLanes {
		text += "; new lanes held"
	}
	return text
}

func WriteStatus(evolveDir string, s Status) error {
	return atomicwrite.JSON(filepath.Join(evolveDir, StatusFile), s)
}

func ReadStatus(evolveDir string) (*Status, error) {
	data, err := os.ReadFile(filepath.Join(evolveDir, StatusFile))
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var s Status
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("%s: %w", StatusFile, err)
	}
	return &s, nil
}
