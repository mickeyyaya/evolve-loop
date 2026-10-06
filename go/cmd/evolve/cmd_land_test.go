package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLand_BadArgumentsAreUsageErrors(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		args []string
	}{
		{"no flags", nil},
		{"missing --branch", []string{"--patch", "p.patch"}},
		{"neither input", []string{"--branch", "b"}},
		{"both inputs", []string{"--branch", "b", "--patch", "p.patch", "--salvage", "leaf"}},
		{"operand", []string{"--branch", "b", "--patch", "p.patch", "extra"}},
		{"unknown flag", []string{"--branch", "b", "--patch", "p.patch", "--force"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			var stdout, stderr bytes.Buffer
			if rc := runLand(c.args, nil, &stdout, &stderr); rc != exitUsage {
				t.Errorf("runLand(%v) = %d, want usage rc %d\n%s", c.args, rc, exitUsage, stderr.String())
			}
		})
	}
}

func TestLand_ExtractUntrackedRefusesEscapesAndOverwrites(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name, entry, preexisting, wantErr string
	}{
		{"parent escape", "../escaped.txt", "", "escapes the worktree"},
		{"absolute path", "/tmp/abs.txt", "", "escapes the worktree"},
		{"existing file kept", "kept.txt", "original\n", "exists"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			dir := filepath.Join(t.TempDir(), "wt")
			if err := os.MkdirAll(dir, 0o755); err != nil {
				t.Fatal(err)
			}
			if c.preexisting != "" {
				if err := os.WriteFile(filepath.Join(dir, c.entry), []byte(c.preexisting), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			archive := writeLandTestArchive(t, c.entry, "payload\n")
			err := extractUntracked(archive, dir)
			if err == nil || !strings.Contains(err.Error(), c.wantErr) {
				t.Fatalf("extractUntracked(%q) err = %v, want one containing %q", c.entry, err, c.wantErr)
			}
			if _, statErr := os.Stat(filepath.Join(filepath.Dir(dir), "escaped.txt")); statErr == nil {
				t.Errorf("an entry was written outside the worktree")
			}
			if c.preexisting != "" {
				if got, _ := os.ReadFile(filepath.Join(dir, c.entry)); string(got) != c.preexisting {
					t.Errorf("existing %s overwritten: %q", c.entry, got)
				}
			}
		})
	}
}

func writeLandTestArchive(t *testing.T, name, content string) string {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	if err := tw.WriteHeader(&tar.Header{Name: name, Mode: 0o644, Size: int64(len(content)), Typeflag: tar.TypeReg}); err != nil {
		t.Fatal(err)
	}
	if _, err := tw.Write([]byte(content)); err != nil {
		t.Fatal(err)
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "untracked.tgz")
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}
