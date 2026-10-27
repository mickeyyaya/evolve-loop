package commitgate

import (
	"encoding/json"
	"fmt"
	"strings"
)

type Attestation struct {
	TreeStateSHA string      `json:"tree_state_sha"`
	TS           string      `json:"ts"`
	ChecksPassed []string    `json:"checks_passed"`
	ReviewersRun []string    `json:"reviewers_run"`
	ReviewWaiver string      `json:"review_waiver,omitempty"`
	ReviewRuns   []ReviewRun `json:"review_runs,omitempty"`
	Tool         string      `json:"tool"`
}

type ReviewRun struct {
	RunID   string `json:"run_id"`
	Profile string `json:"profile"`
	CLI     string `json:"cli"`
	Family  string `json:"family"`
	Model   string `json:"model"`
}

type attestationField struct {
	key   string
	value any
}

func (a *Attestation) Marshal() ([]byte, error) {
	return a.marshalWith(json.Marshal)
}

func (a *Attestation) marshalWith(marshal func(any) ([]byte, error)) ([]byte, error) {
	lines := make([]string, 0, 7)
	for _, f := range a.fields() {
		value, err := marshal(f.value)
		if err != nil {
			return nil, fmt.Errorf("attestation field %s: %w", f.key, err)
		}
		lines = append(lines, fmt.Sprintf("  %q: %s", f.key, value))
	}
	return []byte("{\n" + strings.Join(lines, ",\n") + "\n}\n"), nil
}

func (a *Attestation) fields() []attestationField {
	fields := []attestationField{
		{"tree_state_sha", a.TreeStateSHA},
		{"ts", a.TS},
		{"checks_passed", orEmpty(a.ChecksPassed)},
		{"reviewers_run", orEmpty(a.ReviewersRun)},
	}
	if a.ReviewWaiver != "" {
		fields = append(fields, attestationField{"review_waiver", a.ReviewWaiver})
	}
	if len(a.ReviewRuns) > 0 {
		fields = append(fields, attestationField{"review_runs", a.ReviewRuns})
	}
	return append(fields, attestationField{"tool", a.Tool})
}

func orEmpty(items []string) []string {
	if items == nil {
		return []string{}
	}
	return items
}

func splitReviewers(csv string) []string {
	var out []string
	for _, r := range strings.Split(csv, ",") {
		if r != "" {
			out = append(out, r)
		}
	}
	return out
}
