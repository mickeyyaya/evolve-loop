package inputindex

import (
	"bufio"
	"bytes"
	"cmp"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
)

const (
	AccessSourceSandbox = "sandbox"
	AccessSourceNone    = "none"
	AccessOpen          = "open"
	AccessCell          = "cell"
)

type AccessRow struct {
	ID       string `json:"id"`
	Kind     string `json:"kind"`
	Layer    string `json:"layer"`
	Required bool   `json:"required,omitempty"`
}

type AccessRecord struct {
	Type     string      `json:"type"`
	Cycle    int         `json:"cycle"`
	Phase    string      `json:"phase"`
	Dispatch string      `json:"dispatch"`
	Source   string      `json:"source"`
	Entry    string      `json:"entry,omitempty"`
	Kind     string      `json:"kind,omitempty"`
	Layer    string      `json:"layer,omitempty"`
	Required bool        `json:"required,omitempty"`
	Listed   []AccessRow `json:"listed,omitempty"`
	Opened   []string    `json:"opened,omitempty"`
	Unopened []string    `json:"unopened,omitempty"`
}

func AccessDir(projectRoot string) string {
	return filepath.Join(projectRoot, ".evolve", "context-access")
}

func accessLogPath(projectRoot string, cycle int) string {
	return filepath.Join(AccessDir(projectRoot), "cycle-"+strconv.Itoa(cycle)+".jsonl")
}

func RecordAccess(projectRoot string, idx Index, dispatch, source string, opened []string) ([]string, error) {
	base := AccessRecord{Cycle: idx.Cycle, Phase: idx.Phase, Dispatch: dispatch, Source: source}
	var lines bytes.Buffer
	cell := base
	cell.Type, cell.Opened = AccessCell, []string{}
	for _, e := range idx.Entries {
		cell.Listed = append(cell.Listed, AccessRow{ID: e.ID, Kind: e.Kind, Layer: e.Layer, Required: e.Required})
		if !slices.Contains(opened, e.ID) {
			if e.Required {
				cell.Unopened = append(cell.Unopened, e.ID)
			}
			continue
		}
		rec := base
		rec.Type, rec.Entry, rec.Kind, rec.Layer, rec.Required = AccessOpen, e.ID, e.Kind, e.Layer, e.Required
		cell.Opened = append(cell.Opened, e.ID)
		appendLine(&lines, rec)
	}
	appendLine(&lines, cell)
	if err := os.MkdirAll(AccessDir(projectRoot), 0o755); err != nil {
		return nil, fmt.Errorf("inputindex: make the access log directory: %w", err)
	}
	f, err := os.OpenFile(accessLogPath(projectRoot, idx.Cycle), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err == nil {
		_, err = f.Write(lines.Bytes())
		err = errors.Join(err, f.Close())
	}
	if err != nil {
		return nil, fmt.Errorf("inputindex: write the access log: %w", err)
	}
	return cell.Unopened, nil
}

func appendLine(b *bytes.Buffer, rec AccessRecord) {
	_ = json.NewEncoder(b).Encode(rec)
}

func ReadAccess(projectRoot string) ([]AccessRecord, error) {
	paths, _ := filepath.Glob(filepath.Join(AccessDir(projectRoot), "cycle-*.jsonl"))
	var out []AccessRecord
	for _, path := range paths {
		raw, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("inputindex: read the access log %s: %w", path, err)
		}
		scanner := bufio.NewScanner(bytes.NewReader(raw))
		scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
		for scanner.Scan() {
			var rec AccessRecord
			if json.Unmarshal(scanner.Bytes(), &rec) == nil {
				out = append(out, rec)
			}
		}
	}
	return out, nil
}

type KindStat struct {
	Phase    string  `json:"phase"`
	Kind     string  `json:"kind"`
	Layer    string  `json:"layer"`
	Listed   int     `json:"listed"`
	Opened   int     `json:"opened"`
	Required int     `json:"required"`
	Rate     float64 `json:"rate"`
}

func Stats(records []AccessRecord, phase string) []KindStat {
	byKey := map[[2]string]*KindStat{}
	for _, r := range records {
		openedEverything := len(r.Listed) > 1 && len(r.Opened) == len(r.Listed)
		if r.Type != AccessCell || r.Source != AccessSourceSandbox || (phase != "" && r.Phase != phase) || openedEverything {
			continue
		}
		for _, row := range r.Listed {
			key := [2]string{r.Phase, row.Kind}
			st, ok := byKey[key]
			if !ok {
				st = &KindStat{Phase: r.Phase, Kind: row.Kind, Layer: row.Layer}
				byKey[key] = st
			}
			st.Listed++
			if slices.Contains(r.Opened, row.ID) {
				st.Opened++
			}
			if row.Required {
				st.Required++
			}
		}
	}
	out := make([]KindStat, 0, len(byKey))
	for _, st := range byKey {
		st.Rate = float64(st.Opened) / float64(st.Listed)
		out = append(out, *st)
	}
	slices.SortFunc(out, func(a, b KindStat) int { return cmp.Or(cmp.Compare(a.Phase, b.Phase), cmp.Compare(a.Kind, b.Kind)) })
	return out
}
