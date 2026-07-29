package opscmd

// console_lease_test.go — ADR-0080 S4 writer surface: the lease lands in the
// git COMMON dir (outside every worktree), paths are normalized to the exact
// repo-relative form the guard compares, value flags survive the documented
// paths-first invocation (the ReorderArgs regression class), and --clear
// removes the lease — and only the lease.

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/plane"
)

func TestRunConsoleLease_WritesHubLeaseThenClears(t *testing.T) {
	root := planePrimaryRoot(t, false)
	if err := os.WriteFile(filepath.Join(root, "notes.md"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	// The documented form is paths-first with space-separated value flags —
	// exactly the shape the bool-only ReorderArgs used to scatter.
	var out, errb bytes.Buffer
	rc := RunConsoleLease([]string{"./notes.md", "--project-root", root, "--ttl", "10m", "--reason", "test edit"}, nil, &out, &errb)
	if rc != 0 {
		t.Fatalf("rc = %d, want 0: %s%s", rc, out.String(), errb.String())
	}
	leasePath := filepath.Join(root, ".git", plane.ConsoleLeaseFileName)
	raw, err := os.ReadFile(leasePath)
	if err != nil {
		t.Fatalf("lease must land in the hub (common git dir): %v", err)
	}
	var lease leaseFile
	if err := json.Unmarshal(raw, &lease); err != nil {
		t.Fatal(err)
	}
	if len(lease.Paths) != 1 || lease.Paths[0] != "notes.md" {
		t.Errorf("paths = %v, want the ./ prefix normalized away to [notes.md]", lease.Paths)
	}
	if lease.Reason != "test edit" {
		t.Errorf("reason = %q, want %q — --reason was dropped by the parse", lease.Reason, "test edit")
	}
	exp, err := time.Parse(time.RFC3339, lease.ExpiresAt)
	if err != nil || !exp.After(time.Now()) {
		t.Errorf("expires_at = %q, want a parseable future RFC3339 stamp (err=%v)", lease.ExpiresAt, err)
	}
	if exp.After(time.Now().Add(20 * time.Minute)) {
		t.Errorf("expires_at = %q, beyond now+20m — --ttl 10m was dropped and the 30m default won", lease.ExpiresAt)
	}

	out.Reset()
	errb.Reset()
	if rc := RunConsoleLease([]string{"--project-root", root, "--clear"}, nil, &out, &errb); rc != 0 {
		t.Fatalf("clear rc = %d, want 0: %s%s", rc, out.String(), errb.String())
	}
	if _, err := os.Stat(leasePath); !os.IsNotExist(err) {
		t.Errorf("lease file must be removed on --clear (stat err=%v)", err)
	}
}

func TestRunConsoleLease_RejectsUnleasablePaths(t *testing.T) {
	root := planePrimaryRoot(t, false)
	// Seed an active lease: a rejected invocation must leave it untouched.
	leasePath := filepath.Join(root, ".git", plane.ConsoleLeaseFileName)
	sentinel := []byte(`{"paths":["keep.md"],"expires_at":"2099-01-01T00:00:00Z"}`)
	if err := os.WriteFile(leasePath, sentinel, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "notes.md"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name string
		args []string
	}{
		{"absolute path", []string{"--project-root", root, filepath.Join(root, "notes.md")}},
		{"missing file", []string{"--project-root", root, "no-such-file.md"}},
		{"escapes the repo", []string{"--project-root", root, "../outside.md"}},
		{"no paths at all", []string{"--project-root", root}},
		{"clear mixed with paths", []string{"--project-root", root, "--clear", "notes.md"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var out, errb bytes.Buffer
			if rc := RunConsoleLease(tc.args, nil, &out, &errb); rc != 10 {
				t.Errorf("rc = %d, want 10 (usage error): %s%s", rc, out.String(), errb.String())
			}
			got, err := os.ReadFile(leasePath)
			if err != nil || !bytes.Equal(got, sentinel) {
				t.Errorf("a rejected invocation must not touch the active lease (err=%v got=%s)", err, got)
			}
		})
	}
}

func TestRunConsoleLease_NonRepoRootErrors(t *testing.T) {
	var out, errb bytes.Buffer
	if rc := RunConsoleLease([]string{"--project-root", t.TempDir(), "x.md"}, nil, &out, &errb); rc != 1 {
		t.Errorf("rc = %d, want 1 when the hub cannot be resolved: %s%s", rc, out.String(), errb.String())
	}
}
