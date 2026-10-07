package ledger

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
	"github.com/mickeyyaya/evolve-loop/go/internal/ledgerartifacts"
	"github.com/mickeyyaya/evolve-loop/go/internal/treedelta"
)

type carriedHistory struct {
	repo, evolveDir, worktree         string
	base0, tree0, base1, tree1, patch string
	diff                              []byte
}

func (h carriedHistory) legacyLine(cycle int, patchID string) compositionRecord {
	return compositionRecord{Cycle: cycle, Method: IdenticalRebaseMethod, PatchID: patchID,
		AuditedBase: h.base0, AuditedTreeSHA: h.tree0, GitHead: h.base1, TreeStateSHA: h.tree1,
		AuditedDiffPath:  filepath.Join(h.worktree, "composition-artifacts", "composition-1766-audited.diff"),
		ComposedDiffPath: filepath.Join(h.worktree, "composition-artifacts", "composition-1766-composed.diff")}
}

func identicalCarryHistory(t *testing.T) carriedHistory {
	t.Helper()
	r := gittest.Fixture(t)
	write := func(name, body string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(r.Dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	pend := func() string {
		write("lane.txt", "the lane's audited change\n")
		r.Git("add", "lane.txt")
		tree := r.Git("write-tree")
		r.Git("rm", "-q", "--cached", "lane.txt")
		return tree
	}
	write("base.txt", "base\n")
	r.Git("add", "base.txt")
	r.Git("commit", "-q", "-m", "base")
	h := carriedHistory{repo: r.Dir, evolveDir: filepath.Join(r.Dir, ".evolve"), worktree: filepath.Join(r.Dir, ".evolve", "worktrees", "cycle-cd3ae73e-1766")}
	h.base0, h.tree0 = r.Git("rev-parse", "HEAD"), pend()
	write("peer.txt", "a peer landing\n")
	r.Git("add", "peer.txt")
	r.Git("commit", "-q", "-m", "peer")
	h.base1, h.tree1 = r.Git("rev-parse", "HEAD"), pend()
	h.diff = []byte(r.Git(treedelta.Args(h.base0, h.tree0)...) + "\n")
	var err error
	if h.patch, err = PatchID(h.diff); err != nil {
		t.Fatal(err)
	}
	return h
}

func actionsOf(got []EvidenceRestoration) []EvidenceAction {
	actions := make([]EvidenceAction, 0, len(got))
	for _, r := range got {
		actions = append(actions, r.Action)
	}
	return actions
}

func restoreEvidence(t *testing.T, h carriedHistory) []EvidenceRestoration {
	t.Helper()
	got, err := New(h.evolveDir).RestoreCompositionEvidence(context.Background(), h.repo)
	if err != nil {
		t.Fatal(err)
	}
	return got
}

func previewEvidence(t *testing.T, h carriedHistory) []EvidenceRestoration {
	t.Helper()
	got, err := New(h.evolveDir).PreviewCompositionEvidence(context.Background(), h.repo)
	if err != nil {
		t.Fatal(err)
	}
	return got
}

func TestRestoreCompositionEvidence_RebuildsALegacyCarryFromTheGitObjectsItsLineNames(t *testing.T) {
	h := identicalCarryHistory(t)
	appendCompositionLine(t, h.evolveDir, h.legacyLine(1766, h.patch))
	if err := New(h.evolveDir).Verify(context.Background()); !errors.Is(err, core.ErrLedgerChainBroken) {
		t.Fatalf("Verify before the restore = %v, want the unreadable-diff break", err)
	}

	got := restoreEvidence(t, h)

	if len(got) != 1 || got[0].Action != EvidenceRebuilt || got[0].Line != 0 || got[0].Cycle != 1766 || got[0].Method != IdenticalRebaseMethod {
		t.Fatalf("restore = %+v, want line 0 of cycle 1766 rebuilt from git", got)
	}
	if err := New(h.evolveDir).Verify(context.Background()); err != nil {
		t.Fatalf("the restored ledger does not verify: %v", err)
	}
	if n := evidenceRecords(t, h); n != 1 {
		t.Fatalf("the restore appended %d chained evidence records, want 1 for the one line it restored", n)
	}
	if again := restoreEvidence(t, h); len(again) != 1 || again[0].Action != EvidenceDurable || evidenceRecords(t, h) != 1 {
		t.Fatalf("a second restore = %+v with %d evidence records, want the line already durable and nothing appended", again, evidenceRecords(t, h))
	}
}

func evidenceRecords(t *testing.T, h carriedHistory) int {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(h.evolveDir, "ledger.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	records := 0
	for _, line := range splitLines(raw) {
		var e struct {
			Kind string `json:"kind"`
		}
		if err := json.Unmarshal(line, &e); err != nil {
			t.Fatal(err)
		}
		if e.Kind == compositionEvidenceKind {
			records++
		}
	}
	return records
}

func TestRestoreCompositionEvidence_DryRunWritesNothing(t *testing.T) {
	h := identicalCarryHistory(t)
	appendCompositionLine(t, h.evolveDir, h.legacyLine(1766, h.patch))

	got := previewEvidence(t, h)

	if len(got) != 1 || got[0].Action != EvidenceRebuilt {
		t.Fatalf("dry run = %+v, want the line it would rebuild", got)
	}
	if _, err := os.Stat(filepath.Join(h.evolveDir, ledgerartifacts.DirName)); !os.IsNotExist(err) {
		t.Fatalf("a dry run created the evidence store (stat err=%v)", err)
	}
	if n := evidenceRecords(t, h); n != 0 {
		t.Fatalf("a dry run appended %d evidence records", n)
	}
	if err := New(h.evolveDir).Verify(context.Background()); !errors.Is(err, core.ErrLedgerChainBroken) {
		t.Fatalf("Verify after a dry run = %v, want the break still there", err)
	}
}

func TestRestoreCompositionEvidence_RefusesADiffThatDoesNotReDeriveTheRecordedPatchID(t *testing.T) {
	h := identicalCarryHistory(t)
	other, err := PatchID([]byte(compTestDiff))
	if err != nil {
		t.Fatal(err)
	}
	appendCompositionLine(t, h.evolveDir, h.legacyLine(1766, other))

	got := restoreEvidence(t, h)

	if len(got) != 1 || got[0].Action != EvidenceUnrestorable || got[0].Reason == "" {
		t.Fatalf("restore = %+v, want the line refused with its reason", got)
	}
	if _, err := os.Stat(filepath.Join(h.evolveDir, ledgerartifacts.DirName)); !os.IsNotExist(err) {
		t.Fatalf("a refused line stored evidence (stat err=%v)", err)
	}
}

func TestRestoreCompositionEvidence_StoresALegacyDiffItsWorktreeStillHolds(t *testing.T) {
	h := identicalCarryHistory(t)
	line := h.legacyLine(1810, h.patch)
	for _, p := range []string{line.AuditedDiffPath, line.ComposedDiffPath} {
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, h.diff, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	appendCompositionLine(t, h.evolveDir, line)

	got := restoreEvidence(t, h)
	if err := os.RemoveAll(h.worktree); err != nil {
		t.Fatal(err)
	}

	if len(got) != 1 || got[0].Action != EvidenceStored {
		t.Fatalf("restore = %+v, want the worktree's diff stored", got)
	}
	if err := New(h.evolveDir).Verify(context.Background()); err != nil {
		t.Fatalf("the line no longer verifies once its worktree is cleaned up: %v", err)
	}
}

func TestRestoreCompositionEvidence_NamesALineItCannotRegenerate(t *testing.T) {
	h := identicalCarryHistory(t)
	if err := WriteCompositionVerdict(filepath.Join(h.evolveDir, "ledger.jsonl"), honestWriteInput(t)); err != nil {
		t.Fatal(err)
	}
	legacyRungZero := h.legacyLine(1700, h.patch)
	legacyRungZero.Method, legacyRungZero.AuditedTreeSHA = TrivialRebaseMethod, ""
	appendCompositionLine(t, h.evolveDir, legacyRungZero)

	got := restoreEvidence(t, h)

	if !slices.Equal(actionsOf(got), []EvidenceAction{EvidenceDurable, EvidenceUnrestorable}) || got[1].Line != 1 {
		t.Fatalf("restore = %+v, want the stored line durable and the trivial-rebase line, which names no git tree, unrestorable", got)
	}
}

func TestRestoreCompositionEvidence_RebuildsALostStoredDiffWithoutAnEvidenceRecord(t *testing.T) {
	h := identicalCarryHistory(t)
	if err := WriteCompositionVerdict(filepath.Join(h.evolveDir, "ledger.jsonl"), CompositionVerdictInput{
		Cycle: 1810, Method: IdenticalRebaseMethod, LaneAuditRef: "audit-of-1810", PatchID: h.patch,
		AuditedBase: h.base0, AuditedTreeSHA: h.tree0, GitHead: h.base1, TreeStateSHA: h.tree1,
		GateResults: passingComposedGates(), AuditedDiff: h.diff, ComposedDiff: h.diff,
	}); err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(filepath.Join(h.evolveDir, ledgerartifacts.DirName)); err != nil {
		t.Fatal(err)
	}

	got := restoreEvidence(t, h)

	if len(got) != 1 || got[0].Action != EvidenceRebuilt {
		t.Fatalf("restore = %+v, want the line's own stored diff rebuilt from git", got)
	}
	if n := evidenceRecords(t, h); n != 0 {
		t.Fatalf("the restore appended %d evidence records for a line that names its own diffs by sha", n)
	}
	if err := New(h.evolveDir).Verify(context.Background()); err != nil {
		t.Fatalf("the line no longer verifies once its stored diff is rebuilt: %v", err)
	}
}

func TestRestoreCompositionEvidence_RebuildsAnAliasedLinesLostDiffsWithoutASecondRecord(t *testing.T) {
	h := identicalCarryHistory(t)
	appendCompositionLine(t, h.evolveDir, h.legacyLine(1766, h.patch))
	restoreEvidence(t, h)
	if err := os.RemoveAll(filepath.Join(h.evolveDir, ledgerartifacts.DirName)); err != nil {
		t.Fatal(err)
	}

	got := restoreEvidence(t, h)

	if len(got) != 1 || got[0].Action != EvidenceRebuilt {
		t.Fatalf("restore = %+v, want the diffs its evidence record names rebuilt from git", got)
	}
	if n := evidenceRecords(t, h); n != 1 {
		t.Fatalf("%d evidence records for one line, want the first one only", n)
	}
	if err := New(h.evolveDir).Verify(context.Background()); err != nil {
		t.Fatalf("the line no longer verifies once its evidence is rebuilt: %v", err)
	}
}

func samePatchIDOtherBytes(t *testing.T, diff []byte) []byte {
	t.Helper()
	want, err := PatchID(diff)
	if err != nil {
		t.Fatal(err)
	}
	for _, other := range [][]byte{append(append([]byte{}, diff...), '\n'), []byte(strings.Replace(string(diff), "\n+", "\n+ ", 1))} {
		if got, err := PatchID(other); err == nil && got == want && string(other) != string(diff) {
			return other
		}
	}
	t.Fatal("fixture: no byte variant of the diff keeps its patch-id")
	return nil
}

func TestRestoreCompositionEvidence_AppendsNoRecordWhenOneDiffCannotBeProven(t *testing.T) {
	h := identicalCarryHistory(t)
	line := h.legacyLine(1766, h.patch)
	line.TreeStateSHA = strings.Repeat("0", 40)
	appendCompositionLine(t, h.evolveDir, line)

	got := restoreEvidence(t, h)

	if len(got) != 1 || got[0].Action != EvidenceUnrestorable || !strings.Contains(got[0].Reason, "composed_diff_path") {
		t.Fatalf("restore = %+v, want the line unrestorable on its composed diff, whose tree no longer exists", got)
	}
	if n := evidenceRecords(t, h); n != 0 {
		t.Fatalf("%d evidence records appended for a line only half proven: a record is permanent", n)
	}
}

func TestRestoreCompositionEvidence_RefusesARebuiltDiffThatDoesNotHashToTheRecordedDigest(t *testing.T) {
	h := identicalCarryHistory(t)
	recorded := samePatchIDOtherBytes(t, h.diff)
	if err := WriteCompositionVerdict(filepath.Join(h.evolveDir, "ledger.jsonl"), CompositionVerdictInput{
		Cycle: 1810, Method: IdenticalRebaseMethod, LaneAuditRef: "audit-of-1810", PatchID: h.patch,
		AuditedBase: h.base0, AuditedTreeSHA: h.tree0, GitHead: h.base1, TreeStateSHA: h.tree1,
		GateResults: passingComposedGates(), AuditedDiff: recorded, ComposedDiff: recorded,
	}); err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(filepath.Join(h.evolveDir, ledgerartifacts.DirName)); err != nil {
		t.Fatal(err)
	}

	got := restoreEvidence(t, h)

	if len(got) != 1 || got[0].Action != EvidenceUnrestorable || !strings.Contains(got[0].Reason, "hashes to") {
		t.Fatalf("restore = %+v, want unrestorable: git rebuilds the same patch-id but not the bytes the line's sha commits to", got)
	}
}

func TestRestoreCompositionEvidence_SkipsLinesBeforeTheEpochAnchorAsVerifyDoes(t *testing.T) {
	ctx := context.Background()
	h := identicalCarryHistory(t)
	beforeTheAnchor := h.legacyLine(1700, h.patch)
	beforeTheAnchor.Method, beforeTheAnchor.AuditedTreeSHA = TrivialRebaseMethod, ""
	appendCompositionLine(t, h.evolveDir, beforeTheAnchor)
	l := New(h.evolveDir)
	if err := l.Append(ctx, core.LedgerEntry{Role: "operator", Kind: "k"}); err != nil {
		t.Fatal(err)
	}
	if err := l.Anchor(ctx, 1, "the prefix is adjudicated"); err != nil {
		t.Fatal(err)
	}
	if err := l.Verify(ctx); err != nil {
		t.Fatalf("fixture: Verify must skip the line before the anchor: %v", err)
	}

	got := restoreEvidence(t, h)

	if len(got) != 0 {
		t.Fatalf("restore = %+v, want nothing: a line Verify no longer checks is not restore's to refuse", got)
	}
}

func TestRestoreCompositionEvidence_RefusesALedgerWhoseAnchorLineIsGone(t *testing.T) {
	h := identicalCarryHistory(t)
	appendCompositionLine(t, h.evolveDir, h.legacyLine(1766, h.patch))
	anchor := `{"anchor_seq":0,"anchor_line_sha256":"` + strings.Repeat("c", 64) + `"}`
	if err := os.WriteFile(filepath.Join(h.evolveDir, "ledger-anchor.json"), []byte(anchor), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := New(h.evolveDir).RestoreCompositionEvidence(context.Background(), h.repo)

	if err == nil || !strings.Contains(err.Error(), "epoch anchor line not found") {
		t.Fatalf("restore = %v, want a refusal: with its anchor line gone the ledger has no verified epoch to restore into", err)
	}
}
