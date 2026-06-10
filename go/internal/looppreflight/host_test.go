package looppreflight

import (
	"os"
	"testing"
)

func TestDefaultDirWritable(t *testing.T) {
	t.Run("Writable", func(t *testing.T) {
		if !defaultDirWritable(t.TempDir()) {
			t.Fatal("expected writable temp dir to return true")
		}
	})
	t.Run("Empty", func(t *testing.T) {
		if defaultDirWritable("") {
			t.Fatal("expected empty string to return false")
		}
	})
	t.Run("NoPermission", func(t *testing.T) {
		if os.Getuid() == 0 {
			t.Skip("chmod-based test unreliable as root")
		}
		dir := t.TempDir()
		if err := os.Chmod(dir, 0o000); err != nil {
			t.Fatalf("chmod: %v", err)
		}
		t.Cleanup(func() { _ = os.Chmod(dir, 0o755) })
		if defaultDirWritable(dir) {
			t.Fatal("expected 000-permission dir to return false")
		}
	})
}

func TestDefaultTmuxSessions(t *testing.T) {
	sessions, err := defaultTmuxSessions()
	if err != nil {
		if sessions != nil {
			t.Fatalf("on error, sessions must be nil; got %v", sessions)
		}
		return
	}
	_ = sessions // success path: sessions may be nil or non-nil
}
