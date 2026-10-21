// Package researchindex writes the dated entries of the research index from one file per entry.
// See docs/architecture/packages/internal-researchindex.md.
package researchindex

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/skillcheck"
)

const (
	EntriesDir = "docs/research-index"
	IndexPath  = "docs/research-index.md"
	Begin      = "<!-- GENERATED:research-index BEGIN — do not edit by hand; run `evolve docs index generate` -->"
	End        = "<!-- GENERATED:research-index END -->"
	heading    = "## "
)

var ErrStale = errors.New("the research index is stale")

type entry struct {
	name, body string
}

func Generate(root string) (bool, error) {
	current, next, _, err := render(root)
	if err != nil || next == current {
		return false, err
	}
	if err := os.WriteFile(filepath.Join(root, IndexPath), []byte(next), 0o644); err != nil {
		return false, fmt.Errorf("write %s: %w", IndexPath, err)
	}
	return true, nil
}

func Check(root string) error {
	current, next, entries, err := render(root)
	if err != nil || next == current {
		return err
	}
	var missing []string
	for _, e := range entries {
		if !strings.Contains(current, e.headingLine()) {
			missing = append(missing, e.name)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("%w: %s has no index line for %s; run `evolve docs index generate`", ErrStale, IndexPath, strings.Join(missing, ", "))
	}
	return fmt.Errorf("%w: %s differs from its entries in %s; run `evolve docs index generate`", ErrStale, IndexPath, EntriesDir)
}

func render(root string) (current, next string, entries []entry, err error) {
	entries, err = readEntries(filepath.Join(root, EntriesDir))
	if err != nil {
		return "", "", nil, err
	}
	raw, err := os.ReadFile(filepath.Join(root, IndexPath))
	if err != nil {
		return "", "", nil, fmt.Errorf("read %s: %w", IndexPath, err)
	}
	next, err = skillcheck.SpliceMarkedRegion(string(raw), block(entries), Begin, End, "")
	if err != nil {
		return "", "", nil, fmt.Errorf("splice %s: %w", IndexPath, err)
	}
	return string(raw), next, entries, nil
}

func block(entries []entry) string {
	bodies := make([]string, len(entries))
	for i, e := range entries {
		bodies[i] = e.body
	}
	return Begin + "\n\n" + strings.Join(bodies, "\n") + "\n" + End
}

func readEntries(dir string) ([]entry, error) {
	names, err := filepath.Glob(filepath.Join(dir, "*.md"))
	if err != nil {
		return nil, fmt.Errorf("list the entries in %s: %w", EntriesDir, err)
	}
	if len(names) == 0 {
		return nil, fmt.Errorf("no entry file in %s", EntriesDir)
	}
	slices.Sort(names)
	entries := make([]entry, 0, len(names))
	for _, path := range names {
		raw, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read entry %s: %w", filepath.Base(path), err)
		}
		e := entry{name: filepath.Base(path), body: strings.TrimRight(string(raw), "\n") + "\n"}
		if !strings.HasPrefix(e.body, heading) {
			return nil, fmt.Errorf("entry %s must start with a %q heading line", e.name, heading)
		}
		entries = append(entries, e)
	}
	return entries, nil
}

func (e entry) headingLine() string {
	line, _, _ := strings.Cut(e.body, "\n")
	return line
}
