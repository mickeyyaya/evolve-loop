// Package routingeval is the golden corpus of advisor responses that are replayed through the routing parse and integrity floor.
// See docs/architecture/packages/internal-routingeval.md.
package routingeval

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/router"
)

type Case struct {
	Name        string
	Input       router.RouteInput
	RawResponse string
	Floor       []string

	ExpectRunSet     []string
	ExpectClamps     []string
	ForbiddenPhases  []string
	ExpectParseError bool
}

type descriptor struct {
	IntentRequired   bool     `json:"intent_required"`
	TddPinned        *bool    `json:"tdd_pinned"`
	KnownPhases      []string `json:"known_phases"`
	Floor            []string `json:"floor"`
	ExpectRunSet     []string `json:"expect_run_set"`
	ExpectClamps     []string `json:"expect_clamps"`
	ForbiddenPhases  []string `json:"forbidden_phases"`
	ExpectParseError bool     `json:"expect_parse_error"`
}

func (d descriptor) toRouteInput() router.RouteInput {
	in := router.RouteInput{IntentRequired: d.IntentRequired}
	if d.TddPinned != nil && !*d.TddPinned {
		in.Cfg.Conditional = map[string]config.CondRule{"tdd": config.DefaultTddRule()}
		in.Signals.Triage = router.TriageSignals{CycleSize: "trivial", Present: true}
	}
	for _, p := range d.KnownPhases {
		in.Catalog = append(in.Catalog, router.PhaseCard{Name: p})
	}
	return in
}

func LoadCorpus(dir string) ([]Case, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("routingeval: read corpus dir %q: %w", dir, err)
	}
	var cases []Case
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		c, err := loadCase(filepath.Join(dir, e.Name()), e.Name())
		if err != nil {
			return nil, err
		}
		cases = append(cases, c)
	}
	if len(cases) == 0 {
		return nil, fmt.Errorf("routingeval: corpus %q has no case subdirectories", dir)
	}
	sort.Slice(cases, func(i, j int) bool { return cases[i].Name < cases[j].Name })
	return cases, nil
}

func loadCase(caseDir, name string) (Case, error) {
	raw, err := os.ReadFile(filepath.Join(caseDir, "response.txt"))
	if err != nil {
		return Case{}, fmt.Errorf("routingeval: case %q: read response.txt: %w", name, err)
	}
	buf, err := os.ReadFile(filepath.Join(caseDir, "case.json"))
	if err != nil {
		return Case{}, fmt.Errorf("routingeval: case %q: read case.json: %w", name, err)
	}
	var d descriptor
	dec := json.NewDecoder(bytes.NewReader(buf))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&d); err != nil {
		return Case{}, fmt.Errorf("routingeval: case %q: parse case.json: %w", name, err)
	}
	sort.Strings(d.ExpectRunSet)
	return Case{
		Name:             name,
		Input:            d.toRouteInput(),
		RawResponse:      string(raw),
		Floor:            d.Floor,
		ExpectRunSet:     d.ExpectRunSet,
		ExpectClamps:     d.ExpectClamps,
		ForbiddenPhases:  d.ForbiddenPhases,
		ExpectParseError: d.ExpectParseError,
	}, nil
}
