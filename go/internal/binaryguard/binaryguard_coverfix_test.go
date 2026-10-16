package binaryguard

import (
	"os"
	"strings"
	"testing"
)

func TestScan_EveryKnownExecutableFormatIsRejectedWhenLarge(t *testing.T) {
	t.Parallel()
	formats := map[string][]byte{
		"elf":              elfMagic,
		"macho32":          machO32Magic,
		"macho64":          machO64Magic,
		"macho64-reversed": machO64ReversedMagic,
		"macho32-reversed": machO32ReversedMagic,
		"macho-universal":  machOUniversalMagic,
		"pe":               peMagic,
	}
	const threshold = 64
	dir := t.TempDir()
	var names []string
	for name, magic := range formats {
		writeFile(t, dir, name, magic, threshold+16)
		names = append(names, name)
	}
	got, err := Scan(dir, names, threshold)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if len(got) != len(formats) {
		t.Errorf("rejected %d of %d executable formats: %+v", len(got), len(formats), got)
	}
}

func TestScan_ARealReadErrorIsReturnedNotSkipped(t *testing.T) {
	t.Parallel()
	if os.Geteuid() == 0 {
		t.Skip("root reads any file")
	}
	dir := t.TempDir()
	path := writeFile(t, dir, "locked", machO64, 256)
	if err := os.Chmod(path, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(path, 0o644) })
	offenders, err := Scan(dir, []string{"locked"}, 64)
	if err == nil || !strings.Contains(err.Error(), "locked") {
		t.Fatalf("an unreadable large file must fail the scan naming it, got offenders=%v err=%v", offenders, err)
	}
}

func TestScan_OffendersComeBackSortedByPathWhateverTheInputOrder(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeFile(t, dir, "b.bin", elfMagic, 128)
	writeFile(t, dir, "a.bin", elfMagic, 128)
	got, err := Scan(dir, []string{"b.bin", "a.bin"}, 64)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if len(got) != 2 || got[0].Path != "a.bin" || got[1].Path != "b.bin" {
		t.Errorf("offenders = %+v, want a.bin then b.bin", got)
	}
}

func TestScan_AStatFailureOtherThanMissingIsReturned(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeFile(t, dir, "plain", nil, 8)
	_, err := Scan(dir, []string{"plain/child"}, 64)
	if err == nil || !strings.Contains(err.Error(), "plain/child") {
		t.Fatalf("a path under a regular file is not a deletion; the scan must fail naming it, got %v", err)
	}
}

func TestScan_AFileThatYieldsNoBytesIsAReadErrorNotAPass(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeFile(t, dir, "empty", nil, 0)
	_, err := Scan(dir, []string{"empty"}, -1)
	if err == nil || !strings.Contains(err.Error(), "empty") {
		t.Fatalf("a file over the threshold that reads zero bytes must fail the scan, got %v", err)
	}
}
