// Package supersede finds that the console took the item of a live cycle (ADR-0158 decision 13).
package supersede

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/mickeyyaya/evolve-loop/go/internal/inboxbatch"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxmover"
)

type Verdict string

const (
	None       Verdict = "none"
	Superseded Verdict = "superseded"
)

type Reading struct {
	Cycle int
	Route string
}

type Result struct {
	Verdict Verdict
	Claim   Reading
}

type Reader func(id string) (Reading, error)

func Check(read Reader, id string, cycle int) (Result, error) {
	got, err := read(id)
	if err != nil {
		return Result{}, fmt.Errorf("supersede: read %s: %w", id, err)
	}
	if inboxbatch.IsConsoleRoute(got.Route) || (got.Cycle != 0 && got.Cycle != cycle) {
		return Result{Verdict: Superseded, Claim: got}, nil
	}
	return Result{Verdict: None}, nil
}

func InboxReader(inboxDir string) Reader {
	return inboxReader(inboxDir, os.ReadFile)
}

func inboxReader(inboxDir string, readFile func(string) ([]byte, error)) Reader {
	return func(id string) (Reading, error) {
		loc, err := inboxmover.Locate(inboxDir, id)
		if err != nil {
			return Reading{}, err
		}
		body, err := readFile(loc.Path)
		if err != nil {
			return Reading{}, err
		}
		var item struct {
			Route string `json:"route"`
		}
		if err := json.Unmarshal(body, &item); err != nil {
			return Reading{}, fmt.Errorf("%s: %w", loc.Path, err)
		}
		return Reading{Cycle: loc.Cycle, Route: item.Route}, nil
	}
}
