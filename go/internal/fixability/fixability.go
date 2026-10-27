package fixability

import (
	"regexp"
	"slices"
	"strings"
)

const (
	ReasonHandoffBlocker = "handoff_blocker"

	SignalCode    = "REPAIR_HANDOFF"
	FieldDecision = "decision"
	FieldReason   = "reason"
	FieldBlockers = "blockers"
	DecisionNone  = "none"
	DecisionWould = "would_stop"
	DecisionStop  = "stop"

	BlocksGate     = "gate"
	BlocksSeverity = "severity"
	BlocksRedTest  = "red_test"
)

var (
	findingID    = regexp.MustCompile(`^\s*([CHML])(\d+):`)
	citedPath    = regexp.MustCompile(`(?:^|[^A-Za-z0-9_./-])((?:go|docs|skills|agents|schemas|\.evolve)/[A-Za-z0-9_./-]*[A-Za-z0-9_])`)
	testName     = regexp.MustCompile(`\bTest[A-Za-z0-9_]+`)
	provenRed    = regexp.MustCompile(`(?i)\b(?:tests?|predicates?)(?:\s+\(\d+\))?\s+(?:fail|fails|failed)\b`)
	idRune       = regexp.MustCompile(`^[a-z0-9-]`)
	severityName = map[string]string{"C": "CRITICAL", "H": "HIGH", "M": "MEDIUM", "L": "LOW"}
)

type Input struct {
	Defects          []string
	GateFailures     []string
	OpenConsoleItems []string
	Protected        func(path string) bool
	Salvage          string
}

type Finding struct {
	ID          string   `json:"id"`
	Severity    string   `json:"severity"`
	Text        string   `json:"text"`
	Paths       []string `json:"paths,omitempty"`
	ConsoleItem string   `json:"console_item,omitempty"`
	Protected   []string `json:"protected,omitempty"`
	Blocks      string   `json:"blocks,omitempty"`
}

func (f Finding) Handoff() bool { return f.ConsoleItem != "" || len(f.Protected) > 0 }

type Decision struct {
	Stop         bool      `json:"stop"`
	Reason       string    `json:"reason,omitempty"`
	Lane         []Finding `json:"lane,omitempty"`
	Handoff      []Finding `json:"handoff,omitempty"`
	Blockers     []Finding `json:"blockers,omitempty"`
	ConsoleItems []string  `json:"console_items,omitempty"`
	Salvage      string    `json:"salvage,omitempty"`
}

func Classify(in Input) Decision {
	d := Decision{Salvage: in.Salvage}
	for _, text := range in.Defects {
		f := in.judge(parseFinding(text))
		if !f.Handoff() {
			d.Lane = append(d.Lane, f)
			continue
		}
		d.Handoff = append(d.Handoff, f)
		if f.ConsoleItem != "" && !slices.Contains(d.ConsoleItems, f.ConsoleItem) {
			d.ConsoleItems = append(d.ConsoleItems, f.ConsoleItem)
		}
		if f.Blocks != "" {
			d.Blockers = append(d.Blockers, f)
		}
	}
	if len(d.Blockers) > 0 {
		d.Stop, d.Reason = true, ReasonHandoffBlocker
	}
	return d
}

func parseFinding(text string) Finding {
	f := Finding{Text: text}
	if m := findingID.FindStringSubmatch(text); m != nil {
		f.ID, f.Severity = m[1]+m[2], severityName[m[1]]
	}
	for _, m := range citedPath.FindAllStringSubmatch(text, -1) {
		if p := strings.TrimSuffix(m[1], "."); !slices.Contains(f.Paths, p) {
			f.Paths = append(f.Paths, p)
		}
	}
	return f
}

func (in Input) judge(f Finding) Finding {
	for _, id := range in.OpenConsoleItems {
		if namesItem(f.Text, id) {
			f.ConsoleItem = id
			break
		}
	}
	for _, p := range f.Paths {
		if in.Protected != nil && in.Protected(p) {
			f.Protected = append(f.Protected, p)
		}
	}
	f.Blocks = in.blocks(f)
	return f
}

func (in Input) blocks(f Finding) string {
	switch {
	case in.gateNames(f):
		return BlocksGate
	case f.Severity == "CRITICAL" || f.Severity == "HIGH":
		return BlocksSeverity
	case provenRed.MatchString(f.Text):
		return BlocksRedTest
	}
	return ""
}

func (in Input) gateNames(f Finding) bool {
	tests := testName.FindAllString(f.Text, -1)
	for _, gate := range in.GateFailures {
		for _, p := range f.Paths {
			if strings.Contains(gate, p) {
				return true
			}
		}
		for _, name := range tests {
			if strings.Contains(gate, name) {
				return true
			}
		}
	}
	return false
}

func namesItem(text, id string) bool {
	for start := 0; ; {
		i := strings.Index(text[start:], id)
		if i < 0 {
			return false
		}
		at := start + i
		end := at + len(id)
		if (at == 0 || !idRune.MatchString(text[at-1:at])) && !idRune.MatchString(text[end:]) {
			return true
		}
		start = at + 1
	}
}
