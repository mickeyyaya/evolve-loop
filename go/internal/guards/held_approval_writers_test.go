package guards

import (
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

var heldApprovalWriters = []string{"cmd/evolve/cmd_landing.go"}

func TestHeldApproval_OnlyTheOperatorVerbWritesAnApproval(t *testing.T) {
	writers, err := filesCalling(filepath.Join("..", ".."), "heldproposal.WriteApproval(")
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(writers, heldApprovalWriters) {
		t.Errorf("writers of heldproposal.WriteApproval = %v, want only %v: an approval is the operator's decision, so no lane or host path may write one (ADR-0157)", writers, heldApprovalWriters)
	}
}

func TestFilesCalling_FindsACallerAndSkipsTests(t *testing.T) {
	root := t.TempDir()
	for rel, body := range map[string]string{
		"cmd/a/a.go":      "package a\nfunc f() { heldproposal.WriteApproval(d, x) }\n",
		"cmd/a/a_test.go": "package a\nfunc g() { heldproposal.WriteApproval(d, x) }\n",
		"internal/b/b.go": "package b\n",
	} {
		p := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	got, err := filesCalling(root, "heldproposal.WriteApproval(")
	if err != nil || !slices.Equal(got, []string{"cmd/a/a.go"}) {
		t.Errorf("filesCalling = %v, %v, want only the non-test caller", got, err)
	}
	if _, err := filesCalling(filepath.Join(root, "absent"), "x"); err == nil {
		t.Error("an absent root must be an error")
	}
}

func filesCalling(root, call string) ([]string, error) {
	var out []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		raw, err := os.ReadFile(path)
		if err != nil || !strings.Contains(string(raw), call) {
			return err
		}
		rel, err := filepath.Rel(root, path)
		out = append(out, filepath.ToSlash(rel))
		return err
	})
	slices.Sort(out)
	return out, err
}
