package core

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

// dossier_producer_params_test.go — cycle 1652 RED contract for the inbox item
// dossier-producer-params-struct: writeCycleDossier grew to ELEVEN positional
// parameters by accretion (the 11th, live phase timings, touched every call
// site to append one value), so the producer takes ONE named params value
// mirroring dossier.BuildOpts and a field addition touches only the producer
// and the site that supplies it.
//
// Contract (test-report.md ## AC-Materialization):
//
//	AC1 writeCycleDossier(lock gitMutationLocker, p cycleDossierParams) error —
//	    two parameters; every current input preserved as a NAMED field:
//	      ProjectRoot, WorkspacePath, Cycle, Goal, RunID, Outcome,
//	      SkippedPhases, VerdictsNotAdopted, SpineFailOpens, PhaseTimings
//	    (BuildOpts' spellings where BuildOpts has the field; Outcome keeps the
//	    producer's current name because it is the RAW cycle outcome that
//	    dossierVerdict maps — BuildOpts.FinalVerdict is the mapped value).
//	AC2 keyed construction: a caller that names only the fields it has compiles
//	    and runs — the property that makes a field addition non-breaking.
//	AC3 no behavior change: the bytes for a fixed input are unchanged. The
//	    golden under testdata/dossierparams/ was captured from the 11-argument
//	    producer at HEAD 287aa81c BEFORE this refactor — the JSON and Markdown
//	    the refactored producer emits for the same input must match byte for
//	    byte. The projection is clock-free (no time.Now in internal/dossier), so
//	    the pin is exact, not fuzzy.
//
// RED today as a compile failure: cycleDossierParams does not exist and
// writeCycleDossier has eleven parameters. Restore compilation FIRST (the
// struct + signature), then every other core test runs again.

// goldenDossierParams is the FIXED input the golden was captured from. Do not
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

// TestWriteCycleDossier_ParamsStructPreservesFixedInputBytes — AC1 + AC3. The
// refactored producer, fed the golden's input through the params value, must
// write the exact JSON and Markdown the eleven-argument producer wrote.
func TestWriteCycleDossier_ParamsStructPreservesFixedInputBytes(t *testing.T) {
	root := t.TempDir()
	initDossierRepo(t, root)
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

// TestWriteCycleDossier_ParamsAreKeyedAndOptional — AC2 in executable form: a
// caller supplying only the fields it has (no skipped/not-adopted/spine/timing
// evidence — the shape a hypothetical twelfth field would be absent in at every
// unrelated call site) compiles, runs, and yields a valid PASS record whose
// named fields round-trip. This is the same input the pre-existing
// TestWriteCycleDossier_WritesValidArtifact fed positionally.
func TestWriteCycleDossier_ParamsAreKeyedAndOptional(t *testing.T) {
	root := t.TempDir()
	initDossierRepo(t, root)
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
