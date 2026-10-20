package mergewindow

import (
	"fmt"
	"regexp"
	"slices"
	"strings"
)

type Class string

const (
	Window   Class = "window"
	Boundary Class = "boundary"
)

const (
	ciOnlyPrefix  = ".github/"
	pathLoopStart = "while IFS= read -r -d '' path; do"
	pathLoopEnd   = "done <"
	changesJob    = "changes"
	jobIndent     = "  "
	blockStart    = `case "$path" in`
	blockEnd      = "esac"
	docsFlag      = "docs_found"
	otherFlag     = "other_found"
	goFlag        = "go_required"
	landingFlag   = "landing_required"
	inboxFlag     = "inbox_found"
)

var (
	armLine    = regexp.MustCompile(`^(\S+?)\)\s*(.*?)\s*;;$`)
	actionWord = regexp.MustCompile(`^([a-z_]+)=true$`)
	flagAssign = regexp.MustCompile(`\b(\w+_(?:required|found))=(\S*)`)
	preScript  = []string{
		`runs-on: ubuntu-latest`,
		`outputs:`,
		`go: ${{ steps.paths.outputs.go }}`,
		`landing: ${{ steps.paths.outputs.landing }}`,
		`docs_only: ${{ steps.paths.outputs.docs_only }}`,
		`docs: ${{ steps.paths.outputs.docs }}`,
		`inbox: ${{ steps.paths.outputs.inbox }}`,
		`steps:`,
		`- uses: actions/checkout@v4`,
		`with:`,
		`fetch-depth: 0`,
		`- id: paths`,
		`name: Select required suites`,
		`env:`,
		`EVENT_NAME: ${{ github.event_name }}`,
		`BASE_SHA: ${{ github.event.pull_request.base.sha || github.event.before }}`,
		`HEAD_SHA: ${{ github.event.pull_request.head.sha || github.sha }}`,
		`shell: bash`,
		`run: |`,
		`route() {`,
		`printf 'go=%s\nlanding=%s\ndocs_only=%s\ndocs=%s\ninbox=%s\n' "$1" "$2" "${3:-false}" "${4:-false}" "${5:-false}" >> "$GITHUB_OUTPUT"`,
		`exit`,
		`}`,
		`case "$EVENT_NAME" in`,
		`workflow_dispatch) route true true false false true ;;`,
		`push|pull_request) ;;`,
		`*) echo "unsupported CI event: $EVENT_NAME" >&2; exit 1 ;;`,
		`esac`,
		`if [[ ! "$BASE_SHA" =~ ^[0-9a-f]{40}$ || ! "$HEAD_SHA" =~ ^[0-9a-f]{40}$ ]]; then`,
		`echo 'CI routing requires valid base and head commit IDs' >&2`,
		`exit 1`,
		`fi`,
		`if [[ "$EVENT_NAME" == push && "$BASE_SHA" == 0000000000000000000000000000000000000000 ]]; then`,
		`route true true false false true`,
		`fi`,
		`base="$BASE_SHA"`,
		`if [[ "$EVENT_NAME" == pull_request ]] && ! base=$(git merge-base "$BASE_SHA" "$HEAD_SHA"); then`,
		`echo 'unable to determine the PR merge-base' >&2`,
		`exit 1`,
		`fi`,
		`if ! git diff --no-renames --name-only -z "$base" "$HEAD_SHA" > "$RUNNER_TEMP/ci-paths"; then`,
		`echo 'unable to determine changed paths' >&2`,
		`exit 1`,
		`fi`,
		`if [[ ! -s "$RUNNER_TEMP/ci-paths" ]]; then`,
		`route true true false false true`,
		`fi`,
		`go_required=false`,
		`landing_required=false`,
		`docs_found=false`,
		`other_found=false`,
		`inbox_found=false`,
	}
	postRoutes = []string{
		`if [[ "$EVENT_NAME" != pull_request || "$docs_found" != true ]]; then`,
		`route "$go_required" "$landing_required" false false "$inbox_found"`,
		`fi`,
		`if [[ "$other_found" == true ]]; then`,
		`route "$go_required" "$landing_required" false true "$inbox_found"`,
		`fi`,
		`route false false true true`,
	}
	suiteJobs = []struct {
		name  string
		lines []string
	}{
		{"go", []string{`needs: changes`, `if: needs.changes.outputs.go == 'true'`, `uses: ./.github/workflows/go.yml`}},
		{"landing", []string{`needs: changes`, `if: needs.changes.outputs.landing == 'true'`, `uses: ./.github/workflows/landing-validation.yml`}},
	}
	blockRules = [2]blockRule{
		{known: map[string]bool{docsFlag: true, otherFlag: true}, needs: []string{docsFlag}, catchAllSets: []string{otherFlag}},
		{known: map[string]bool{goFlag: true, landingFlag: true, inboxFlag: true}, catchAllSets: []string{goFlag, landingFlag}},
	}
)

type blockRule struct {
	known        map[string]bool
	needs        []string
	catchAllSets []string
}

type arm struct {
	patterns []*regexp.Regexp
	sets     map[string]bool
	catchAll bool
}

type Rule struct {
	classes []arm
	suites  []arm
}

type Verdict struct {
	Class         Class
	BoundaryPaths []string
}

func Parse(workflow []byte) (Rule, error) {
	lines := strings.Split(string(workflow), "\n")
	if err := checkSuiteJobs(lines); err != nil {
		return Rule{}, err
	}
	job, err := jobLines(lines, changesJob)
	if err != nil {
		return Rule{}, err
	}
	pre, body, post, err := pathLoop(job)
	if err != nil {
		return Rule{}, err
	}
	if err := checkOutsideLoop(pre, post); err != nil {
		return Rule{}, err
	}
	blocks, err := caseBlocks(body)
	if err != nil {
		return Rule{}, err
	}
	for i, block := range blocks {
		if err := blockRules[i].check(block); err != nil {
			return Rule{}, fmt.Errorf("required.yml case block %d: %w", i+1, err)
		}
	}
	return Rule{classes: blocks[0], suites: blocks[1]}, nil
}

func checkSuiteJobs(lines []string) error {
	for _, suite := range suiteJobs {
		job, err := jobLines(lines, suite.name)
		if err != nil {
			return err
		}
		if got := codeLines(job); !slices.Equal(got, suite.lines) {
			return fmt.Errorf("required.yml: the gate of job %s changed: %q", suite.name, got)
		}
	}
	return nil
}

func jobLines(lines []string, name string) ([]string, error) {
	start := slices.Index(lines, jobIndent+name+":") + 1
	if start == 0 {
		return nil, fmt.Errorf("required.yml: no job %q", name)
	}
	var job []string
	for _, line := range lines[start:] {
		if strings.HasPrefix(line, jobIndent) && !strings.HasPrefix(line, jobIndent+" ") && strings.TrimSpace(line) != "" {
			break
		}
		job = append(job, strings.TrimSpace(line))
	}
	return job, nil
}

func pathLoop(job []string) (pre, body, post []string, err error) {
	start, end, starts := -1, -1, 0
	for i, line := range job {
		switch {
		case line == pathLoopStart:
			start, starts = i, starts+1
		case start >= 0 && end < 0 && strings.HasPrefix(line, pathLoopEnd):
			end = i
		}
	}
	if starts != 1 {
		return nil, nil, nil, fmt.Errorf("required.yml: want one path loop %q, found %d", pathLoopStart, starts)
	}
	if end < 0 {
		return nil, nil, nil, fmt.Errorf("required.yml: no path loop end %q", pathLoopEnd)
	}
	return job[:start], job[start+1 : end], job[end+1:], nil
}

func checkOutsideLoop(pre, post []string) error {
	for _, line := range append(append([]string{}, pre...), post...) {
		for _, m := range flagAssign.FindAllStringSubmatch(line, -1) {
			if m[2] != "false" {
				return fmt.Errorf("required.yml: routing flag %s set outside the case tables: %q", m[1], line)
			}
		}
	}
	if got := codeLines(pre); !slices.Equal(got, preScript) {
		return fmt.Errorf("required.yml: the routing before the path loop changed: %q", got)
	}
	if got := codeLines(post); !slices.Equal(got, postRoutes) {
		return fmt.Errorf("required.yml: the routing after the path loop changed: %q", got)
	}
	return nil
}

func codeLines(lines []string) []string {
	var out []string
	for _, line := range lines {
		if line != "" && !strings.HasPrefix(line, "#") {
			out = append(out, line)
		}
	}
	return out
}

func caseBlocks(lines []string) ([][]arm, error) {
	var blocks [][]arm
	var current []arm
	open := false
	for _, line := range lines {
		switch {
		case line == "" || strings.HasPrefix(line, "#"):
		case line == blockStart && !open:
			open, current = true, nil
		case line == blockEnd && open:
			open, blocks = false, append(blocks, current)
		case open:
			a, err := parseArm(line)
			if err != nil {
				return nil, err
			}
			current = append(current, a)
		default:
			return nil, fmt.Errorf("required.yml: unparsed line in the path loop: %q", line)
		}
	}
	if open || len(blocks) != 2 {
		return nil, fmt.Errorf("required.yml: the path loop must have 2 case blocks, found %d complete", len(blocks))
	}
	return blocks, nil
}

func parseArm(line string) (arm, error) {
	m := armLine.FindStringSubmatch(line)
	if m == nil {
		return arm{}, fmt.Errorf("required.yml: unreadable arm %q", line)
	}
	a := arm{sets: map[string]bool{}}
	for _, glob := range strings.Split(m[1], "|") {
		a.catchAll = a.catchAll || glob == "*"
		re, err := globRegexp(glob)
		if err != nil {
			return arm{}, err
		}
		a.patterns = append(a.patterns, re)
	}
	for _, action := range strings.Split(m[2], ";") {
		action = strings.TrimSpace(action)
		if action == "" {
			continue
		}
		w := actionWord.FindStringSubmatch(action)
		if w == nil {
			return arm{}, fmt.Errorf("required.yml: unreadable action %q in %q", action, line)
		}
		a.sets[w[1]] = true
	}
	return a, nil
}

func globRegexp(glob string) (*regexp.Regexp, error) {
	if glob == "" || strings.ContainsAny(glob, "?[]\\\"'$") {
		return nil, fmt.Errorf("required.yml: unsupported glob %q", glob)
	}
	parts := strings.Split(glob, "*")
	for i, p := range parts {
		parts[i] = regexp.QuoteMeta(p)
	}
	return regexp.MustCompile("(?s)^" + strings.Join(parts, ".*") + "$"), nil
}

func (rule blockRule) check(block []arm) error {
	last := len(block) - 1
	if last < 0 || !block[last].catchAll {
		return fmt.Errorf("no catch-all arm `*)` at the end")
	}
	for _, flag := range rule.catchAllSets {
		if !block[last].sets[flag] {
			return fmt.Errorf("the catch-all arm must set %s", flag)
		}
	}
	seen := map[string]bool{}
	for i, a := range block {
		if a.catchAll && i != last {
			return fmt.Errorf("a catch-all arm before the end hides arm %d", i+2)
		}
		for flag := range a.sets {
			if !rule.known[flag] {
				return fmt.Errorf("unknown flag %s", flag)
			}
			seen[flag] = true
		}
	}
	for _, flag := range rule.needs {
		if !seen[flag] {
			return fmt.Errorf("the block never sets %s", flag)
		}
	}
	return nil
}

func (a arm) matches(path string) bool {
	for _, re := range a.patterns {
		if re.MatchString(path) {
			return true
		}
	}
	return false
}

func firstMatch(arms []arm, path string) map[string]bool {
	last := len(arms) - 1
	for _, a := range arms[:last] {
		if a.matches(path) {
			return a.sets
		}
	}
	return arms[last].sets
}

func (r Rule) Classify(paths []string) Verdict {
	if len(paths) == 0 {
		return Verdict{Class: Boundary}
	}
	var docs, other bool
	var suitePaths []string
	seen := map[string]bool{}
	for _, p := range paths {
		if seen[p] || strings.HasPrefix(p, ciOnlyPrefix) {
			continue
		}
		seen[p] = true
		c, s := firstMatch(r.classes, p), firstMatch(r.suites, p)
		docs, other = docs || c[docsFlag], other || c[otherFlag]
		if s[goFlag] || s[landingFlag] {
			suitePaths = append(suitePaths, p)
		}
	}
	if len(suitePaths) == 0 || docs && !other {
		return Verdict{Class: Window}
	}
	return Verdict{Class: Boundary, BoundaryPaths: suitePaths}
}
