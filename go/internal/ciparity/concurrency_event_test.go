package ciparity

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

type ciEvent struct {
	name   string
	prNum  int
	ref    string
	runID  int
	wantCI bool
}

type exprEnv map[string]string

func (e exprEnv) lookup(ref string) string {
	return e[ref]
}

func evalWorkflowTemplate(t *testing.T, template string, env exprEnv) string {
	t.Helper()
	pattern := regexp.MustCompile(`\$\{\{(.*?)\}\}`)
	return pattern.ReplaceAllStringFunc(template, func(m string) string {
		inner := strings.TrimSuffix(strings.TrimPrefix(m, "${{"), "}}")
		return evalWorkflowExpr(t, strings.TrimSpace(inner), env)
	})
}

func evalWorkflowExpr(t *testing.T, expr string, env exprEnv) string {
	t.Helper()
	if parts := splitTopLevel(expr, "||"); len(parts) > 1 {
		for _, p := range parts {
			if v := evalWorkflowExpr(t, p, env); truthy(v) {
				return v
			}
		}
		return evalWorkflowExpr(t, parts[len(parts)-1], env)
	}
	if parts := splitTopLevel(expr, "&&"); len(parts) > 1 {
		var v string
		for _, p := range parts {
			v = evalWorkflowExpr(t, p, env)
			if !truthy(v) {
				return v
			}
		}
		return v
	}
	if parts := splitTopLevel(expr, "=="); len(parts) == 2 {
		return strconv.FormatBool(evalWorkflowExpr(t, parts[0], env) == evalWorkflowExpr(t, parts[1], env))
	}
	if parts := splitTopLevel(expr, "!="); len(parts) == 2 {
		return strconv.FormatBool(evalWorkflowExpr(t, parts[0], env) != evalWorkflowExpr(t, parts[1], env))
	}
	expr = strings.TrimSpace(expr)
	if strings.HasPrefix(expr, "'") && strings.HasSuffix(expr, "'") && len(expr) >= 2 {
		return strings.ReplaceAll(expr[1:len(expr)-1], "''", "'")
	}
	if strings.HasPrefix(expr, "format(") && strings.HasSuffix(expr, ")") {
		args := splitTopLevel(expr[len("format("):len(expr)-1], ",")
		out := evalWorkflowExpr(t, args[0], env)
		for i, a := range args[1:] {
			out = strings.ReplaceAll(out, fmt.Sprintf("{%d}", i), evalWorkflowExpr(t, a, env))
		}
		return out
	}
	if expr == "true" || expr == "false" {
		return expr
	}
	if !strings.HasPrefix(expr, "github.") {
		t.Fatalf("unsupported expression %q", expr)
	}
	return env.lookup(expr)
}

func truthy(v string) bool {
	return v != "" && v != "false" && v != "0"
}

func splitTopLevel(expr, sep string) []string {
	var parts []string
	depth, quoted, start := 0, false, 0
	for i := 0; i < len(expr); i++ {
		switch c := expr[i]; {
		case c == '\'':
			quoted = !quoted
		case quoted:
		case c == '(':
			depth++
		case c == ')':
			depth--
		case depth == 0 && strings.HasPrefix(expr[i:], sep):
			if sep == "==" && i > 0 && expr[i-1] == '!' {
				continue
			}
			parts = append(parts, strings.TrimSpace(expr[start:i]))
			i += len(sep) - 1
			start = i + 1
		}
	}
	return append(parts, strings.TrimSpace(expr[start:]))
}

func (e ciEvent) env(workflow string) exprEnv {
	env := exprEnv{
		"github.workflow":   workflow,
		"github.event_name": e.name,
		"github.ref":        e.ref,
		"github.run_id":     strconv.Itoa(e.runID),
	}
	if e.prNum != 0 {
		env["github.event.pull_request.number"] = strconv.Itoa(e.prNum)
	}
	return env
}

func ciEvents() []ciEvent {
	return []ciEvent{
		{name: "pull_request", prNum: 42, ref: "refs/pull/42/merge", runID: 1001, wantCI: true},
		{name: "push", ref: "refs/heads/main", runID: 1002},
		{name: "workflow_dispatch", ref: "refs/heads/feature", runID: 1003},
	}
}

func eventByName(t *testing.T, name string) ciEvent {
	t.Helper()
	for _, e := range ciEvents() {
		if e.name == name {
			return e
		}
	}
	t.Fatalf("no event %q", name)
	return ciEvent{}
}

func evaluateConcurrency(t *testing.T, file string, e ciEvent) (group, cancel string) {
	t.Helper()
	c := readWorkflowContract(t, file).Concurrency
	return evalWorkflowTemplate(t, c.Group, e.env("CI required")), evalWorkflowTemplate(t, c.CancelInProgress, e.env("CI required"))
}

func TestCIConcurrency_PullRequestGroupsByNumberAndCancels(t *testing.T) {
	t.Parallel()
	e := eventByName(t, "pull_request")
	group, cancel := evaluateConcurrency(t, RequiredWorkflow, e)
	if want := "CI required-pr-42"; group != want {
		t.Errorf("pull_request group = %q, want %q", group, want)
	}
	if cancel != "true" {
		t.Errorf("pull_request cancel-in-progress = %q, want true", cancel)
	}
}

func TestCIConcurrency_PushGroupsByRefAndNeverCancels(t *testing.T) {
	t.Parallel()
	e := eventByName(t, "push")
	group, cancel := evaluateConcurrency(t, RequiredWorkflow, e)
	if want := "CI required-refs/heads/main"; group != want {
		t.Errorf("push group = %q, want %q", group, want)
	}
	if cancel != "false" {
		t.Errorf("push cancel-in-progress = %q, want false", cancel)
	}
}

func TestCIConcurrency_DispatchGroupsByRunID(t *testing.T) {
	t.Parallel()
	e := eventByName(t, "workflow_dispatch")
	group, cancel := evaluateConcurrency(t, RequiredWorkflow, e)
	if want := "CI required-dispatch-1003"; group != want {
		t.Errorf("workflow_dispatch group = %q, want %q", group, want)
	}
	if cancel != "false" {
		t.Errorf("workflow_dispatch cancel-in-progress = %q, want false", cancel)
	}
}

func TestCIConcurrency_RequiredAndGoWorkflowsAreEvaluatedForEveryEvent(t *testing.T) {
	t.Parallel()
	events := ciEvents()
	if len(events) != 3 || events[0].name != "pull_request" || events[1].name != "push" || events[2].name != "workflow_dispatch" {
		t.Fatalf("events = %+v, want pull_request, push and workflow_dispatch", events)
	}
	for _, e := range events {
		group, cancel := evaluateConcurrency(t, RequiredWorkflow, e)
		if group == "" || group == "CI required-" {
			t.Errorf("%s: required.yml yields an empty group %q", e.name, group)
		}
		if got := cancel == "true"; got != e.wantCI {
			t.Errorf("%s: required.yml cancel-in-progress = %q, want %v", e.name, cancel, e.wantCI)
		}
		goGroup, goCancel := evaluateConcurrency(t, "go.yml", e)
		if goGroup != "" || goCancel != "" {
			t.Errorf("%s: go.yml declares concurrency (%q, %q); a called workflow shares the caller's github.workflow and can cancel its caller", e.name, goGroup, goCancel)
		}
	}
}

func TestCIConcurrency_DistinctPullRequestsRefsAndRunsNeverShareAGroup(t *testing.T) {
	t.Parallel()
	seen := map[string]string{}
	record := func(label string, e ciEvent) {
		group, _ := evaluateConcurrency(t, RequiredWorkflow, e)
		if prior, dup := seen[group]; dup {
			t.Errorf("%s and %s share group %q", prior, label, group)
		}
		seen[group] = label
	}
	record("pull_request #1", ciEvent{name: "pull_request", prNum: 1, ref: "refs/pull/1/merge", runID: 1})
	record("pull_request #2", ciEvent{name: "pull_request", prNum: 2, ref: "refs/pull/2/merge", runID: 2})
	record("push main", ciEvent{name: "push", ref: "refs/heads/main", runID: 3})
	record("push release", ciEvent{name: "push", ref: "refs/heads/release", runID: 4})
	record("workflow_dispatch run 5", ciEvent{name: "workflow_dispatch", ref: "refs/heads/main", runID: 5})
	record("workflow_dispatch run 6", ciEvent{name: "workflow_dispatch", ref: "refs/heads/main", runID: 6})
	if len(seen) != 6 {
		t.Errorf("recorded %d distinct groups, want 6", len(seen))
	}
}

func TestCIConcurrencyEvaluator_HandlesPrecedenceAndFormat(t *testing.T) {
	t.Parallel()
	env := exprEnv{"github.a": "x", "github.b": "y"}
	for expr, want := range map[string]string{
		"github.a == 'x' && 'yes' || 'no'":      "yes",
		"github.a == 'z' && 'yes' || 'no'":      "no",
		"github.a != 'z'":                       "true",
		"format('{0}-{1}', github.a, github.b)": "x-y",
		"github.a == 'z' && 'p' || github.b":    "y",
		"github.missing || 'fallback'":          "fallback",
	} {
		if got := evalWorkflowExpr(t, expr, env); got != want {
			t.Errorf("%s = %q, want %q", expr, got, want)
		}
	}
}
