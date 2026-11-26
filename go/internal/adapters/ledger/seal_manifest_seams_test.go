package ledger

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

var errForced = errors.New("forced failure")

func failingMarshalFrom(n int) func(any) ([]byte, error) {
	calls := 0
	return func(v any) ([]byte, error) {
		calls++
		if calls >= n {
			return nil, errForced
		}
		return json.Marshal(v)
	}
}

func failingReadF(match func(path string) bool) func(string) ([]byte, error) {
	return func(path string) ([]byte, error) {
		if match(path) {
			return nil, errForced
		}
		return os.ReadFile(path)
	}
}

func sealedOnce(t *testing.T) (*FileLedger, string, *core.LedgerEntry) {
	t.Helper()
	l, dir := seedLedger(t, 12)
	anchor, err := l.sealLocked(4)
	if err != nil {
		t.Fatal(err)
	}
	return l, dir, anchor
}

func TestSealRecordFor_ReportsASegmentDirectoryItCannotList(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, segmentsDirName), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := New(dir).writeManifestAndSealRecord(&core.LedgerEntry{ArtifactPath: segmentsDirName + "/seg-0001.jsonl.gz"}); err == nil {
		t.Fatal("want a segment list error")
	}
}

func TestSealRecordFor_ReportsAMissingSegment(t *testing.T) {
	l, _, _ := sealedOnce(t)
	if _, err := l.writeManifestAndSealRecord(&core.LedgerEntry{ArtifactPath: segmentsDirName + "/seg-0009.jsonl.gz"}); err == nil {
		t.Fatal("want a segment open error")
	}
}

func TestSealRecordFor_ReportsAnUnreadableGzFileAndAnUnreadableLiveFile(t *testing.T) {
	l, _, anchor := sealedOnce(t)
	cases := map[string]func(string) bool{
		"gz":   func(p string) bool { return strings.HasSuffix(p, ".jsonl.gz") },
		"live": func(p string) bool { return strings.HasSuffix(p, "ledger.jsonl") },
	}
	for name, match := range cases {
		withHooks(ledgerHooks{readF: failingReadF(match)}, func() {
			if _, err := l.writeManifestAndSealRecord(anchor); !errors.Is(err, errForced) {
				t.Errorf("%s: err = %v, want the read failure", name, err)
			}
		})
	}
}

func TestSealRecordFor_ReportsMarshalFailuresOfTheSummaryAndTheLine(t *testing.T) {
	for _, n := range []int{1, 2} {
		l, _, anchor := sealedOnce(t)
		withHooks(ledgerHooks{marshal: failingMarshalFrom(n)}, func() {
			if _, err := l.writeManifestAndSealRecord(anchor); !errors.Is(err, errForced) {
				t.Errorf("fail from call %d: err = %v, want the marshal failure", n, err)
			}
		})
	}
}

func TestSealRecordFor_ReportsAManifestItCannotWrite(t *testing.T) {
	l, _, anchor := sealedOnce(t)
	old := manifestWrite
	t.Cleanup(func() { manifestWrite = old })
	manifestWrite = func(string, []byte, os.FileMode) error { return errForced }
	if _, err := l.writeManifestAndSealRecord(anchor); !errors.Is(err, errForced) {
		t.Fatalf("err = %v, want the write failure", err)
	}
}

func TestSealRecordFor_ReportsAPrefixItCannotHeal(t *testing.T) {
	l, dir := sealOnceCrafted(t, func(rec *sealRecord) { rec.ManifestSHA256 = strings.Repeat("1", 64) })
	if err := os.Remove(manifestPathOf(dir)); err != nil {
		t.Fatal(err)
	}
	anchor, err := l.sealLocked(3)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := l.writeManifestAndSealRecord(anchor); !errors.Is(err, errManifestMismatch) {
		t.Fatalf("err = %v, want errManifestMismatch", err)
	}
}

func TestHealedPrefix_RefusesATailThatHoldsNoSealForThePreviousSegment(t *testing.T) {
	l, _ := seedLedger(t, 2)
	_, err := l.healedPrefix([]string{segmentsDirName + "/seg-0001.jsonl.gz"}, nil)
	if !errors.Is(err, errManifestMismatch) {
		t.Fatalf("err = %v, want errManifestMismatch", err)
	}
	if prefix, err := l.healedPrefix(nil, nil); prefix != nil || err != nil {
		t.Fatalf("first segment has no prefix, got %v %v", prefix, err)
	}
}

func TestReadManifest_TreatsAnyMalformedLineAsNoManifest(t *testing.T) {
	path := filepath.Join(t.TempDir(), "manifest.jsonl")
	if err := os.WriteFile(path, []byte("{\"segment\":\"a\",\"summary\":{}}\nnot json\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := readManifest(path); got != nil {
		t.Fatalf("readManifest = %v, want nil", got)
	}
	if got := readManifest(path + ".missing"); got != nil {
		t.Fatalf("missing manifest = %v, want nil", got)
	}
}

func TestRebuildLines_ReportsAChainAndAGzFileItCannotRead(t *testing.T) {
	l, _, _ := sealedOnce(t)
	cases := map[string]func(string) bool{
		"live": func(p string) bool { return strings.HasSuffix(p, "ledger.jsonl") },
		"gz":   func(p string) bool { return strings.HasSuffix(p, ".jsonl.gz") },
	}
	for name, match := range cases {
		withHooks(ledgerHooks{readF: failingReadF(match)}, func() {
			if _, err := l.rebuildLines(1); err == nil {
				t.Errorf("%s: want the read failure", name)
			}
		})
	}
}

func TestManifestLineFor_ChecksEachRecordedFactAgainstTheSegment(t *testing.T) {
	lines := [][]byte{[]byte(`{"kind":"a"}`), []byte(`{"kind":"b"}`)}
	seg := sealedSegment{rel: "ledger-segments/seg-0001.jsonl.gz", sha: "raw", lines: lines}
	good := summarizeSegment(lines, "raw", "gz")
	mutate := func(f func(s *segmentSummary)) sealRecord {
		s := good
		f(&s)
		raw, _ := json.Marshal(s)
		return testSeal("raw", raw)
	}
	bad := map[string]sealRecord{
		"raw":    mutate(func(s *segmentSummary) { s.RawSHA256 = "other" }),
		"gz":     mutate(func(s *segmentSummary) { s.GzSHA256 = "other" }),
		"lines":  mutate(func(s *segmentSummary) { s.Lines = 99 }),
		"decode": testSeal("raw", json.RawMessage(`"text"`)),
		"anchor": testSeal("different", mutate(func(*segmentSummary) {}).Summary),
	}
	for name, seal := range bad {
		if _, err := manifestLineFor(seg, "gz", seal); !errors.Is(err, errManifestMismatch) {
			t.Errorf("%s: err = %v, want errManifestMismatch", name, err)
		}
	}
	ok, err := manifestLineFor(seg, "gz", mutate(func(*segmentSummary) {}))
	if err != nil || !strings.Contains(string(ok.raw), `"lines":2`) {
		t.Fatalf("matching summary: %v %s", err, ok.raw)
	}
	for name, seal := range map[string]sealRecord{"recorded": mutate(func(*segmentSummary) {}), "computed": {}} {
		withHooks(ledgerHooks{marshal: failingMarshalFrom(1)}, func() {
			if _, err := manifestLineFor(seg, "gz", seal); err == nil {
				t.Errorf("%s: want the marshal failure", name)
			}
		})
	}
}

func TestSeal_ReportsAManifestFailureAndAppendsNoAnchor(t *testing.T) {
	l, dir := seedLedger(t, 12)
	old := manifestWrite
	t.Cleanup(func() { manifestWrite = old })
	manifestWrite = func(string, []byte, os.FileMode) error { return errForced }
	if err := l.Seal(context.Background(), 4); err == nil {
		t.Fatal("want the manifest failure from Seal")
	}
	raw, _ := os.ReadFile(filepath.Join(dir, "ledger.jsonl"))
	if len(decodeSeals(splitLines(raw))) != 0 {
		t.Error("an anchor was appended although the manifest failed")
	}
	manifestWrite = old
	if err := l.Seal(context.Background(), 4); err != nil {
		t.Fatalf("re-run after the failure: %v", err)
	}
	if err := l.VerifyDeep(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func testSeal(artifactSHA string, summary json.RawMessage) sealRecord {
	return sealRecord{LedgerEntry: core.LedgerEntry{ArtifactSHA256: artifactSHA}, sealExtension: sealExtension{Summary: summary}}
}

func TestDecodeSealRecord_KeepsTheSummaryAndTheManifestHashOfASealLine(t *testing.T) {
	want := sealRecord{
		LedgerEntry:   core.LedgerEntry{Kind: SealKind, ArtifactPath: segmentsDirName + "/seg-0001.jsonl.gz"},
		sealExtension: sealExtension{Summary: json.RawMessage(`{"lines":3}`), ManifestSHA256: "abc"},
	}
	line, err := json.Marshal(want)
	if err != nil {
		t.Fatal(err)
	}
	got, ok := decodeSealRecord(line)
	if !ok || got.ArtifactPath != want.ArtifactPath || string(got.Summary) != `{"lines":3}` || got.ManifestSHA256 != "abc" {
		t.Fatalf("decodeSealRecord = %+v, %v; want the summary and manifest_sha256 of the line", got, ok)
	}
	for _, other := range []string{`{"kind":"phase_complete"}`, `not json`} {
		if _, ok := decodeSealRecord([]byte(other)); ok {
			t.Errorf("decodeSealRecord(%q) reported a seal", other)
		}
	}
}

func TestSeal_AnIntactManifestIsKeptWithoutReadingOlderSegmentsAgain(t *testing.T) {
	l, dir, _ := sealTwice(t)
	older := filepath.Join(dir, segmentsDirName, "seg-0001.jsonl.gz")
	reads := 0
	counting := func(path string) ([]byte, error) {
		if path == older {
			reads++
		}
		return os.ReadFile(path)
	}
	withHooks(ledgerHooks{readF: counting}, func() {
		if err := l.Seal(context.Background(), 4); err != nil {
			t.Fatal(err)
		}
	})
	if reads != 0 {
		t.Errorf("the second seal read the older segment %d times, want 0 while the manifest matches its binding", reads)
	}
}

func TestSealRecordFor_ReportsASegmentPathItCannotRelate(t *testing.T) {
	l, _, _ := sealTwice(t)
	anchor, err := l.sealLocked(4)
	if err != nil {
		t.Fatal(err)
	}
	segmentRel = func(string, string) (string, error) { return "", errForced }
	t.Cleanup(func() { segmentRel = filepath.Rel })
	if _, err := l.writeManifestAndSealRecord(anchor); !errors.Is(err, errForced) {
		t.Fatalf("err = %v, want the relative path failure", err)
	}
}

func TestUnanchoredSegment_ReportsASegmentPathItCannotRelate(t *testing.T) {
	l, dir, _ := sealTwice(t)
	if err := l.Seal(context.Background(), 4); err != nil {
		t.Fatal(err)
	}
	segs, err := segmentFiles(filepath.Join(dir, segmentsDirName))
	if err != nil {
		t.Fatal(err)
	}
	liveRaw, err := os.ReadFile(l.ledgerPath)
	if err != nil {
		t.Fatal(err)
	}
	live := splitLines(liveRaw)
	if path, _, _, err := l.unanchoredSegment(segs, live); err != nil || path != "" {
		t.Fatalf("unanchoredSegment = %q, %v; want every sealed segment anchored by a seal line inside the next segment or the live file", path, err)
	}
	segmentRel = func(string, string) (string, error) { return "", errForced }
	t.Cleanup(func() { segmentRel = filepath.Rel })
	if _, _, _, err := l.unanchoredSegment(segs, live); !errors.Is(err, errForced) {
		t.Fatalf("err = %v, want the relative path failure", err)
	}
}

func TestUnanchoredSegment_ReportsASegmentItCannotRead(t *testing.T) {
	l, dir, _ := sealTwice(t)
	segs, err := segmentFiles(filepath.Join(dir, segmentsDirName))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(segs[0], []byte("not gzip"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, _, err := l.unanchoredSegment(segs, nil); !errors.Is(err, io.ErrUnexpectedEOF) {
		t.Fatalf("err = %v, want the truncated-gzip failure of the broken segment", err)
	}
}

func TestSeal_ACrashReRunKeepsTheManifestWithoutReadingOlderSegments(t *testing.T) {
	l, dir, _ := sealTwice(t)
	withHooks(ledgerHooks{openF: func(string, int, os.FileMode) (*os.File, error) { return nil, errForced }}, func() {
		_ = l.Seal(context.Background(), 3)
	})
	older := filepath.Join(dir, segmentsDirName, "seg-0001.jsonl.gz")
	reads := 0
	counting := func(path string) ([]byte, error) {
		if path == older {
			reads++
		}
		return os.ReadFile(path)
	}
	withHooks(ledgerHooks{readF: counting}, func() {
		if err := l.Seal(context.Background(), 3); err != nil {
			t.Fatal(err)
		}
	})
	if reads != 0 {
		t.Errorf("the re-run read the older segment %d times, want 0 while the kept manifest lines match the binding", reads)
	}
}
