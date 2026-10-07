package ledger

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/ledgerartifacts"
)

func appendCompositionLine(t *testing.T, dir string, rec compositionRecord) {
	t.Helper()
	rec.Kind = CompositionVerdictKind
	if err := New(dir).appendChained(func(seq int, prevHash string) any {
		rec.EntrySeq, rec.PrevHash = seq, prevHash
		return rec
	}); err != nil {
		t.Fatal(err)
	}
}

func TestWriteCompositionVerdict_StoresBothDiffsInTheEvidenceStore(t *testing.T) {
	dir := t.TempDir()
	ledgerPath := filepath.Join(dir, "ledger.jsonl")
	in := honestWriteInput(t)

	if err := WriteCompositionVerdict(ledgerPath, in); err != nil {
		t.Fatalf("WriteCompositionVerdict: %v", err)
	}

	raw, err := os.ReadFile(ledgerPath)
	if err != nil {
		t.Fatal(err)
	}
	var line map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(string(raw))), &line); err != nil {
		t.Fatal(err)
	}
	store := ledgerartifacts.Open(dir)
	for _, slot := range []string{"audited", "composed"} {
		digest, _ := line[slot+"_diff_sha256"].(string)
		if digest != sha256Of(compTestDiff) {
			t.Fatalf("%s_diff_sha256 = %q, want the sha256 of the diff", slot, digest)
		}
		if got, err := store.Get(digest); err != nil || string(got) != compTestDiff {
			t.Fatalf("store.Get(%s) = (%q, %v), want the %s diff", digest, got, err, slot)
		}
		if want, _ := store.Path(digest); line[slot+"_diff_path"] != want {
			t.Fatalf("%s_diff_path = %v, want the store object %s so an older reader still finds it", slot, line[slot+"_diff_path"], want)
		}
	}
}

func TestVerify_ResolvesACompositionLineFromTheStoreBeforeItsPath(t *testing.T) {
	dir := t.TempDir()
	digest, err := ledgerartifacts.Open(dir).Put([]byte(compTestDiff))
	if err != nil {
		t.Fatal(err)
	}
	gone := filepath.Join(dir, "worktrees", "cycle-1766", "composition-1766-audited.diff")
	appendCompositionLine(t, dir, compositionRecord{Cycle: 1766, PatchID: honestWriteInput(t).PatchID,
		AuditedDiffPath: gone, ComposedDiffPath: gone, AuditedDiffSHA256: digest, ComposedDiffSHA256: digest})

	if err := New(dir).Verify(context.Background()); err != nil {
		t.Fatalf("a line whose diffs the store holds verifies whatever became of its path: %v", err)
	}
}

func TestVerify_AStoredDiffThatIsGoneIsStillAChainBreak(t *testing.T) {
	dir := t.TempDir()
	ledgerPath := filepath.Join(dir, "ledger.jsonl")
	if err := WriteCompositionVerdict(ledgerPath, honestWriteInput(t)); err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(filepath.Join(dir, ledgerartifacts.DirName)); err != nil {
		t.Fatal(err)
	}

	if err := New(dir).Verify(context.Background()); !errors.Is(err, core.ErrLedgerChainBroken) {
		t.Fatalf("Verify = %v, want a chain break: a committed diff that cannot be found is never skipped", err)
	}
}

func legacyLineWithItsDiffGone(t *testing.T, dir string) []byte {
	t.Helper()
	gone := filepath.Join(dir, "worktrees", "cycle-1766", "composition-1766-audited.diff")
	appendCompositionLine(t, dir, compositionRecord{Cycle: 1766, PatchID: honestWriteInput(t).PatchID,
		AuditedDiffPath: gone, ComposedDiffPath: gone})
	if err := New(dir).Verify(context.Background()); !errors.Is(err, core.ErrLedgerChainBroken) {
		t.Fatalf("Verify before the evidence record = %v, want the unreadable-path break", err)
	}
	raw, err := os.ReadFile(filepath.Join(dir, "ledger.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	return splitLines(raw)[0]
}

func appendEvidenceRecord(t *testing.T, dir string, line []byte, digest string) {
	t.Helper()
	rec := compositionEvidenceRecord{Kind: compositionEvidenceKind, Role: operatorRole, Cycle: 1766,
		LineSHA256: sha256Hex(line), AuditedDiffSHA256: digest, ComposedDiffSHA256: digest}
	if err := New(dir).appendChained(func(seq int, prevHash string) any {
		rec.EntrySeq, rec.PrevHash = seq, prevHash
		return rec
	}); err != nil {
		t.Fatal(err)
	}
}

func TestVerify_ResolvesALegacyCompositionLineThroughALaterChainedEvidenceRecord(t *testing.T) {
	dir := t.TempDir()
	line := legacyLineWithItsDiffGone(t, dir)
	digest, err := ledgerartifacts.Open(dir).Put([]byte(compTestDiff))
	if err != nil {
		t.Fatal(err)
	}

	appendEvidenceRecord(t, dir, line, digest)

	if err := New(dir).Verify(context.Background()); err != nil {
		t.Fatalf("a legacy line whose evidence a chained record names in the store verifies: %v", err)
	}
	if err := New(dir).VerifyDeep(context.Background()); err != nil {
		t.Fatalf("VerifyDeep resolves it the same way: %v", err)
	}
}

func TestVerify_AnEvidenceRecordStillHasToReDeriveThePatchID(t *testing.T) {
	dir := t.TempDir()
	line := legacyLineWithItsDiffGone(t, dir)
	digest, err := ledgerartifacts.Open(dir).Put([]byte(compTestDriftedDiff))
	if err != nil {
		t.Fatal(err)
	}

	appendEvidenceRecord(t, dir, line, digest)

	if err := New(dir).Verify(context.Background()); !errors.Is(err, core.ErrLedgerChainBroken) {
		t.Fatalf("Verify = %v, want a break: an evidence record names where to look, never what to trust", err)
	}
}

func TestWriteCompositionVerdict_TwoCarriesOfOneCycleBothStayVerifiable(t *testing.T) {
	dir := t.TempDir()
	ledgerPath := filepath.Join(dir, "ledger.jsonl")
	first := honestWriteInput(t)
	second := honestWriteInput(t)
	drifted, err := PatchID([]byte(compTestDriftedDiff))
	if err != nil {
		t.Fatal(err)
	}
	second.PatchID, second.AuditedDiff, second.ComposedDiff = drifted, []byte(compTestDriftedDiff), []byte(compTestDriftedDiff)

	for _, in := range []CompositionVerdictInput{first, second} {
		if err := WriteCompositionVerdict(ledgerPath, in); err != nil {
			t.Fatalf("WriteCompositionVerdict: %v", err)
		}
	}

	if err := New(dir).Verify(context.Background()); err != nil {
		t.Fatalf("a second carry of cycle %d overwrote the first one's evidence: %v", first.Cycle, err)
	}
}

func TestWriteCompositionVerdict_AppendsNothingWhenTheDiffsCannotBeStored(t *testing.T) {
	dir := t.TempDir()
	ledgerPath := filepath.Join(dir, "ledger.jsonl")
	if err := os.WriteFile(filepath.Join(dir, ledgerartifacts.DirName), []byte("not a directory"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := WriteCompositionVerdict(ledgerPath, honestWriteInput(t)); err == nil {
		t.Fatal("WriteCompositionVerdict must refuse a line whose diffs it could not store")
	}
	if size := ledgerSize(t, ledgerPath); size != 0 {
		t.Fatalf("a refused write appended %d bytes: verify would flag the line as unreadable", size)
	}
}

func TestVerify_AStoredDiffWhoseBytesNoLongerHashToItsNameIsAChainBreak(t *testing.T) {
	dir := t.TempDir()
	if err := WriteCompositionVerdict(filepath.Join(dir, "ledger.jsonl"), honestWriteInput(t)); err != nil {
		t.Fatal(err)
	}
	object, err := ledgerartifacts.Open(dir).Path(sha256Of(compTestDiff))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(object, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(object, samePatchIDOtherBytes(t, []byte(compTestDiff)), 0o644); err != nil {
		t.Fatal(err)
	}

	err = New(dir).Verify(context.Background())

	if !errors.Is(err, core.ErrLedgerChainBroken) {
		t.Fatalf("Verify = %v, want a break: the stored bytes still re-derive the patch-id but no longer hash to the sha the line commits to", err)
	}
}

func TestVerifyDeep_ResolvesASealedLegacyLineThroughItsSealedEvidenceRecord(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	line := legacyLineWithItsDiffGone(t, dir)
	digest, err := ledgerartifacts.Open(dir).Put([]byte(compTestDiff))
	if err != nil {
		t.Fatal(err)
	}
	appendEvidenceRecord(t, dir, line, digest)
	if err := New(dir).Append(ctx, core.LedgerEntry{Role: "builder", Kind: "k"}); err != nil {
		t.Fatal(err)
	}
	if err := New(dir).Seal(ctx, 1); err != nil {
		t.Fatal(err)
	}

	err = New(dir).VerifyDeep(ctx)

	if err != nil {
		t.Fatalf("VerifyDeep after a seal = %v, want OK: the evidence record was sealed into the segment beside its line", err)
	}
}

func TestWriteCompositionVerdict_RefusesALedgerPathThatIsALinkToAnotherLedger(t *testing.T) {
	project := t.TempDir()
	if err := New(project).Append(context.Background(), core.LedgerEntry{Role: "auditor", Kind: "agent_subprocess"}); err != nil {
		t.Fatal(err)
	}
	worktreeEvolve := filepath.Join(t.TempDir(), ".evolve")
	if err := os.MkdirAll(worktreeEvolve, 0o755); err != nil {
		t.Fatal(err)
	}
	linked := filepath.Join(worktreeEvolve, "ledger.jsonl")
	if err := os.Symlink(filepath.Join(project, "ledger.jsonl"), linked); err != nil {
		t.Fatal(err)
	}
	before := ledgerSize(t, filepath.Join(project, "ledger.jsonl"))

	err := WriteCompositionVerdict(linked, honestWriteInput(t))

	if err == nil || !strings.Contains(err.Error(), "link") {
		t.Fatalf("WriteCompositionVerdict through a linked ledger = %v, want a refusal naming the link: its tip, lock and store would be the link's directory's, forking the chain it points at", err)
	}
	if after := ledgerSize(t, filepath.Join(project, "ledger.jsonl")); after != before {
		t.Fatalf("the refused write still reached the linked ledger: %d → %d bytes", before, after)
	}
}
