package dossier

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/flock"
	"github.com/mickeyyaya/evolve-loop/go/internal/atomicwrite"
)

const RecalledLessonsFile = "recalled-lessons.json"

func AppendRecalledLessons(workspace string, ids []string) error {
	path := filepath.Join(workspace, RecalledLessonsFile)
	return flock.WithPathLock(path, func() error { return appendRecalledLessons(workspace, path, ids) })
}

func appendRecalledLessons(workspace, path string, ids []string) error {
	merged := readRecalledLessons(workspace)
	for _, id := range ids {
		if id = strings.TrimSpace(id); id != "" && !slices.Contains(merged, id) {
			merged = append(merged, id)
		}
	}
	if len(merged) == 0 {
		return nil
	}
	return atomicwrite.JSON(path, merged)
}

func readRecalledLessons(workspace string) []string {
	body, err := os.ReadFile(filepath.Join(workspace, RecalledLessonsFile))
	if err != nil {
		return nil
	}
	var ids []string
	if json.Unmarshal(body, &ids) != nil {
		return nil
	}
	return ids
}
