package inputindex

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/atomicwrite"
)

const (
	Schema            = "evolve.inputs.index/1"
	inputsDir         = "inputs"
	indexFile         = "index.json"
	DefaultLineageMax = 5
)

const (
	ClassDeliverable = "deliverable"
	ClassLineage     = "lineage"
)

type Entry struct {
	ID      string `json:"id"`
	Class   string `json:"class"`
	Kind    string `json:"kind"`
	Path    string `json:"path"`
	Cycle   int    `json:"cycle"`
	Phase   string `json:"phase,omitempty"`
	Attempt int    `json:"attempt"`
	Summary string `json:"summary"`
	Bytes   int64  `json:"bytes"`
	SHA256  string `json:"sha256"`
}

type Index struct {
	Schema  string   `json:"schema"`
	Cycle   int      `json:"cycle"`
	Phase   string   `json:"phase"`
	ItemIDs []string `json:"item_ids"`
	Attempt int      `json:"attempt"`
	Lineage []int    `json:"lineage"`
	Entries []Entry  `json:"entries"`
	Notes   []string `json:"notes"`
}

func Path(workspace string) string {
	return filepath.Join(workspace, inputsDir, indexFile)
}

func Write(workspace string, idx Index) error {
	path := Path(workspace)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("inputindex: make %s: %w", filepath.Dir(path), err)
	}
	return atomicwrite.JSON(path, idx)
}

func Read(workspace string) (Index, error) {
	raw, err := os.ReadFile(Path(workspace))
	if err != nil {
		return Index{}, err
	}
	var idx Index
	if err := json.Unmarshal(raw, &idx); err != nil {
		return Index{}, fmt.Errorf("inputindex: %s: %w", Path(workspace), err)
	}
	if idx.Schema != Schema {
		return Index{}, fmt.Errorf("inputindex: %s has schema %q, want %q", Path(workspace), idx.Schema, Schema)
	}
	return idx, nil
}

func ReadablePaths(projectRoot, workspace string) ([]string, error) {
	idx, err := Read(workspace)
	if err != nil {
		return nil, err
	}
	out := []string{Path(workspace)}
	for _, e := range idx.Entries {
		if abs, ok := underRoot(projectRoot, e.Path); ok {
			out = append(out, abs)
		}
	}
	return out, nil
}

func underRoot(projectRoot, rel string) (string, bool) {
	clean := filepath.Clean(filepath.FromSlash(rel))
	if rel == "" || filepath.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", false
	}
	return filepath.Join(projectRoot, clean), true
}
