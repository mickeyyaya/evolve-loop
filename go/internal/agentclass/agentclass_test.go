package agentclass

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"
)

func TestOf_ClassifiesEachFormOfAVerb(t *testing.T) {
	t.Parallel()
	cases := []struct {
		args []string
		want Match
	}{
		{[]string{"phase", "verify", "build"}, Match{Name: "phase verify", Class: Allowed}},
		{[]string{"phase", "scout", "--cycle", "3"}, Match{Name: "phase scout", Class: Request, Kind: "dispatch"}},
		{[]string{"phase", "lint"}, Match{Name: "phase lint", Class: Denied}},
		{[]string{"phase", "--cycle", "3"}, Match{Name: "phase", Class: Denied}},
		{[]string{"phase"}, Match{Name: "phase", Class: Denied}},
		{[]string{"inbox", "add", "--file", "-"}, Match{Name: "inbox add", Class: Request, Kind: "inbox-add"}},
		{[]string{"inbox", "show", "x"}, Match{Name: "inbox show", Class: Allowed}},
		{[]string{"inbox", "claim", "x", "--cycle", "2"}, Match{Name: "inbox claim", Class: Denied}},
		{[]string{"inbox", "--project-root", "P", "list"}, Match{Name: "inbox", Class: Denied}},
		{[]string{"wave", "stop"}, Match{Name: "wave stop", Class: Denied}},
		{[]string{"loop-stop", "--wait", "1m"}, Match{Name: "loop-stop", Class: Denied}},
		{[]string{"status", "--json"}, Match{Name: "status", Class: Allowed}},
		{[]string{"postedit-validate"}, Match{Name: "postedit-validate", Class: Allowed}},
		{[]string{"cli-routing", "show", "--static"}, Match{Name: "cli-routing show", Class: Allowed}},
		{[]string{"cli-routing", "show", "-static=true"}, Match{Name: "cli-routing show", Class: Allowed}},
		{[]string{"cli-routing", "show"}, Match{Name: "cli-routing show", Class: Denied}},
		{[]string{"cli-routing", "show", "--static=false"}, Match{Name: "cli-routing show", Class: Denied}},
		{[]string{"cli-routing", "show", "--", "--static"}, Match{Name: "cli-routing show", Class: Denied}},
		{[]string{"ci", "classify", "--json"}, Match{Name: "ci classify", Class: Allowed}},
		{[]string{"ci", "classify", "123", "--json", "--rerun"}, Match{Name: "ci classify", Class: Denied}},
		{[]string{"ci", "classify", "--rerun=maybe"}, Match{Name: "ci classify", Class: Denied}},
		{[]string{"ci", "classify", "--rerun=0"}, Match{Name: "ci classify", Class: Allowed}},
		{[]string{"ci", "classify", "--rerun=false", "--rerun"}, Match{Name: "ci classify", Class: Denied}},
		{[]string{"ci", "classify", "--rerun", "--rerun=false"}, Match{Name: "ci classify", Class: Allowed}},
		{[]string{"cli-routing", "show", "--static", "--static=false"}, Match{Name: "cli-routing show", Class: Denied}},
		{[]string{"cli-routing", "show", "--static=false", "--static"}, Match{Name: "cli-routing show", Class: Allowed}},
		{[]string{"cli-routing", "show", "--cli", "--static"}, Match{Name: "cli-routing show", Class: Denied}},
		{[]string{"cli-routing", "show", "--static", "--json"}, Match{Name: "cli-routing show", Class: Allowed}},
		{[]string{"cli-routing", "show", "--json", "--static"}, Match{Name: "cli-routing show", Class: Denied}},
		{[]string{"cli-routing", "show", "--static=maybe"}, Match{Name: "cli-routing show", Class: Denied}},
		{[]string{"prune-ephemeral", "--quiet"}, Match{Name: "prune-ephemeral", Class: Denied}},
		{[]string{"no-such-verb"}, Match{Name: "no-such-verb", Class: Denied}},
		{nil, Match{Class: Denied}},
	}
	for _, c := range cases {
		if got := Of(c.args); got != c.want {
			t.Errorf("Of(%q) = %+v, want %+v", c.args, got, c.want)
		}
	}
}

func TestVerbs_IsACopyOfTheTableWithOneRowForEachPath(t *testing.T) {
	t.Parallel()
	verbs := Verbs()
	seen := map[string]bool{}
	for _, v := range verbs {
		if seen[v.Path] {
			t.Errorf("%s: two rows", v.Path)
		}
		seen[v.Path] = true
		if (v.Class == Request) != (v.Kind != "") {
			t.Errorf("%s: class %s with request kind %q; only an R verb has a kind", v.Path, v.Class, v.Kind)
		}
		if !slices.Contains([]string{Allowed, Request, Denied}, v.Class) {
			t.Errorf("%s: class %q", v.Path, v.Class)
		}
	}
	verbs[0].Class = "X"
	if Verbs()[0].Class == "X" {
		t.Fatal("Verbs must return a copy; a caller changed the table")
	}
}

func TestMatch_RefusalNamesTheRequestPathForClassROnly(t *testing.T) {
	t.Parallel()
	r := Of([]string{"inbox", "add"}).Refusal(".evolve/runs/cycle-2101")
	want := "refused inside a phase: inbox add is class R; write the request to .evolve/runs/cycle-2101/requests/inbox-add-<n>.json (schema evolve.request/1, see evolve agent capabilities --json)"
	if r != want {
		t.Errorf("R refusal = %q, want %q", r, want)
	}
	if r := Of([]string{"agent", "run"}).Refusal(""); !strings.Contains(r, ".evolve/runs/cycle-<N>/requests/dispatch-<n>.json") {
		t.Errorf("an R refusal with no workspace names the template path, got %q", r)
	}
	if d := Of([]string{"wave", "next"}).Refusal("ws"); d != "refused inside a phase: wave next is class D; it is an operator or boundary verb" || strings.Contains(d, "requests/") {
		t.Errorf("D refusal = %q, want no request path", d)
	}
}

func TestSurface_ListsEachVerbWithItsClassAndDefaultsToD(t *testing.T) {
	t.Parallel()
	output := func(path string) string {
		if path == "status" {
			return "evolve.status/1"
		}
		return ""
	}
	s := Surface(nil, output)
	raw, err := json.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Cell         *Cell            `json:"cell"`
		DefaultClass string           `json:"default_class"`
		Caps         map[string]any   `json:"caps"`
		Verbs        []map[string]any `json:"verbs"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	if doc.Cell != nil || doc.DefaultClass != Denied || doc.Caps == nil || len(doc.Verbs) != len(Verbs()) {
		t.Fatalf("surface = %s; want cell null, default_class D, caps {} and one row for each verb", raw)
	}
	byVerb := map[string]map[string]any{}
	for _, v := range doc.Verbs {
		byVerb[v["verb"].(string)] = v
	}
	if v := byVerb["status"]; v["class"] != Allowed || v["input"] != "args/1" || v["output"] != "evolve.status/1" || v["request"] != nil {
		t.Errorf("status row = %v", v)
	}
	if v := byVerb["code find"]; v["output"] != "text/1" {
		t.Errorf("an A verb with no envelope has output text/1, got %v", v)
	}
	if v := byVerb["cli-routing show"]; v["only_with"] != "--static" {
		t.Errorf("cli-routing show row = %v, want only_with --static", v)
	}
	if v := byVerb["ci classify"]; v["denied_with"] != "--rerun" {
		t.Errorf("ci classify row = %v, want denied_with --rerun", v)
	}
	req, _ := byVerb["inbox add"]["request"].(map[string]any)
	if req["kind"] != "inbox-add" || req["schema"] != RequestSchema || req["path"] != ".evolve/runs/cycle-<N>/requests/inbox-add-<n>.json" {
		t.Errorf("inbox add request = %v", byVerb["inbox add"])
	}
	if v := byVerb["wave next"]; len(v) != 2 || v["class"] != Denied {
		t.Errorf("a D row names only the verb and the class, got %v", v)
	}
	var cell Capabilities = Surface(&Cell{Cycle: 2101, Phase: "build", Workspace: "ws"}, output)
	want := SurfaceVerb{Verb: "agent run", Class: Request, Request: &RequestRef{Kind: "dispatch", Schema: RequestSchema, Path: "ws/requests/dispatch-<n>.json"}}
	for _, v := range cell.Verbs {
		if v.Verb == want.Verb && (v.Class != want.Class || *v.Request != *want.Request) {
			t.Errorf("in a cell the request path is under the workspace: got %+v %+v, want %+v", v, v.Request, want.Request)
		}
	}
}

func TestBrief_PointsTheCellToTheCapabilitiesVerbInNeutralText(t *testing.T) {
	t.Parallel()
	b := Brief("/ws/cycle-7")
	for _, want := range []string{
		"evolve agent capabilities --json",
		"/ws/cycle-7/requests/<kind>-<n>.json",
		"schema evolve.request/1",
		"Do not run a class D verb",
	} {
		if !strings.Contains(b, want) {
			t.Errorf("brief has no %q:\n%s", want, b)
		}
	}
	if strings.Contains(b, "Bash(") || len(b) > 400 {
		t.Errorf("the brief must be short and name no tool syntax of a family (%d bytes):\n%s", len(b), b)
	}
}

func TestRequestPath_IsUnderTheWorkspaceOrTheCellTemplate(t *testing.T) {
	t.Parallel()
	if got := RequestPath("ws", "inbox-add"); got != "ws/requests/inbox-add-<n>.json" {
		t.Errorf("RequestPath(ws) = %q", got)
	}
	if got := RequestPath("", "dispatch"); got != ".evolve/runs/cycle-<N>/requests/dispatch-<n>.json" {
		t.Errorf("RequestPath(\"\") = %q", got)
	}
}
