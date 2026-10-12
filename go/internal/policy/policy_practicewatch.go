package policy

import (
	"fmt"
	"strings"
)

const practiceWatchMaxTimeoutS = 120

type PracticeWatchSource struct {
	ID      string `json:"id"`
	Family  string `json:"family,omitempty"`
	Kind    string `json:"kind,omitempty"`
	Locator string `json:"locator,omitempty"`
	Enabled *bool  `json:"enabled,omitempty"`
}

type PracticeWatchPolicy struct {
	TimeoutS *int                  `json:"timeout_s,omitempty"`
	Sources  []PracticeWatchSource `json:"sources,omitempty"`
}

func (p Policy) PracticeWatchConfig() (PracticeWatchPolicy, error) {
	timeoutS := 20
	out := PracticeWatchPolicy{TimeoutS: &timeoutS}
	c := p.PracticeWatch
	if c == nil {
		return out, nil
	}
	if c.TimeoutS != nil {
		if *c.TimeoutS <= 0 {
			return PracticeWatchPolicy{}, fmt.Errorf("policy: practice_watch.timeout_s must be > 0, got %d", *c.TimeoutS)
		}
		if *c.TimeoutS > practiceWatchMaxTimeoutS {
			return PracticeWatchPolicy{}, fmt.Errorf("policy: practice_watch.timeout_s must be at most %d, got %d", practiceWatchMaxTimeoutS, *c.TimeoutS)
		}
		out.TimeoutS = c.TimeoutS
	}
	seen := map[string]bool{}
	for _, s := range c.Sources {
		id := strings.TrimSpace(s.ID)
		switch {
		case id == "":
			return PracticeWatchPolicy{}, fmt.Errorf("policy: practice_watch.sources: id must not be blank")
		case seen[id]:
			return PracticeWatchPolicy{}, fmt.Errorf("policy: practice_watch.sources: duplicate source id %q", id)
		}
		seen[id] = true
	}
	out.Sources = c.Sources
	return out, nil
}

func (p *PracticeWatchPolicy) UnmarshalJSON(raw []byte) error {
	type block PracticeWatchPolicy
	var decoded block
	if err := decodeStrict(raw, &decoded); err != nil {
		return fmt.Errorf("practice_watch: %w", err)
	}
	*p = PracticeWatchPolicy(decoded)
	return nil
}
