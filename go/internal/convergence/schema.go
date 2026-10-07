package convergence

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"slices"

	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

func Parse(data []byte) (Input, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	var in Input
	if err := dec.Decode(&in); err != nil {
		return Input{}, fmt.Errorf("rounds: %w", err)
	}
	if err := dec.Decode(new(json.RawMessage)); !errors.Is(err, io.EOF) {
		return Input{}, errors.New("rounds: trailing data after the document")
	}
	return in, nil
}

func (in Input) Validate() error {
	if _, ok := rulesByLoop[in.Loop]; !ok {
		return fmt.Errorf("loop %q is not one of %v", in.Loop, slices.Sorted(maps.Keys(rulesByLoop)))
	}
	if len(in.Rounds) == 0 {
		return errors.New("no judgment: rounds needs at least J_0")
	}
	if in.Round != len(in.Rounds)-1 {
		return fmt.Errorf("round %d does not match the %d judgments J_0..J_%d", in.Round, len(in.Rounds), len(in.Rounds)-1)
	}
	for i, j := range in.Rounds {
		if err := j.validate(i); err != nil {
			return err
		}
	}
	return validateConfig(in.Config)
}

func (j Judgment) validate(i int) error {
	if j.Index != i {
		return fmt.Errorf("rounds[%d]: index %d, want %d", i, j.Index, i)
	}
	seen := map[string]bool{}
	for _, f := range j.Findings {
		if err := f.validate(); err != nil {
			return fmt.Errorf("rounds[%d]: %w", i, err)
		}
		if seen[f.ID] {
			return fmt.Errorf("rounds[%d]: finding id %q is listed twice", i, f.ID)
		}
		seen[f.ID] = true
	}
	for _, h := range j.FixHunks {
		if h.File == "" || h.From < 1 || h.To < h.From {
			return fmt.Errorf("rounds[%d]: fix hunk %+v needs a file and 1 <= from <= to", i, h)
		}
	}
	return nil
}

func (f Finding) validate() error {
	switch {
	case f.ID == "":
		return errors.New("a finding id is empty")
	case severityRank[f.Severity] == 0:
		return fmt.Errorf("finding %q: severity %q is not one of CRITICAL, HIGH, MEDIUM, LOW, INFO", f.ID, f.Severity)
	case !slices.Contains(statuses, f.Status):
		return fmt.Errorf("finding %q: status %q is not one of %v", f.ID, f.Status, statuses)
	case f.Kind != "" && !slices.Contains(kinds, f.Kind):
		return fmt.Errorf("finding %q: kind %q is not one of %v", f.ID, f.Kind, kinds)
	case f.Class != "" && !slices.Contains(knownClasses, f.Class):
		return fmt.Errorf("finding %q: class %q is not one of %v", f.ID, f.Class, knownClasses)
	case f.Falsification != "" && !slices.Contains(falsifications, f.Falsification):
		return fmt.Errorf("finding %q: falsification %q is not one of %v", f.ID, f.Falsification, falsifications)
	case f.Severity == SeverityCritical && (f.Status == StatusDeferred || f.Status == StatusFiled):
		return fmt.Errorf("finding %q: a CRITICAL is never %s", f.ID, f.Status)
	}
	return nil
}

func validateConfig(c policy.ConvergenceConfig) error {
	if err := c.Validate(); err != nil {
		return fmt.Errorf("config is not resolved (%w): build it with policy.Policy.ConvergenceConfig", err)
	}
	return nil
}
