package usageprobe

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/quotastate"
)

const HistoryFile = "usage-history.ndjson"

type historyLine struct {
	CLI        string    `json:"cli"`
	ObservedAt time.Time `json:"observed_at"`
	quotastate.UsageWindow
}

func HistoryPath(evolveDir string) string { return filepath.Join(evolveDir, HistoryFile) }

func appendHistory(evolveDir string, obs Observation) error {
	if len(obs.Windows) == 0 {
		return nil
	}
	var buf bytes.Buffer
	for _, w := range obs.Windows {
		line, err := json.Marshal(historyLine{CLI: obs.CLI, ObservedAt: obs.ObservedAt, UsageWindow: w})
		if err != nil {
			return wrapHistory(err)
		}
		buf.Write(line)
		buf.WriteByte('\n')
	}
	f, err := os.OpenFile(HistoryPath(evolveDir), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return wrapHistory(err)
	}
	_, writeErr := f.Write(buf.Bytes())
	return wrapHistory(errors.Join(writeErr, f.Close()))
}

func wrapHistory(err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("usage history: %w", err)
}
