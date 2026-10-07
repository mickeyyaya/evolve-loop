//go:build acs

package cycle1827

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/ledger"
	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/storage"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/ledgerartifacts"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

func TestC1827_001_PlainVerifyPassesOnASealedUntamperedLedger(t *testing.T) {
	ctx := context.Background()
	t.Run("one seal, then an append", func(t *testing.T) {
		l, _ := seedLedger(t, 20)
		if err := l.Verify(ctx); err != nil {
			t.Fatalf("Verify before any seal: %v", err)
		}
		sealOrFail(t, l, 5)
		if err := l.Verify(ctx); err != nil {
			t.Fatalf("Verify after Seal(5) of an untampered ledger: %v", err)
		}
		appendEntries(t, l, 20, 1)
		if err := l.Verify(ctx); err != nil {
			t.Fatalf("Verify after an append to a sealed ledger: %v", err)
		}
	})
	t.Run("a seal that keeps one line live", func(t *testing.T) {
		l, _ := seedLedger(t, 9)
		sealOrFail(t, l, 1)
		if err := l.Verify(ctx); err != nil {
			t.Fatalf("Verify after Seal(1): %v", err)
		}
	})
	t.Run("two seals with appends between them", func(t *testing.T) {
		l, dir := seedLedger(t, 12)
		sealOrFail(t, l, 4)
		appendEntries(t, l, 12, 6)
		sealOrFail(t, l, 3)
		if n := segmentCount(t, dir); n != 2 {
			t.Fatalf("fixture: %d segments, want 2", n)
		}
		if err := l.Verify(ctx); err != nil {
			t.Fatalf("Verify after a second seal sealed the first seal's anchor: %v", err)
		}
		if err := l.VerifyDeep(ctx); err != nil {
			t.Fatalf("VerifyDeep of the same ledger: %v", err)
		}
	})
}

func TestC1827_002_PlainVerifyStillFailsOnATamperedLiveLineAfterSeal(t *testing.T) {
	ctx := context.Background()
	tamperings := []struct {
		name   string
		tamper func(lines []string) []string
	}{
		{"a kept line's bytes edited", func(lines []string) []string {
			edited := slices.Clone(lines)
			edited[1] = strings.Replace(edited[1], "entry-16", "entry-61", 1)
			return edited
		}},
		{"the first live line removed, so the seam no longer meets the newest segment", func(lines []string) []string {
			return slices.Clone(lines[1:])
		}},
		{"the last live line edited", func(lines []string) []string {
			edited := slices.Clone(lines)
			last := len(edited) - 1
			edited[last] = strings.Replace(edited[last], `"role":"operator"`, `"role":"auditor"`, 1)
			return edited
		}},
	}
	for _, tc := range tamperings {
		t.Run(tc.name, func(t *testing.T) {
			l, dir := seedLedger(t, 20)
			sealOrFail(t, l, 5)
			before := liveLines(t, dir)
			after := tc.tamper(before)
			if slices.Equal(before, after) {
				t.Fatal("fixture: the tampering changed nothing")
			}
			writeLiveLines(t, dir, after)

			err := l.Verify(ctx)

			if !errors.Is(err, core.ErrLedgerChainBroken) {
				t.Fatalf("Verify of a sealed ledger with %s = %v, want core.ErrLedgerChainBroken", tc.name, err)
			}
		})
	}
}

func TestC1827_003_ByteIdenticalCarryReProvesOnASealedLedger(t *testing.T) {
	frozen := []string{
		"TestCarrySatisfied_ReProvesOnASealedLedger",
		"TestCarrySatisfied_DeclinesWhenASealedLedgersLiveTailLostALine",
	}
	out, err := goTestPackage(t, "./internal/phases/ship", "-v", "-run", "^("+strings.Join(frozen, "|")+")$")
	if err != nil {
		t.Fatalf("carrySatisfied does not re-prove a byte-identical carry on a sealed ledger: %v\n%s", err, out)
	}
	requirePassed(t, out, frozen...)
}

func TestC1827_004_VerifyDeepInsideSealsGapReportsNoResidue(t *testing.T) {
	const rounds = 5
	for round := 0; round < rounds; round++ {
		l, dir := seedLedger(t, 30)

		race := verifyDeepDuringSeal(t, l, dir, 5)

		if race.sealErr != nil {
			t.Fatalf("round %d: Seal alongside a VerifyDeep: %v", round, race.sealErr)
		}
		if errors.Is(race.verifyErr, ledger.ErrSealResidue) {
			t.Fatalf("round %d: a VerifyDeep queued behind Seal reported ErrSealResidue for the seal still in progress: %v", round, race.verifyErr)
		}
		if race.verifyErr != nil {
			t.Fatalf("round %d: a VerifyDeep queued behind Seal failed although nothing is damaged: %v", round, race.verifyErr)
		}
		if n := segmentCount(t, dir); n != 1 {
			t.Fatalf("round %d: %d segments after the seal, want 1", round, n)
		}
		if err := l.VerifyDeep(context.Background()); err != nil {
			t.Fatalf("round %d: VerifyDeep after the seal completed: %v", round, err)
		}
	}
}

func TestC1827_005_AnInterruptedSealStillReportsResidue(t *testing.T) {
	ctx := context.Background()
	l, dir := seedLedger(t, 20)
	sealOrFail(t, l, 5)
	lines := liveLines(t, dir)
	var anchor core.LedgerEntry
	if err := json.Unmarshal([]byte(lines[len(lines)-1]), &anchor); err != nil || anchor.Kind != ledger.SealKind {
		t.Fatalf("fixture: the last live line after Seal is not its %s anchor (kind %q, err %v)", ledger.SealKind, anchor.Kind, err)
	}
	crashedBeforeTheAnchor := lines[:len(lines)-1]
	writeLiveLines(t, dir, crashedBeforeTheAnchor)
	pointTipAt(t, dir, crashedBeforeTheAnchor[len(crashedBeforeTheAnchor)-1])

	err := l.VerifyDeep(ctx)

	if !errors.Is(err, ledger.ErrSealResidue) {
		t.Fatalf("VerifyDeep of a seal that truncated but never anchored, with no seal running = %v, want ErrSealResidue", err)
	}
	sealOrFail(t, l, 5)
	if err := l.VerifyDeep(ctx); err != nil {
		t.Fatalf("re-running Seal must complete the interrupted seal: %v", err)
	}
}

// acs-predicate: config-check
func TestC1827_006_LedgerPlanListsTheItemAsAHardPrerequisiteOfC12AndC13(t *testing.T) {
	plan := filepath.Join(acsassert.RepoRoot(t), "docs", "plans", "ledger-restructure-2026-10.md")
	body, err := os.ReadFile(plan)
	if err != nil {
		t.Fatalf("reading the ledger plan: %v", err)
	}
	rows := map[string]string{}
	for _, line := range strings.Split(string(body), "\n") {
		for _, id := range []string{"C12", "C13"} {
			if strings.HasPrefix(line, "| "+id+" |") {
				rows[id] = line
			}
		}
	}
	for _, id := range []string{"C12", "C13"} {
		row, ok := rows[id]
		if !ok {
			t.Errorf("the plan has no %s row", id)
			continue
		}
		lower := strings.ToLower(row)
		if !strings.Contains(row, "seal-breaks-live-only-verify-and-carry") || !(strings.Contains(lower, "prerequisite") || strings.Contains(lower, "depends on")) {
			t.Errorf("the %s row does not list seal-breaks-live-only-verify-and-carry as a prerequisite:\n%s", id, row)
		}
	}
}

func TestC1827_007_AtomicwriteOwnsOneDurableWriterWithEveryFaultBranchTested(t *testing.T) {
	frozen := []string{
		"TestDurable_WritesTheBytesWithTheRequestedMode",
		"TestDurable_WritesAZeroLengthPayload",
		"TestDurable_ReplacesAnExistingTarget",
		"TestDurable_ARenameFailureLeavesNoTempFile",
		"TestDurable_ACreateFailureLeavesNoTempFile",
		"TestDurable_ReportsADirectoryItCannotSync",
	}
	out, err := goTestPackage(t, "./internal/atomicwrite", "-v", "-cover")
	if err != nil {
		t.Fatalf("atomicwrite's tests, including the durable variant's contract, fail: %v\n%s", err, out)
	}
	requirePassed(t, out, frozen...)
	if !strings.Contains(out, "coverage: 100.0% of statements") {
		t.Errorf("atomicwrite is no longer fully covered, so a fault branch of the durable writer runs in no test:\n%s", out)
	}
}

func TestC1827_008_StateJSONWriterDelegatesToTheDurableWriterUnchanged(t *testing.T) {
	t.Run("state.json is written as before", func(t *testing.T) {
		dir := t.TempDir()
		st := storage.New(dir)
		for _, cycle := range []int{1824, 1825} {
			if err := st.WriteState(context.Background(), core.State{LastCycleNumber: cycle}); err != nil {
				t.Fatalf("WriteState(%d): %v", cycle, err)
			}
		}
		path := filepath.Join(dir, "state.json")
		body, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.HasSuffix(body, []byte("}\n")) || !json.Valid(body) {
			t.Errorf("state.json is not indented JSON with its trailing newline: %q", body)
		}
		var back core.State
		if err := json.Unmarshal(body, &back); err != nil || back.LastCycleNumber != 1825 {
			t.Errorf("state.json does not hold the latest write: %+v, %v", back, err)
		}
		if mode := permOf(t, path); mode != 0o600 {
			t.Errorf("state.json mode = %v, want the writer's 0600", mode)
		}
		if names := entryNames(t, dir); !slices.Equal(names, []string{"state.json"}) {
			t.Errorf("the state dir holds %v, want only state.json", names)
		}
	})
	t.Run("writeJSONAtomic delegates to atomicwrite.Durable", func(t *testing.T) {
		for _, problem := range durableDelegationProblems(t, "internal/adapters/storage", "writeJSONAtomic") {
			t.Error(problem)
		}
	})
}

func TestC1827_009_SegmentWriterDelegatesToTheDurableWriterUnchanged(t *testing.T) {
	t.Run("the segment is written as before", func(t *testing.T) {
		l, dir := seedLedger(t, 12)
		pre, err := os.ReadFile(filepath.Join(dir, "ledger.jsonl"))
		if err != nil {
			t.Fatal(err)
		}
		sealOrFail(t, l, 4)
		segDir := filepath.Join(dir, "ledger-segments")
		seg := filepath.Join(segDir, "seg-0001.jsonl.gz")
		f, err := os.Open(seg)
		if err != nil {
			t.Fatalf("the sealed segment is missing: %v", err)
		}
		defer func() { _ = f.Close() }()
		zr, err := gzip.NewReader(f)
		if err != nil {
			t.Fatalf("the segment is not gzip: %v", err)
		}
		sealed, err := io.ReadAll(zr)
		if err != nil {
			t.Fatal(err)
		}
		lines := strings.SplitAfter(string(pre), "\n")
		if want := strings.Join(lines[:8], ""); string(sealed) != want {
			t.Errorf("the segment does not hold the sealed prefix byte for byte:\n got %q\nwant %q", sealed, want)
		}
		if mode := permOf(t, seg); mode != 0o600 {
			t.Errorf("segment mode = %v, want the writer's 0600", mode)
		}
		if names := entryNames(t, segDir); !slices.Equal(names, []string{"seg-0001.jsonl.gz"}) {
			t.Errorf("ledger-segments holds %v, want only the segment", names)
		}
		if err := l.VerifyDeep(context.Background()); err != nil {
			t.Errorf("VerifyDeep after the seal: %v", err)
		}
	})
	t.Run("writeSegment delegates to atomicwrite.Durable", func(t *testing.T) {
		for _, problem := range durableDelegationProblems(t, "internal/adapters/ledger", "writeSegment") {
			t.Error(problem)
		}
	})
}

func TestC1827_010_EvidenceStoreWriterDelegatesToTheDurableWriterUnchanged(t *testing.T) {
	t.Run("an evidence object is written as before", func(t *testing.T) {
		store := ledgerartifacts.Open(t.TempDir())
		body := []byte("diff --git a/x b/x\n+evidence\n")
		digest, err := store.Put(body)
		if err != nil || digest != ledgerartifacts.Digest(body) {
			t.Fatalf("Put = (%q, %v), want the body's digest", digest, err)
		}
		path, err := store.Path(digest)
		if err != nil {
			t.Fatal(err)
		}
		assertObject := func(stage string) {
			t.Helper()
			got, err := store.Get(digest)
			if err != nil || !bytes.Equal(got, body) {
				t.Errorf("%s: Get = (%q, %v), want the stored body", stage, got, err)
			}
			if mode := permOf(t, path); mode != 0o444 {
				t.Errorf("%s: object mode = %v, want the store's write-once 0444", stage, mode)
			}
			if names := entryNames(t, filepath.Dir(path)); !slices.Equal(names, []string{filepath.Base(path)}) {
				t.Errorf("%s: the fan-out dir holds %v, want only the object", stage, names)
			}
		}
		assertObject("first put")
		if again, err := store.Put(body); err != nil || again != digest {
			t.Errorf("a second Put of the same body = (%q, %v)", again, err)
		}
		assertObject("idempotent put")
		if err := os.Chmod(path, 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("corrupted in place\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.Chmod(path, 0o444); err != nil {
			t.Fatal(err)
		}
		if _, err := store.Put(body); err != nil {
			t.Fatalf("Put over a corrupted object: %v", err)
		}
		assertObject("put over a corrupted object")
	})
	t.Run("writeAtomically delegates to atomicwrite.Durable", func(t *testing.T) {
		for _, problem := range durableDelegationProblems(t, "internal/ledgerartifacts", "writeAtomically") {
			t.Error(problem)
		}
	})
}

func TestC1827_011_TheThreeWritersExistingTestsStayGreen(t *testing.T) {
	for _, pkg := range []string{"./internal/adapters/storage", "./internal/ledgerartifacts", "./internal/adapters/ledger"} {
		t.Run(pkg, func(t *testing.T) {
			if out, err := goTestPackage(t, pkg, "-short"); err != nil {
				t.Errorf("the tests of %s fail after the move onto atomicwrite.Durable: %v\n%s", pkg, err, out)
			}
		})
	}
}

func TestC1827_012_SegmentWriterSyncsItsDirectoryAfterTheRename(t *testing.T) {
	t.Run("writeSegment goes through the writer that syncs the directory", func(t *testing.T) {
		for _, problem := range durableDelegationProblems(t, "internal/adapters/ledger", "writeSegment") {
			t.Error(problem)
		}
	})
	t.Run("the durable writer fails when it cannot sync the directory it renamed into", func(t *testing.T) {
		const dirSync = "TestDurable_ReportsADirectoryItCannotSync"
		out, err := goTestPackage(t, "./internal/atomicwrite", "-v", "-run", "^"+dirSync+"$")
		if err != nil {
			t.Fatalf("atomicwrite.Durable does not sync the directory after the rename: %v\n%s", err, out)
		}
		requirePassed(t, out, dirSync)
	})
}
