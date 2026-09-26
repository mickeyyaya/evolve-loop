package looppreflight

import (
	"encoding/json"
	"fmt"
	"strings"
)

type checkResultWire struct {
	Name    string `json:"name"`
	Level   string `json:"level"`
	Message string `json:"message"`
	Detail  string `json:"detail,omitempty"`
}

// MarshalJSON renders Level as its string token so loop-preflight.json is self-describing.
func (c CheckResult) MarshalJSON() ([]byte, error) {
	return json.Marshal(checkResultWire{
		Name:    c.Name,
		Level:   c.Level.String(),
		Message: c.Message,
		Detail:  c.Detail,
	})
}

type resultWire struct {
	Checks       []CheckResult     `json:"checks"`
	ChecksPassed int               `json:"checks_passed"`
	ChecksTotal  int               `json:"checks_total"`
	OverallLevel string            `json:"overall_level"`
	GeneratedAt  string            `json:"generated_at"`
	CLIVersions  map[string]string `json:"cli_versions,omitempty"`
}

// MarshalJSON renders OverallLevel as its string token, as persisted in loop-preflight.json.
func (r Result) MarshalJSON() ([]byte, error) {
	return json.Marshal(resultWire{
		Checks:       r.Checks,
		ChecksPassed: r.ChecksPassed,
		ChecksTotal:  r.ChecksTotal,
		OverallLevel: r.OverallLevel.String(),
		GeneratedAt:  r.GeneratedAt,
		CLIVersions:  r.CLIVersions,
	})
}

// PrettyJSON returns the indented payload persisted to .evolve/loop-preflight.json.
func (r Result) PrettyJSON() []byte {
	b, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		// Unreachable for these field types; surfaced rather than returned as an empty payload.
		return []byte(fmt.Sprintf("{\"overall_level\":\"halt\",\"error\":%q}", err.Error()))
	}
	return b
}

// Summary is the human-readable block printed to stderr before the loop starts.
func (r Result) Summary() string {
	var b strings.Builder
	fmt.Fprintf(&b, "Loop readiness: %s (%d/%d checks passed)\n",
		strings.ToUpper(r.OverallLevel.String()), r.ChecksPassed, r.ChecksTotal)
	for _, c := range r.Checks {
		fmt.Fprintf(&b, "  [%s] %s: %s\n", c.Level, c.Name, c.Message)
		if c.Detail != "" {
			for _, line := range strings.Split(c.Detail, "\n") {
				fmt.Fprintf(&b, "      %s\n", line)
			}
		}
	}
	return b.String()
}
