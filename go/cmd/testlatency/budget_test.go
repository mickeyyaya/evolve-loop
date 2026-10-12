package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const budgetStream = `
{"Action":"run","Package":"x/over","Test":"TestA"}
{"Action":"pass","Package":"x/over","Test":"TestA","Elapsed":200}
{"Action":"pass","Package":"x/over","Elapsed":200.5}
{"Action":"pass","Package":"x/at","Test":"TestB","Elapsed":180}
{"Action":"pass","Package":"x/at","Elapsed":180}
{"Action":"pass","Package":"x/under","Elapsed":179.99}
{"Action":"fail","Package":"x/failed","Test":"TestC","Elapsed":250}
{"Action":"fail","Package":"x/failed","Elapsed":300}
{"Action":"run","Package":"x/noelapsed","Test":"TestD"}
{"Action":"pass","Package":"x/zero","Elapsed":0}
`

func parseBudgetStream(t *testing.T) *Report {
	t.Helper()
	rep, err := Parse(strings.NewReader(budgetStream))
	if err != nil {
		t.Fatal(err)
	}
	return rep
}

func TestOverBudget_SelectsStrictlyOverSortedByWall(t *testing.T) {
	got := parseBudgetStream(t).OverBudget(180)
	if len(got) != 2 || got[0].Pkg != "x/failed" || got[1].Pkg != "x/over" {
		t.Fatalf("over budget = %+v, want x/failed then x/over (a package at the budget or with no Elapsed is not over)", got)
	}
}

func TestOverBudget_ZeroBudgetDisablesTheCheck(t *testing.T) {
	if got := parseBudgetStream(t).OverBudget(0); len(got) != 0 {
		t.Fatalf("a zero budget flagged %+v", got)
	}
}

func TestMarkdown_BudgetTableLeadsTheReport(t *testing.T) {
	md := parseBudgetStream(t).Markdown(MarkdownOptions{Title: "T", Top: 3, ThresholdPkg: 5, ThresholdTst: 1, Budget: 180})
	want := "# T\n\n" +
		"## Packages over budget (> 180.0s wall)\n\n" +
		"| Package | Wall (s) | Over by (s) | Status |\n" +
		"|---|--:|--:|---|\n" +
		"| x/failed | 300.00 | 120.00 | fail |\n" +
		"| x/over | 200.50 | 20.50 | pass |\n\n" +
		"- Packages: **"
	if !strings.HasPrefix(md, want) {
		t.Fatalf("report head:\n%s\nwant prefix:\n%s", md, want)
	}
}

func TestMarkdown_BudgetWithNoOffenderSaysSo(t *testing.T) {
	rep, err := Parse(strings.NewReader(`{"Action":"pass","Package":"x/ok","Elapsed":1}`))
	if err != nil {
		t.Fatal(err)
	}
	md := rep.Markdown(MarkdownOptions{Title: "T", Top: 1, ThresholdPkg: 5, ThresholdTst: 1, Budget: 180})
	if !strings.Contains(md, "## Packages over budget (> 180.0s wall)\n\n_(none)_ All packages are within the budget.\n\n") {
		t.Fatalf("missing the within-budget line:\n%s", md)
	}
}

func TestMarkdown_NoBudgetOmitsTheSection(t *testing.T) {
	md := parseBudgetStream(t).Markdown(MarkdownOptions{Title: "T", Top: 3, ThresholdPkg: 5, ThresholdTst: 1})
	if strings.Contains(md, "over budget") {
		t.Fatalf("budget section rendered with no budget:\n%s", md)
	}
}

func runLatency(t *testing.T, stdin string, args ...string) (int, string, string) {
	t.Helper()
	var out, errOut bytes.Buffer
	code := run(args, strings.NewReader(stdin), &out, &errOut)
	return code, out.String(), errOut.String()
}

func TestRun_BudgetIsAdvisoryByDefault(t *testing.T) {
	code, out, _ := runLatency(t, budgetStream, "--budget", "180s")
	if code != 0 || !strings.Contains(out, "| x/over | 200.50 |") {
		t.Fatalf("exit %d, want 0 with the over-budget table:\n%s", code, out)
	}
}

func TestRun_FailOverBudgetExitsOneOnlyWhenOver(t *testing.T) {
	if code, out, _ := runLatency(t, budgetStream, "--budget", "180s", "--fail-over-budget"); code != 1 || !strings.Contains(out, "x/over") {
		t.Fatalf("exit %d, want 1 with the report still written:\n%s", code, out)
	}
	if code, _, _ := runLatency(t, budgetStream, "--budget", "301s", "--fail-over-budget"); code != 0 {
		t.Fatalf("exit %d, want 0 when no package is over", code)
	}
}

func TestRun_FailOverBudgetNeedsABudget(t *testing.T) {
	code, out, errOut := runLatency(t, budgetStream, "--fail-over-budget")
	if code != 2 || out != "" || !strings.Contains(errOut, "--fail-over-budget needs --budget") {
		t.Fatalf("exit %d out %q err %q", code, out, errOut)
	}
}

func TestRun_RejectsABadFlag(t *testing.T) {
	if code, _, _ := runLatency(t, "", "--budget", "soon"); code != 2 {
		t.Fatalf("exit %d, want 2", code)
	}
}

func TestRun_ReportsAStreamReadError(t *testing.T) {
	var out, errOut bytes.Buffer
	code := run(nil, &failingReader{data: []byte("{}\n")}, &out, &errOut)
	if code != 1 || !strings.Contains(errOut.String(), "testlatency: scan test2json stream") {
		t.Fatalf("exit %d err %q", code, errOut.String())
	}
}

func TestRun_DefaultsMatchTheEarlierReport(t *testing.T) {
	code, out, _ := runLatency(t, characterizationStream)
	if code != 0 || !strings.HasPrefix(out, "# Test latency report\n\n- Packages: **5**\n") || strings.Contains(out, "over budget") {
		t.Fatalf("exit %d:\n%s", code, out)
	}
}

func TestMain_PipesTheProcessStreamsThroughRunAndExitsWithItsCode(t *testing.T) {
	dir := t.TempDir()
	in, err := os.Create(filepath.Join(dir, "in.json"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := in.WriteString(budgetStream); err != nil {
		t.Fatal(err)
	}
	if _, err := in.Seek(0, 0); err != nil {
		t.Fatal(err)
	}
	outPath := filepath.Join(dir, "out.md")
	out, err := os.Create(outPath)
	if err != nil {
		t.Fatal(err)
	}
	var code = -1
	oldExit, oldArgs, oldIn, oldOut := exitProcess, os.Args, os.Stdin, os.Stdout
	t.Cleanup(func() { exitProcess, os.Args, os.Stdin, os.Stdout = oldExit, oldArgs, oldIn, oldOut })
	exitProcess = func(c int) { code = c }
	os.Args = []string{"testlatency", "--budget", "180s", "--fail-over-budget"}
	os.Stdin, os.Stdout = in, out

	main()

	os.Stdin, os.Stdout = oldIn, oldOut
	got, readErr := os.ReadFile(outPath)
	if readErr != nil || code != 1 || !strings.Contains(string(got), "| x/over | 200.50 |") {
		t.Fatalf("exit %d, report %q, %v; want exit 1 and the over-budget table", code, got, readErr)
	}
}
