package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeMislabelFixtureDossier(t *testing.T, root string, cycle int, body string) string {
	t.Helper()
	dir := filepath.Join(root, "knowledge-base", "cycles")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, fmt.Sprintf("cycle-%d.json", cycle))
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

const (
	legacyRetroSkip = `{"cycle":%d,"goal":"g","final_verdict":"FAIL","phases":[{"name":"cycle-recorded","verdict":"FAIL"}],
"defects":[{"id":"audit-fail","severity":"HIGH","summary":"s"}],"carryover":[{"id":"a","action":"b"}],
"skipped_phases":[{"phase":"retro","reason":"FAIL"}]}`
	versionedRetroSkip = `{"schema_version":%d,"cycle":%d,"goal":"g","final_verdict":"WARN","phases":[{"name":"scout","verdict":"PASS"}],
"skipped_phases":[{"phase":"retro","reason":"abnormal exit"}]}`
	legacyMemoSkipRetroNotAdopted = `{"cycle":%d,"goal":"g","final_verdict":"PASS","phases":[{"name":"scout","verdict":"PASS"}],
"skipped_phases":[{"phase":"memo","reason":"quota"}],"phases_run_verdict_not_adopted":[{"phase":"retro","verdict":"FAIL"}]}`
)

// buildMislabelCorpus lays down six records that together cover every
// classification the audit must make, plus the two receipt sources:
//
//	10 legacy, retro skip, ledger receipt              → mislabeled
//	11 legacy, retro skip, run-dir retrospective report → mislabeled
//	12 legacy, retro skip, nothing survives             → uncorroborated
//	13 VERSIONED, retro skip                            → not a candidate (trusted skip)
//	14 legacy, memo skip + retro verdict-not-adopted    → not a candidate
//	15 legacy, retro skip, receipt for the WRONG cycle  → uncorroborated
func buildMislabelCorpus(t *testing.T, root string) map[string][]byte {
	t.Helper()
	files := map[string][]byte{}
	put := func(cycle int, body string) {
		p := writeMislabelFixtureDossier(t, root, cycle, body)
		files[p] = []byte(body)
	}
	put(10, fmt.Sprintf(legacyRetroSkip, 10))
	put(11, fmt.Sprintf(legacyRetroSkip, 11))
	put(12, fmt.Sprintf(legacyRetroSkip, 12))
	put(13, fmt.Sprintf(versionedRetroSkip, 2, 13))
	put(14, fmt.Sprintf(legacyMemoSkipRetroNotAdopted, 14))
	put(15, fmt.Sprintf(legacyRetroSkip, 15))

	runDir := filepath.Join(root, ".evolve", "runs", "cycle-11")
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(runDir, "retrospective-report.md"), []byte("# Retrospective\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	ledger := `{"ts":"2026-07-14T01:12:21Z","cycle":10,"role":"retro","kind":"agent_subprocess","exit_code":0,"artifact_path":"/x/.evolve/runs/cycle-10/retro-report.md","entry_seq":1}
{"ts":"2026-07-14T01:12:21Z","cycle":16,"role":"retro","kind":"agent_subprocess","exit_code":0,"artifact_path":"/x/.evolve/runs/cycle-16/retro-report.md","entry_seq":2}
{"ts":"2026-07-14T01:12:21Z","cycle":15,"role":"retro","kind":"phase_skipped","exit_code":0,"source":"router","entry_seq":3}
`
	if err := os.WriteFile(filepath.Join(root, ".evolve", "ledger.jsonl"), []byte(ledger), 0o644); err != nil {
		t.Fatal(err)
	}
	return files
}

type mislabelReport struct {
	Candidates     int   `json:"candidates"`
	Mislabeled     []int `json:"mislabeled"`
	Uncorroborated []int `json:"uncorroborated"`
}

func TestDossierRetroMislabel_DerivedCountCrossChecksArtifacts(t *testing.T) {
	root := t.TempDir()
	files := buildMislabelCorpus(t, root)

	var out, errb bytes.Buffer
	rc := runDossier([]string{"retro-mislabel", "--project-root", root, "--json"}, nil, &out, &errb)
	if rc != 0 {
		t.Fatalf("RED: evolve dossier retro-mislabel --json exited %d\nstdout:\n%s\nstderr:\n%s", rc, out.String(), errb.String())
	}
	var rep mislabelReport
	if err := json.Unmarshal(out.Bytes(), &rep); err != nil {
		t.Fatalf("--json output is not a JSON object: %v\n%s", err, out.String())
	}
	if rep.Candidates != 4 {
		t.Errorf("candidates = %d, want 4 (cycles 10, 11, 12, 15 — the unversioned records whose skipped_phases names retro)", rep.Candidates)
	}
	if want := []int{10, 11}; fmt.Sprint(rep.Mislabeled) != fmt.Sprint(want) {
		t.Errorf("mislabeled = %v, want %v (10 via the ledger receipt, 11 via the run-dir report)", rep.Mislabeled, want)
	}
	if want := []int{12, 15}; fmt.Sprint(rep.Uncorroborated) != fmt.Sprint(want) {
		t.Errorf("uncorroborated = %v, want %v (no receipt for 12; 15's only receipt belongs to cycle 16)", rep.Uncorroborated, want)
	}
	if rep.Candidates != len(rep.Mislabeled)+len(rep.Uncorroborated) {
		t.Errorf("candidates (%d) != mislabeled (%d) + uncorroborated (%d) — every candidate must be classified exactly once",
			rep.Candidates, len(rep.Mislabeled), len(rep.Uncorroborated))
	}
	for path, before := range files {
		after, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read back %s: %v", path, err)
		}
		if !bytes.Equal(before, after) {
			t.Errorf("%s was REWRITTEN by the audit — the command is read-only; the backfill is the remedy this cycle did NOT choose", filepath.Base(path))
		}
	}

	out.Reset()
	errb.Reset()
	if rc := runDossier([]string{"retro-mislabel", "--project-root", root}, nil, &out, &errb); rc != 0 {
		t.Errorf("human-mode exit = %d, want 0\nstderr:\n%s", rc, errb.String())
	}
	if !strings.Contains(out.String(), "2") || !strings.Contains(strings.ToLower(out.String()), "mislabel") {
		t.Errorf("human-mode stdout should state the derived mislabeled count (2), got:\n%s", out.String())
	}
}

func TestDossierRetroMislabel_AbsentCorpusFailsLoudly(t *testing.T) {
	root := t.TempDir()
	var out, errb bytes.Buffer
	rc := runDossier([]string{"retro-mislabel", "--project-root", root, "--json"}, nil, &out, &errb)
	if rc == 0 {
		t.Errorf("absent knowledge-base/cycles must fail loudly, got exit 0\nstdout:\n%s", out.String())
	}
	if !strings.Contains(errb.String(), "knowledge-base/cycles") {
		t.Errorf("stderr should name the missing corpus dir, got:\n%s", errb.String())
	}
}
