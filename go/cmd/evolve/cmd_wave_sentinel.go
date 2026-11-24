package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"

	"github.com/mickeyyaya/evolve-loop/go/internal/wave"
)

type waveHostReading struct {
	Sentinel *sentinelLine   `json:"sentinel,omitempty"`
	Load     *hostSampleLine `json:"load,omitempty"`
}

func readWaveHost(evolveDir string, rec wave.Record) (*waveHostReading, []string) {
	cfg := loopConfig{EvolveDir: evolveDir, LogPath: rec.LogPath}
	var reading waveHostReading
	var warnings []string
	if err := lastJSONLine(sentinelSamplesPath(cfg), &reading.Sentinel); err != nil {
		warnings = append(warnings, "host sentinel sample: "+err.Error())
	}
	if err := lastJSONLine(hostSamplesPath(cfg), &reading.Load); err != nil {
		warnings = append(warnings, "host pressure sample: "+err.Error())
	}
	if reading.Sentinel == nil && reading.Load == nil {
		return nil, warnings
	}
	return &reading, warnings
}

func lastJSONLine[T any](path string, into **T) error {
	line, err := lastLine(path)
	if errors.Is(err, fs.ErrNotExist) || (err == nil && line == nil) {
		return nil
	}
	if err != nil {
		return err
	}
	var v T
	if err := json.Unmarshal(line, &v); err != nil {
		return fmt.Errorf("%s: last line is not JSON: %w", path, err)
	}
	*into = &v
	return nil
}

func lastLine(path string) ([]byte, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	trimmed := bytes.TrimRight(data, "\n ")
	if len(trimmed) == 0 {
		return nil, nil
	}
	return trimmed[bytes.LastIndexByte(trimmed, '\n')+1:], nil
}

func writeWaveHostText(w io.Writer, h *waveHostReading) {
	if h == nil {
		return
	}
	if s := h.Sentinel; s != nil {
		fmt.Fprintf(w, "host:    free %s at %s, hold %s (held=%t), release %s, at %s\n", gcSize(int64(s.FreeBytes)), s.Path, s.HoldStage, s.Held, s.ReleaseStage, s.TS)
		if s.CacheRead {
			fmt.Fprintf(w, "         go cache %s\n", gcSize(s.CacheBytes))
		}
		if s.CountsRead {
			fmt.Fprintf(w, "         owner-dead tmux servers %d, orphan processes %d\n", s.Servers, s.Orphans)
		}
	}
	if l := h.Load; l != nil {
		fmt.Fprintf(w, "load:    %.2f per core, memory available %.0f%%, window cap %d, at %s\n", l.LoadPerCore, l.MemAvailablePct, l.Cap, l.TS)
	}
}
