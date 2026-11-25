package binaryguard

import (
	"os"
	"path/filepath"
	"testing"
)

var machO64 = []byte{0xcf, 0xfa, 0xed, 0xfe}

func writeFile(t *testing.T, dir, name string, head []byte, size int) string {
	t.Helper()
	buf := make([]byte, size)
	copy(buf, head)
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, buf, 0o644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
	return path
}

func TestScan_RejectsSyntheticLargeExecutable(t *testing.T) {
	dir := t.TempDir()
	const threshold = int64(1024)

	writeFile(t, dir, "evolve", machO64, int(threshold)+512)
	writeFile(t, dir, "fixture.json", []byte("{"), int(threshold)+512)
	writeFile(t, dir, "tiny", machO64, 16)

	offenders, err := Scan(dir, []string{"evolve", "fixture.json", "tiny"}, threshold)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if len(offenders) != 1 {
		t.Fatalf("want exactly 1 offender, got %d: %+v", len(offenders), offenders)
	}
	if offenders[0].Path != "evolve" {
		t.Errorf("want offender path=evolve, got %q", offenders[0].Path)
	}
	if offenders[0].Size <= threshold {
		t.Errorf("offender size %d must exceed threshold %d", offenders[0].Size, threshold)
	}
}

func TestScan_SkipsMissingFiles(t *testing.T) {
	offenders, err := Scan(t.TempDir(), []string{"gone.bin"}, DefaultThresholdBytes)
	if err != nil {
		t.Fatalf("Scan on missing file must not error: %v", err)
	}
	if len(offenders) != 0 {
		t.Fatalf("missing file must yield no offenders, got %+v", offenders)
	}
}

func TestHasExecutableMagic(t *testing.T) {
	cases := []struct {
		name string
		head []byte
		want bool
	}{
		{"elf", []byte{0x7f, 'E', 'L', 'F'}, true},
		{"macho64", machO64, true},
		{"pe", []byte{'M', 'Z', 0x00}, true},
		{"text", []byte("package main"), false},
		{"empty", nil, false},
	}
	for _, c := range cases {
		if got := HasExecutableMagic(c.head); got != c.want {
			t.Errorf("%s: HasExecutableMagic=%v want %v", c.name, got, c.want)
		}
	}
}

func TestOffender_RecordsPathAndSize(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "evolve", []byte{0x7f, 'E', 'L', 'F'}, 4096)
	got, err := Scan(dir, []string{"evolve"}, 1024)
	if err != nil {
		t.Fatal(err)
	}
	want := []Offender{{Path: "evolve", Size: 4096}}
	if len(got) != 1 || got[0] != want[0] {
		t.Errorf("Scan = %+v, want %+v", got, want)
	}
}
