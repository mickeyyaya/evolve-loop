//go:build integration && darwin

package bridge

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"
)

type parityTarget struct {
	pkg, tags string
	named     []string
	filtered  bool
}

type goReport struct {
	results testResults
	dropped []string
	rc      int
}

func newGoReport(out string, rc int) goReport {
	results, dropped := parseTestJSON(out)
	return goReport{results: results, dropped: dropped, rc: rc}
}

func (r goReport) validate() error {
	if len(r.dropped) > 0 {
		return fmt.Errorf("the report has %d line(s) that are not test2json: %q", len(r.dropped), r.dropped)
	}
	pkg, ok := r.results[packageResult]
	if !ok {
		return fmt.Errorf("the report has no package result: it is truncated")
	}
	failed := slices.Contains(slices.Collect(maps.Values(r.results)), "fail")
	if r.rc != 0 && !failed {
		return fmt.Errorf("exit status %d but no test failed", r.rc)
	}
	if r.rc == 0 && (failed || pkg != "pass") {
		return fmt.Errorf("exit status 0 but the package result is %s", pkg)
	}
	return nil
}

func (p parityTarget) hostEvidence(r goReport) error {
	var missing []string
	for _, name := range p.named {
		if got := r.results[name]; got != "pass" {
			missing = append(missing, fmt.Sprintf("%s %s: host=%s, want pass", p.pkg, name, orMissing(got)))
		}
	}
	if len(missing) > 0 {
		return errors.New(strings.Join(missing, "; "))
	}
	return nil
}

func (p parityTarget) goTestArgs() []string {
	args := []string{"test", "-json", "-count=1"}
	if p.tags != "" {
		args = append(args, "-tags", p.tags)
	}
	if p.filtered {
		args = append(args, "-run", "^("+strings.Join(p.named, "|")+")$")
	}
	return append(args, p.pkg)
}

type testResults map[string]string

func orMissing(action string) string {
	if action == "" {
		return "missing"
	}
	return action
}

func parseTestJSON(out string) (testResults, []string) {
	results, dropped := testResults{}, []string{}
	scanner := bufio.NewScanner(strings.NewReader(out))
	scanner.Buffer(make([]byte, 1<<20), 1<<24)
	for scanner.Scan() {
		var ev struct{ Action, Test string }
		if json.Unmarshal(scanner.Bytes(), &ev) != nil {
			dropped = append(dropped, scanner.Text())
			continue
		}
		name := ev.Test
		if name == "" {
			name = packageResult
		}
		if ev.Action == "pass" || ev.Action == "fail" || ev.Action == "skip" {
			results[name] = ev.Action
		}
	}
	return results, dropped
}

const packageResult = "(package)"
