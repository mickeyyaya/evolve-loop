package core

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

// goldenDossierParams is the fixed input the golden was captured from. Do not
// change a value here without regenerating the golden through the pre-refactor
// producer — the point is that the bytes come from the OLD signature.
func goldenDossierParams(projectRoot string) cycleDossierParams {
	return cycleDossierParams{
		ProjectRoot:        projectRoot,
		WorkspacePath:      "/nonexistent/workspace/for/golden",
		Cycle:              4242,
		Goal:               "fixed goal text for the byte-equivalence pin",
		RunID:              "01JFIXEDRUNIDFORGOLDEN00",
		Outcome:            VerdictFAIL,
		SkippedPhases:      []SkippedPhase{{Phase: "closeout", Reason: "abnormal exit in phase build"}},
		VerdictsNotAdopted: []VerdictNotAdopted{{Phase: "retro", Verdict: VerdictFAIL}},
		SpineFailOpens:     []SpineFailOpen{{Phase: "tdd", MissingArtifact: "handoff-scout.json", Reason: "would-block at enforce"}},
		PhaseTimings: []phaseTimingEntry{
			{Phase: "scout", DurationMS: 1500, Verdict: VerdictPASS, StartedAt: "2026-09-13T10:00:00Z", EndedAt: "2026-09-13T10:00:01.5Z", AttemptCount: 1, ResolvedModel: "gpt-5.6-terra", ModelSource: "profile"},
			{Phase: "triage", DurationMS: 800, Verdict: VerdictPASS, StartedAt: "2026-09-13T10:00:02Z", EndedAt: "2026-09-13T10:00:02.8Z", AttemptCount: 1},
			{Phase: "build", DurationMS: 42000, Verdict: VerdictFAIL, StartedAt: "2026-09-13T10:00:03Z", EndedAt: "2026-09-13T10:00:45Z", AttemptCount: 2, AbortReason: "contract-gate: build-report.md missing ## Files Changed"},
		},
	}
}

func readGolden(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", "dossierparams", name))
	if err != nil {
		t.Fatalf("golden %s: %v", name, err)
	}
	return b
}

func TestWriteCycleDossier_ParamsStructPreservesFixedInputBytes(t *testing.T) {
	root := initDossierRepo(t)
	if err := writeCycleDossier(nil, goldenDossierParams(root)); err != nil {
		t.Fatalf("writeCycleDossier: %v", err)
	}
	dir := filepath.Join(root, "knowledge-base", "cycles")
	for _, tc := range []struct{ got, golden string }{
		{"cycle-4242.json", "cycle-4242.golden.json"},
		{"cycle-4242.md", "cycle-4242.golden.md"},
	} {
		got, err := os.ReadFile(filepath.Join(dir, tc.got))
		if err != nil {
			t.Fatalf("%s not written: %v", tc.got, err)
		}
		if want := readGolden(t, tc.golden); !bytes.Equal(got, want) {
			t.Errorf("RED: %s differs from the pre-refactor bytes (a field was dropped, renamed, or re-mapped in the params migration)\n--- got ---\n%s\n--- want ---\n%s", tc.got, got, want)
		}
	}
}

func TestWriteCycleDossier_ParamsAreKeyedAndOptional(t *testing.T) {
	root := initDossierRepo(t)
	p := cycleDossierParams{
		ProjectRoot:   root,
		WorkspacePath: t.TempDir(),
		Cycle:         7,
		Goal:          "improve X",
		RunID:         "run-ulid",
		Outcome:       CycleOutcomeShippedViaBuild,
	}
	if err := writeCycleDossier(nil, p); err != nil {
		t.Fatalf("writeCycleDossier: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(root, "knowledge-base", "cycles", "cycle-7.json"))
	if err != nil {
		t.Fatalf("dossier not written: %v", err)
	}
	for _, want := range []string{`"cycle": 7`, `"goal": "improve X"`, `"run_id": "run-ulid"`, `"final_verdict": "PASS"`} {
		if !bytes.Contains(data, []byte(want)) {
			t.Errorf("RED: dossier lacks %s; got:\n%s", want, data)
		}
	}
	for _, absent := range []string{`"skipped_phases"`, `"phases_run_verdict_not_adopted"`, `"spine_fail_opens"`} {
		if bytes.Contains(data, []byte(absent)) {
			t.Errorf("RED: an omitted evidence field was fabricated into the record: %s", absent)
		}
	}
}
