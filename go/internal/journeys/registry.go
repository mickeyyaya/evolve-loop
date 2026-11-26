// Package journeys holds the journey registry model: each row names a journey and its test.
// See docs/architecture/packages/internal-journeys.md.
package journeys

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"slices"
)

const RegistryPath = "go/test/journeys/journeys.json"

type Row struct {
	ID      string `json:"id"`
	Journey string `json:"journey"`
	Test    string `json:"test"`
	Issue   string `json:"issue"`
	Retired string `json:"retired,omitempty"`
}

var testRefRE = regexp.MustCompile(`^[A-Za-z0-9_-]+(/[A-Za-z0-9_-]+)*\.Test[A-Za-z0-9_]+$`)

func Load(path string) ([]Row, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	rows, err := Parse(raw)
	if err != nil {
		return nil, fmt.Errorf("%w (%s)", err, path)
	}
	return rows, nil
}

func Parse(raw []byte) ([]Row, error) {
	var rows []Row
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&rows); err != nil {
		return nil, fmt.Errorf("journeys: decode: %w", err)
	}
	seen := map[string]bool{}
	for _, r := range rows {
		if err := r.validate(); err != nil {
			return nil, err
		}
		if seen[r.ID] {
			return nil, fmt.Errorf("journeys: duplicate id %q", r.ID)
		}
		seen[r.ID] = true
	}
	return rows, nil
}

func (r Row) validate() error {
	for field, value := range map[string]string{"id": r.ID, "journey": r.Journey, "issue": r.Issue} {
		if value == "" {
			return fmt.Errorf("journeys: row %q has an empty %s", r.ID, field)
		}
	}
	if !testRefRE.MatchString(r.Test) {
		return fmt.Errorf("journeys: row %q names test %q, want <package>.Test<Name>", r.ID, r.Test)
	}
	return nil
}

func Removed(before, after []Row) []string {
	kept := map[string]Row{}
	for _, r := range after {
		kept[r.ID] = r
	}
	var gone []string
	for _, r := range before {
		now, ok := kept[r.ID]
		if !ok || (r.Retired != "" && now.Retired == "") {
			gone = append(gone, r.ID)
		}
	}
	slices.Sort(gone)
	return gone
}
