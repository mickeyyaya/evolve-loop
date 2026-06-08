package looppreflight

import (
	"encoding/json"
	"fmt"
	"strings"
)

// checkResultWire is the JSON shape for a CheckResult — Level renders as its
// stable lowercase string token, and an empty Detail is omitted.
type checkResultWire struct {
	Name    string `json:"name"`
	Level   string `json:"level"`
	Message string `json:"message"`
	Detail  string `json:"detail,omitempty"`
}

// MarshalJSON emits the level as a string ("pass"|"warn"|"halt") rather than
// the underlying int, so the persisted .evolve/loop-preflight.json is
// self-describing.
func (c CheckResult) MarshalJSON() ([]byte, error) {
	return json.Marshal(checkResultWire{
		Name:    c.Name,
		Level:   c.Level.String(),
		Message: c.Message,
		Detail:  c.Detail,
	})
}

// resultWire is the JSON shape for a Result; OverallLevel renders as a string
// and Checks marshal via CheckResult.MarshalJSON.
type resultWire struct {
	Checks       []CheckResult `json:"checks"`
	ChecksPassed int           `json:"checks_passed"`
	ChecksTotal  int           `json:"checks_total"`
	OverallLevel string        `json:"overall_level"`
	GeneratedAt  string        `json:"generated_at"`
}

// MarshalJSON emits the persisted readiness payload.
func (r Result) MarshalJSON() ([]byte, error) {
	return json.Marshal(resultWire{
		Checks:       r.Checks,
		ChecksPassed: r.ChecksPassed,
		ChecksTotal:  r.ChecksTotal,
		OverallLevel: r.OverallLevel.String(),
		GeneratedAt:  r.GeneratedAt,
	})
}

// PrettyJSON returns the readiness payload as 2-space-indented JSON (the bytes
// persisted to .evolve/loop-preflight.json).
func (r Result) PrettyJSON() []byte {
	b, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		// Result holds only strings/ints/[]CheckResult, so this is unreachable;
		// surface it rather than return a silent empty payload.
		return []byte(fmt.Sprintf("{\"overall_level\":\"halt\",\"error\":%q}", err.Error()))
	}
	return b
}

// Summary is the human-readable block printed to stderr before the loop starts
// (mirrors preflight.Profile.Summary). Halt/warn details are indented beneath
// their check so the operator sees exactly what to fix.
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
