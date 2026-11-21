package handoff

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
)

const Unattributed = "unattributed"

var ErrBadVerdicts = errors.New("the judge verdicts do not match the asks")

type Seat struct {
	CLI  string `json:"cli"`
	Tier string `json:"tier"`
}

type Verdict struct {
	Ask    string `json:"ask"`
	Agrees bool   `json:"agrees"`
	Note   string `json:"note"`
}

type Basis struct {
	AuthorModel   string   `json:"author_model"`
	SeatModel     string   `json:"seat_model"`
	AuthorVendors []string `json:"author_vendors"`
	SeatVendors   []string `json:"seat_vendors"`
	TierCeiling   string   `json:"tier_ceiling"`
}

type Judgement struct {
	Seat     Seat      `json:"seat"`
	Verdicts []Verdict `json:"verdicts,omitempty"`
	Refused  string    `json:"refused,omitempty"`
	Error    string    `json:"error,omitempty"`
	Basis    Basis     `json:"basis"`
}

func (j Judgement) Agrees(asks []string) bool {
	return len(j.Disagreements(asks)) == 0
}

func (j Judgement) Disagreements(asks []string) []string {
	var out []string
	for _, ask := range asks {
		idx := slices.IndexFunc(j.Verdicts, func(v Verdict) bool { return v.Ask == ask })
		switch {
		case idx < 0:
			out = append(out, ask+": no verdict")
		case !j.Verdicts[idx].Agrees:
			out = append(out, ask+": "+j.Verdicts[idx].Note)
		}
	}
	return out
}

func (j Judgement) Outcome(asks []string) string {
	switch {
	case j.Refused != "":
		return "refused"
	case j.Error != "":
		return "error"
	case j.Agrees(asks):
		return "agree"
	}
	return "disagree"
}

func CheckVerdicts(asks []string, raw []byte) ([]Verdict, error) {
	var reply struct {
		Verdicts *[]json.RawMessage `json:"verdicts"`
	}
	if err := json.Unmarshal(raw, &reply); err != nil {
		return nil, fmt.Errorf("%w: invalid reply: %v", ErrBadVerdicts, err)
	}
	if reply.Verdicts == nil {
		return nil, fmt.Errorf("%w: the reply has no verdicts key", ErrBadVerdicts)
	}
	verdicts := make([]Verdict, 0, len(*reply.Verdicts))
	for _, item := range *reply.Verdicts {
		var v Verdict
		dec := json.NewDecoder(bytes.NewReader(item))
		dec.DisallowUnknownFields()
		if err := dec.Decode(&v); err != nil {
			return nil, fmt.Errorf("%w: invalid verdict: %v", ErrBadVerdicts, err)
		}
		verdicts = append(verdicts, v)
	}
	if err := checkAskCoverage(asks, verdicts); err != nil {
		return nil, err
	}
	return verdicts, nil
}

func checkAskCoverage(asks []string, verdicts []Verdict) error {
	seen := make(map[string]bool, len(verdicts))
	for _, v := range verdicts {
		switch {
		case !slices.Contains(asks, v.Ask):
			return fmt.Errorf("%w: extra ask %q", ErrBadVerdicts, v.Ask)
		case seen[v.Ask]:
			return fmt.Errorf("%w: duplicate ask %q", ErrBadVerdicts, v.Ask)
		}
		seen[v.Ask] = true
	}
	for _, ask := range asks {
		if !seen[ask] {
			return fmt.Errorf("%w: missing ask %q", ErrBadVerdicts, ask)
		}
	}
	return nil
}
