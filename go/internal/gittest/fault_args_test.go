package gittest

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInstallFault_ArgsNarrowTheVerbToCallsCarryingEveryArg(t *testing.T) {
	r := committedFixture(t)

	InstallFault(t, GitFault{Verb: "rev-parse", Args: []string{"--absolute-git-dir"}, Code: 7})

	if _, code := gitExit(t, r.Dir, "rev-parse", "HEAD"); code != 0 {
		t.Errorf("rev-parse HEAD under a --absolute-git-dir fault exit = %d, want 0", code)
	}
	if _, code := gitExit(t, r.Dir, "rev-parse", "--absolute-git-dir"); code != 7 {
		t.Errorf("rev-parse --absolute-git-dir exit = %d, want 7", code)
	}
}

func TestInstallFault_TheVerbIsTheSubcommandNotAnyArgument(t *testing.T) {
	r := committedFixture(t)
	statusDir := filepath.Join(t.TempDir(), "status")
	if err := os.Mkdir(statusDir, 0o755); err != nil {
		t.Fatal(err)
	}

	FaultOn(t, "status")

	if _, code := gitExit(t, r.Dir, "-c", "core.quotePath=false", "log", "-1", "--format=%s", "--", "status"); code != 0 {
		t.Errorf("log naming a status path exit = %d, want the real git", code)
	}
	if _, code := gitExit(t, r.Dir, "-c", "core.quotePath=false", "status"); code != 128 {
		t.Errorf("status after a -c option exit = %d, want 128", code)
	}
	if _, code := gitExit(t, statusDir, "init", "-q"); code != 0 {
		t.Errorf("init in a dir named status exit = %d, want the real git", code)
	}
}

func TestInstallFault_StackedFaultsBothApply(t *testing.T) {
	r := committedFixture(t)

	FaultOn(t, "status")
	InstallFault(t, GitFault{Verb: "show-ref", Code: 9})

	if _, code := gitExit(t, r.Dir, "status"); code != 128 {
		t.Errorf("status under the outer fault exit = %d, want 128", code)
	}
	if _, code := gitExit(t, r.Dir, "show-ref"); code != 9 {
		t.Errorf("show-ref under the inner fault exit = %d, want 9", code)
	}
}

func TestBranchExists_SeesThroughAShowRefFault(t *testing.T) {
	r := committedFixture(t)

	InstallFault(t, GitFault{Verb: "show-ref", Code: 0})

	if !BranchExists(t, r.Dir, "main") {
		t.Error("BranchExists missed main under a show-ref fault")
	}
	if BranchExists(t, r.Dir, "nope") {
		t.Error("BranchExists found a missing branch under a show-ref fault")
	}
}

func TestGitFaultValidate(t *testing.T) {
	cases := []struct {
		name    string
		f       GitFault
		wantErr string
	}{
		{"valid", GitFault{Verb: "status", Code: 255}, ""},
		{"empty verb", GitFault{}, "is not a git subcommand"},
		{"option as verb", GitFault{Verb: "-C"}, "is not a git subcommand"},
		{"negative skip", GitFault{Verb: "status", Skip: -1}, "skip -1 is negative"},
		{"code too high", GitFault{Verb: "status", Code: 256}, "outside 0..255"},
		{"negative code", GitFault{Verb: "status", Code: -1}, "outside 0..255"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.f.validate()
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("validate = %v, want nil", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Errorf("validate = %v, want %q", err, tc.wantErr)
			}
		})
	}
}

func TestGitFaultStderr(t *testing.T) {
	cases := []struct {
		name string
		f    GitFault
		want string
	}{
		{"declared stderr wins", GitFault{Verb: "status", Code: 128, Stderr: "fatal: bad object"}, "fatal: bad object"},
		{"a failure without stderr names the verb", GitFault{Verb: "status", Code: 1}, "fatal: gittest: injected fault on git status\n"},
		{"a faked success stays quiet", GitFault{Verb: "status"}, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.f.stderr(); got != tc.want {
				t.Errorf("stderr = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestShellQuote_SurvivesASingleQuote(t *testing.T) {
	r := committedFixture(t)
	r.Git("branch", "it's")

	InstallFault(t, GitFault{Verb: "show-ref", Args: []string{"refs/heads/it's"}, Code: 4, Stdout: "a 'quoted' $HOME"})

	if out, code := gitExit(t, r.Dir, "show-ref", "--verify", "refs/heads/it's"); code != 4 || out != "a 'quoted' $HOME" {
		t.Errorf("show-ref = %q exit %d, want the literal stdout and exit 4", out, code)
	}
}
