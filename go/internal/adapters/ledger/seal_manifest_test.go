package ledger

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

func fileSHA(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return sha256Hex(b)
}

func manifestPathOf(dir string) string {
	return filepath.Join(dir, segmentsDirName, "manifest.jsonl")
}

func manifestFileLines(t *testing.T, dir string) [][]byte {
	t.Helper()
	b, err := os.ReadFile(manifestPathOf(dir))
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	return splitLines(b)
}

func liveSealKeys(t *testing.T, dir string) map[string]json.RawMessage {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(dir, "ledger.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	lines := splitLines(raw)
	keys := map[string]json.RawMessage{}
	if err := json.Unmarshal(lines[len(lines)-1], &keys); err != nil {
		t.Fatal(err)
	}
	return keys
}

func sealKeysString(t *testing.T, keys map[string]json.RawMessage, key string) string {
	t.Helper()
	var s string
	if err := json.Unmarshal(keys[key], &s); err != nil {
		t.Fatalf("key %s: %v", key, err)
	}
	return s
}

func appendMore(t *testing.T, l *FileLedger, from, n int) {
	t.Helper()
	for i := 0; i < n; i++ {
		if err := l.Append(context.Background(), core.LedgerEntry{Cycle: from + i, Role: "builder", Kind: "k"}); err != nil {
			t.Fatal(err)
		}
	}
}

func TestSealLine_CarriesSummaryAndManifestSHAMatchingTheFileOnDisk(t *testing.T) {
	l, dir := seedLedger(t, 20)
	if err := l.Seal(context.Background(), 5); err != nil {
		t.Fatalf("Seal: %v", err)
	}
	keys := liveSealKeys(t, dir)
	var sum map[string]any
	if err := json.Unmarshal(keys["summary"], &sum); err != nil {
		t.Fatalf("seal line has no summary object: %v", err)
	}
	if got := sealKeysString(t, keys, "manifest_sha256"); got != fileSHA(t, manifestPathOf(dir)) {
		t.Errorf("manifest_sha256 = %s, want sha of manifest.jsonl %s", got, fileSHA(t, manifestPathOf(dir)))
	}
	gz := fileSHA(t, filepath.Join(dir, segmentsDirName, "seg-0001.jsonl.gz"))
	want := map[string]any{
		"lines": float64(15), "seq_first": float64(0), "seq_last": float64(14),
		"raw_sha256": sealKeysString(t, keys, "artifact_sha256"), "gz_sha256": gz,
		"kind_counts": map[string]any{"phase_complete": float64(15)},
	}
	for k, v := range want {
		if !reflect.DeepEqual(sum[k], v) {
			t.Errorf("summary[%s] = %v, want %v", k, sum[k], v)
		}
	}
	cycles, _ := sum["cycles"].([]any)
	if len(cycles) != 14 || cycles[0] != float64(1) || cycles[13] != float64(14) {
		t.Errorf("summary cycles = %v, want 1..14 ascending without 0", sum["cycles"])
	}
	if n := len(manifestFileLines(t, dir)); n != 1 {
		t.Errorf("manifest lines = %d, want 1", n)
	}
	if err := l.VerifyDeep(context.Background()); err != nil {
		t.Fatalf("VerifyDeep: %v", err)
	}
}

func TestSealLine_DecodesThroughTheProductionReaderWithItsNewKeys(t *testing.T) {
	l, dir := seedLedger(t, 20)
	if err := l.Seal(context.Background(), 5); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(dir, "ledger.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	seals := decodeSeals(splitLines(raw))
	if len(seals) != 1 {
		t.Fatalf("decodeSeals = %d seals, want 1", len(seals))
	}
	if len(seals[0].Summary) == 0 || seals[0].ManifestSHA256 == "" {
		t.Errorf("decoded seal lost its keys: summary=%q manifest_sha256=%q", seals[0].Summary, seals[0].ManifestSHA256)
	}
}

func TestSeal_SecondSealExtendsTheManifestAndKeepsEarlierLinesByteIdentical(t *testing.T) {
	l, dir := seedLedger(t, 12)
	if err := l.Seal(context.Background(), 4); err != nil {
		t.Fatal(err)
	}
	first := manifestFileLines(t, dir)
	appendMore(t, l, 100, 6)
	if err := l.Seal(context.Background(), 3); err != nil {
		t.Fatal(err)
	}
	both := manifestFileLines(t, dir)
	if len(both) != 2 || !bytes.Equal(both[0], first[0]) {
		t.Fatalf("manifest after second seal = %d lines, first line kept=%v", len(both), len(both) > 0 && bytes.Equal(both[0], first[0]))
	}
	if got := sealKeysString(t, liveSealKeys(t, dir), "manifest_sha256"); got != fileSHA(t, manifestPathOf(dir)) {
		t.Errorf("second seal manifest_sha256 = %s, want %s", got, fileSHA(t, manifestPathOf(dir)))
	}
}

func sealTwice(t *testing.T) (*FileLedger, string, [][]byte) {
	t.Helper()
	l, dir := seedLedger(t, 12)
	if err := l.Seal(context.Background(), 4); err != nil {
		t.Fatal(err)
	}
	first := manifestFileLines(t, dir)
	appendMore(t, l, 100, 6)
	return l, dir, first
}

func TestSeal_DeletedManifestIsBuiltAgainAndBoundAgain(t *testing.T) {
	l, dir, first := sealTwice(t)
	if err := os.Remove(manifestPathOf(dir)); err != nil {
		t.Fatal(err)
	}
	if err := l.Seal(context.Background(), 3); err != nil {
		t.Fatalf("Seal with a deleted manifest: %v", err)
	}
	lines := manifestFileLines(t, dir)
	if len(lines) != 2 || !bytes.Equal(lines[0], first[0]) {
		t.Fatalf("rebuilt manifest = %d lines, line 1 identical=%v", len(lines), len(lines) > 0 && bytes.Equal(lines[0], first[0]))
	}
	if got := sealKeysString(t, liveSealKeys(t, dir), "manifest_sha256"); got != fileSHA(t, manifestPathOf(dir)) {
		t.Errorf("rebuilt manifest not bound: %s != %s", got, fileSHA(t, manifestPathOf(dir)))
	}
}

func TestSeal_TamperedManifestIsNotBoundIntoTheChain(t *testing.T) {
	l, dir, first := sealTwice(t)
	forged := append([]byte(`{"segment":"ledger-segments/seg-0001.jsonl.gz","summary":{"lines":999}}`), '\n')
	if err := os.WriteFile(manifestPathOf(dir), forged, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := l.Seal(context.Background(), 3); err != nil {
		t.Fatalf("Seal with a tampered manifest: %v", err)
	}
	lines := manifestFileLines(t, dir)
	if len(lines) != 2 || !bytes.Equal(lines[0], first[0]) {
		t.Fatalf("tamper survived: line 1 = %s", lines[0])
	}
	if strings.Contains(string(bytes.Join(lines, nil)), "999") {
		t.Error("forged summary is bound into the manifest")
	}
}

func TestSeal_CrashBeforeAnchorRerunWritesTheSameManifestWithoutADuplicateLine(t *testing.T) {
	l, dir, _ := sealTwice(t)
	withHooks(ledgerHooks{
		openF: func(string, int, os.FileMode) (*os.File, error) { return nil, errors.New("crash before anchor") },
	}, func() {
		if err := l.Seal(context.Background(), 3); err == nil {
			t.Fatal("expected the anchor append to fail")
		}
	})
	afterCrash := fileSHA(t, manifestPathOf(dir))
	if err := l.Seal(context.Background(), 3); err != nil {
		t.Fatalf("re-seal after crash: %v", err)
	}
	if len(manifestFileLines(t, dir)) != 2 {
		t.Fatalf("manifest has %d lines after the re-run, want 2", len(manifestFileLines(t, dir)))
	}
	if got := fileSHA(t, manifestPathOf(dir)); got != afterCrash {
		t.Errorf("re-run changed the manifest bytes: %s != %s", got, afterCrash)
	}
	if got := sealKeysString(t, liveSealKeys(t, dir), "manifest_sha256"); got != afterCrash {
		t.Errorf("anchor manifest_sha256 = %s, want %s", got, afterCrash)
	}
}

func appendCraftedSeal(t *testing.T, l *FileLedger, rec sealRecord) {
	t.Helper()
	if err := l.appendChained(chainSealInto(&rec)); err != nil {
		t.Fatal(err)
	}
}

func sealOnceCrafted(t *testing.T, mutate func(rec *sealRecord)) (*FileLedger, string) {
	t.Helper()
	l, dir := seedLedger(t, 12)
	anchor, err := l.sealLocked(4)
	if err != nil {
		t.Fatal(err)
	}
	rec, err := l.writeManifestAndSealRecord(anchor)
	if err != nil {
		t.Fatal(err)
	}
	mutate(rec)
	appendCraftedSeal(t, l, *rec)
	appendMore(t, l, 100, 6)
	return l, dir
}

func TestSeal_RebuildRefusesASealWhoseSummaryDisagreesWithItsSegmentAndWritesNothing(t *testing.T) {
	l, dir := sealOnceCrafted(t, func(rec *sealRecord) {
		var sum segmentSummary
		_ = json.Unmarshal(rec.Summary, &sum)
		sum.RawSHA256 = strings.Repeat("0", 64)
		rec.Summary, _ = json.Marshal(sum)
	})
	if err := os.Remove(manifestPathOf(dir)); err != nil {
		t.Fatal(err)
	}
	err := l.Seal(context.Background(), 3)
	if !errors.Is(err, errManifestMismatch) {
		t.Fatalf("Seal error = %v, want errManifestMismatch", err)
	}
	if _, statErr := os.Stat(manifestPathOf(dir)); !os.IsNotExist(statErr) {
		t.Errorf("manifest written despite the refusal: %v", statErr)
	}
}

func TestSeal_RebuildRefusesAPrefixThatDoesNotMatchThePreviousSealBinding(t *testing.T) {
	l, dir := sealOnceCrafted(t, func(rec *sealRecord) { rec.ManifestSHA256 = strings.Repeat("1", 64) })
	if err := os.Remove(manifestPathOf(dir)); err != nil {
		t.Fatal(err)
	}
	if err := l.Seal(context.Background(), 3); !errors.Is(err, errManifestMismatch) {
		t.Fatalf("Seal error = %v, want errManifestMismatch", err)
	}
}

func TestSeal_PreC11SealIsBackfilledFromTheSegment(t *testing.T) {
	l, dir := seedLedger(t, 12)
	anchor, err := l.sealLocked(4)
	if err != nil {
		t.Fatal(err)
	}
	if err := l.appendChainedLocked(chainInto(anchor)); err != nil {
		t.Fatal(err)
	}
	appendMore(t, l, 100, 6)
	if err := l.Seal(context.Background(), 3); err != nil {
		t.Fatalf("Seal over a pre-C11 seal: %v", err)
	}
	lines := manifestFileLines(t, dir)
	if len(lines) != 2 || !strings.Contains(string(lines[0]), `"lines":8`) {
		t.Fatalf("backfilled manifest = %q", lines)
	}
	if err := l.VerifyDeep(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestSummarizeSegment_RulesForOrderUndecodableLinesAndCycles(t *testing.T) {
	lines := [][]byte{
		[]byte(`{"ts":"","cycle":3,"kind":"a","entry_seq":7}`),
		[]byte(`not json`),
		[]byte(`{"ts":"2026-01-02T00:00:00Z","cycle":2,"kind":"a","entry_seq":8}`),
		[]byte(`{"ts":"2026-01-01T00:00:00Z","cycle":3,"kind":"b","entry_seq":0}`),
		[]byte(`{"ts":"","cycle":0,"kind":"b","entry_seq":9}`),
	}
	got := summarizeSegment(lines, "raw", "gz")
	want := segmentSummary{
		Version: summaryVersion, Lines: 5, SeqFirst: 7, SeqLast: 9,
		TSFirst: "2026-01-02T00:00:00Z", TSLast: "2026-01-01T00:00:00Z",
		Cycles:         []int{2, 3},
		KindCounts:     map[string]int{"a": 2, "b": 2, undecodableKind: 1},
		RawSHA256:      "raw",
		GzSHA256:       "gz",
		LastLineSHA256: sha256Hex(lines[4]),
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("summary = %+v, want %+v", got, want)
	}
	empty := summarizeSegment(nil, "r", "g")
	if empty.Cycles == nil || empty.KindCounts == nil || empty.Lines != 0 {
		t.Errorf("empty summary must carry empty containers, got %+v", empty)
	}
}
