package recovery

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestPromoteSignature_StatFailureOtherThanAbsentIsReturnedAndWritesNothing(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	blocker := filepath.Join(root, "blocker")
	if err := os.WriteFile(blocker, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	id, err := PromoteSignature(filepath.Join(blocker, "sigs"), FatalSignature{Substr: "a fatal banner long enough", Cause: CauseModelInvalid})

	if err == nil {
		t.Fatalf("PromoteSignature under a regular file returned id %q and no error", id)
	}
	if id != "" {
		t.Fatalf("id = %q on a stat failure, want empty", id)
	}
	var pathErr *os.PathError
	if !errors.As(err, &pathErr) || pathErr.Op != "stat" {
		t.Fatalf("err = %v, want the stat error itself, not a later write error", err)
	}
}

func TestPromoteSignature_WriteFailureIsReturnedAndLeavesNoFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	if err := os.Chmod(dir, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o755) })
	if f, err := os.Create(filepath.Join(dir, "probe")); err == nil {
		_ = f.Close()
		t.Skip("directory stays writable (running as root)")
	}

	id, err := PromoteSignature(dir, FatalSignature{Substr: "a fatal banner long enough", Cause: CauseModelInvalid})

	if err == nil {
		t.Fatalf("PromoteSignature into a read-only dir returned id %q and no error", id)
	}
	if id != "" {
		t.Fatalf("id = %q on a write failure, want empty", id)
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 0 {
		t.Fatalf("read-only dir holds %d entries after a failed promotion", len(entries))
	}
}
