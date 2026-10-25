package skillcheck

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const sharedFixture = "# AGENTS.md\n\n> Read this first.\n\n" + digestBegin + "## Invariants\n\nThe pipeline is the only controller.\n" + digestEnd + "\n\n## Full rules\n\nThe long text stays in AGENTS.md.\n"

func writeRel(t *testing.T, root, rel, body string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func readRel(t *testing.T, root, rel string) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

func geminiRender() []instructionRender {
	return []instructionRender{{target: "GEMINI.md", source: sharedSource, body: digestBody}}
}

func TestInstructionDiffs_TheCheckedInGeminiFileMatchesItsRender(t *testing.T) {
	diffs, err := instructionDiffs(repoRoot(t))
	if err != nil {
		t.Fatalf("instructionDiffs: %v", err)
	}
	seen := map[string]bool{}
	for _, d := range diffs {
		seen[d.rel] = true
		if d.drifted {
			t.Errorf("%s differs from its render (run `evolve skills generate`)", d.rel)
		}
	}
	for _, r := range instructionRenders() {
		if !seen[r.target] {
			t.Errorf("%s has no render: its source %s is missing", r.target, r.source)
		}
	}
}

func TestInstructionRenders_CoverEachCLIFileAndEachReviewer(t *testing.T) {
	want := map[string]string{
		"CLAUDE.md": "AGENTS.md",
		"GEMINI.md": "AGENTS.md",
		".claude/agents/architecture-reviewer.md": "agents/evolve-diff-architecture-reviewer.md",
		".claude/agents/code-reviewer.md":         "agents/evolve-diff-code-reviewer.md",
		".claude/agents/code-simplifier.md":       "agents/evolve-diff-code-simplifier.md",
		".claude/agents/go-reviewer.md":           "agents/evolve-diff-go-reviewer.md",
	}
	got := map[string]string{}
	for _, r := range instructionRenders() {
		got[r.target] = r.source
	}
	if len(got) != len(want) {
		t.Fatalf("renders = %v, want %v", got, want)
	}
	for target, source := range want {
		if got[target] != source {
			t.Errorf("%s renders from %q, want %q", target, got[target], source)
		}
	}
}

func TestInstructionDiffs_AStaleGeminiRegionDriftsAndGenerateRewritesIt(t *testing.T) {
	root := t.TempDir()
	writeRel(t, root, "AGENTS.md", sharedFixture)
	writeRel(t, root, "GEMINI.md", "# Gemini overlay\n\n"+instructionBlock(sharedSource, "## Old\n"))

	diffs, err := instructionDiffsFor(root, geminiRender())
	if err != nil {
		t.Fatal(err)
	}
	if len(diffs) != 1 || !diffs[0].drifted {
		t.Fatalf("want GEMINI.md drifted, got %+v", diffs)
	}
	var out, errBuf bytes.Buffer
	if drift, err := projectCommandDiffs(diffs, false, &out, &errBuf); err != nil || !drift {
		t.Fatalf("check: drift=%v err=%v", drift, err)
	}
	if !strings.Contains(errBuf.String(), "DRIFT: GEMINI.md") {
		t.Errorf("check stderr = %q, want a DRIFT line that names GEMINI.md", errBuf.String())
	}
	if _, err := projectCommandDiffs(diffs, true, &out, &errBuf); err != nil {
		t.Fatal(err)
	}
	got := readRel(t, root, "GEMINI.md")
	want := "# Gemini overlay\n\n" + instructionBlock(sharedSource, "## Invariants\n\nThe pipeline is the only controller.\n")
	if got != want {
		t.Fatalf("generated GEMINI.md =\n%s\nwant\n%s", got, want)
	}
	again, err := instructionDiffsFor(root, geminiRender())
	if err != nil || again[0].drifted {
		t.Fatalf("after generate: drifted=%v err=%v", again[0].drifted, err)
	}
}

func TestInstructionDiffs_APersonaRendersWithoutItsFrontmatter(t *testing.T) {
	root := t.TempDir()
	writeRel(t, root, "agents/evolve-diff-go-reviewer.md", "---\nname: evolve-diff-go-reviewer\n---\n\nYou review the diff.\n")
	writeRel(t, root, ".claude/agents/go-reviewer.md", "---\nname: go-reviewer\ntools: [Read]\n---\n\n")
	r := []instructionRender{{target: ".claude/agents/go-reviewer.md", source: "agents/evolve-diff-go-reviewer.md", body: personaBody}}

	diffs, err := instructionDiffsFor(root, r)
	if err != nil {
		t.Fatal(err)
	}
	want := "---\nname: go-reviewer\ntools: [Read]\n---\n\n" + instructionBlock("agents/evolve-diff-go-reviewer.md", "You review the diff.\n")
	if diffs[0].next != want {
		t.Fatalf("render =\n%s\nwant\n%s", diffs[0].next, want)
	}
}

func TestInstructionDiffs_AnAbsentSourceOrTargetIsSkipped(t *testing.T) {
	cases := map[string]string{"AGENTS.md": sharedFixture, "GEMINI.md": "# Gemini\n"}
	for present, body := range cases {
		t.Run("only "+present, func(t *testing.T) {
			root := t.TempDir()
			writeRel(t, root, present, body)
			diffs, err := instructionDiffsFor(root, geminiRender())
			if err != nil || len(diffs) != 0 {
				t.Fatalf("diffs=%v err=%v, want none", diffs, err)
			}
		})
	}
}

func TestInstructionDiffs_Errors(t *testing.T) {
	cases := []struct {
		name  string
		setup func(t *testing.T, root string)
		want  string
	}{
		{"unreadable source", func(t *testing.T, root string) {
			if err := os.MkdirAll(filepath.Join(root, "AGENTS.md"), 0o755); err != nil {
				t.Fatal(err)
			}
			writeRel(t, root, "GEMINI.md", "# Gemini\n")
		}, "read the render GEMINI.md from AGENTS.md"},
		{"unreadable target", func(t *testing.T, root string) {
			writeRel(t, root, "AGENTS.md", sharedFixture)
			if err := os.MkdirAll(filepath.Join(root, "GEMINI.md"), 0o755); err != nil {
				t.Fatal(err)
			}
		}, "read the render GEMINI.md from AGENTS.md"},
		{"source with no digest region", func(t *testing.T, root string) {
			writeRel(t, root, "AGENTS.md", "# AGENTS.md\n")
			writeRel(t, root, "GEMINI.md", "# Gemini\n")
		}, "AGENTS.md: no instructions-digest region"},
		{"source with the digest END only before its BEGIN", func(t *testing.T, root string) {
			writeRel(t, root, "AGENTS.md", digestEnd+"\n"+digestBegin)
			writeRel(t, root, "GEMINI.md", "# Gemini\n")
		}, "AGENTS.md: no instructions-digest region"},
		{"target with an unclosed region", func(t *testing.T, root string) {
			writeRel(t, root, "AGENTS.md", sharedFixture)
			writeRel(t, root, "GEMINI.md", instructionsBegin+" -->\n")
		}, "GEMINI.md: found"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			tc.setup(t, root)
			_, err := instructionDiffsFor(root, geminiRender())
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("err = %v, want it to contain %q", err, tc.want)
			}
		})
	}
}

func TestPersonaBody_AnUnterminatedFrontmatterIsAnError(t *testing.T) {
	if _, err := personaBody("---\nname: x\n"); err == nil {
		t.Fatal("want an error for an unterminated frontmatter")
	}
}

func TestRunAndCheck_ReportAnInstructionFileDrift(t *testing.T) {
	root := prepareSkillsTree(t)
	writeRel(t, root, "AGENTS.md", sharedFixture)
	writeRel(t, root, "GEMINI.md", "# Gemini overlay\n")

	drift, err := Check(root)
	if err != nil {
		t.Fatal(err)
	}
	if !containsString(drift, "GEMINI.md") {
		t.Fatalf("Check = %v, want GEMINI.md", drift)
	}
	var out, errBuf bytes.Buffer
	if code := Run(root, false, &out, &errBuf); code != 2 {
		t.Fatalf("check exit = %d, want 2; stderr=%s", code, errBuf.String())
	}
	if code := Run(root, true, &out, &errBuf); code != 0 {
		t.Fatalf("generate exit = %d; stderr=%s", code, errBuf.String())
	}
	if !strings.Contains(readRel(t, root, "GEMINI.md"), "The pipeline is the only controller.") {
		t.Fatal("generate did not render the shared body into GEMINI.md")
	}
}

func TestRunAndCheck_FailOnAnInstructionRenderError(t *testing.T) {
	root := prepareSkillsTree(t)
	writeRel(t, root, "AGENTS.md", "# AGENTS.md\n")
	writeRel(t, root, "GEMINI.md", "# Gemini\n")

	if _, err := Check(root); err == nil {
		t.Fatal("Check: want an error for a source with no digest region")
	}
	var out, errBuf bytes.Buffer
	if code := Run(root, false, &out, &errBuf); code != 1 {
		t.Fatalf("check exit = %d, want 1", code)
	}
}

func containsString(list []string, want string) bool {
	for _, s := range list {
		if s == want {
			return true
		}
	}
	return false
}

func TestCheck_AnUnlistableSkillsDirectoryIsAnError(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("root lists a directory with no read permission")
	}
	root := prepareSkillsTree(t)
	writeRel(t, root, ".claude-plugin/plugin.json", `{"skills": [], "agents": []}`)
	skills := filepath.Join(root, "skills")
	if err := os.Chmod(skills, 0o311); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(skills, 0o755) })

	_, err := Check(root)
	if err == nil || !strings.Contains(err.Error(), "read skills dir") {
		t.Fatalf("err = %v, want the skills directory read error", err)
	}
}

func TestInstructionBudget_TheCheckedInCLIFilesStayWithinTheirByteBudget(t *testing.T) {
	root := repoRoot(t)
	for _, target := range []string{"CLAUDE.md", "GEMINI.md"} {
		if size := len(readRel(t, root, target)); size > cliFileByteBudget {
			t.Errorf("%s is %d bytes; the budget is %d bytes", target, size, cliFileByteBudget)
		}
	}
	diffs, err := instructionDiffs(root)
	if problems := budgetProblems(diffs, instructionRenders()); err != nil || len(problems) != 0 {
		t.Fatalf("problems=%v err=%v, want none", problems, err)
	}
}

func TestInstructionBudget_ARenderOverItsBudgetIsAProblem(t *testing.T) {
	root := t.TempDir()
	writeRel(t, root, "AGENTS.md", sharedFixture)
	writeRel(t, root, "GEMINI.md", "# Gemini\n")
	render := geminiRender()
	render[0].byteBudget = 10

	diffs, err := instructionDiffsFor(root, render)
	if err != nil {
		t.Fatal(err)
	}
	problems := budgetProblems(diffs, render)
	if len(problems) != 1 || !strings.HasPrefix(problems[0], "BUDGET: GEMINI.md renders to ") || !strings.Contains(problems[0], "the budget is 10 bytes") {
		t.Fatalf("problems = %v, want one BUDGET line for GEMINI.md", problems)
	}
	render[0].byteBudget = 0
	if problems := budgetProblems(diffs, render); len(problems) != 0 {
		t.Fatalf("a render with no budget has problems %v", problems)
	}
}

func TestRunAndCheck_ReportAnOverBudgetCLIFile(t *testing.T) {
	root := prepareSkillsTree(t)
	long := strings.Repeat("The pipeline is the only controller. ", cliFileByteBudget/30)
	writeRel(t, root, "AGENTS.md", digestBegin+long+"\n"+digestEnd+"\n")
	writeRel(t, root, "CLAUDE.md", "# Claude\n")

	var out, errBuf bytes.Buffer
	if code := Run(root, true, &out, &errBuf); code != 0 {
		t.Fatalf("generate exit = %d; stderr=%s", code, errBuf.String())
	}
	errBuf.Reset()
	if code := Run(root, false, &out, &errBuf); code != 2 || !strings.Contains(errBuf.String(), "BUDGET: CLAUDE.md") {
		t.Fatalf("check exit = %d, stderr = %q; want 2 and a BUDGET line", code, errBuf.String())
	}
	drift, err := Check(root)
	if err != nil || len(drift) != 1 || !strings.HasPrefix(drift[0], "BUDGET: CLAUDE.md") {
		t.Fatalf("Check = %v, %v; want the BUDGET problem only", drift, err)
	}
}
