package core

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

const maxNamedRefusals = 20

var shipRowMarker = []byte(`"role":"ship"`)

func landedShips(evolveDir string) (map[int]time.Time, error) {
	ships := map[int]time.Time{}
	f, err := os.Open(filepath.Join(evolveDir, "ledger.jsonl"))
	if errors.Is(err, os.ErrNotExist) {
		return ships, nil
	}
	if err != nil {
		return ships, err
	}
	defer func() { _ = f.Close() }()
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 16*1024*1024)
	for sc.Scan() {
		line := sc.Bytes()
		if !bytes.Contains(line, shipRowMarker) {
			continue
		}
		var row struct {
			TS       string `json:"ts"`
			Cycle    int    `json:"cycle"`
			Role     string `json:"role"`
			Kind     string `json:"kind"`
			ExitCode int    `json:"exit_code"`
		}
		if json.Unmarshal(line, &row) != nil || row.Role != "ship" || row.Kind != "phase" || row.ExitCode != 0 {
			continue
		}
		if at, err := time.Parse(time.RFC3339, row.TS); err == nil && at.After(ships[row.Cycle]) {
			ships[row.Cycle] = at
		}
	}
	return ships, sc.Err()
}

type refusalList struct {
	named  []string
	others map[string]int
}

func (r *refusalList) add(log io.Writer, st perRunState, verdict string) {
	if strings.HasPrefix(verdict, "checkpoint reason ") {
		if r.others == nil {
			r.others = map[string]int{}
		}
		r.others[st.reason]++
		return
	}
	fmt.Fprintf(log, "[resume] considered %s (cycle %d): %s\n", st.path, st.cycle, verdict)
	r.named = append(r.named, fmt.Sprintf("cycle %d: %s", st.cycle, verdict))
}

func (r refusalList) othersText() string {
	if len(r.others) == 0 {
		return ""
	}
	total := 0
	var parts []string
	for _, reason := range slices.Sorted(maps.Keys(r.others)) {
		total += r.others[reason]
		parts = append(parts, fmt.Sprintf("%s %d", reason, r.others[reason]))
	}
	return fmt.Sprintf("%d other cycle(s) hold a checkpoint that is not resumable (%s)", total, strings.Join(parts, ", "))
}

func (r refusalList) logSummary(log io.Writer) {
	if text := r.othersText(); text != "" {
		fmt.Fprintf(log, "[resume] considered: %s\n", text)
	}
}

func (r refusalList) lines() []string {
	lines := r.named
	if extra := len(lines) - maxNamedRefusals; extra > 0 {
		lines = append(slices.Clone(lines[:maxNamedRefusals]), fmt.Sprintf("%d more refused cycle(s)", extra))
	}
	if text := r.othersText(); text != "" {
		lines = append(slices.Clone(lines), text)
	}
	return lines
}
