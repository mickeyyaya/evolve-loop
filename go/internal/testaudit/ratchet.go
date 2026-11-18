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
	have := map[Finding]bool{}
	for _, f := range found {
		have[f] = true
	}
	want := map[Finding]bool{}
	for _, f := range baseline {
		want[f] = true
	}
	var problems []error
	for _, f := range found {
		if !want[f] {
			problems = append(problems, fmt.Errorf("testaudit: %s %s %s has no baseline row", f.Package, f.Test, f.Rule))
		}
	}
	for _, f := range baseline {
		if !have[f] {
			problems = append(problems, fmt.Errorf("testaudit: baseline row %s %s %s matches no finding", f.Package, f.Test, f.Rule))
		}
	}
	return errors.Join(problems...)
}

func readShard(dir string) ([]Finding, error) {
	raw, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(shardRelPath)))
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var s shard
	if err := json.Unmarshal(raw, &s); err != nil {
		return nil, fmt.Errorf("testaudit: %s: %w", shardRelPath, err)
	}
	return s.Rows, nil
}
