package dossier

// skipevidence_test.go — the consumer-side contract of
// dossier-corpus-carries-retro-mislabel (cycle 1666).
//
// No production reader interprets skipped_phases today (premise-challenge,
// cycle 1666: contextfillcorrelate decodes only cycle + final_verdict; the
// loop-outcome roll-up reads only spine_fail_opens). So the "disposition
// assembler" the inbox record names is any FUTURE consumer that asks the
// committed corpus which judgment phases executed — and the record type's own
// package is the one boundary every such consumer must cross. PhaseSkipEvidence
// is that boundary: it is the ONLY sanctioned way to read a skipped_phases
// entry, and it refuses to treat a pre-discriminator record's entry as evidence
// of anything. A legacy record is cross-checked against the run artifacts
// instead:
//
//	Contradicted — an execution receipt proves the phase RAN (the mislabel)
//	Unverified   — no receipt survives: unknown, NOT a skip
//	Trusted      — the record carries the discriminator: the phase did not run
//	None         — no skipped_phases entry for the phase at all
//
// Two receipt sources, because the historical run dirs are gone: the phase's
// report in <root>/.evolve/runs/cycle-N/ (retrospective-report.md, or the
// pre-rename retro-report.md that 201 of the ledger's receipts still name), or
// the hash-chained ledger's {cycle:N, role:retro, kind:agent_subprocess} entry
// (<root>/.evolve/ledger.jsonl) — the record of the subprocess that wrote it.

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
)

func legacyRetroSkipDossier(cycle int) *Dossier {
	return &Dossier{
		Cycle:         cycle,
		Goal:          "legacy fixture",
		FinalVerdict:  VerdictFail,
		Phases:        []PhaseRecord{{Name: "cycle-recorded", Verdict: VerdictFail}},
		SkippedPhases: []cyclestate.SkippedPhase{{Phase: "retro", Reason: "FAIL"}},
	}
}

func writeRunArtifact(t *testing.T, root string, cycle int, name string) {
	t.Helper()
	dir := filepath.Join(root, ".evolve", "runs", fmt.Sprintf("cycle-%d", cycle))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, name), []byte("# Retrospective\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func writeLedger(t *testing.T, root string, lines ...string) {
	t.Helper()
	dir := filepath.Join(root, ".evolve")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	body := ""
	for _, l := range lines {
		body += l + "\n"
	}
	if err := os.WriteFile(filepath.Join(dir, "ledger.jsonl"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// retroReceipt is the ledger line the real corpus carries for every one of
// the 134 affected cycles (cycle-823's verbatim shape, entry_seq 57180).
func retroReceipt(cycle int) string {
	return fmt.Sprintf(`{"ts":"2026-07-14T01:12:21Z","cycle":%d,"role":"retro","kind":"agent_subprocess","exit_code":0,"artifact_path":"/x/.evolve/runs/cycle-%d/retro-report.md","git_head":"b140da6b","entry_seq":57180,"prev_hash":"146689a4","run_id":"01KXEYRET2P9R0CXBEMYPK5FFC"}`, cycle, cycle)
}

// TestPhaseSkipEvidence_LegacyRetroSkipIsNeverTrusted — a pre-discriminator
// record's retro entry is NEVER evidence retro was skipped: with a receipt it
// is contradicted, without one it is unverified. Both are "not skipped" to a
// consumer; neither may be read as the skip the label claims.
func TestPhaseSkipEvidence_LegacyRetroSkipIsNeverTrusted(t *testing.T) {
	cases := []struct {
		name  string
		setup func(t *testing.T, root string, cycle int)
		want  SkipEvidence
	}{
		{"run-dir retrospective-report.md corroborates", func(t *testing.T, root string, cycle int) {
			writeRunArtifact(t, root, cycle, "retrospective-report.md")
		}, SkipEvidenceContradicted},
		{"run-dir pre-rename retro-report.md corroborates", func(t *testing.T, root string, cycle int) {
			writeRunArtifact(t, root, cycle, "retro-report.md")
		}, SkipEvidenceContradicted},
		{"ledger retro agent_subprocess receipt corroborates", func(t *testing.T, root string, cycle int) {
			writeLedger(t, root, retroReceipt(cycle))
		}, SkipEvidenceContradicted},
		{"no artifact and no ledger: unverified, not a skip", func(t *testing.T, root string, cycle int) {},
			SkipEvidenceUnverified},
		{"ledger present but nothing for THIS cycle's retro run: unverified", func(t *testing.T, root string, cycle int) {
			writeLedger(t, root,
				retroReceipt(cycle+1), // another cycle's receipt must not corroborate this one
				fmt.Sprintf(`{"ts":"2026-07-14T01:12:21Z","cycle":%d,"role":"retro","kind":"phase_skipped","exit_code":0,"source":"router"}`, cycle),
				fmt.Sprintf(`{"ts":"2026-07-14T01:12:21Z","cycle":%d,"role":"scout","kind":"agent_subprocess","exit_code":0}`, cycle),
				"{not json — a corrupt line must be skipped, never fatal",
			)
		}, SkipEvidenceUnverified},
		{"empty run dir without the report: unverified", func(t *testing.T, root string, cycle int) {
			if err := os.MkdirAll(filepath.Join(root, ".evolve", "runs", fmt.Sprintf("cycle-%d", cycle)), 0o755); err != nil {
				t.Fatal(err)
			}
		}, SkipEvidenceUnverified},
	}
	for i, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			cycle := 823 + i
			tc.setup(t, root, cycle)
			d := legacyRetroSkipDossier(cycle)
			got := PhaseSkipEvidence(root, d, "retro")
			if got != tc.want {
				t.Errorf("PhaseSkipEvidence(legacy cycle %d, retro) = %q, want %q", cycle, got, tc.want)
			}
			if got == SkipEvidenceTrusted {
				t.Errorf("a legacy record's retro entry was TRUSTED as a skip — the exact mislabel this seam exists to refuse")
			}
		})
	}
}

// TestPhaseSkipEvidence_VersionedAndAbsentEntries — the positive and edge
// rows: a record carrying the discriminator is trusted at face value (that is
// the discriminator's purpose — no artifact lookup needed), and a phase with
// no entry, on any record or on a nil record, is None.
func TestPhaseSkipEvidence_VersionedAndAbsentEntries(t *testing.T) {
	root := t.TempDir() // deliberately empty: no runs, no ledger
	versioned := legacyRetroSkipDossier(4300)
	versioned.SchemaVersion = CurrentSchemaVersion
	if got := PhaseSkipEvidence(root, versioned, "retro"); got != SkipEvidenceTrusted {
		t.Errorf("versioned record with a retro skip = %q, want %q (post-fix records are trustworthy by construction)", got, SkipEvidenceTrusted)
	}
	if got := PhaseSkipEvidence(root, versioned, "memo"); got != SkipEvidenceNone {
		t.Errorf("versioned record, no memo entry = %q, want %q", got, SkipEvidenceNone)
	}
	legacyNoRetro := legacyRetroSkipDossier(4301)
	legacyNoRetro.SkippedPhases = []cyclestate.SkippedPhase{{Phase: "memo", Reason: "quota"}}
	legacyNoRetro.PhasesRunVerdictNotAdopted = []cyclestate.VerdictNotAdopted{{Phase: "retro", Verdict: "FAIL"}}
	if got := PhaseSkipEvidence(root, legacyNoRetro, "retro"); got != SkipEvidenceNone {
		t.Errorf("legacy record whose retro is in phases_run_verdict_not_adopted (not skipped_phases) = %q, want %q", got, SkipEvidenceNone)
	}
	if got := PhaseSkipEvidence(root, nil, "retro"); got != SkipEvidenceNone {
		t.Errorf("nil record = %q, want %q (nil-safe like HasCommitment)", got, SkipEvidenceNone)
	}
	if got := PhaseSkipEvidence(root, legacyRetroSkipDossier(4302), ""); got != SkipEvidenceNone {
		t.Errorf("empty phase name = %q, want %q", got, SkipEvidenceNone)
	}
}
