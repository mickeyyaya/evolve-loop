package selfheal

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/wave"
)

type TreeVerifier struct {
	Root      string
	MainGreen bool
	Waves     []wave.Record
}

func (v TreeVerifier) ReproGreen(inc Incident) bool {
	path, name, ok := strings.Cut(inc.ReproTest, ":")
	if !v.MainGreen || !ok || name == "" {
		return false
	}
	raw, err := os.ReadFile(filepath.Join(v.Root, filepath.FromSlash(path)))
	return err == nil && strings.Contains(string(raw), "func "+name+"(")
}

func (v TreeVerifier) WaveEndedAfter(t time.Time) bool {
	for _, w := range v.Waves {
		if w.StartedAt.After(t) && w.EndedAt != nil {
			return true
		}
	}
	return false
}

func (v TreeVerifier) RetrospectiveExists(id string) bool {
	found := false
	_ = filepath.WalkDir(filepath.Join(v.Root, "docs", "retrospectives"), func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		raw, readErr := os.ReadFile(path)
		found = readErr == nil && strings.Contains(string(raw), id)
		if found {
			return fs.SkipAll
		}
		return nil
	})
	return found
}
