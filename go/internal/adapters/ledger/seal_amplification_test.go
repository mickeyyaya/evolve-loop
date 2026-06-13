package ledger

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAmplifyReadSegmentRejectsPlainTextAndPreservesValidNeighbor(t *testing.T) {
	dir := t.TempDir()
	valid := filepath.Join(dir, "valid.gz")
	invalid := filepath.Join(dir, "invalid.gz")

	want := [][]byte{
		[]byte(`{"entry_seq":1,"prev_hash":""}`),
		[]byte(`{"entry_seq":2,"prev_hash":"abc"}`),
	}
	raw := append(append([]byte{}, want[0]...), '\n')
	raw = append(raw, want[1]...)
	raw = append(raw, '\n')
	if err := writeSegment(valid, raw); err != nil {
		t.Fatalf("write valid segment: %v", err)
	}
	if err := os.WriteFile(invalid, []byte("not gzip data\n"), 0o644); err != nil {
		t.Fatalf("write invalid segment: %v", err)
	}

	if _, _, err := readSegment(invalid); err == nil {
		t.Fatalf("readSegment accepted a plain-text file with .gz suffix")
	}
	got, _, err := readSegment(valid)
	if err != nil {
		t.Fatalf("readSegment valid neighbor after invalid read: %v", err)
	}
	if !linesEqual(got, want) {
		t.Fatalf("valid neighbor changed after invalid read: got %#v want %#v", got, want)
	}
}

func TestAmplifyLinesEqualDistinguishesNilEmptyAndMutatedBytes(t *testing.T) {
	empty := [][]byte{}
	if !linesEqual(empty, empty) {
		t.Fatalf("linesEqual should accept the same empty slice value")
	}
	if !linesEqual(nil, empty) {
		t.Fatalf("linesEqual should treat nil and empty slices as equivalent empty content")
	}

	left := [][]byte{[]byte("same"), []byte("before")}
	right := [][]byte{[]byte("same"), []byte("before")}
	if !linesEqual(left, right) {
		t.Fatalf("linesEqual rejected byte-identical lines")
	}
	right[1][0] = 'B'
	if linesEqual(left, right) {
		t.Fatalf("linesEqual accepted mutated line bytes")
	}
}
