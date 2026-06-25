package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func seedResetProject(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	evolveDir := filepath.Join(root, ".evolve")
	if err := os.MkdirAll(evolveDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(evolveDir, "state.json"),
		[]byte(`{"expected_ship_sha":"OLDPIN","lastCycleNumber":3}`), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

func TestRunResetSHA_OperatorRepinsToRunningBinary(t *testing.T) {
	root := seedResetProject(t)
	var out, errb bytes.Buffer
	code := runResetSHA([]string{"--operator", "--project-root", root}, nil, &out, &errb)
	if code != 0 {
		t.Fatalf("exit=%d stderr=%s", code, errb.String())
	}
	b, _ := os.ReadFile(filepath.Join(root, ".evolve", "state.json"))
	var s map[string]any
	if err := json.Unmarshal(b, &s); err != nil {
		t.Fatalf("torn state.json: %v\n%s", err, b)
	}
	if sha, _ := s["expected_ship_sha"].(string); sha == "OLDPIN" || len(sha) != 64 {
		t.Errorf("expected_ship_sha not re-pinned to a real sha: %q", sha)
	}
	if s["lastCycleNumber"] != float64(3) {
		t.Errorf("unrelated state key lost: %+v", s)
	}
	if !strings.Contains(out.String(), "re-pinned") {
		t.Errorf("missing success output: %s", out.String())
	}
}

func TestRunResetSHA_RefusesWithoutProvenanceOrOperator(t *testing.T) {
	// The test binary has no verifiable provenance against the (non-git) temp
	// project, and --operator is absent → RepinShipSHA refuses → non-zero exit,
	// pin unchanged.
	root := seedResetProject(t)
	var out, errb bytes.Buffer
	code := runResetSHA([]string{"--project-root", root}, nil, &out, &errb)
	if code == 0 {
		t.Fatal("expected refusal without provenance or --operator")
	}
	b, _ := os.ReadFile(filepath.Join(root, ".evolve", "state.json"))
	if !strings.Contains(string(b), `"OLDPIN"`) {
		t.Errorf("pin must be UNCHANGED on refusal: %s", b)
	}
}
