package core

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func captureStderr(t *testing.T, fn func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	orig := os.Stderr
	os.Stderr = w
	fn()
	os.Stderr = orig
	if err := w.Close(); err != nil {
		t.Fatalf("close pipe writer: %v", err)
	}
	buf := make([]byte, 64*1024)
	n, _ := r.Read(buf)
	_ = r.Close()
	return string(buf[:n])
}

func TestWriteBuildSelfCheck_WriteFailureSurfaces(t *testing.T) {
	wt := t.TempDir()
	if err := os.WriteFile(filepath.Join(wt, ".evolve"), []byte("not a dir"), 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}
	fails := []selfCheckFailure{{Pkg: "./internal/foo", Output: "--- FAIL: TestFoo"}}

	out := captureStderr(t, func() {
		writeBuildSelfCheckArtifact(wt, fails)
	})

	if !strings.Contains(out, "WARN") {
		t.Fatalf("expected a WARN on build-selfcheck artifact write failure; got stderr:\n%q", out)
	}
	if !strings.Contains(out, "build-selfcheck") {
		t.Errorf("WARN should name the build-selfcheck artifact so an operator can find it; got:\n%q", out)
	}
}

func TestWriteBuildSelfCheck_HealthyWriteIsSilent(t *testing.T) {
	wt := t.TempDir()
	fails := []selfCheckFailure{{Pkg: "./internal/foo", Output: "--- FAIL: TestFoo"}}

	out := captureStderr(t, func() {
		writeBuildSelfCheckArtifact(wt, fails)
	})

	if out != "" {
		t.Errorf("a successful write must not emit stderr noise; got:\n%q", out)
	}
	data, err := os.ReadFile(filepath.Join(wt, ".evolve", "build-selfcheck.json"))
	if err != nil {
		t.Fatalf("artifact not written on the healthy path: %v", err)
	}
	if !strings.Contains(string(data), "./internal/foo") {
		t.Errorf("artifact must still contain the failing package: %s", data)
	}
}
