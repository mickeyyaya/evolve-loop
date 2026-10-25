package core

import (
	"context"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/isolation"
)

func judgeRow(t *testing.T, id string, failure error) {
	t.Helper()
	note, err := isolation.Judge(id, failure)
	if err != nil {
		t.Fatal(err)
	}
	t.Log(note)
}

func TestIsolationMatrix_N12_AJudgeVerdictInProseWithNoFileFailsTheContract(t *testing.T) {
	t.Parallel()
	in := judgeInput()
	in.Workspace = t.TempDir()
	fb := &fakeBridge{stdout: `I looked at the plan. My verdict: {"score":0.8,"rationale":"fine","missing_phases":[]}`}
	var failure error
	v := NewPlanJudge(fb).GradePlan(context.Background(), in, samplePlan())
	if _, err := os.Stat(filepath.Join(in.Workspace, "routing-judge.json")); !os.IsNotExist(err) {
		t.Fatalf("inconclusive: the row needs a judge that wrote no verdict file, got %v", err)
	}
	if v.Score != judgeNoOpinion.Score {
		failure = fmt.Errorf("the plan judge took score %.1f from a JSON span in its stdout prose; %s was never written", v.Score, filepath.Join(in.Workspace, "routing-judge.json"))
	}
	judgeRow(t, "N12", failure)
}

var familyWords = map[string]bool{
	"claude": true, "claude-p": true, "claude-tmux": true,
	"agy": true, "agy-tmux": true, "agy-claude": true, "agy-claude-tmux": true,
	"codex": true, "codex-tmux": true, "ollama": true, "ollama-tmux": true, "gemini": true,
}

func familyLiteralsIn(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var hits []string
	fset := token.NewFileSet()
	for _, e := range entries {
		name := e.Name()
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, filepath.Join(dir, name), nil, parser.SkipObjectResolution)
		if err != nil {
			return nil, err
		}
		ast.Inspect(f, func(n ast.Node) bool {
			lit, ok := n.(*ast.BasicLit)
			if !ok || lit.Kind != token.STRING {
				return true
			}
			if s, err := strconv.Unquote(lit.Value); err == nil && familyWords[s] {
				hits = append(hits, fmt.Sprintf("%s %q", fset.Position(lit.Pos()), s))
			}
			return true
		})
	}
	return hits, nil
}

func TestFamilyLiteralsIn_FindsAFamilyStringAndSkipsTestsAndOtherWords(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	for name, body := range map[string]string{
		"flow.go":      "package p\n\nvar cli = \"claude-tmux\"\nvar other = \"opus\"\n",
		"flow_test.go": "package p\n\nvar fake = \"agy\"\n",
		"notes.txt":    "codex",
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	hits, err := familyLiteralsIn(dir)
	if err != nil || len(hits) != 1 || !strings.Contains(hits[0], `flow.go:3:11 "claude-tmux"`) {
		t.Fatalf("hits = %v, err = %v; want only the claude-tmux literal of flow.go", hits, err)
	}
	if _, err := familyLiteralsIn(filepath.Join(dir, "absent")); err == nil {
		t.Fatal("an absent directory must fail, not report a clean tree")
	}
	if err := os.WriteFile(filepath.Join(dir, "broken.go"), []byte("package"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := familyLiteralsIn(dir); err == nil {
		t.Fatal("a file that does not parse must fail, not report a clean tree")
	}
}

func TestIsolationMatrix_N13_FlowCodeNamesNoFamily(t *testing.T) {
	t.Parallel()
	hits, err := familyLiteralsIn(".")
	if err != nil {
		t.Fatal(err)
	}
	var failure error
	if len(hits) > 0 {
		failure = fmt.Errorf("internal/core names a family in %d string literal(s), first %s", len(hits), hits[0])
	}
	judgeRow(t, "N13", failure)
}
