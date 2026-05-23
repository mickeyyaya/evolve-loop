package failurelog

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestRecord_StateUnreadable covers the os.ReadFile error branch that
// is NOT os.ErrNotExist (permission denied / IO error). chmod 000.
func TestRecord_StateUnreadable(t *testing.T) {
	t.Parallel()
	if os.Geteuid() == 0 {
		t.Skip("running as root — chmod 000 doesn't block reads")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")
	if err := os.WriteFile(path, []byte(`{}`), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := os.Chmod(path, 0o000); err != nil {
		t.Fatalf("chmod: %v", err)
	}
	defer os.Chmod(path, 0o644)

	_, err := Record(path, "", RecordRequest{Cycle: 1, Classification: "audit-fail"})
	if err == nil {
		t.Fatalf("expected error from unreadable state.json")
	}
	if errors.Is(err, ErrStateMissing) {
		t.Fatalf("unreadable should NOT be ErrStateMissing (file exists, just no perms); got %v", err)
	}
}

// TestExtractSummary_CapsAtMaxLines covers the `captured >= maxLines`
// break — feed 12 captured lines, assert only 8 land in the summary.
func TestExtractSummary_CapsAtMaxLines(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "report.md")
	body := "## Failure Root Cause\n"
	for i := 0; i < 12; i++ {
		body += fmt.Sprintf("line%d\n", i)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	s := extractSummary(path)
	// 8 lines kept; lines 0-7 included, lines 8-11 dropped.
	if !strings.Contains(s, "line7") {
		t.Fatalf("summary should include line7: %q", s)
	}
	if strings.Contains(s, "line8") || strings.Contains(s, "line11") {
		t.Fatalf("summary should cap at 8 lines: %q", s)
	}
}

// TestMustMarshalToAny_Defensive — the only way json.Marshal of
// Recorded fails is if a field is unmarshalable. Recorded uses plain
// strings + ints, so this is true-defensive: the fallback path
// returns {}. Cover it explicitly.
func TestMustMarshalToAny_Defensive(t *testing.T) {
	t.Parallel()
	// Pass an unmarshalable value (channel). mustMarshalToAny is
	// internal; we exercise it directly.
	got := mustMarshalToAny(make(chan int))
	if got == nil {
		t.Fatalf("must not return nil")
	}
	if len(got) != 0 {
		t.Fatalf("expected empty map for unmarshalable input; got %v", got)
	}
}

// TestAtomicWriteJSONReal_MarshalError exercises the
// json.MarshalIndent error branch by passing an unmarshalable map.
func TestAtomicWriteJSONReal_MarshalError(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")
	state := map[string]any{"chan": make(chan int)}
	if err := atomicWriteJSONReal(path, state); err == nil {
		t.Fatalf("expected marshal error")
	}
}

// TestAtomicWriteJSONReal_WriteTmpError exercises the os.WriteFile
// error branch by writing to a parent directory that doesn't exist.
func TestAtomicWriteJSONReal_WriteTmpError(t *testing.T) {
	t.Parallel()
	// Target path under a nonexistent dir → write fails.
	target := filepath.Join(t.TempDir(), "nope", "subdir", "state.json")
	err := atomicWriteJSONReal(target, map[string]any{"k": "v"})
	if err == nil {
		t.Fatalf("expected write error")
	}
	if !strings.Contains(err.Error(), "write tmp") {
		t.Fatalf("error should mention 'write tmp': %v", err)
	}
}

// TestAtomicWriteJSONReal_RenameError exercises the os.Rename error
// branch by trying to rename over a directory (which fails).
func TestAtomicWriteJSONReal_RenameError(t *testing.T) {
	t.Parallel()
	if os.Geteuid() == 0 {
		t.Skip("root can rename over anything")
	}
	dir := t.TempDir()
	// Make target a directory — os.Rename over a dir from a file fails
	// on most filesystems.
	target := filepath.Join(dir, "state.json")
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	// Place an unwriteable child inside so the dir-is-non-empty error fires.
	if err := os.WriteFile(filepath.Join(target, "child.txt"), nil, 0o644); err != nil {
		t.Fatalf("seed child: %v", err)
	}
	err := atomicWriteJSONReal(target, map[string]any{"k": "v"})
	if err == nil {
		t.Fatalf("expected rename error")
	}
}

// TestPruneExpired_StateUnreadable covers the os.ReadFile non-NotExist
// error branch in PruneExpired (permission denied).
func TestPruneExpired_StateUnreadable(t *testing.T) {
	t.Parallel()
	if os.Geteuid() == 0 {
		t.Skip("running as root — chmod 000 doesn't block reads")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")
	if err := os.WriteFile(path, []byte(`{}`), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := os.Chmod(path, 0o000); err != nil {
		t.Fatalf("chmod: %v", err)
	}
	defer os.Chmod(path, 0o644)

	_, err := PruneExpired(path, time.Now())
	if err == nil {
		t.Fatalf("expected read error from unreadable state.json")
	}
}

// TestPruneExpired_ZeroNow ensures time.Time{} input defaults to
// time.Now().UTC().
func TestPruneExpired_ZeroNow(t *testing.T) {
	t.Parallel()
	// Entry expired 1d ago → must be pruned even when caller passes
	// time.Time{}.
	yesterday := time.Now().UTC().Add(-24 * time.Hour).Format(time.RFC3339)
	path := seedStateWithEntries(t, []map[string]any{
		{"cycle": float64(1), "expiresAt": yesterday},
	})
	res, err := PruneExpired(path, time.Time{})
	if err != nil {
		t.Fatalf("PruneExpired: %v", err)
	}
	if res.Removed != 1 {
		t.Fatalf("removed=%d want 1 (zero now → time.Now)", res.Removed)
	}
}
