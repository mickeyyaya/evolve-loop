package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/research"
)

func TestRunKBRecallScore_JSONAndText(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	corpus := filepath.Join(dir, "corpus")
	if err := os.MkdirAll(corpus, 0o755); err != nil {
		t.Fatal(err)
	}
	lesson := "- id: l1\n  pattern: gatealpha\n  description: gatealpha contract block\n  confidence: 0.9\n  failureContext:\n    failedStep: build\n    errorCategory: gate\n"
	if err := os.WriteFile(filepath.Join(corpus, "l1.yaml"), []byte(lesson), 0o644); err != nil {
		t.Fatal(err)
	}
	gold := filepath.Join(dir, "gold.json")
	raw, err := json.Marshal(research.GoldSet{Queries: []research.GoldQuery{
		{ID: "q1", Plan: "cycle-1", Query: research.Query{FailureMode: "gatealpha"}, Relevant: []string{"l1"}},
		{ID: "q2", Plan: "cycle-2", Query: research.Query{FailureMode: "absentterm"}, Relevant: []string{"l1"}},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(gold, raw, 0o644); err != nil {
		t.Fatal(err)
	}

	var out, errb bytes.Buffer
	rc := runKB([]string{"recall", "score", "--gold", gold, "--corpus", corpus, "--json"}, nil, &out, &errb)
	if rc != 0 {
		t.Fatalf("rc=%d stderr=%s", rc, errb.String())
	}
	var rep kbRecallReport
	if err := json.Unmarshal(out.Bytes(), &rep); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if rep.Ranker != "FileKB.Lookup" || rep.K != 5 || rep.Queries != 2 || rep.RecallAt5 != 0.5 || rep.MRR != 0.5 {
		t.Errorf("unexpected report: %+v", rep)
	}

	out.Reset()
	errb.Reset()
	rc = runKB([]string{"recall", "score", "--gold", gold, "--corpus", corpus}, nil, &out, &errb)
	if rc != 0 {
		t.Fatalf("text rc=%d stderr=%s", rc, errb.String())
	}
	text := out.String()
	if !strings.Contains(text, "recall@5: 0.5") || !strings.Contains(text, "MRR: 0.5") {
		t.Errorf("text output missing figures: %s", text)
	}
}

func TestRunKBRecallScore_Refusals(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	corpus := filepath.Join(dir, "corpus")
	if err := os.MkdirAll(corpus, 0o755); err != nil {
		t.Fatal(err)
	}
	emptyGold := filepath.Join(dir, "empty.json")
	if err := os.WriteFile(emptyGold, []byte(`{"queries":[]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	unscorableGold := filepath.Join(dir, "unscorable.json")
	if err := os.WriteFile(unscorableGold, []byte(`{"queries":[{"id":"q1","plan":"cycle-1","query":{"failure_mode":"x"},"relevant":[]}]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	cases := [][]string{
		{"bogus", "arg"},
		{"recall"},
		{"recall", "bogus"},
		{"recall", "score", "extra"},
		{"recall", "score", "--gold", filepath.Join(dir, "missing.json"), "--corpus", corpus},
		{"recall", "score", "--gold", emptyGold, "--corpus", corpus},
		{"recall", "score", "--gold", unscorableGold, "--corpus", corpus},
		{"recall", "score", "--gold", emptyGold, "--corpus", filepath.Join(dir, "missing-corpus")},
	}
	for _, args := range cases {
		var out, errb bytes.Buffer
		if rc := runKB(args, nil, &out, &errb); rc == 0 {
			t.Errorf("runKB(%v) = 0, want non-zero", args)
		}
		if strings.Contains(out.String(), "recall@5") {
			t.Errorf("runKB(%v) printed recall@5 on failure: %s", args, out.String())
		}
	}
}

func TestRunKBGraph_BuildErrorAndNeighbors(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	inbox := filepath.Join(root, ".evolve", "inbox")
	if err := os.MkdirAll(inbox, 0o755); err != nil {
		t.Fatal(err)
	}
	badFile := filepath.Join(inbox, "bad.json")
	if err := os.WriteFile(badFile, []byte("{"), 0o644); err != nil {
		t.Fatal(err)
	}
	var out, errb bytes.Buffer
	if rc := runKB([]string{"graph", "neighbors", "cycle:1", "--project-root", root}, nil, &out, &errb); rc != 1 {
		t.Errorf("runKB on corrupt inbox = %d, want 1 (stderr=%s)", rc, errb.String())
	}
	if err := os.Remove(badFile); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(inbox, "i1.json"), []byte(`{"id":"i1","title":"t1"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	errb.Reset()
	if rc := runKB([]string{"graph", "neighbors", "item:i1", "--project-root", root, "--json"}, nil, &out, &errb); rc != 0 {
		t.Errorf("runKB neighbors = %d, want 0 (stderr=%s)", rc, errb.String())
	}
}

func TestRunKBRecallScore_DefaultPaths(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	corpus := filepath.Join(root, filepath.FromSlash(defaultCorpusRel))
	if err := os.MkdirAll(corpus, 0o755); err != nil {
		t.Fatal(err)
	}
	lesson := "- id: l1\n  pattern: gatealpha\n  description: gatealpha contract block\n  confidence: 0.9\n  failureContext:\n    failedStep: build\n    errorCategory: gate\n"
	if err := os.WriteFile(filepath.Join(corpus, "l1.yaml"), []byte(lesson), 0o644); err != nil {
		t.Fatal(err)
	}
	gold := filepath.Join(root, filepath.FromSlash(defaultGoldRel))
	raw, err := json.Marshal(research.GoldSet{Queries: []research.GoldQuery{
		{ID: "q1", Plan: "cycle-1", Query: research.Query{FailureMode: "gatealpha"}, Relevant: []string{"l1"}},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(gold, raw, 0o644); err != nil {
		t.Fatal(err)
	}
	var out, errb bytes.Buffer
	if rc := runKB([]string{"recall", "score", "--project-root", root, "--json"}, nil, &out, &errb); rc != 0 {
		t.Fatalf("rc=%d stderr=%s", rc, errb.String())
	}
	var rep kbRecallReport
	if err := json.Unmarshal(out.Bytes(), &rep); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if rep.Queries != 1 || rep.RecallAt5 != 1 || rep.MRR != 1 {
		t.Errorf("unexpected report: %+v", rep)
	}
}

func TestPrintJSON_MarshalError(t *testing.T) {
	t.Parallel()
	var out, errb bytes.Buffer
	if rc := printJSON(make(chan int), &out, &errb); rc != 1 {
		t.Errorf("printJSON(chan) = %d, want 1", rc)
	}
}
