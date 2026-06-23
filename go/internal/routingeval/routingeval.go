// Package routingeval is the golden routing-eval corpus (ADR-0052 WS4): a
// curated set of captured advisor responses, each replayed through the SAME
// parse + integrity-floor clamp the live planning path runs
// (core.ReplayPlanFromResponse, WS3-S5) and asserted against an expected
// run-set, the clamps that must fire, and the phases that must NEVER run. It
// locks the parse + floor against the real entry point, so a regression there
// (a parse change, a floor weakening) breaks a corpus case rather than silently
// shipping an unsafe plan.
//
// Leaf package: imports router + config + stdlib only. The replay assertion
// (which calls core) lives in the test, so routingeval itself never imports
// core — same isolation rule routingtest documents. Nothing in production
// imports it; only *_test.go consumers do.
package routingeval

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/mickeyyaya/evolveloop/go/internal/config"
	"github.com/mickeyyaya/evolveloop/go/internal/router"
)

// Case is one golden routing scenario: a captured advisor response replayed
// through core.ReplayPlanFromResponse and asserted against expectations. A
// nil/empty expectation field is NOT asserted (the opt-in convention from
// routingtest.ExpectSpec), so a case pins only what it specifies.
type Case struct {
	Name        string            // case subdir name (subtest id; corpus is name-sorted)
	Input       router.RouteInput // realized from the descriptor; threaded into Replay
	RawResponse string            // verbatim response.txt — the advisor output to replay
	Floor       []string          // ship floor; empty ⇒ router.DefaultShipFloor()

	ExpectRunSet     []string // sorted phases with Run==true after parse+clamp
	ExpectClamps     []string // Clamp.Rule names that MUST fire
	ForbiddenPhases  []string // phases that MUST NOT be Run==true (the safety assertion)
	ExpectParseError bool     // true ⇒ RawResponse must fail to parse (negative case)
}

// descriptor is the on-disk case.json: a MINIMAL, intent-revealing input
// instead of a full router.RouteInput dump (RouteInput has no JSON tags and
// embeds heavy nested types). The only floor-relevant inputs are IntentRequired
// and tddPinned(in); TddPinned captures the latter's essence (the cycle_size
// specifics matter only as trivial-vs-not). LoadCorpus realizes it into a
// router.RouteInput.
type descriptor struct {
	IntentRequired   bool     `json:"intent_required"`
	TddPinned        *bool    `json:"tdd_pinned"` // nil ⇒ pinned (floor's default-mandatory side)
	Floor            []string `json:"floor"`
	ExpectRunSet     []string `json:"expect_run_set"`
	ExpectClamps     []string `json:"expect_clamps"`
	ForbiddenPhases  []string `json:"forbidden_phases"`
	ExpectParseError bool     `json:"expect_parse_error"`
}

// toRouteInput realizes the descriptor into the minimal router.RouteInput whose
// IntentRequired + tddPinned(in) drive the floor clamp. When tdd is NOT pinned
// it installs the conventional `cycle_size != trivial` conditional rule plus a
// trivial Triage signal, so tddPinned(in) evaluates false (the trivial
// exemption); otherwise the absent rule leaves tddPinned at its mandatory
// default (floor.go: absent rule ⇒ pinned).
func (d descriptor) toRouteInput() router.RouteInput {
	in := router.RouteInput{IntentRequired: d.IntentRequired}
	if d.TddPinned != nil && !*d.TddPinned {
		in.Cfg.Conditional = map[string]config.CondRule{
			"tdd": {Field: "cycle_size", Op: "ne", Value: "trivial"},
		}
		in.Signals.Triage = router.TriageSignals{CycleSize: "trivial", Present: true}
	}
	return in
}

// LoadCorpus reads every case subdirectory under dir, returning them
// name-sorted for deterministic subtest order. Each subdir must hold
// response.txt (the verbatim advisor response) and case.json (the descriptor +
// expectations). A missing file, malformed JSON, or unreadable dir is a LOUD
// error — never a skipped case — because detecting exactly that corruption is
// the point (Core Rule 12). An empty corpus is also an error: a no-op corpus
// silently locks nothing.
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
	dec.DisallowUnknownFields() // a typo'd expectation key is a loud authoring error, not a silent miss
	if err := dec.Decode(&d); err != nil {
		return Case{}, fmt.Errorf("routingeval: case %q: parse case.json: %w", name, err)
	}
	// The run-set is a SET; the replay assertion compares it sorted (runSetOf
	// sorts the actual). Normalize the expected here so a case author need not
	// pre-sort expect_run_set — an unsorted entry would otherwise be a spurious
	// failure (or, worse, a false pass against a mis-sorted output).
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
