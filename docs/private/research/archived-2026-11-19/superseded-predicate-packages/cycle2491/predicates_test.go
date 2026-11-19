//go:build acs

package cycle2491

import (
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/incidentkb"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

var helpLine = regexp.MustCompile(`(?m)^\s*issue\s`)

const modulePath = "github.com/mickeyyaya/evolve-loop/go"

const recordID = "2026-11-19-fixture-record"

const secondRecordID = "2026-11-19-second-record"

const frontmatter = `---
schema: incident-record/1
id: 2026-11-19-fixture-record
title: fixture record
level: record
status: fixed
selfheal: inc-2026-11-19-ab12
fingerprint: integration-ejection:derived
signals: ["cycle:2481:57"]
prs: [1555, 1558]
items: [edit-layout-c2]
actions:
  - {id: A1, type: prevent, owner: console, item: edit-layout-c2, pr: 1558, done: "the journey test composes two shard PRs", state: done}
effectiveness:
  metric: {code: SHIP_INTEGRATION_EJECTED, filters: [{field: paths, op: contains, value: docs/research-index.md}], unit: wave, since: 051ad74bf}
  baseline: {value: 4, units: 2}
  target: 0
  checks: 3
  results: []
verified_at: ""
---
`

const bodySkeleton = `
# fixture record

## Summary

<!-- GENERATED:status BEGIN -->
<!-- GENERATED:status END -->

The fixture summary.

## Trigger and level

The fixture trigger.

## Timeline

The fixture timeline.

## Key logs

The fixture logs.

## Root cause and contributing factors

The fixture cause.

## Analysis

The fixture analysis.

## Approaches taken

The fixture approaches.

## Actions

<!-- GENERATED:actions BEGIN -->
<!-- GENERATED:actions END -->

## Effectiveness

<!-- GENERATED:effectiveness BEGIN -->
<!-- GENERATED:effectiveness END -->

## Lessons

The fixture lessons.
`

var (
	binOnce sync.Once
	binPath string
	binErr  string
	binDir  string
)

func TestMain(m *testing.M) {
	code := m.Run()
	if binDir != "" {
		os.RemoveAll(binDir)
	}
	os.Exit(code)
}

func evolveBinary(t *testing.T) string {
	t.Helper()
	binOnce.Do(func() {
		dir, err := os.MkdirTemp("", "evolve-c2491-")
		if err != nil {
			binErr = err.Error()
			return
		}
		binDir = dir
		modDir, err := acsassert.ModuleDir(".")
		if err != nil {
			binErr = err.Error()
			return
		}
		out := filepath.Join(dir, "evolve")
		stdout, stderr, code, _ := acsassert.SubprocessOutput("go", "-C", modDir, "build", "-o", out, "./cmd/evolve")
		if code != 0 {
			binErr = acsassert.OutputTail(stdout+stderr, 30)
			return
		}
		binPath = out
	})
	if binErr != "" {
		t.Fatalf("RED: cannot build the evolve binary: %s", binErr)
	}
	return binPath
}

func runIssue(t *testing.T, root string, verb string, extra ...string) (string, int) {
	t.Helper()
	args := append([]string{"issue", verb, "--project-root", root}, extra...)
	stdout, stderr, code, _ := acsassert.SubprocessOutput(evolveBinary(t), args...)
	return stdout + "\n" + stderr, code
}

func writeTree(t *testing.T, files map[string]string) string {
	t.Helper()
	root := acsassert.SetupTempProject(t)
	for rel, body := range files {
		writeIn(t, filepath.Join(root, filepath.FromSlash(rel)), body)
	}
	return root
}

func writeIn(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func renderedRecord(t *testing.T, mutateFrontmatter func(string) string) string {
	t.Helper()
	head := frontmatter
	if mutateFrontmatter != nil {
		head = mutateFrontmatter(head)
	}
	out, err := incidentkb.Render([]byte(head + bodySkeleton))
	if err != nil {
		t.Fatalf("RED: incidentkb.Render of a well-formed record failed: %v", err)
	}
	return string(out)
}

func secondRecord(t *testing.T) string {
	t.Helper()
	return renderedRecord(t, func(head string) string {
		head = strings.Replace(head, "id: "+recordID, "id: "+secondRecordID, 1)
		head = strings.Replace(head, "status: fixed", "status: open", 1)
		head = strings.Replace(head, "state: done", "state: open", 1)
		return strings.Replace(head, "SHIP_INTEGRATION_EJECTED", "LOOP_ORCH_POOL_IDLE", 1)
	})
}

func incidentTree(t *testing.T, extra map[string]string) string {
	t.Helper()
	files := map[string]string{
		"docs/incidents/" + recordID + ".md": renderedRecord(t, nil),
	}
	for rel, body := range extra {
		files["docs/incidents/"+rel] = body
	}
	return writeTree(t, files)
}

func recordPath(root string) string {
	return filepath.Join(root, "docs", "incidents", recordID+".md")
}

func mustPass(t *testing.T, root, label string) {
	t.Helper()
	out, code := runIssue(t, root, "check")
	if code != 0 {
		t.Fatalf("RED: %s: issue check exit %d, want 0; output:\n%s", label, code, acsassert.OutputTail(out, 20))
	}
}

func mustFailNaming(t *testing.T, root, label string, needles ...string) {
	t.Helper()
	out, code := runIssue(t, root, "check")
	if code == 0 || code == -1 {
		t.Fatalf("RED: %s: issue check exit %d, want a non-zero refusal; output:\n%s", label, code, acsassert.OutputTail(out, 20))
	}
	lower := strings.ToLower(out)
	for _, needle := range needles {
		if !strings.Contains(lower, strings.ToLower(needle)) {
			t.Errorf("RED: %s: refusal does not name %q; output:\n%s", label, needle, acsassert.OutputTail(out, 20))
		}
	}
}

func editFile(t *testing.T, path string, edit func(string) string) {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(edit(string(body))), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestC2491_001_IssueCheckPassesOnTheCurrentTree(t *testing.T) {
	root := acsassert.RepoRoot(t)
	out, code := runIssue(t, root, "check")
	if code != 0 {
		t.Fatalf("RED: issue check on the current tree: exit %d, want 0; output:\n%s", code, acsassert.OutputTail(out, 20))
	}
}

func TestC2491_002_IssueCheckRefusesANewIncidentFileWithNoFrontmatter(t *testing.T) {
	root := incidentTree(t, map[string]string{
		"2026-08-09-old-incident.md": "# an old incident\n\nNo frontmatter, dated before 2026-11-19.\n",
	})
	mustPass(t, root, "control: one valid record and one legacy file")
	newName := "2026-11-19-new-incident-without-frontmatter.md"
	if err := os.WriteFile(filepath.Join(root, "docs", "incidents", newName), []byte("# a new incident\n\nNo frontmatter.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	mustFailNaming(t, root, "a new dated file with no frontmatter", newName)
}

func TestC2491_003_AHandEditOfTheRenderedActionsSectionFailsNamingTheField(t *testing.T) {
	root := incidentTree(t, nil)
	mustPass(t, root, "control: a freshly rendered record")
	editFile(t, recordPath(root), func(body string) string {
		return strings.Replace(body, "<!-- GENERATED:actions END -->", "| A9 | a hand-written action row |\n<!-- GENERATED:actions END -->", 1)
	})
	mustFailNaming(t, root, "a hand edit inside the actions markers", recordID, "actions")
}

func TestC2491_004_AHandEditOfAnyRenderedSectionFailsNamingItsField(t *testing.T) {
	for _, field := range []string{"status", "actions", "effectiveness"} {
		t.Run(field, func(t *testing.T) {
			root := incidentTree(t, nil)
			mustPass(t, root, "control: a freshly rendered record")
			end := "<!-- GENERATED:" + field + " END -->"
			editFile(t, recordPath(root), func(body string) string {
				return strings.Replace(body, end, "a hand-written line\n"+end, 1)
			})
			mustFailNaming(t, root, "a hand edit inside the "+field+" markers", field)
		})
	}
}

func TestC2491_005_AnEditOutsideTheGeneratedPartsStillPasses(t *testing.T) {
	root := incidentTree(t, nil)
	mustPass(t, root, "control: a freshly rendered record")
	editFile(t, recordPath(root), func(body string) string {
		return strings.Replace(body, "The fixture analysis.", "The analysis that a person rewrote.", 1)
	})
	mustPass(t, root, "a hand edit of a prose section")
}

func TestC2491_006_AMissingSectionHeadingFailsNamingTheSection(t *testing.T) {
	root := incidentTree(t, nil)
	mustPass(t, root, "control: a freshly rendered record")
	editFile(t, recordPath(root), func(body string) string {
		return strings.Replace(body, "## Approaches taken\n", "", 1)
	})
	mustFailNaming(t, root, "a record without its Approaches taken heading", recordID, "Approaches taken")
}

func TestC2491_007_ABadFrontmatterFieldFailsNamingTheField(t *testing.T) {
	cases := []struct {
		field  string
		mutate func(string) string
	}{
		{"schema", func(h string) string { return strings.Replace(h, "incident-record/1", "incident-record/9", 1) }},
		{"level", func(h string) string { return strings.Replace(h, "level: record", "level: urgent", 1) }},
		{"status", func(h string) string { return strings.Replace(h, "status: fixed", "status: banana", 1) }},
		{"title", func(h string) string { return strings.Replace(h, "title: fixture record\n", "", 1) }},
		{"id", func(h string) string { return strings.Replace(h, "id: "+recordID, "id: 2026-11-19-another-name", 1) }},
	}
	for _, c := range cases {
		t.Run(c.field, func(t *testing.T) {
			control := incidentTree(t, nil)
			mustPass(t, control, "control: a freshly rendered record")
			body := frontmatter
			body = c.mutate(body) + bodySkeleton
			root := writeTree(t, map[string]string{"docs/incidents/" + recordID + ".md": body})
			mustFailNaming(t, root, "a record with a bad "+c.field, c.field)
		})
	}
}

func TestC2491_008_TheLegacyRuleAndTheNameSkips(t *testing.T) {
	root := incidentTree(t, map[string]string{
		"2026-08-09-old-incident.md":            "# old\n",
		"cycle-102-111.md":                      "# an old record with no date prefix\n",
		"REGRESSION-COVERAGE-INDEX.md":          "# the index, not a record\n",
		"2026-11-20-session-log.md":             "# a session log, not a record\n",
		"2026-11-18-the-day-before-the-rule.md": "# dated before the first day of the rule\n",
	})
	mustPass(t, root, "legacy files, the index and a session log with no frontmatter")
	if err := os.WriteFile(filepath.Join(root, "docs", "incidents", "2026-11-19-first-day-of-the-rule.md"), []byte("# a new record with no frontmatter\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	mustFailNaming(t, root, "a file dated on the first day of the rule", "2026-11-19-first-day-of-the-rule")
}

type listed struct {
	ID     string `json:"id"`
	Status string `json:"status"`
	Legacy bool   `json:"legacy"`
}

func listJSON(t *testing.T, root string, extra ...string) []listed {
	t.Helper()
	args := append([]string{"--json"}, extra...)
	out, code := runIssue(t, root, "list", args...)
	if code != 0 {
		t.Fatalf("RED: issue list --json %v: exit %d; output:\n%s", extra, code, acsassert.OutputTail(out, 20))
	}
	start := strings.Index(out, "[")
	end := strings.LastIndex(out, "]")
	if start < 0 && strings.Contains(out, "null") {
		return nil
	}
	if start < 0 || end < start {
		t.Fatalf("RED: issue list --json prints no JSON array:\n%s", acsassert.OutputTail(out, 20))
	}
	var rows []listed
	if err := json.Unmarshal([]byte(out[start:end+1]), &rows); err != nil {
		t.Fatalf("RED: issue list --json is not a JSON array of objects: %v\n%s", err, acsassert.OutputTail(out, 20))
	}
	return rows
}

func ids(rows []listed) []string {
	var out []string
	for _, r := range rows {
		out = append(out, r.ID)
	}
	sort.Strings(out)
	return out
}

func listTree(t *testing.T) string {
	t.Helper()
	return incidentTree(t, map[string]string{
		secondRecordID + ".md":         secondRecord(t),
		"2026-08-09-old-incident.md":   "# old\n",
		"REGRESSION-COVERAGE-INDEX.md": "# the index, not a record\n",
	})
}

func TestC2491_009_ListReportsRecordsAndLegacyFilesAsJSON(t *testing.T) {
	rows := listJSON(t, listTree(t))
	got := strings.Join(ids(rows), ",")
	want := strings.Join([]string{"2026-08-09-old-incident", recordID, secondRecordID}, ",")
	if got != want {
		t.Fatalf("RED: issue list --json ids = %s, want %s (the index file is not a record and not listed)", got, want)
	}
	byID := map[string]listed{}
	for _, r := range rows {
		byID[r.ID] = r
	}
	if r := byID[recordID]; r.Status != "fixed" || r.Legacy {
		t.Errorf("RED: record %s = %+v, want status fixed and legacy false", recordID, r)
	}
	if r := byID[secondRecordID]; r.Status != "open" || r.Legacy {
		t.Errorf("RED: record %s = %+v, want status open and legacy false", secondRecordID, r)
	}
	if r := byID["2026-08-09-old-incident"]; !r.Legacy {
		t.Errorf("RED: the legacy file = %+v, want legacy true", r)
	}
}

func TestC2491_010_ListFiltersByStatusAndByCode(t *testing.T) {
	root := listTree(t)
	if got := ids(listJSON(t, root)); len(got) != 3 {
		t.Fatalf("RED: control: unfiltered list has %d rows, want 3: %v", len(got), got)
	}
	if got := ids(listJSON(t, root, "--status", "fixed")); strings.Join(got, ",") != recordID {
		t.Errorf("RED: list --status fixed = %v, want only %s", got, recordID)
	}
	if got := ids(listJSON(t, root, "--status", "open")); strings.Join(got, ",") != secondRecordID {
		t.Errorf("RED: list --status open = %v, want only %s", got, secondRecordID)
	}
	if got := ids(listJSON(t, root, "--code", "LOOP_ORCH_POOL_IDLE")); strings.Join(got, ",") != secondRecordID {
		t.Errorf("RED: list --code LOOP_ORCH_POOL_IDLE = %v, want only %s", got, secondRecordID)
	}
	if got := ids(listJSON(t, root, "--status", "verified")); len(got) != 0 {
		t.Errorf("RED: list --status verified = %v, want no row", got)
	}
}

func TestC2491_011_ListOnTheCurrentTreeReportsEveryOldRecordAsLegacy(t *testing.T) {
	rows := listJSON(t, acsassert.RepoRoot(t))
	legacy := 0
	for _, r := range rows {
		if r.Legacy {
			legacy++
		}
	}
	if legacy < 100 {
		t.Errorf("RED: issue list --json on the current tree reports %d legacy files, want at least 100", legacy)
	}
}

func TestC2491_012_RenderFillsThePartsFromTheFrontmatterAndIsIdempotent(t *testing.T) {
	once := renderedRecord(t, nil)
	twice, err := incidentkb.Render([]byte(once))
	if err != nil {
		t.Fatalf("RED: Render of a rendered record failed: %v", err)
	}
	if string(twice) != once {
		t.Errorf("RED: Render is not idempotent: second pass differs from the first")
	}
	region := func(name string) string {
		begin := strings.Index(once, "<!-- GENERATED:"+name+" BEGIN")
		end := strings.Index(once, "<!-- GENERATED:"+name+" END -->")
		if begin < 0 || end < begin {
			t.Fatalf("RED: rendered record has no %s markers", name)
		}
		return once[begin:end]
	}
	for _, c := range []struct{ region, want string }{
		{"status", "fixed"},
		{"actions", "A1"},
		{"effectiveness", "SHIP_INTEGRATION_EJECTED"},
	} {
		if !strings.Contains(region(c.region), c.want) {
			t.Errorf("RED: the %s part holds no %q", c.region, c.want)
		}
	}
	if !strings.Contains(once, "The fixture analysis.") || !strings.Contains(once, "# fixture record") {
		t.Errorf("RED: Render changed prose outside the generated parts")
	}
	other := renderedRecord(t, func(h string) string { return strings.Replace(h, "status: fixed", "status: reopened", 1) })
	if other == once {
		t.Errorf("RED: Render gives the same bytes for status fixed and status reopened")
	}
}

func TestC2491_013_RenderRefusesADocumentItCannotRender(t *testing.T) {
	if _, err := incidentkb.Render([]byte("# no frontmatter\n")); err == nil {
		t.Errorf("RED: Render accepts a document with no frontmatter")
	}
	noActions := strings.Replace(frontmatter+bodySkeleton, "<!-- GENERATED:actions BEGIN -->\n<!-- GENERATED:actions END -->\n", "", 1)
	_, err := incidentkb.Render([]byte(noActions))
	if err == nil {
		t.Fatalf("RED: Render accepts a document with no actions markers")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "actions") {
		t.Errorf("RED: the Render error does not name the actions part: %v", err)
	}
}

func TestC2491_014_IncidentkbIsALeafWithNoInternalImport(t *testing.T) {
	modDir := acsassert.GoModuleDir(t)
	stdout, stderr, code, _ := acsassert.SubprocessOutput("go", "-C", modDir, "list", "-f", `{{join .Imports "\n"}}`, "./internal/incidentkb")
	if code != 0 {
		t.Fatalf("RED: go list ./internal/incidentkb: exit %d: %s", code, acsassert.OutputTail(stdout+stderr, 10))
	}
	for _, imp := range strings.Fields(stdout) {
		if strings.HasPrefix(imp, modulePath+"/") {
			t.Errorf("RED: incidentkb imports %s; it must be a leaf", imp)
		}
	}
}

func exportedSymbols(t *testing.T, dir string) []string {
	t.Helper()
	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, dir, func(fi os.FileInfo) bool {
		return !strings.HasSuffix(fi.Name(), "_test.go")
	}, 0)
	if err != nil {
		t.Fatalf("RED: parse %s: %v", dir, err)
	}
	var names []string
	for _, pkg := range pkgs {
		for _, file := range pkg.Files {
			for _, decl := range file.Decls {
				switch d := decl.(type) {
				case *ast.FuncDecl:
					if d.Recv == nil && d.Name.IsExported() {
						names = append(names, d.Name.Name)
					}
				case *ast.GenDecl:
					for _, spec := range d.Specs {
						switch s := spec.(type) {
						case *ast.TypeSpec:
							if s.Name.IsExported() {
								names = append(names, s.Name.Name)
							}
						case *ast.ValueSpec:
							for _, n := range s.Names {
								if n.IsExported() {
									names = append(names, n.Name)
								}
							}
						}
					}
				}
			}
		}
	}
	return names
}

// acs-predicate: config-check
func TestC2491_015_IncidentkbGraduatesIntoTheRepoWideApicoverGate(t *testing.T) {
	root := acsassert.RepoRoot(t)
	enforce := filepath.Join(root, "go", ".apicover-enforce")
	body, err := os.ReadFile(enforce)
	if err != nil {
		t.Fatal(err)
	}
	enrolled := false
	for _, line := range strings.Split(string(body), "\n") {
		if strings.TrimSpace(line) == "./internal/incidentkb" {
			enrolled = true
		}
	}
	if !enrolled {
		t.Errorf("RED: go/.apicover-enforce has no ./internal/incidentkb line")
	}
	pkgDir := filepath.Join(root, "go", "internal", "incidentkb")
	symbols := exportedSymbols(t, pkgDir)
	if len(symbols) == 0 {
		t.Fatalf("RED: go/internal/incidentkb exports no symbol")
	}
	named := filepath.Join(pkgDir, "apicover_named_test.go")
	if !acsassert.FileExists(t, named) {
		t.Fatalf("RED: %s is missing", named)
	}
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, named, nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	ast.Inspect(file, func(n ast.Node) bool {
		if id, ok := n.(*ast.Ident); ok {
			seen[id.Name] = true
		}
		return true
	})
	for _, name := range symbols {
		if !seen[name] {
			t.Errorf("RED: apicover_named_test.go does not name the exported symbol %s", name)
		}
	}
}

func TestC2491_016_TheIssueVerbIsReachableFromTheRegistry(t *testing.T) {
	out, code := runIssue(t, t.TempDir(), "list", "--json")
	if code != 0 {
		t.Fatalf("RED: issue list --json on a tree with no docs/incidents: exit %d, want 0 (an empty list); output:\n%s", code, acsassert.OutputTail(out, 20))
	}
	if !strings.Contains(out, "[") {
		t.Errorf("RED: issue list --json on an empty tree prints no JSON array:\n%s", acsassert.OutputTail(out, 20))
	}
	stdout, stderr, _, _ := acsassert.SubprocessOutput(evolveBinary(t), "help")
	if !helpLine.MatchString(stdout + stderr) {
		t.Errorf("RED: evolve help does not list the issue command on a line of its own")
	}
}
