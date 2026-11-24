package auditfacts

import (
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/covchanged"
)

const (
	RegressionHeading = "## Final Diff Regression Review"
	HonestyHeading    = "## Guard Honesty Check"
	FactsHeading      = "## Gate and Floor Facts"
)

const (
	listCap       = 60
	failLineCap   = 10
	failRuneCap   = 300
	notRecorded   = "not recorded"
	factsPreamble = "The host derived these facts from its own records. They are fixed. Do not argue them. Do not run these gates again.\n\n"
)

const Rubric = "Two sections are mandatory in the report, for every audit round:\n\n" +
	"- `" + RegressionHeading + "`: one line for each entry of `changed_unprotected_functions`: `path:Func - regression found or none, evidence file:line`. Judge the final diff, not the build report. Look for a changed function that breaks a caller, a guard or a test that the diff does not show.\n" +
	"- `" + HonestyHeading + "`: one line for each entry of `changed_tests`, and one for each change that makes a guard or a test pass: `path:Test - honest or hollow, why`. A test with an empty body, no assertion or a weakened guard is hollow.\n\n" +
	"The gate facts above settle what the gates decided. Do not spend a finding on them.\n"

type ACS struct {
	Round        int
	RedCount     int
	ShipEligible *bool
	RedIDs       []string
}

type Handoff struct {
	Pair    string
	Outcome string
}

type Inputs struct {
	Round       int
	GateNames   []string
	ACS         *ACS
	Handoffs    []Handoff
	FailReasons []string
	Diff        []byte
	Untracked   []string
	DiffKnown   bool
	ReadFile    func(rel string) ([]byte, error)
	Protected   func(path string) bool
}

func Block(in Inputs) string {
	var b strings.Builder
	b.WriteString("\n" + FactsHeading + "\n\n" + factsPreamble)
	fmt.Fprintf(&b, "- round: %d\n", in.Round)
	fmt.Fprintf(&b, "- gates_the_host_decides_after_your_report: %s\n", strings.Join(in.GateNames, ", "))
	b.WriteString(acsLine(in.ACS))
	for _, h := range in.Handoffs {
		fmt.Fprintf(&b, "- handoff_%s: outcome=%s\n", h.Pair, h.Outcome)
	}
	for _, r := range firstN(in.FailReasons, failLineCap) {
		fmt.Fprintf(&b, "- audit_gate_fail_untrusted_json: %q\n", truncateRunes(r, failRuneCap))
	}
	funcs, tests, known := changed(in)
	b.WriteString(listLine("changed_unprotected_functions", funcs, known))
	b.WriteString(listLine("changed_tests", tests, known))
	b.WriteString("\n## Final Diff Review Sections (mandatory)\n\n" + Rubric)
	return b.String()
}

func acsLine(a *ACS) string {
	if a == nil {
		return "- acs_verdict: " + notRecorded + "\n"
	}
	ship := "unknown"
	if a.ShipEligible != nil {
		ship = fmt.Sprint(*a.ShipEligible)
	}
	return fmt.Sprintf("- acs_verdict_round%d: red_count=%d ship_eligible=%s red=[%s]\n", a.Round, a.RedCount, ship, strings.Join(a.RedIDs, " "))
}

func listLine(name string, items []string, known bool) string {
	if !known {
		return "- " + name + ": " + notRecorded + "\n"
	}
	shown := firstN(items, listCap)
	line := strings.Join(shown, " ")
	if extra := len(items) - len(shown); extra > 0 {
		line += fmt.Sprintf(" +%d more", extra)
	}
	if line == "" {
		line = "none"
	}
	return "- " + name + ": " + line + "\n"
}

func firstN(items []string, n int) []string {
	if len(items) > n {
		return items[:n]
	}
	return items
}

func truncateRunes(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n])
}

func changed(in Inputs) (funcs, tests []string, known bool) {
	if !in.DiffKnown {
		return nil, nil, false
	}
	diff, err := covchanged.ParseDiff(in.Diff)
	if err != nil {
		return nil, nil, false
	}
	lines := map[string][]int{}
	for file, added := range diff.Added {
		lines[file] = added
	}
	whole := map[string]bool{}
	for _, file := range in.Untracked {
		lines[file] = nil
		whole[file] = true
	}
	files := make([]string, 0, len(lines))
	for file := range lines {
		files = append(files, file)
	}
	slices.Sort(files)
	for _, file := range files {
		names, ok := namesIn(in, file, lines[file], whole[file])
		if !ok {
			return nil, nil, false
		}
		for _, name := range names {
			if strings.HasSuffix(file, "_test.go") {
				if isTestFunc(name) {
					tests = append(tests, file+":"+name)
				}
				continue
			}
			funcs = append(funcs, file+":"+name)
		}
	}
	return funcs, tests, true
}

func namesIn(in Inputs, file string, lines []int, whole bool) ([]string, bool) {
	if !strings.HasSuffix(file, ".go") || in.Protected(file) {
		return nil, true
	}
	data, err := in.ReadFile(file)
	if err != nil {
		return nil, false
	}
	if !whole {
		names, err := covchanged.ChangedFuncNames(file, data, lines)
		return names, err == nil
	}
	declared, err := covchanged.DeclaredFuncs(file, data)
	return slices.Sorted(maps.Keys(declared)), err == nil
}

func isTestFunc(name string) bool {
	for _, prefix := range []string{"Test", "Benchmark", "Fuzz"} {
		if strings.HasPrefix(name, prefix) {
			return true
		}
	}
	return false
}
