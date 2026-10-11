//go:build integration

package preflight

import (
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/fakeclitest"
)

func TestUnameR_ReportsTheKernelReleaseWithoutItsNewline(t *testing.T) {
	want, err := exec.Command("uname", "-r").Output()
	if err != nil {
		t.Skipf("uname unavailable: %v", err)
	}

	got := unameR()

	if got == "" || strings.ContainsAny(got, "\n") || got != strings.TrimRight(string(want), "\n") {
		t.Fatalf("unameR() = %q, want %q without the trailing newline", got, want)
	}
}

func TestUnameR_KeepsInnerWhitespaceAndStripsOnlyTrailingNewlines(t *testing.T) {
	bin := t.TempDir()
	fakeclitest.Install(t, filepath.Join(bin, "uname"), "#!/bin/sh\n[ \"$1\" = \"-r\" ] || exit 3\nprintf ' 6.1 custom \\n\\n'\n")
	t.Setenv("PATH", bin)

	if got := unameR(); got != " 6.1 custom " {
		t.Fatalf("unameR() = %q, want %q", got, " 6.1 custom ")
	}
}

func TestUnameR_FailureIsUnknown(t *testing.T) {
	for name, script := range map[string]string{"exits non-zero": "exit 1", "missing": ""} {
		t.Run(name, func(t *testing.T) {
			bin := t.TempDir()
			if script != "" {
				fakeclitest.Install(t, filepath.Join(bin, "uname"), "#!/bin/sh\necho partial\n"+script+"\n")
			}
			t.Setenv("PATH", bin)

			if got := unameR(); got != "unknown" {
				t.Fatalf("unameR() = %q, want unknown", got)
			}
		})
	}
}
