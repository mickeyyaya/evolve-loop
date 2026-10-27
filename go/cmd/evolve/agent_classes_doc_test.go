package main

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/agentclass"
)

var agentClauseRE = regexp.MustCompile("^(?:`([a-z-]+)` )?([ARD])(?: \\(`([a-z-]+)`\\))?(?: only with `(--[a-z-]+)`)?(?: unless `(--[a-z-]+)`)?")

func agentClassesOfCell(paths []string, cell string) map[string]agentclass.Verb {
	whole := agentclass.Verb{}
	bySub := map[string]agentclass.Verb{}
	for _, clause := range strings.Split(cell, ", ") {
		m := agentClauseRE.FindStringSubmatch(clause)
		if m == nil {
			continue
		}
		v := agentclass.Verb{Class: m[2], Kind: m[3], OnlyWith: m[4], DeniedWith: m[5]}
		if m[1] == "" {
			whole = v
			continue
		}
		bySub[m[1]] = v
	}
	out := map[string]agentclass.Verb{}
	for _, path := range paths {
		words := strings.Fields(path)
		v, ok := bySub[words[len(words)-1]]
		if !ok {
			v = whole
		}
		v.Path = path
		out[path] = v
	}
	return out
}

func agentClassesInTheContract(t *testing.T) map[string]agentclass.Verb {
	t.Helper()
	inDoc := map[string]agentclass.Verb{}
	add := func(rows []contractDocRow, col int) {
		for _, row := range rows {
			for path, v := range agentClassesOfCell(contractVerbPaths(row.cells[0]), row.cells[col]) {
				if v.Class == "" {
					continue
				}
				if _, dup := inDoc[path]; dup {
					t.Errorf("%s: cli-contract.md gives it an agent class twice", path)
				}
				inDoc[path] = v
			}
		}
	}
	add(contractDocSection(t, "## 5. The core verbs"), 2)
	add(contractDocSection(t, "### 4.1 Agent classes"), 1)
	return inDoc
}

func TestAgentClasses_MatchTheContract(t *testing.T) {
	t.Parallel()
	inDoc := agentClassesInTheContract(t)
	inGo := map[string]bool{}
	for _, v := range agentclass.Verbs() {
		inGo[v.Path] = true
		d, ok := inDoc[v.Path]
		if !ok {
			t.Errorf("%s: the Go table gives it class %s, but cli-contract.md §4.1 and §5 do not name it", v.Path, v.Class)
			continue
		}
		if d != v {
			t.Errorf("%s: Go %+v, cli-contract.md %+v", v.Path, v, d)
		}
		if lookupCommand(strings.Fields(v.Path)[0]) == nil {
			t.Errorf("%s: the registry has no command %q", v.Path, strings.Fields(v.Path)[0])
		}
	}
	for path, d := range inDoc {
		if !inGo[path] {
			t.Errorf("%s: cli-contract.md gives it class %s, but the Go table has no row", path, d.Class)
		}
	}
	for _, v := range coreVerbs {
		if !inGo[v.path] {
			t.Errorf("%s: a core verb with no agent class in the Go table", v.path)
		}
	}
}

func TestAgentClassesOfCell_ReadsEachClauseForm(t *testing.T) {
	t.Parallel()
	got := agentClassesOfCell([]string{"x a", "x b", "x c"}, "`a` R (`inbox-add`), `b` A only with `--static` (F7), D")
	want := map[string]agentclass.Verb{
		"x a": {Path: "x a", Class: "R", Kind: "inbox-add"},
		"x b": {Path: "x b", Class: "A", OnlyWith: "--static"},
		"x c": {Path: "x c", Class: "D"},
	}
	for p, w := range want {
		if got[p] != w {
			t.Errorf("%s = %+v, want %+v", p, got[p], w)
		}
	}
	if v := agentClassesOfCell([]string{"y"}, "A unless `--rerun`")["y"]; v.DeniedWith != "--rerun" || v.Class != "A" {
		t.Errorf("unless clause = %+v", v)
	}
}

var personaVerbRE = regexp.MustCompile("(?m)(?:^[ \\t#$]*|`|/)evolve ([a-z][a-z-]*)(?: ([a-z][a-z-]*))?")

var personaVerbsOutsideACell = map[string]string{
	"subagent run": "the orchestrator persona and the templates describe how the pipeline dispatches a phase; no cell runs it",
	"subagent":     "plan-reviewer.md and orchestrator-reference.md name the fan-out that the pipeline starts; no cell runs it",
	"aggregator":   "plan-reviewer.md names the merge of the fan-out that the pipeline runs; no cell runs it",
	"doctor":       "F13: post-ship-monitor runs the full doctor, which execs LLM CLIs; at enforce it refuses",
	"loop":         "F13: post-ship-monitor runs loop --dry-run; at enforce it refuses",
	"inbox-mover":  "F11: the triage probe; the host claims, and the probe must go before enforce",
}

func TestAgentClasses_EachVerbThatAPersonaRunsIsAOrR(t *testing.T) {
	t.Parallel()
	files, err := filepath.Glob(filepath.Join(findRepoRoot(t), "agents", "*.md"))
	if err != nil || len(files) == 0 {
		t.Fatalf("no persona files: %v", err)
	}
	for _, f := range files {
		body, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		for _, m := range personaVerbRE.FindAllStringSubmatch(string(body), -1) {
			args := []string{m[1]}
			if m[2] != "" {
				args = append(args, m[2])
			}
			if agentClassOf(args).Class != agentclass.Denied || lookupCommand(m[1]) == nil {
				continue
			}
			if _, known := personaVerbsOutsideACell[agentClassOf(args).Name]; !known {
				t.Errorf("%s names `evolve %s`, which is class D: a cell refuses it at enforce. Give it a row in go/internal/agentclass and cli-contract.md §4.1, or name it in personaVerbsOutsideACell with the reason", filepath.Base(f), strings.Join(args, " "))
			}
		}
	}
}
