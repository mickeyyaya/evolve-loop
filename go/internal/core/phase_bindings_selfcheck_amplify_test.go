package core

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func amplCaptureStderr(t *testing.T, fn func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	orig := os.Stderr
	os.Stderr = w
	defer func() { os.Stderr = orig }()

	fn()

	w.Close()
	buf := make([]byte, 64*1024)
	n, _ := r.Read(buf)
	r.Close()
	return string(buf[:n])
}

func TestWriteBuildSelfCheckArtifact_MkdirAllFailure_WARNsAndDoesNotPanic(t *testing.T) {
	tmp := t.TempDir()
	// worktree itself is a REGULAR FILE, not a directory: MkdirAll(worktree/.evolve)
	// must fail because a path component is not a directory (ENOTDIR).
	blocked := filepath.Join(tmp, "blocked-worktree")
	if err := os.WriteFile(blocked, []byte("not a directory"), 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}

	stderr := amplCaptureStderr(t, func() {
		writeBuildSelfCheckArtifact(blocked, []selfCheckFailure{{Pkg: "./internal/foo", Output: "FAIL"}})
	})

	if !strings.Contains(stderr, "[selfcheck] WARN") || !strings.Contains(stderr, "could not persist build-selfcheck artifact") {
		t.Fatalf("expected a WARN naming the persist failure on MkdirAll error; got stderr:\n%q", stderr)
	}
}

func TestWriteBuildSelfCheckArtifact_HealthyRoundTrip_LargeAndUnicodeContent(t *testing.T) {
	worktree := t.TempDir()

	fails := make([]selfCheckFailure, 0, 500)
	for i := 0; i < 500; i++ {
		fails = append(fails, selfCheckFailure{
			Pkg:    "./internal/pkg" + string(rune('a'+i%26)),
			Output: "line one\nline two \"quoted\" — 日本語 emoji \U0001F680 tab\tend",
		})
	}

	stderr := amplCaptureStderr(t, func() {
		writeBuildSelfCheckArtifact(worktree, fails)
	})
	if stderr != "" {
		t.Fatalf("expected healthy write to stay silent; got stderr:\n%q", stderr)
	}

	data, err := os.ReadFile(filepath.Join(worktree, ".evolve", "build-selfcheck.json"))
	if err != nil {
		t.Fatalf("expected artifact to be written: %v", err)
	}
	var got []selfCheckFailure
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("artifact is not valid JSON: %v", err)
	}
	if len(got) != len(fails) {
		t.Fatalf("round-trip count mismatch: wrote %d, read back %d", len(fails), len(got))
	}
	if got[0] != fails[0] || got[len(got)-1] != fails[len(fails)-1] {
		t.Fatalf("round-trip content mismatch at boundary entries")
	}
}

func TestWriteBuildSelfCheckArtifact_EmptyNonNilSlice_WritesEmptyJSONArray(t *testing.T) {
	worktree := t.TempDir()
	writeBuildSelfCheckArtifact(worktree, []selfCheckFailure{})

	data, err := os.ReadFile(filepath.Join(worktree, ".evolve", "build-selfcheck.json"))
	if err != nil {
		t.Fatalf("expected artifact to be written: %v", err)
	}
	trimmed := strings.TrimSpace(string(data))
	if trimmed != "[]" {
		t.Fatalf("expected an empty JSON array for an empty-but-non-nil fails slice, got %q", trimmed)
	}
}

func TestWriteBuildSelfCheckArtifact_Idempotent_SecondCallOverwritesFirst(t *testing.T) {
	worktree := t.TempDir()
	writeBuildSelfCheckArtifact(worktree, []selfCheckFailure{{Pkg: "./internal/first", Output: "FAIL 1"}})
	writeBuildSelfCheckArtifact(worktree, []selfCheckFailure{{Pkg: "./internal/second", Output: "FAIL 2"}})

	data, err := os.ReadFile(filepath.Join(worktree, ".evolve", "build-selfcheck.json"))
	if err != nil {
		t.Fatalf("expected artifact to be written: %v", err)
	}
	var got []selfCheckFailure
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("artifact is not valid JSON: %v", err)
	}
	if len(got) != 1 || got[0].Pkg != "./internal/second" {
		t.Fatalf("expected the second call to fully overwrite the first, got %+v", got)
	}
}
