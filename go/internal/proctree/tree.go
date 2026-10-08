package proctree

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type treeEntry struct {
	Pid     int       `json:"pid"`
	Started time.Time `json:"started"`
}

type treeFile struct {
	Dispatch string      `json:"dispatch"`
	Members  []treeEntry `json:"members"`
}

func TreeFile(dir, id string) string {
	name := strings.Map(func(r rune) rune {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' {
			return r
		}
		return '_'
	}, id)
	return filepath.Join(dir, name+".json")
}

func SaveTree(dir, id string, ids []Identity) error {
	members := make([]treeEntry, 0, len(ids))
	for _, i := range ids {
		members = append(members, treeEntry{Pid: i.Pid, Started: i.Started.UTC()})
	}
	data, err := json.Marshal(treeFile{Dispatch: id, Members: members})
	if err != nil {
		return fmt.Errorf("encode dispatch tree: %w", err)
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("make dispatch tree dir: %w", err)
	}
	tmp, err := os.CreateTemp(dir, ".tree-*")
	if err != nil {
		return fmt.Errorf("write dispatch tree: %w", err)
	}
	_, werr := tmp.Write(data)
	cerr := tmp.Close()
	if werr != nil || cerr != nil {
		_ = os.Remove(tmp.Name())
		return fmt.Errorf("write dispatch tree: %w", errors.Join(werr, cerr))
	}
	return os.Rename(tmp.Name(), TreeFile(dir, id))
}

func LoadTree(dir, id string) ([]Identity, error) {
	data, err := os.ReadFile(TreeFile(dir, id))
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read dispatch tree: %w", err)
	}
	var f treeFile
	if err := json.Unmarshal(data, &f); err != nil {
		return nil, fmt.Errorf("decode dispatch tree: %w", err)
	}
	if f.Dispatch != id {
		return nil, fmt.Errorf("dispatch tree %s names dispatch %q, want %q", TreeFile(dir, id), f.Dispatch, id)
	}
	ids := make([]Identity, 0, len(f.Members))
	for _, m := range f.Members {
		ids = append(ids, Identity(m))
	}
	return ids, nil
}

func RemoveTree(dir, id string) error {
	if err := os.Remove(TreeFile(dir, id)); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("remove dispatch tree: %w", err)
	}
	return nil
}

const treeDirName = "dispatch-trees"

func TreeDir(projectRoot string) string {
	return filepath.Join(projectRoot, ".evolve", treeDirName)
}
