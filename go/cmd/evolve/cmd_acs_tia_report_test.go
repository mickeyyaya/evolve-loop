package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/regressiontia"
)

func tiaEvolveDir(t *testing.T) string {
	t.Helper()
	evolveDir := filepath.Join(t.TempDir(), ".evolve")
	ws := filepath.Join(evolveDir, "runs", "cycle-7")
	if err := os.MkdirAll(ws, 0o755); err != nil {
		t.Fatal(err)
	}
	d, _ := json.Marshal(regressiontia.Decision{Stage: "shadow", Selected: []string{"./acs/regression/a"}, WouldSkip: []string{"./acs/regression/b"}, WouldSkipCount: 1})
	v := `{"results":[{"predicate":"go/acs/b/...:TestB","result":"red","is_regression":true}]}`
	for name, body := range map[string]string{regressiontia.ArtifactName: string(d), "acs-verdict.json": v} {
		if err := os.WriteFile(filepath.Join(ws, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return evolveDir
}

func TestACSTIAReport_PrintsTheRollupAndEachMiss(t *testing.T) {
	t.Parallel()
	var out, errb bytes.Buffer
	code := runACS([]string{"tia-report", "--evolve-dir", tiaEvolveDir(t)}, nil, &out, &errb)
	if code != 0 {
		t.Fatalf("code %d, stderr %q", code, errb.String())
	}
	for _, want := range []string{"1 cycle(s) with a shadow decision: 1 selected, 1 would skip (50%)", "MISS cycle 7: ./acs/regression/b would skip go/acs/b/...:TestB, which was red"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("stdout = %q, want %q", out.String(), want)
		}
	}
}

func TestACSTIAReport_JSONPrintsTheSummary(t *testing.T) {
	t.Parallel()
	var out, errb bytes.Buffer
	if code := runACS([]string{"tia-report", "--evolve-dir", tiaEvolveDir(t), "--json"}, nil, &out, &errb); code != 0 {
		t.Fatalf("code %d, stderr %q", code, errb.String())
	}
	var s regressiontia.Summary
	if err := json.Unmarshal(out.Bytes(), &s); err != nil || s.Cycles != 1 || len(s.Misses) != 1 {
		t.Fatalf("summary = %+v (%v) from %q", s, err, out.String())
	}
}

func TestACSTIAReport_RefusesABadFlagOrAMissingStateDir(t *testing.T) {
	t.Parallel()
	var out, errb bytes.Buffer
	if code := runACS([]string{"tia-report", "--bogus"}, nil, &out, &errb); code != 10 {
		t.Errorf("bad flag: code %d, want 10", code)
	}
	if code := runACS([]string{"tia-report", "--evolve-dir", filepath.Join(t.TempDir(), "none")}, nil, &out, &errb); code != 10 {
		t.Errorf("missing dir: code %d, want 10", code)
	}
	empty := filepath.Join(t.TempDir(), ".evolve")
	if err := os.MkdirAll(empty, 0o755); err != nil {
		t.Fatal(err)
	}
	errb.Reset()
	if code := runACS([]string{"tia-report", "--evolve-dir", empty}, nil, &out, &errb); code != 1 || !strings.Contains(errb.String(), "runs") {
		t.Errorf("no runs dir: code %d stderr %q, want 1 naming the runs dir", code, errb.String())
	}
}

func TestACSTIAReport_NoShadowCyclesSaysSo(t *testing.T) {
	t.Parallel()
	evolveDir := filepath.Join(t.TempDir(), ".evolve")
	if err := os.MkdirAll(filepath.Join(evolveDir, "runs", "cycle-1"), 0o755); err != nil {
		t.Fatal(err)
	}
	var out, errb bytes.Buffer
	if code := runACS([]string{"tia-report", "--evolve-dir", evolveDir}, nil, &out, &errb); code != 0 || !strings.Contains(out.String(), "0 cycle(s) with a shadow decision") {
		t.Fatalf("code %d stdout %q", code, out.String())
	}
}

func TestACSTIAReport_NamesACycleWithoutAVerdictAndAnUnreadableFile(t *testing.T) {
	t.Parallel()
	evolveDir := filepath.Join(t.TempDir(), ".evolve")
	for cycle, body := range map[string]string{"8": `{"stage":"shadow"}`, "9": "{broken"} {
		ws := filepath.Join(evolveDir, "runs", "cycle-"+cycle)
		if err := os.MkdirAll(ws, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(ws, regressiontia.ArtifactName), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	var out, errb bytes.Buffer
	if code := runACS([]string{"tia-report", "--evolve-dir", evolveDir}, nil, &out, &errb); code != 0 {
		t.Fatalf("code %d", code)
	}
	for _, want := range []string{"cycle 8 has a shadow decision but no acs-verdict.json", "unreadable: cycle-9/" + regressiontia.ArtifactName} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("stdout = %q, want %q", out.String(), want)
		}
	}
}
