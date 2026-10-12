package ledger

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

func ledgerFileOnlyF2(t *testing.T, lines ...[]byte) (*FileLedger, string) {
	t.Helper()
	dir := t.TempDir()
	var raw []byte
	for _, line := range lines {
		raw = append(append(raw, line...), '\n')
	}
	if err := os.WriteFile(filepath.Join(dir, "ledger.jsonl"), raw, 0o644); err != nil {
		t.Fatal(err)
	}
	return New(dir), dir
}

func TestAppendChainedFromTail_RefusesWithoutAReadableNonEmptyChainAndAValidTipF2(t *testing.T) {
	t.Parallel()
	fill := func(seq int, prevHash string) any { return rawEntry(t, seq, prevHash, "x") }
	first := rawEntry(t, 0, ZeroSeed, "first")
	cases := []struct {
		name  string
		setup func(t *testing.T, dir string)
		want  string
	}{
		{"ledger file is absent", func(*testing.T, string) {}, "ledger read:"},
		{"ledger file is empty", func(t *testing.T, dir string) {
			writeFileF2(t, filepath.Join(dir, "ledger.jsonl"), "")
		}, "appendChainedFromTail on an empty chain"},
		{"tip file is malformed", func(t *testing.T, dir string) {
			writeFileF2(t, filepath.Join(dir, "ledger.jsonl"), string(first)+"\n")
			writeFileF2(t, filepath.Join(dir, "ledger.tip"), "garbage")
		}, "tip malformed"},
		{"tip seq is not a number", func(t *testing.T, dir string) {
			writeFileF2(t, filepath.Join(dir, "ledger.jsonl"), string(first)+"\n")
			writeFileF2(t, filepath.Join(dir, "ledger.tip"), "x:"+sha256Hex(first))
		}, "tip seq parse"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			tc.setup(t, dir)
			before := fileBytes(t, dir)

			err := New(dir).appendChainedFromTail(fill)

			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("appendChainedFromTail = %v, want an error containing %q", err, tc.want)
			}
			assertSameFiles(t, before, fileBytes(t, dir))
		})
	}
}

func TestAppendChainedFromTail_ChainsFromThePhysicalLastLineNotTheTipF2(t *testing.T) {
	t.Parallel()
	e0 := rawEntry(t, 0, ZeroSeed, "a")
	foreign := rawEntry(t, 1, sha256Hex(e0), "foreign append past the tip")
	l := writeChain(t, [][]byte{e0}, 0)
	dir := filepath.Dir(l.ledgerPath)
	raw, _ := os.ReadFile(l.ledgerPath)
	if err := os.WriteFile(l.ledgerPath, append(raw, append(foreign, '\n')...), 0o644); err != nil {
		t.Fatal(err)
	}
	var gotSeq int
	var gotPrev string

	err := l.appendChainedFromTail(func(seq int, prevHash string) any {
		gotSeq, gotPrev = seq, prevHash
		return rawEntry(t, seq, prevHash, "repair")
	})

	if err != nil || gotPrev != sha256Hex(foreign) || gotSeq != 1 {
		t.Fatalf("appendChainedFromTail = %v with (seq %d, prev %s), want the physical tail %s and tip seq+1", err, gotSeq, gotPrev, sha256Hex(foreign))
	}
	if tip := readFileF2(t, filepath.Join(dir, "ledger.tip")); !strings.HasPrefix(tip, "1:") {
		t.Errorf("tip = %q, want it moved to the appended line", tip)
	}
}

func TestVerifyScope_ReadFailuresAndEmptyFilesAreNotChainBreaksF2(t *testing.T) {
	line := rawEntry(t, 0, ZeroSeed, "a")
	otherErr := errors.New("disk on fire")
	cases := []struct {
		name      string
		content   []byte
		readErr   error
		wantErr   string
		wantScope VerifiedScope
	}{
		{"file vanishes between stat and read", append(line, '\n'), fs.ErrNotExist, "", VerifiedScope{}},
		{"read fails for another reason", append(line, '\n'), otherErr, "ledger read: disk on fire", VerifiedScope{}},
		{"file is empty", nil, nil, "", VerifiedScope{}},
		{"file holds only blank lines", []byte("\n\n"), nil, "", VerifiedScope{}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			ledgerPath := filepath.Join(dir, "ledger.jsonl")
			if err := os.WriteFile(ledgerPath, tc.content, 0o644); err != nil {
				t.Fatal(err)
			}
			var got VerifiedScope
			var err error

			withHooks(ledgerHooks{readF: func(path string) ([]byte, error) {
				if path == ledgerPath && tc.readErr != nil {
					return nil, tc.readErr
				}
				return os.ReadFile(path)
			}}, func() { got, err = New(dir).VerifyScope(context.Background()) })

			if tc.wantErr == "" && err != nil || tc.wantErr != "" && (err == nil || !strings.Contains(err.Error(), tc.wantErr)) || got != tc.wantScope {
				t.Errorf("VerifyScope = (%+v, %v), want (%+v, %q)", got, err, tc.wantScope, tc.wantErr)
			}
		})
	}
}

func TestVerifyScope_ASealedPredecessorThatCannotBeReadFailsLoudlyF2(t *testing.T) {
	t.Parallel()
	cases := map[string]func(t *testing.T, dir string){
		"segments path is a file": func(t *testing.T, dir string) {
			writeFileF2(t, filepath.Join(dir, segmentsDirName), "not a dir")
		},
		"newest segment is not gzip": func(t *testing.T, dir string) {
			writeFileF2(t, filepath.Join(dir, segmentsDirName, "seg-0001.jsonl.gz"), "not gzip")
		},
	}
	for name, setup := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			l, dir := ledgerFileOnlyF2(t, rawEntry(t, 0, ZeroSeed, "a"))
			setup(t, dir)

			got, err := l.VerifyScope(context.Background())

			if err == nil || got != (VerifiedScope{}) {
				t.Errorf("VerifyScope = (%+v, %v), want the segment error and a zero scope", got, err)
			}
			if errors.Is(err, core.ErrLedgerChainBroken) {
				t.Errorf("an unreadable segment is an I/O failure, not a chain break: %v", err)
			}
		})
	}
}

func TestSealedPredecessor_ReportsEachSegmentFailureAndSkipsASegmentStillLiveF2(t *testing.T) {
	t.Parallel()
	t.Run("segments path is a file", func(t *testing.T) {
		t.Parallel()
		l, dir := ledgerFileOnlyF2(t, rawEntry(t, 0, ZeroSeed, "a"))
		writeFileF2(t, filepath.Join(dir, segmentsDirName), "x")

		seed, err := l.sealedPredecessor(nil)

		if seed != nil || err == nil || !strings.Contains(err.Error(), "ledger segments read") {
			t.Errorf("sealedPredecessor = (%q, %v), want the segment dir error", seed, err)
		}
	})
	t.Run("segment is corrupt", func(t *testing.T) {
		t.Parallel()
		l, dir := ledgerFileOnlyF2(t, rawEntry(t, 0, ZeroSeed, "a"))
		writeFileF2(t, filepath.Join(dir, segmentsDirName, "seg-0001.jsonl.gz"), "not gzip")

		seed, err := l.sealedPredecessor(nil)

		if seed != nil || err == nil || !strings.Contains(err.Error(), "gunzip") {
			t.Errorf("sealedPredecessor = (%q, %v), want the gunzip error", seed, err)
		}
	})
}

func TestLiveScope_FallsBackToAChainedSealWhenTheSidecarAnchorIsAbsentF2(t *testing.T) {
	t.Parallel()
	seed := []byte(`{"ts":"2026-05-01T00:00:00Z","cycle":1,"role":"orchestrator","kind":"phase","exit_code":0,"entry_seq":3,"prev_hash":"` + sha256Of("older") + `"}`)
	seal := []byte(operatorSealLine(sha256Hex(seed)))
	lines := [][]byte{seed, seal}
	absent := sha256Of("a line no ledger carries")

	cases := []struct {
		name   string
		seed   []byte
		lines  [][]byte
		anchor string
		want   VerifiedScope
	}{
		{"sidecar absent from the lines and a chained seal exists", seed, lines, absent, VerifiedScope{AnchorLineSHA: sha256Hex(seal), AnchorSeq: 4}},
		{"sidecar absent and no seal: the sealed segment is the scope", seed, [][]byte{seed}, absent, VerifiedScope{AnchorLineSHA: sha256Hex(seed), AnchorSeq: 3, FromSealedSegment: true}},
		{"sidecar carried by the lines", seed, lines, sha256Hex(seed), VerifiedScope{AnchorLineSHA: sha256Hex(seal), AnchorSeq: 4}},
		{"no sealed predecessor keeps the sidecar", nil, lines, absent, VerifiedScope{AnchorLineSHA: absent}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := liveScope(tc.seed, tc.lines, tc.anchor)

			if got != tc.want {
				t.Errorf("liveScope = %+v, want %+v", got, tc.want)
			}
		})
	}
}

func TestWalkChain_AReplayedLineIsADuplicatePrevHashNotAForkSiblingF2(t *testing.T) {
	t.Parallel()
	x := rawEntry(t, 0, ZeroSeed, "x")
	y := rawEntry(t, 1, sha256Hex(x), "y")

	_, _, _, err := walkChain([][]byte{x, y, x, y}, "", compositionEvidenceIndex{})

	if !errors.Is(err, core.ErrLedgerChainBroken) || !strings.Contains(err.Error(), "line 3 duplicate prev_hash") {
		t.Errorf("walkChain = %v, want a duplicate prev_hash break at line 3", err)
	}
	if _, _, _, err := walkChain([][]byte{x, y, x}, "", compositionEvidenceIndex{}); err != nil {
		t.Errorf("a re-genesis replay of the first line alone is a valid seam: %v", err)
	}
}

func writeFileF2(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func readFileF2(t *testing.T, path string) string {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(body)
}

func TestVerifyScope_AnEmptyLiveTailBehindASealedSegmentVerifiesNothingF2(t *testing.T) {
	t.Parallel()
	e0 := rawEntry(t, 0, ZeroSeed, "a")
	e1 := rawEntry(t, 1, sha256Hex(e0), "b")
	l, dir := ledgerFileOnlyF2(t)
	if err := writeSegment(filepath.Join(dir, segmentsDirName, "seg-0001.jsonl.gz"), append(append(append([]byte{}, e0...), '\n'), append(e1, '\n')...)); err != nil {
		t.Fatal(err)
	}

	got, err := l.VerifyScope(context.Background())

	if err != nil || got != (VerifiedScope{}) {
		t.Errorf("VerifyScope = (%+v, %v), want a zero scope: an empty live tail has nothing to verify even when a segment precedes it", got, err)
	}
}
