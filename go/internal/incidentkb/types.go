// Package incidentkb parses, validates, renders, and lists Markdown incident records.
// See docs/architecture/packages/internal-incidentkb.md.
package incidentkb

const (
	Schema       = "incident-record/1"
	RelDir       = "docs/incidents"
	LegacyCutoff = "2026-11-19"
)

var RequiredSections = []string{
	"Summary",
	"Trigger and level",
	"Timeline",
	"Key logs",
	"Root cause and contributing factors",
	"Analysis",
	"Approaches taken",
	"Actions",
	"Effectiveness",
	"Lessons",
}

var generatedParts = []string{"status", "actions", "effectiveness"}

type Action struct {
	ID    string `json:"id"`
	Type  string `json:"type"`
	Owner string `json:"owner"`
	Item  string `json:"item"`
	PR    int    `json:"pr"`
	Done  string `json:"done"`
	State string `json:"state"`
}

type Filter struct {
	Field string `json:"field"`
	Op    string `json:"op"`
	Value string `json:"value"`
}

type Metric struct {
	Code    string   `json:"code"`
	Filters []Filter `json:"filters"`
	Unit    string   `json:"unit"`
	Since   string   `json:"since"`
}

type Baseline struct {
	Value int `json:"value"`
	Units int `json:"units"`
}

type CheckResult struct {
	Check    int    `json:"check"`
	Observed int    `json:"observed"`
	Verdict  string `json:"verdict"`
	At       string `json:"at"`
}

type Effectiveness struct {
	Metric   Metric        `json:"metric"`
	Baseline Baseline      `json:"baseline"`
	Target   int           `json:"target"`
	Checks   int           `json:"checks"`
	Results  []CheckResult `json:"results"`
}

type Record struct {
	Schema        string        `json:"schema"`
	ID            string        `json:"id"`
	Title         string        `json:"title"`
	Level         string        `json:"level"`
	Status        string        `json:"status"`
	SelfHeal      string        `json:"selfheal"`
	Fingerprint   string        `json:"fingerprint"`
	Signals       []string      `json:"signals"`
	PRs           []int         `json:"prs"`
	Items         []string      `json:"items"`
	Actions       []Action      `json:"actions"`
	Effectiveness Effectiveness `json:"effectiveness"`
	VerifiedAt    string        `json:"verified_at"`
	Body          string        `json:"-"`
}

type ListEntry struct {
	ID     string `json:"id"`
	Status string `json:"status"`
	Legacy bool   `json:"legacy"`
	Title  string `json:"title,omitempty"`
	Level  string `json:"level,omitempty"`
	Code   string `json:"code,omitempty"`
}

type CheckReport struct {
	Records  []string            `json:"records"`
	Legacy   []string            `json:"legacy"`
	Problems map[string][]string `json:"problems"`
}
