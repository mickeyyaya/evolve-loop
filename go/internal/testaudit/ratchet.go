package testaudit

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

const shardRelPath = "testdata/testaudit-baseline.json"

type shard struct {
	Rows []Finding `json:"rows"`
}

func WriteShard(dir string, found []Finding) error {
	path := filepath.Join(dir, filepath.FromSlash(shardRelPath))
	rows := finish(found, Config{})
	if len(rows) == 0 {
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		return nil
	}
	raw, err := json.MarshalIndent(shard{Rows: rows}, "", "  ")
	if err == nil {
		err = writeAtomic(path, append(raw, '\n'))
	}
	return err
}

func writeAtomic(path string, raw []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func CheckShard(dir string, cfg Config) error {
	found, err := analyze(dir, cfg, false)
	if err != nil {
		return err
	}
	baseline, err := readShard(dir)
	if err != nil {
		return err
	}
	return diffFindings(found, baseline)
}

type RowKind int

const (
	RowMissing RowKind = iota
	RowStale
)

var (
	ErrNoBaselineRow    = errors.New("testaudit: finding has no baseline row")
	ErrStaleBaselineRow = errors.New("testaudit: baseline row matches no finding")
)

type RowError struct {
	Kind    RowKind
	Finding Finding
}

func (e *RowError) Error() string {
	f := e.Finding
	if e.Kind == RowStale {
		return fmt.Sprintf("testaudit: baseline row %s %s %s matches no finding", f.Package, f.Test, f.Rule)
	}
	return fmt.Sprintf("testaudit: %s %s %s has no baseline row", f.Package, f.Test, f.Rule)
}

func (e *RowError) Unwrap() error {
	if e.Kind == RowStale {
		return ErrStaleBaselineRow
	}
	return ErrNoBaselineRow
}

func Problems(err error) []error {
	if err == nil {
		return nil
	}
	if joined, ok := err.(interface{ Unwrap() []error }); ok {
		var out []error
		for _, e := range joined.Unwrap() {
			out = append(out, Problems(e)...)
		}
		return out
	}
	return []error{err}
}

func diffFindings(found, baseline []Finding) error {
	have := make(map[Finding]bool, len(found))
	for _, f := range found {
		have[f] = true
	}
	want := make(map[Finding]bool, len(baseline))
	for _, f := range baseline {
		want[f] = true
	}
	var problems []error
	for _, f := range found {
		if !want[f] {
			problems = append(problems, &RowError{Kind: RowMissing, Finding: f})
		}
	}
	for _, f := range baseline {
		if !have[f] {
			problems = append(problems, &RowError{Kind: RowStale, Finding: f})
		}
	}
	return errors.Join(problems...)
}

func readShard(dir string) ([]Finding, error) {
	rows, _, err := readShardFile(filepath.Join(dir, filepath.FromSlash(shardRelPath)))
	return rows, err
}

func readShardFile(path string) ([]Finding, bool, error) {
	raw, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	var s shard
	if err := json.Unmarshal(raw, &s); err != nil {
		return nil, true, fmt.Errorf("testaudit: %s: %w", shardRelPath, err)
	}
	return s.Rows, true, nil
}
