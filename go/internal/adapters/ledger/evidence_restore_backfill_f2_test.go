package ledger

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/ledgerartifacts"
)

type recordFailsSinkF2 struct {
	previewSink
	err error
}

func (s recordFailsSinkF2) record(compositionEvidenceRecord) error { return s.err }

func TestWalkCompositionEvidence_ReportsAnUnreadableLedgerF2(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "ledger.jsonl"), 0o755); err != nil {
		t.Fatal(err)
	}

	for name, run := range map[string]func(*FileLedger) ([]EvidenceRestoration, error){
		"restore": func(l *FileLedger) ([]EvidenceRestoration, error) {
			return l.RestoreCompositionEvidence(context.Background(), dir)
		},
		"preview": func(l *FileLedger) ([]EvidenceRestoration, error) {
			return l.PreviewCompositionEvidence(context.Background(), dir)
		},
	} {
		got, err := run(New(dir))

		if got != nil || err == nil || !strings.Contains(err.Error(), "ledger evidence restore:") {
			t.Errorf("%s = (%v, %v), want nil and the wrapped read error", name, got, err)
		}
	}
}

func TestWalkCompositionEvidence_SkipsLinesThatMerelyMentionTheKindF2(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	ledgerPath := filepath.Join(dir, "ledger.jsonl")
	if err := WriteCompositionVerdict(ledgerPath, honestWriteInput(t)); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(ledgerPath)
	if err != nil {
		t.Fatal(err)
	}
	noise := "not json but it says \"composition-verdict\"\n" +
		"{\"kind\":\"phase\",\"message\":\"composition-verdict\"}\n"
	if err := os.WriteFile(ledgerPath, append(append([]byte(noise), raw...), []byte(noise)...), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := New(dir).PreviewCompositionEvidence(context.Background(), dir)

	if err != nil || len(got) != 1 || got[0].Line != 2 || got[0].Action != EvidenceDurable || got[0].Cycle != 787 {
		t.Errorf("PreviewCompositionEvidence = (%+v, %v), want only the real line 2, durable, cycle 787", got, err)
	}
}

func TestWalkCompositionEvidence_AnEmptyLedgerRestoresNothingF2(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "ledger.jsonl"), nil, 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := New(dir).RestoreCompositionEvidence(context.Background(), dir)

	if err != nil || len(got) != 0 {
		t.Errorf("RestoreCompositionEvidence = (%v, %v), want nothing and no error", got, err)
	}
}

func TestRestore_AFailingEvidenceRecordMakesTheLineUnrestorableAndNamesTheCauseF2(t *testing.T) {
	h := identicalCarryHistory(t)
	appendCompositionLine(t, h.evolveDir, h.legacyLine(1766, h.patch))
	boom := errors.New("record boom")

	got, err := New(h.evolveDir).walkCompositionEvidence(context.Background(), h.repo, recordFailsSinkF2{err: boom})

	if err != nil || len(got) != 1 || got[0].Action != EvidenceUnrestorable || got[0].Reason != "evidence record: record boom" || got[0].Cycle != 1766 {
		t.Errorf("walkCompositionEvidence = (%+v, %v), want the line unrestorable with the record failure as reason", got, err)
	}
	if n := evidenceRecords(t, h); n != 0 {
		t.Errorf("a failed record left %d evidence record(s)", n)
	}
}

func TestEvidenceSlotProves_AcceptsOnlyADiffThatReDerivesItsPatchIDAndDigestF2(t *testing.T) {
	t.Parallel()
	patchID := honestWriteInput(t).PatchID
	good := ledgerartifacts.Digest([]byte(compTestDiff))
	cases := []struct {
		name    string
		slot    evidenceSlot
		diff    string
		patchID string
		want    string
	}{
		{"patch id and digest match", evidenceSlot{compositionEvidence: compositionEvidence{digest: good}}, compTestDiff, patchID, ""},
		{"no digest recorded", evidenceSlot{}, compTestDiff, patchID, ""},
		{"not a patch", evidenceSlot{}, notAPatchF2, patchID, "empty output"},
		{"empty diff", evidenceSlot{}, "", patchID, "empty output"},
		{"patch id differs", evidenceSlot{}, compTestDiff, "deadbeef", "re-derives patch-id " + patchID},
		{"digest differs", evidenceSlot{compositionEvidence: compositionEvidence{digest: strings.Repeat("0", 64)}}, compTestDiff, patchID, "hashes to " + good},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			err := tc.slot.proves([]byte(tc.diff), tc.patchID)

			if tc.want == "" && err != nil || tc.want != "" && (err == nil || !strings.Contains(err.Error(), tc.want)) {
				t.Errorf("proves = %v, want %q", err, tc.want)
			}
		})
	}
}
