package research

import (
	"context"
	"testing"
)

func TestFileKB_SatisfiesKBInterface(t *testing.T) {
	dir := t.TempDir()
	writeLesson(t, dir, "audit.yaml", auditEgpsLesson)

	var kb KB = NewFileKB([]string{dir})

	got, err := kb.Lookup(context.Background(), Query{Source: "audit"})
	if err != nil {
		t.Fatalf("Lookup via KB interface: %v", err)
	}
	if len(got) != 1 || got[0].ID != "inst-L001" {
		t.Fatalf("Lookup through KB interface returned %v, want [inst-L001]", ids(got))
	}
}

func TestFileKB_RootsFieldBound(t *testing.T) {
	var fkb *FileKB = NewFileKB(nil)
	got, err := fkb.Lookup(context.Background(), Query{Source: "audit"})
	if err != nil {
		t.Fatalf("empty-roots FileKB should not error (novel-failure signal), got %v", err)
	}
	if len(got) != 0 {
		t.Errorf("empty-roots FileKB returned %v, want no matches", ids(got))
	}
}

func TestGoldSetAndRecallScore_ExportedSymbolsExecuted(t *testing.T) {
	q := GoldQuery{ID: "sym-1", Plan: "cycle-1000", Query: Query{Source: "audit"}, Relevant: []string{"inst-L001"}}
	set := GoldSet{Queries: []GoldQuery{q}}
	dir := t.TempDir()
	writeLesson(t, dir, "audit.yaml", auditEgpsLesson)
	var score RecallScore
	var err error
	score, err = ScoreRecall(context.Background(), NewFileKB([]string{dir}), set, 5)
	if err != nil {
		t.Fatalf("ScoreRecall: %v", err)
	}
	if score.Queries != 1 || score.K != 5 || score.Recall != 1 || score.MRR != 1 {
		t.Errorf("unexpected score %+v", score)
	}
	if _, err := LoadGoldSet(dir + "/missing.json"); err == nil {
		t.Errorf("LoadGoldSet on missing path should fail")
	}
}
