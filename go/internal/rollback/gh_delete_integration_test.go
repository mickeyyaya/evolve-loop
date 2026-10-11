//go:build integration

package rollback

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/fakeclitest"
)

func stubGh(t *testing.T, body string) string {
	t.Helper()
	bin := t.TempDir()
	record := filepath.Join(bin, "calls")
	if body == "garbage" {
		if err := os.WriteFile(filepath.Join(bin, "gh"), []byte("not an executable format"), 0o755); err != nil {
			t.Fatal(err)
		}
	} else {
		fakeclitest.Install(t, filepath.Join(bin, "gh"), "#!/bin/sh\necho \"$PWD|$*\" >> '"+record+"'\n"+body+"\n")
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+"/bin:/usr/bin")
	return record
}

func TestDefaultGhDeleteReleaseIn_MapsEveryGhOutcome(t *testing.T) {
	for _, tc := range []struct {
		name, body, want string
	}{
		{"unlaunchable gh", "garbage", stepFailed},
		{"release absent", `[ "$2" = view ] && { echo 'release not found' >&2; exit 1; }`, stepNotPresent},
		{"release absent as 404", `[ "$2" = view ] && { echo 'HTTP 404' ; exit 1; }`, stepNotPresent},
		{"view fails otherwise", `[ "$2" = view ] && { echo 'auth required' >&2; exit 4; }`, stepFailed},
		{"delete fails", `[ "$2" = delete ] && exit 1; exit 0`, stepFailed},
		{"deleted", `exit 0`, stepDeleted},
	} {
		t.Run(tc.name, func(t *testing.T) {
			stubGh(t, tc.body)

			if got := defaultGhDeleteReleaseIn(t.TempDir(), "v1.2.3"); got != tc.want {
				t.Fatalf("defaultGhDeleteReleaseIn = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestDefaultGhDeleteReleaseIn_RunsBothCallsInTheRepoForTheTag(t *testing.T) {
	repo, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	record := stubGh(t, "exit 0")

	if got := defaultGhDeleteReleaseIn(repo, "v9.9.9"); got != stepDeleted {
		t.Fatalf("defaultGhDeleteReleaseIn = %q, want deleted", got)
	}
	calls, err := os.ReadFile(record)
	if err != nil {
		t.Fatal(err)
	}
	if want := repo + "|release view v9.9.9\n" + repo + "|release delete v9.9.9 --yes\n"; string(calls) != want {
		t.Fatalf("gh calls = %q, want %q", calls, want)
	}
}

func TestDefaultGhDeleteReleaseIn_NoGhIsSkipped(t *testing.T) {
	t.Setenv("PATH", t.TempDir())

	if got := defaultGhDeleteReleaseIn(t.TempDir(), "v1.2.3"); got != stepSkipped {
		t.Fatalf("defaultGhDeleteReleaseIn = %q, want skipped without gh", got)
	}
}
