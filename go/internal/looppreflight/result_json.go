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

func (r Result) PrettyJSON() []byte {
	b, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return []byte(fmt.Sprintf("{\"overall_level\":\"halt\",\"error\":%q}", err.Error()))
	}
	return b
}

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
