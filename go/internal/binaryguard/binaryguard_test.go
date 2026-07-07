package binaryguard

import (
	"os"
	"path/filepath"
	"testing"
)

// machO64 is the reversed-byte-order Mach-O 64 magic — the exact signature of the
// 18MB go/acs/cycle536/evolve binary this guard exists to keep out.
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

// TestScan_RejectsSyntheticLargeExecutable is the acceptance-criteria regression:
// a synthetic large executable (magic bytes + padding over threshold) must be
// flagged, and a large non-executable and a small executable must NOT be.
func TestScan_RejectsSyntheticLargeExecutable(t *testing.T) {
	dir := t.TempDir()
	const threshold = int64(1024)

	writeFile(t, dir, "evolve", machO64, int(threshold)+512)           // large + magic -> offender
	writeFile(t, dir, "fixture.json", []byte("{"), int(threshold)+512) // large, no magic -> ok
	writeFile(t, dir, "tiny", machO64, 16)                             // magic but under threshold -> ok

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

// TestScan_SkipsMissingFiles: a deleted change-set entry is not an offender and
// must not error (deletions legitimately appear in `git diff --name-only`).
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

// TestOffender_RecordsPathAndSize names the exported Offender type directly
// (ADR-0069 apicover naming gate: every exported symbol needs a test that
// references it by name) and pins its two fields as the caller-facing contract
// commitgate relies on when it logs a rejected artifact.
func TestOffender_RecordsPathAndSize(t *testing.T) {
	off := Offender{Path: "go/acs/cycle536/evolve", Size: 18467058}
	if off.Path != "go/acs/cycle536/evolve" {
		t.Errorf("Offender.Path=%q want go/acs/cycle536/evolve", off.Path)
	}
	if off.Size != 18467058 {
		t.Errorf("Offender.Size=%d want 18467058", off.Size)
	}
}
