package guidance

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

var testFamilies = []string{"claude", "gemini", "gpt", "local"}

func TestCheck_ABuiltTreeHasNoProblem(t *testing.T) {
	root := builtTree(t, ruleSpec{id: "safe", extra: "enforced_by: [AGENTS.md]\n"})

	problems, err := Check(root, testFamilies)

	if err != nil || len(problems) != 0 {
		t.Fatalf("Check = %v, %v; want none", problems, err)
	}
}

func TestCheck_ReportsEachKindOfProblem(t *testing.T) {
	cases := []struct {
		name, prefix, fragment string
		specs                  []ruleSpec
		mutate                 func(t *testing.T, root string)
	}{
		{name: "stale index", prefix: "STALE: ", fragment: IndexMarkdownPath, specs: []ruleSpec{{id: "a"}},
			mutate: func(t *testing.T, root string) { writeFile(t, root, IndexMarkdownPath, "hand edit\n") }},
		{name: "missing region", prefix: "MISSING: ", fragment: "GENERATED:guidance-l0", specs: []ruleSpec{{id: "a"}},
			mutate: func(t *testing.T, root string) { writeFile(t, root, AgentsPath, "# AGENTS\n") }},
		{name: "bad registry file", prefix: "guidance/rules/bad.md: ", fragment: "no frontmatter", specs: []ruleSpec{{id: "a"}},
			mutate: func(t *testing.T, root string) { writeFile(t, root, "guidance/rules/bad.md", "x\n") }},
		{name: "duplicate statement", prefix: "DUPLICATE: ", fragment: "a and b", specs: []ruleSpec{{id: "a", statement: "Same  words."}, {id: "b", statement: "same words."}}},
		{name: "unknown conflict", prefix: "CONFLICT: ", fragment: "unknown rule ghost", specs: []ruleSpec{{id: "a", extra: "conflicts_with: [ghost]\n"}}},
		{name: "active conflict", prefix: "CONFLICT: ", fragment: "a and b are both active", specs: []ruleSpec{{id: "a", extra: "conflicts_with: [b]\n"}, {id: "b"}}},
		{name: "absent enforcer", prefix: "ENFORCED_BY: ", fragment: "go/absent.go", specs: []ruleSpec{{id: "a", extra: "enforced_by: [go/absent.go]\n"}}},
		{name: "enforcer outside the repo", prefix: "ENFORCED_BY: ", fragment: "not a path inside", specs: []ruleSpec{{id: "a", extra: "enforced_by: [../outside.go]\n"}}},
		{name: "L0 with families", prefix: "FAMILIES: ", fragment: "a is L0", specs: []ruleSpec{{id: "a", extra: "families: [claude]\n"}}},
		{name: "unknown family", prefix: "FAMILIES: ", fragment: "names cobol", specs: []ruleSpec{{id: "a", layer: "L2", extra: "families: [cobol]\n"}}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			root := builtTree(t, c.specs...)
			if c.mutate != nil {
				c.mutate(t, root)
			}
			problems, err := Check(root, testFamilies)
			if err != nil || !hasProblem(problems, c.prefix, c.fragment) {
				t.Fatalf("Check = %v, %v; want %s%s", problems, err, c.prefix, c.fragment)
			}
		})
	}
}

func TestCheck_AnInactiveRuleCanNameAConflictAndIsNotChecked(t *testing.T) {
	root := builtTree(t, ruleSpec{id: "a", extra: "conflicts_with: [b]\n"}, ruleSpec{id: "b", status: "superseded", extra: "conflicts_with: [a]\nenforced_by: [go/absent.go]\n"})

	problems, err := Check(root, testFamilies)

	if err != nil || len(problems) != 0 {
		t.Fatalf("Check = %v, %v; want none", problems, err)
	}
}

func TestCheck_RefusesAnL0OverItsRuleAndLineBudgets(t *testing.T) {
	cases := []struct {
		rules     int
		wantLines bool
	}{{MaxL0Rules + 1, false}, {MaxL0Lines, true}}
	for _, c := range cases {
		t.Run(fmt.Sprint(c.rules), func(t *testing.T) {
			var specs []ruleSpec
			for i := 0; i < c.rules; i++ {
				specs = append(specs, ruleSpec{id: fmt.Sprintf("r%02d", i)})
			}
			problems, _ := Check(builtTree(t, specs...), testFamilies)
			if !hasProblem(problems, "BUDGET: ", fmt.Sprintf("%d active rules", c.rules)) || hasProblem(problems, "BUDGET: ", "L0 region has") != c.wantLines {
				t.Fatalf("Check = %v, want the rule budget and lines=%v", problems, c.wantLines)
			}
		})
	}
}

func TestCheck_ReturnsAnErrorOnAnIOFault(t *testing.T) {
	cases := map[string]func(t *testing.T, root string){
		"no registry": func(t *testing.T, root string) {
			if err := os.RemoveAll(filepath.Join(root, "guidance", "rules")); err != nil {
				t.Fatal(err)
			}
		},
		"no AGENTS.md": func(t *testing.T, root string) {
			if err := os.Remove(filepath.Join(root, AgentsPath)); err != nil {
				t.Fatal(err)
			}
		},
		"an output is a directory": func(t *testing.T, root string) {
			if err := os.Remove(filepath.Join(root, "guidance", "index.json")); err != nil {
				t.Fatal(err)
			}
			if err := os.Mkdir(filepath.Join(root, "guidance", "index.json"), 0o755); err != nil {
				t.Fatal(err)
			}
		},
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			root := builtTree(t, ruleSpec{id: "a"})
			mutate(t, root)
			if _, err := Check(root, testFamilies); err == nil {
				t.Fatal("Check = nil error, want an I/O error")
			}
		})
	}
}

func TestCheck_AnEnforcerPathThatCannotBeStatedIsAnIOError(t *testing.T) {
	root := builtTree(t, ruleSpec{id: "a", extra: "enforced_by: [AGENTS.md/below]\n"})

	_, err := Check(root, testFamilies)

	if err == nil {
		t.Fatal("Check = nil error, want a stat error for a path below a file")
	}
}
