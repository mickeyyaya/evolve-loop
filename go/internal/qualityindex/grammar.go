package qualityindex

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/reportdoc"
)

const (
	ScoresSection     = "Scores"
	PlanSection       = "Review Plan"
	inputsReadKey     = "inputs read"
	notApplicable     = "N/A"
	depthAlternatives = "light|standard|deep"
)

var separators = []string{" — ", " – ", " - "}

var requiredDepth = regexp.MustCompile(`(?i)^required\s*\((` + depthAlternatives + `)\)$`)

type PlanLine struct {
	Required      bool
	Depth         string
	Justification string
}

type Plan struct {
	InputsRead string
	Lines      map[string]PlanLine
}

type entry struct {
	key, raw, value, text string
	hasSeparator          bool
}

func ParseScores(report string) (Scores, []string) {
	entries, problems := sectionEntries(report, ScoresSection)
	if entries == nil {
		return nil, problems
	}
	scores := Scores{}
	problems = append(problems, scanDimensions(ScoresSection, entries, func(e entry) string {
		s, p := parseScore(e)
		if p == "" {
			scores[e.key] = s
		}
		return p
	})...)
	return scores, problems
}

func ParsePlan(report string) (Plan, []string) {
	entries, problems := sectionEntries(report, PlanSection)
	if entries == nil {
		return Plan{}, problems
	}
	plan := Plan{Lines: map[string]PlanLine{}}
	var dimensions, inputs []entry
	for _, e := range entries {
		if e.key == inputsReadKey {
			inputs = append(inputs, e)
			continue
		}
		dimensions = append(dimensions, e)
	}
	if len(inputs) == 0 {
		problems = append(problems, "## Review Plan has no `- Inputs read: <list>` line naming what you read")
	} else if read, problem := planInputs(inputs[0]); problem != "" {
		problems = append(problems, problem)
	} else {
		plan.InputsRead = read
	}
	if len(inputs) > 1 {
		problems = append(problems, "## Review Plan has more than one `Inputs read` line")
	}
	problems = append(problems, scanDimensions(PlanSection, dimensions, func(e entry) string {
		line, p := parsePlanLine(e)
		if p == "" {
			plan.Lines[e.key] = line
		}
		return p
	})...)
	return plan, problems
}

func Agree(plan Plan, scores Scores) []string {
	var problems []string
	for _, key := range Keys() {
		line, planned := plan.Lines[key]
		score, scored := scores[key]
		if planned && scored && line.Required == score.NA {
			problems = append(problems, fmt.Sprintf("dimension %s: the Review Plan marks it %s but ## Scores marks it %s — an N/A dimension is N/A in both", key, planWord(line), scoreWord(score)))
		}
	}
	return problems
}

func sectionEntries(report, title string) ([]entry, []string) {
	body, found, err := reportdoc.Section(report, title)
	if err != nil {
		return nil, []string{err.Error()}
	}
	if !found {
		return nil, []string{fmt.Sprintf("the report has no ## %s section", title)}
	}
	entries := []entry{}
	for _, raw := range strings.Split(body, "\n") {
		line := strings.TrimSpace(raw)
		if !strings.HasPrefix(line, "- ") {
			continue
		}
		key, rest, _ := strings.Cut(strings.ReplaceAll(strings.TrimPrefix(line, "- "), "**", ""), ":")
		e := entry{key: strings.ToLower(strings.TrimSpace(key)), raw: strings.TrimSpace(rest)}
		e.value, e.text, e.hasSeparator = splitValue(e.raw)
		entries = append(entries, e)
	}
	return entries, nil
}

func splitValue(rest string) (value, text string, separated bool) {
	padded := " " + rest
	cut, sepLen := -1, 0
	for _, sep := range separators {
		if i := strings.Index(padded, sep); i >= 0 && (cut < 0 || i < cut) {
			cut, sepLen = i, len(sep)
		}
	}
	if cut < 0 {
		return strings.TrimSpace(rest), "", false
	}
	return strings.TrimSpace(padded[:cut]), strings.TrimSpace(padded[cut+sepLen:]), true
}

func scanDimensions(section string, entries []entry, parse func(entry) string) []string {
	var problems []string
	seen := map[string]bool{}
	for _, e := range entries {
		switch {
		case !Known(e.key):
			problems = append(problems, fmt.Sprintf("%q is not a quality-index dimension; the dimensions are %v", e.key, Keys()))
		case seen[e.key]:
			problems = append(problems, fmt.Sprintf("dimension %s appears more than once", e.key))
		default:
			seen[e.key] = true
			if p := parse(e); p != "" {
				problems = append(problems, p)
			}
		}
	}
	return append(problems, missingDimensions(section, seen)...)
}

func parseScore(e entry) (Score, string) {
	if !e.hasSeparator || e.text == "" {
		return Score{}, fmt.Sprintf("dimension %s has no rationale: write `- %s: <1-5 or N/A> — <why>`", e.key, e.key)
	}
	if strings.EqualFold(e.value, notApplicable) {
		if NeverNA(e.key) {
			return Score{}, fmt.Sprintf("dimension %s is never N/A for a code change: score it 1-5", e.key)
		}
		return Score{NA: true, Rationale: e.text}, ""
	}
	n, err := strconv.Atoi(e.value)
	if err != nil || !inRange(n) {
		return Score{}, fmt.Sprintf("dimension %s scores %q: a score is an integer %d-%d or N/A", e.key, e.value, minScore, maxScore)
	}
	return Score{Value: n, Rationale: e.text}, ""
}

func parsePlanLine(e entry) (PlanLine, string) {
	if !e.hasSeparator || e.text == "" {
		return PlanLine{}, fmt.Sprintf("plan line %s has no justification: write `- %s: required (%s) — <why>` or `- %s: N/A — <why>`", e.key, e.key, depthAlternatives, e.key)
	}
	if strings.EqualFold(e.value, notApplicable) {
		if NeverNA(e.key) {
			return PlanLine{}, fmt.Sprintf("plan line %s is never N/A for a code change: mark it required (%s)", e.key, depthAlternatives)
		}
		return PlanLine{Justification: e.text}, ""
	}
	m := requiredDepth.FindStringSubmatch(e.value)
	if m == nil {
		return PlanLine{}, fmt.Sprintf("plan line %s says %q: write `required (%s)` or `N/A`", e.key, e.value, depthAlternatives)
	}
	return PlanLine{Required: true, Depth: strings.ToLower(m[1]), Justification: e.text}, ""
}

func planInputs(e entry) (inputs, problem string) {
	if e.raw == "" {
		return "", "## Review Plan's `Inputs read` line is empty: name the deliverables and the diff you read"
	}
	return e.raw, ""
}

func missingDimensions(section string, seen map[string]bool) []string {
	var missing []string
	for _, key := range Keys() {
		if !seen[key] {
			missing = append(missing, key)
		}
	}
	if len(missing) == 0 {
		return nil
	}
	return []string{fmt.Sprintf("## %s is missing dimension(s) %s: every dimension appears exactly once", section, strings.Join(missing, ", "))}
}

func planWord(l PlanLine) string {
	if l.Required {
		return "required"
	}
	return notApplicable
}

func scoreWord(s Score) string {
	if s.NA {
		return notApplicable
	}
	return strconv.Itoa(s.Value)
}
