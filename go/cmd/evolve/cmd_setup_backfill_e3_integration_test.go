//go:build integration

package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunSetupApply_DryRunPrintsTheMergeAndWritesNothing(t *testing.T) {
	root := e3IsolatedCLIHost(t, e3FakeOllama)
	policyPath := filepath.Join(root, ".evolve", "policy.json")
	setupWrite(t, policyPath, `{"version":1,"floor":[{"id":"dossier-closeout"}]}`)
	var out, errb bytes.Buffer

	code := runSetup([]string{"apply", "--preset", "economy", "--dry-run", "--project-root", root}, nil, &out, &errb)

	if code != 0 {
		t.Fatalf("code %d stderr %q", code, errb.String())
	}
	if !strings.Contains(out.String(), `"floor"`) || !strings.Contains(out.String(), `"version"`) || strings.Contains(out.String(), "[setup apply] wrote") {
		t.Errorf("dry-run stdout = %q, want the merged policy and no write notice", out.String())
	}
	raw, err := os.ReadFile(policyPath)
	if err != nil || string(raw) != `{"version":1,"floor":[{"id":"dossier-closeout"}]}` {
		t.Errorf("dry-run changed policy.json to %q (err %v)", raw, err)
	}
}

func TestRunSetupApply_WritesThePolicyAndNamesThePreset(t *testing.T) {
	root := e3IsolatedCLIHost(t, e3FakeOllama)
	var out, errb bytes.Buffer

	code := runSetup([]string{"apply", "--preset", "recommended", "--project-root", root}, nil, &out, &errb)

	policyPath := filepath.Join(root, ".evolve", "policy.json")
	if code != 0 || out.String() != `[setup apply] wrote preset "recommended" to `+policyPath+"\n" {
		t.Fatalf("code %d stdout %q stderr %q", code, out.String(), errb.String())
	}
	if _, err := os.Stat(policyPath); err != nil {
		t.Errorf("policy.json not written: %v", err)
	}
}

func TestRunSetupApply_AnUnwritableEvolveDirExitsOne(t *testing.T) {
	root := e3IsolatedCLIHost(t, e3FakeOllama)
	evolveDir := filepath.Join(root, ".evolve")
	if err := os.MkdirAll(evolveDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(evolveDir, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(evolveDir, 0o755) })
	var out, errb bytes.Buffer

	code := runSetup([]string{"apply", "--preset", "recommended", "--project-root", root}, nil, &out, &errb)

	if code != 1 || out.Len() != 0 || !strings.HasPrefix(errb.String(), "evolve setup apply: ") {
		t.Fatalf("code %d stdout %q stderr %q", code, out.String(), errb.String())
	}
	if _, err := os.Stat(filepath.Join(evolveDir, "policy.json")); !os.IsNotExist(err) {
		t.Errorf("a failed write left policy.json: %v", err)
	}
}
