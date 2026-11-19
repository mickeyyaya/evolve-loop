//go:build acs

package cycle2530

import (
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/mickeyyaya/evolve-loop/go/internal/incidentkb"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

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
	evolveBin string
	buildFail string
)

func TestMain(m *testing.M) {
	scratch, err := os.MkdirTemp("", "evolve-c2530-")
	if err != nil {
		buildFail = err.Error()
		os.Exit(m.Run())
	}
	moduleDir, err := acsassert.ModuleDir(".")
	if err != nil {
		buildFail = err.Error()
	} else {
		evolveBin = filepath.Join(scratch, "evolve")
		stdout, stderr, code, _ := acsassert.SubprocessOutput("go", "-C", moduleDir, "build", "-o", evolveBin, "./cmd/evolve")
		if code != 0 {
			buildFail = acsassert.OutputTail(stdout+stderr, 30)
		}
	}
	code := m.Run()
	os.RemoveAll(scratch)
	os.Exit(code)
}

func issueSplit(t *testing.T, root, verb string, extra ...string) (string, string, int) {
	t.Helper()
	if buildFail != "" {
		t.Fatalf("RED: cannot build the evolve binary: %s", buildFail)
	}
	stdout, stderr, code, _ := acsassert.SubprocessOutput(evolveBin, append([]string{"issue", verb, "--project-root", root}, extra...)...)
	return stdout, stderr, code
}

func issue(t *testing.T, root, verb string, extra ...string) (string, int) {
	t.Helper()
	stdout, stderr, code := issueSplit(t, root, verb, extra...)
	return stdout + "\n" + stderr, code
}

func plant(t *testing.T, files map[string]string) string {
	t.Helper()
	tree := fstest.MapFS{}
	for rel, body := range files {
		tree["docs/incidents/"+rel] = &fstest.MapFile{Data: []byte(body)}
	}
	root := acsassert.SetupTempProject(t)
	if err := os.CopyFS(root, tree); err != nil {
		t.Fatalf("plant: %v", err)
	}
	return root
}

func render(t *testing.T, head, body string) string {
	t.Helper()
	out, err := incidentkb.Render([]byte(head + body))
	if err != nil {
		t.Fatalf("RED: incidentkb.Render of a well-formed record failed: %v", err)
	}
	return string(out)
}

func goodRecord(t *testing.T) string {
	return render(t, frontmatter, bodySkeleton)
}

func openRecord(t *testing.T) string {
	head := strings.NewReplacer(
		"id: "+recordID, "id: "+secondRecordID,
		"status: fixed", "status: open",
		"state: done", "state: open",
		"SHIP_INTEGRATION_EJECTED", "LOOP_ORCH_POOL_IDLE",
	).Replace(frontmatter)
	return render(t, head, bodySkeleton)
}

func mustPass(t *testing.T, root, label string) {
	t.Helper()
	out, code := issue(t, root, "check")
	if code != 0 {
		t.Fatalf("RED: %s: issue check exit %d, want 0; output:\n%s", label, code, acsassert.OutputTail(out, 20))
	}
}

func mustFailNaming(t *testing.T, root, label string, needles ...string) {
	t.Helper()
	out, code := issue(t, root, "check")
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

func checkPair(t *testing.T, control, broken map[string]string, label string, needles ...string) {
	t.Helper()
	mustPass(t, plant(t, control), "control for "+label)
	mustFailNaming(t, plant(t, broken), label, needles...)
}

func oneRecord(content string) map[string]string {
	return map[string]string{recordID + ".md": content}
}

func TestC2530_001_IssueCheckPassesOnTheCurrentTreeAndReportsLegacyFiles(t *testing.T) {
	out, code := issue(t, acsassert.RepoRoot(t), "check")
	if code != 0 {
		t.Fatalf("RED: issue check on the current tree: exit %d, want 0; output:\n%s", code, acsassert.OutputTail(out, 20))
	}
	if !strings.Contains(strings.ToLower(out), "legacy") {
		t.Errorf("RED: issue check on the current tree does not report the legacy files of docs/incidents; output:\n%s", acsassert.OutputTail(out, 20))
	}
}

func TestC2530_002_IssueCheckRefusesANewIncidentFileWithNoFrontmatter(t *testing.T) {
	control := oneRecord(goodRecord(t))
	control["2026-08-09-old-incident.md"] = "# an old incident\n\nNo frontmatter, dated before 2026-11-19.\n"
	newName := "2026-11-19-new-incident-without-frontmatter"
	broken := map[string]string{newName + ".md": "# a new incident\n\nNo frontmatter.\n"}
	for name, body := range control {
		broken[name] = body
	}
	checkPair(t, control, broken, "a new dated file with no frontmatter", newName)
}

func TestC2530_003_AHandEditOfTheRenderedActionsSectionFailsNamingTheField(t *testing.T) {
	good := goodRecord(t)
	edited := strings.Replace(good, "<!-- GENERATED:actions END -->", "| A9 | a hand-written action row |\n<!-- GENERATED:actions END -->", 1)
	checkPair(t, oneRecord(good), oneRecord(edited), "a hand edit inside the actions markers", recordID, "actions")
}

func TestC2530_004_AHandEditOfAnyRenderedSectionFailsNamingItsField(t *testing.T) {
	good := goodRecord(t)
	for _, field := range []string{"status", "actions", "effectiveness"} {
		t.Run(field, func(t *testing.T) {
			end := "<!-- GENERATED:" + field + " END -->"
			edited := strings.Replace(good, end, "a hand-written line\n"+end, 1)
			checkPair(t, oneRecord(good), oneRecord(edited), "a hand edit inside the "+field+" markers", field)
		})
	}
}

func TestC2530_005_AnEditOutsideTheGeneratedPartsStillPasses(t *testing.T) {
	good := goodRecord(t)
	edited := strings.Replace(good, "The fixture analysis.", "The analysis that a person rewrote.", 1)
	if edited == good {
		t.Fatalf("RED: the fixture has no prose line to rewrite")
	}
	mustPass(t, plant(t, oneRecord(edited)), "a hand edit of a prose section")
}

func TestC2530_006_AFrontmatterEditWithoutARenderFailsNamingTheField(t *testing.T) {
	good := goodRecord(t)
	edited := strings.Replace(good, "status: fixed", "status: reopened", 1)
	checkPair(t, oneRecord(good), oneRecord(edited), "a status edit that was not rendered", recordID, "status")
}

func TestC2530_007_AMissingSectionHeadingFailsNamingTheSection(t *testing.T) {
	good := goodRecord(t)
	edited := strings.Replace(good, "## Approaches taken\n", "", 1)
	checkPair(t, oneRecord(good), oneRecord(edited), "a record without its Approaches taken heading", recordID, "Approaches taken")
}

func TestC2530_008_ABadFrontmatterFieldFailsNamingTheField(t *testing.T) {
	good := goodRecord(t)
	cases := []struct{ field, from, to string }{
		{"schema", "incident-record/1", "incident-record/9"},
		{"level", "level: record", "level: urgent"},
		{"status", "status: fixed", "status: banana"},
		{"title", "title: fixture record\n", ""},
		{"id", "id: " + recordID, "id: 2026-11-19-another-name"},
	}
	for _, c := range cases {
		t.Run(c.field, func(t *testing.T) {
			edited := strings.Replace(good, c.from, c.to, 1)
			if edited == good {
				t.Fatalf("RED: the rendered fixture has no %q to change", c.from)
			}
			checkPair(t, oneRecord(good), oneRecord(edited), "a record with a bad "+c.field, c.field)
		})
	}
}

func TestC2530_009_TheLegacyRuleAndTheNameSkips(t *testing.T) {
	skipped := map[string]string{
		"2026-08-09-old-incident.md":            "# old\n",
		"cycle-102-111.md":                      "# an old record with no date prefix\n",
		"REGRESSION-COVERAGE-INDEX.md":          "# the index, not a record\n",
		"2026-11-20-session-log.md":             "# a session log, not a record\n",
		"2026-11-18-the-day-before-the-rule.md": "# dated before the first day of the rule\n",
	}
	firstDay := "2026-11-19-first-day-of-the-rule"
	broken := map[string]string{firstDay + ".md": "# a new record with no frontmatter\n"}
	for name, body := range skipped {
		broken[name] = body
	}
	checkPair(t, skipped, broken, "a file dated on the first day of the rule", firstDay)
}

type listed struct {
	ID     string `json:"id"`
	Status string `json:"status"`
	Legacy bool   `json:"legacy"`
}

func listRows(t *testing.T, root string, extra ...string) []listed {
	t.Helper()
	out, errOut, code := issueSplit(t, root, "list", append([]string{"--json"}, extra...)...)
	if code != 0 {
		t.Fatalf("RED: issue list --json %v: exit %d; output:\n%s", extra, code, acsassert.OutputTail(out+"\n"+errOut, 20))
	}
	start, end := strings.Index(out, "["), strings.LastIndex(out, "]")
	if start < 0 || end < start {
		t.Fatalf("RED: issue list --json prints no JSON array:\n%s", acsassert.OutputTail(out, 20))
	}
	var rows []listed
	if err := json.Unmarshal([]byte(out[start:end+1]), &rows); err != nil {
		t.Fatalf("RED: issue list --json is not a JSON array of objects: %v\n%s", err, acsassert.OutputTail(out, 20))
	}
	return rows
}

func idsOf(rows []listed) string {
	names := make([]string, 0, len(rows))
	for _, r := range rows {
		names = append(names, r.ID)
	}
	sort.Strings(names)
	return strings.Join(names, ",")
}

func mixedTree(t *testing.T) string {
	return plant(t, map[string]string{
		recordID + ".md":               goodRecord(t),
		secondRecordID + ".md":         openRecord(t),
		"2026-08-09-old-incident.md":   "# old\n",
		"REGRESSION-COVERAGE-INDEX.md": "# the index, not a record\n",
	})
}

func TestC2530_010_ListReportsRecordsAndLegacyFilesAsJSON(t *testing.T) {
	rows := listRows(t, mixedTree(t))
	want := strings.Join([]string{"2026-08-09-old-incident", recordID, secondRecordID}, ",")
	if got := idsOf(rows); got != want {
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

func TestC2530_011_ListFiltersByStatusAndByCode(t *testing.T) {
	root := mixedTree(t)
	if got := listRows(t, root); len(got) != 3 {
		t.Fatalf("RED: control: unfiltered list has %d rows, want 3: %s", len(got), idsOf(got))
	}
	cases := []struct {
		flag, value, want string
	}{
		{"--status", "fixed", recordID},
		{"--status", "open", secondRecordID},
		{"--code", "LOOP_ORCH_POOL_IDLE", secondRecordID},
		{"--code", "SHIP_INTEGRATION_EJECTED", recordID},
		{"--status", "verified", ""},
	}
	for _, c := range cases {
		if got := idsOf(listRows(t, root, c.flag, c.value)); got != c.want {
			t.Errorf("RED: list %s %s = [%s], want [%s]", c.flag, c.value, got, c.want)
		}
	}
}

func TestC2530_012_ListOnTheCurrentTreeReportsTheOldRecordsAsLegacy(t *testing.T) {
	legacy := 0
	for _, r := range listRows(t, acsassert.RepoRoot(t)) {
		if r.Legacy {
			legacy++
		}
	}
	if legacy < 100 {
		t.Errorf("RED: issue list --json on the current tree reports %d legacy files, want at least 100", legacy)
	}
}

func TestC2530_013_RenderFillsThePartsFromTheFrontmatterAndIsIdempotent(t *testing.T) {
	once := goodRecord(t)
	twice, err := incidentkb.Render([]byte(once))
	if err != nil {
		t.Fatalf("RED: Render of a rendered record failed: %v", err)
	}
	if string(twice) != once {
		t.Errorf("RED: Render is not idempotent: the second pass differs from the first")
	}
	for _, c := range []struct{ part, want string }{
		{"status", "fixed"},
		{"actions", "A1"},
		{"effectiveness", "SHIP_INTEGRATION_EJECTED"},
	} {
		begin := strings.Index(once, "<!-- GENERATED:"+c.part+" BEGIN")
		end := strings.Index(once, "<!-- GENERATED:"+c.part+" END -->")
		if begin < 0 || end < begin {
			t.Fatalf("RED: the rendered record has no %s markers", c.part)
		}
		if !strings.Contains(once[begin:end], c.want) {
			t.Errorf("RED: the %s part holds no %q", c.part, c.want)
		}
	}
	if !strings.Contains(once, "The fixture analysis.") || !strings.Contains(once, "# fixture record") {
		t.Errorf("RED: Render changed prose outside the generated parts")
	}
	reopened := render(t, strings.Replace(frontmatter, "status: fixed", "status: reopened", 1), bodySkeleton)
	if reopened == once {
		t.Errorf("RED: Render gives the same bytes for status fixed and status reopened")
	}
}

func TestC2530_014_RenderRefusesADocumentItCannotRender(t *testing.T) {
	if _, err := incidentkb.Render([]byte("# no frontmatter\n")); err == nil {
		t.Errorf("RED: Render accepts a document with no frontmatter")
	}
	noActions := strings.Replace(bodySkeleton, "<!-- GENERATED:actions BEGIN -->\n<!-- GENERATED:actions END -->\n", "", 1)
	_, err := incidentkb.Render([]byte(frontmatter + noActions))
	if err == nil {
		t.Fatalf("RED: Render accepts a document with no actions markers")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "actions") {
		t.Errorf("RED: the Render error does not name the actions part: %v", err)
	}
}

func TestC2530_015_IncidentkbIsALeafWithNoInternalImport(t *testing.T) {
	stdout, stderr, code, _ := acsassert.SubprocessOutput("go", "-C", acsassert.GoModuleDir(t), "list", "-f", `{{join .Imports "\n"}}`, "./internal/incidentkb")
	if code != 0 {
		t.Fatalf("RED: go list ./internal/incidentkb: exit %d: %s", code, acsassert.OutputTail(stdout+stderr, 10))
	}
	for _, imp := range strings.Fields(stdout) {
		if strings.HasPrefix(imp, modulePath+"/") {
			t.Errorf("RED: incidentkb imports %s; it must be a leaf", imp)
		}
	}
}

func exportedNames(t *testing.T, dir string) []string {
	t.Helper()
	pkgs, err := parser.ParseDir(token.NewFileSet(), dir, func(fi os.FileInfo) bool {
		return !strings.HasSuffix(fi.Name(), "_test.go")
	}, 0)
	if err != nil {
		t.Fatalf("RED: parse %s: %v", dir, err)
	}
	var names []string
	for _, pkg := range pkgs {
		ast.Inspect(pkg, func(n ast.Node) bool {
			switch d := n.(type) {
			case *ast.FuncDecl:
				if d.Recv == nil && d.Name.IsExported() {
					names = append(names, d.Name.Name)
				}
			case *ast.TypeSpec:
				if d.Name.IsExported() {
					names = append(names, d.Name.Name)
				}
			case *ast.ValueSpec:
				for _, id := range d.Names {
					if id.IsExported() {
						names = append(names, id.Name)
					}
				}
			}
			return true
		})
	}
	return names
}

// acs-predicate: config-check
func TestC2530_016_IncidentkbGraduatesIntoTheRepoWideApicoverGate(t *testing.T) {
	root := acsassert.RepoRoot(t)
	if !acsassert.FileContainsAny(filepath.Join(root, "go", ".apicover-enforce"), "\n./internal/incidentkb\n") {
		t.Errorf("RED: go/.apicover-enforce has no ./internal/incidentkb line")
	}
	pkgDir := filepath.Join(root, "go", "internal", "incidentkb")
	symbols := exportedNames(t, pkgDir)
	if len(symbols) == 0 {
		t.Fatalf("RED: go/internal/incidentkb exports no symbol")
	}
	named := filepath.Join(pkgDir, "apicover_named_test.go")
	file, err := parser.ParseFile(token.NewFileSet(), named, nil, 0)
	if err != nil {
		t.Fatalf("RED: %v", err)
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

func TestC2530_017_ListOnATreeWithNoIncidentsPrintsAnEmptyArray(t *testing.T) {
	rows := listRows(t, t.TempDir())
	if len(rows) != 0 {
		t.Errorf("RED: issue list --json on a tree with no docs/incidents has %d rows, want 0", len(rows))
	}
}
