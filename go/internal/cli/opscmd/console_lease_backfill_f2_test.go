package opscmd

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/plane"
)

func restoreLeaseSeamsF2(t *testing.T) {
	t.Helper()
	t.Cleanup(func() {
		leaseMarshalIndent = json.MarshalIndent
		leaseCloseTemp = (*os.File).Close
	})
}

func leaseDirEntriesF2(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	return names
}

func TestRunConsoleLease_RefusesBadFlagsAndANonCheckoutRootF2(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name   string
		args   []string
		wantRC int
		want   string
	}{
		{"unknown flag", []string{"--bogus"}, 10, "flag provided but not defined"},
		{"unparseable ttl", []string{"notes.md", "--ttl", "soon"}, 10, "invalid value"},
		{"ttl without value", []string{"--ttl"}, 10, "flag needs an argument"},
		{"root that is not a checkout", []string{"notes.md", "--project-root", filepath.Join(t.TempDir(), "gone")}, 1, "not a git checkout"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var out, errb bytes.Buffer

			rc := RunConsoleLease(tc.args, nil, &out, &errb)

			if rc != tc.wantRC || out.Len() != 0 || !strings.Contains(errb.String(), tc.want) {
				t.Errorf("rc=%d out=%q err=%q, want rc %d and %q", rc, out.String(), errb.String(), tc.wantRC, tc.want)
			}
		})
	}
}

func TestParseConsoleLeaseArgs_KeepsPathsInOrderAcrossInterspersedFlagsF2(t *testing.T) {
	t.Parallel()
	var errb bytes.Buffer

	ttl, reason, root, clear, paths, code, done := parseConsoleLeaseArgs([]string{"a.md", "--ttl", "5m", "b/c.md", "--reason", "why", "--clear", "d.md", "--project-root", "/r"}, &errb)

	if code != 0 || done || ttl != 5*time.Minute || reason != "why" || root != "/r" || !clear || strings.Join(paths, ",") != "a.md,b/c.md,d.md" {
		t.Errorf("got ttl=%v reason=%q root=%q clear=%v paths=%v code=%d done=%v", ttl, reason, root, clear, paths, code, done)
	}
}

func TestHubLeasePath_ResolvesThePrimaryAndLinkedCheckoutsAndReportsABrokenLinkF2(t *testing.T) {
	t.Parallel()

	t.Run("primary checkout", func(t *testing.T) {
		t.Parallel()
		root := planePrimaryRoot(t, false)

		got, err := hubLeasePath(root)

		if err != nil || got != filepath.Join(root, ".git", plane.ConsoleLeaseFileName) {
			t.Errorf("hubLeasePath = (%q, %v), want the lease file inside .git", got, err)
		}
	})
	t.Run("linked worktree resolves its relative commondir", func(t *testing.T) {
		t.Parallel()
		hub := t.TempDir()
		gitdir := filepath.Join(hub, "worktrees", "wt")
		wt := t.TempDir()
		writeLeaseFileF2(t, filepath.Join(gitdir, "commondir"), "../..\n")
		writeLeaseFileF2(t, filepath.Join(wt, ".git"), "gitdir: "+gitdir+"\n")

		got, err := hubLeasePath(wt)

		if err != nil || got != filepath.Join(hub, plane.ConsoleLeaseFileName) {
			t.Errorf("hubLeasePath = (%q, %v), want the lease in the common dir %s", got, err, hub)
		}
	})
	t.Run("linked worktree without commondir", func(t *testing.T) {
		t.Parallel()
		gitdir := t.TempDir()
		wt := t.TempDir()
		writeLeaseFileF2(t, filepath.Join(wt, ".git"), "gitdir: "+gitdir+"\n")

		got, err := hubLeasePath(wt)

		if got != "" || err == nil || !strings.Contains(err.Error(), "linked worktree without commondir") {
			t.Errorf("hubLeasePath = (%q, %v), want the commondir error", got, err)
		}
		var out, errb bytes.Buffer
		if rc := RunConsoleLease([]string{"--project-root", wt, "--clear"}, nil, &out, &errb); rc != 1 || !strings.Contains(errb.String(), "commondir") {
			t.Errorf("RunConsoleLease rc=%d err=%q, want exit 1 naming commondir", rc, errb.String())
		}
	})
	t.Run("not a checkout", func(t *testing.T) {
		t.Parallel()

		got, err := hubLeasePath(t.TempDir())

		if got != "" || err == nil {
			t.Errorf("hubLeasePath = (%q, %v), want an error", got, err)
		}
	})
}

func writeLeaseFileF2(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestClearConsoleLease_RemovesOnlyTheLeaseAndReportsWhatItCannotRemoveF2(t *testing.T) {
	t.Parallel()

	t.Run("absent lease clears cleanly", func(t *testing.T) {
		t.Parallel()
		var out, errb bytes.Buffer

		rc := clearConsoleLease(filepath.Join(t.TempDir(), "lease.json"), nil, &out, &errb)

		if rc != 0 || !strings.Contains(out.String(), "cleared") || errb.Len() != 0 {
			t.Errorf("rc=%d out=%q err=%q", rc, out.String(), errb.String())
		}
	})
	t.Run("clear with paths is a usage error", func(t *testing.T) {
		t.Parallel()
		lease := filepath.Join(t.TempDir(), "lease.json")
		writeLeaseFileF2(t, lease, "{}")
		var out, errb bytes.Buffer

		rc := clearConsoleLease(lease, []string{"a.md"}, &out, &errb)

		if _, err := os.Stat(lease); rc != 10 || err != nil || !strings.Contains(errb.String(), "--clear takes no paths") {
			t.Errorf("rc=%d err=%q stat=%v, want exit 10 and the lease untouched", rc, errb.String(), err)
		}
	})
	t.Run("a lease that cannot be removed", func(t *testing.T) {
		t.Parallel()
		lease := filepath.Join(t.TempDir(), "lease.json")
		writeLeaseFileF2(t, filepath.Join(lease, "inner"), "x")
		var out, errb bytes.Buffer

		rc := clearConsoleLease(lease, nil, &out, &errb)

		if rc != 1 || out.Len() != 0 || !strings.Contains(errb.String(), "evolve console-lease: clear:") {
			t.Errorf("rc=%d out=%q err=%q, want exit 1 with the clear failure", rc, out.String(), errb.String())
		}
	})
}

func TestWriteConsoleLease_WritesTheLeaseAtomicallyWithItsPathsReasonAndExpiryF2(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	lease := filepath.Join(dir, "lease.json")
	var out, errb bytes.Buffer
	before := time.Now()

	rc := writeConsoleLease(lease, []string{"a.md", "dir/b.md"}, "because", 90*time.Second, &out, &errb)

	var got leaseFile
	raw, err := os.ReadFile(lease)
	if rc != 0 || err != nil || json.Unmarshal(raw, &got) != nil {
		t.Fatalf("rc=%d read=%v err=%q", rc, err, errb.String())
	}
	exp, perr := time.Parse(time.RFC3339, got.ExpiresAt)
	if perr != nil || exp.Before(before.Add(89*time.Second)) || exp.After(time.Now().Add(91*time.Second)) {
		t.Errorf("expires_at = %q, want about 90s from now (%v)", got.ExpiresAt, perr)
	}
	if strings.Join(got.Paths, ",") != "a.md,dir/b.md" || got.Reason != "because" {
		t.Errorf("lease = %+v", got)
	}
	if !strings.Contains(out.String(), "2 path(s) leased until") || !strings.Contains(out.String(), lease) {
		t.Errorf("stdout = %q", out.String())
	}
	if names := leaseDirEntriesF2(t, dir); len(names) != 1 {
		t.Errorf("dir holds %v, want only the lease (no temp file left)", names)
	}
}

func TestWriteConsoleLease_ReportsEachFailingStepAndLeavesNoTempFileF2(t *testing.T) {
	restoreLeaseSeamsF2(t)
	boom := errors.New("seam boom")
	cases := []struct {
		name  string
		setup func(t *testing.T, dir string) string
		want  string
	}{
		{"marshal fails", func(t *testing.T, dir string) string {
			leaseMarshalIndent = func(any, string, string) ([]byte, error) { return nil, boom }
			return filepath.Join(dir, "lease.json")
		}, "seam boom"},
		{"temp file cannot be created", func(t *testing.T, dir string) string {
			return filepath.Join(dir, "missing-dir", "lease.json")
		}, "no such file or directory"},
		{"close fails", func(t *testing.T, dir string) string {
			leaseCloseTemp = func(f *os.File) error { _ = f.Close(); return boom }
			return filepath.Join(dir, "lease.json")
		}, "seam boom"},
		{"rename onto a directory fails", func(t *testing.T, dir string) string {
			lease := filepath.Join(dir, "lease.json")
			writeLeaseFileF2(t, filepath.Join(lease, "inner"), "x")
			return lease
		}, "console-lease"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			restoreLeaseSeamsF2(t)
			dir := t.TempDir()
			lease := tc.setup(t, dir)
			var out, errb bytes.Buffer

			rc := writeConsoleLease(lease, []string{"a.md"}, "r", time.Minute, &out, &errb)

			if rc != 1 || out.Len() != 0 || !strings.HasPrefix(errb.String(), "evolve console-lease: ") || !strings.Contains(errb.String(), tc.want) {
				t.Errorf("rc=%d out=%q err=%q, want exit 1 containing %q", rc, out.String(), errb.String(), tc.want)
			}
			for _, name := range leaseDirEntriesF2(t, dir) {
				if strings.HasPrefix(name, "console-lease-") {
					t.Errorf("temp file %s left behind", name)
				}
			}
		})
	}
}
