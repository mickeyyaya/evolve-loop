package selfheal

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/flock"
	"github.com/mickeyyaya/evolve-loop/go/internal/atomicwrite"
)

const dirName = "incidents"

func Dir(evolveDir string) string { return filepath.Join(evolveDir, dirName) }

func WriteIncident(dir string, inc Incident) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("incident %s: %w", inc.ID, err)
	}
	return atomicwrite.JSON(filepath.Join(dir, inc.ID+".json"), inc)
}

func UpdateIncidents(dir string, update func([]Incident) ([]Incident, error)) error {
	return flock.WithPathLock(dir, func() error {
		existing, err := LoadIncidents(dir)
		if err != nil {
			return err
		}
		next, err := update(existing)
		if err != nil {
			return err
		}
		return writeChanged(dir, existing, next)
	})
}

func writeChanged(dir string, before, after []Incident) error {
	old := map[string]Incident{}
	for _, inc := range before {
		old[inc.ID] = inc
	}
	for _, inc := range after {
		if prev, ok := old[inc.ID]; ok && reflect.DeepEqual(prev, inc) {
			continue
		}
		if err := WriteIncident(dir, inc); err != nil {
			return err
		}
	}
	return nil
}

func LoadIncidents(dir string) ([]Incident, error) {
	entries, err := os.ReadDir(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("incidents: %w", err)
	}
	var out []Incident
	owners := map[string]string{}
	for _, e := range entries {
		if e.IsDir() || !strings.HasPrefix(e.Name(), "inc-") || filepath.Ext(e.Name()) != ".json" {
			continue
		}
		inc, err := loadIncident(filepath.Join(dir, e.Name()))
		if err != nil {
			return nil, err
		}
		if other, dup := owners[inc.Fingerprint]; dup {
			return nil, fmt.Errorf("incidents: %s and %s have one fingerprint %q", other, inc.ID, inc.Fingerprint)
		}
		owners[inc.Fingerprint] = inc.ID
		out = append(out, inc)
	}
	slices.SortFunc(out, func(a, b Incident) int { return strings.Compare(a.ID, b.ID) })
	return out, nil
}

func loadIncident(path string) (Incident, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return Incident{}, fmt.Errorf("incident: %w", err)
	}
	var inc Incident
	if err := json.Unmarshal(raw, &inc); err != nil {
		return Incident{}, fmt.Errorf("incident %s: %w", path, err)
	}
	return inc, nil
}
