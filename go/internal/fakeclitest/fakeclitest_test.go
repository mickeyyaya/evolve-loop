package fakeclitest

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func run(t *testing.T, path, stdin string, args ...string) (string, int) {
	t.Helper()
	cmd := exec.Command(path, args...)
	cmd.Stdin = strings.NewReader(stdin)
	out, err := cmd.CombinedOutput()
	var exit *exec.ExitError
	switch {
	case err == nil:
		return string(out), 0
	case errors.As(err, &exit):
		return string(out), exit.ExitCode()
	default:
		t.Fatalf("run %s: %v\n%s", path, err, out)
		return "", 0
	}
}

func TestInstall_TheFakeIsTheRunningTestBinaryNotANewExecutable(t *testing.T) {
	fake := filepath.Join(t.TempDir(), "tool")
	Install(t, fake, "#!/bin/sh\nexit 0\n")

	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	selfInfo, err := os.Stat(self)
	if err != nil {
		t.Fatal(err)
	}
	fakeInfo, err := os.Stat(fake)
	if err != nil {
		t.Fatal(err)
	}
	if !os.SameFile(selfInfo, fakeInfo) {
		t.Fatalf("%s is a new file; an executable written for the test is scanned on its first run, which can take longer than the test's deadline", fake)
	}
}

func TestInstall_RunsTheScriptUnderItsShebangWithTheArgumentsStdinAndExitCode(t *testing.T) {
	for _, tc := range []struct {
		name, body string
	}{
		{"sh shebang", "#!/bin/sh\nread -r line\nprintf '%s|%s|%s' \"$1\" \"$2\" \"$line\"\nexit 3\n"},
		{"env bash shebang", "#!/usr/bin/env bash\nread -r line\nargs=(\"$@\")\nprintf '%s|%s|%s' \"${args[0]}\" \"${args[1]}\" \"$line\"\nexit 3\n"},
		{"no shebang runs under sh", "read -r line\nprintf '%s|%s|%s' \"$1\" \"$2\" \"$line\"\nexit 3\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fake := filepath.Join(t.TempDir(), "tool")
			Install(t, fake, tc.body)

			out, code := run(t, fake, "from-stdin\n", "first", "second arg")

			if out != "first|second arg|from-stdin" || code != 3 {
				t.Fatalf("fake ran as %q, exit %d; want first|second arg|from-stdin, exit 3", out, code)
			}
		})
	}
}

func TestInstall_ReinstallingReplacesTheScript(t *testing.T) {
	fake := filepath.Join(t.TempDir(), "tool")
	Install(t, fake, "#!/bin/sh\necho first\n")
	Install(t, fake, "#!/bin/sh\necho second\n")

	if out, code := run(t, fake, ""); out != "second\n" || code != 0 {
		t.Fatalf("reinstalled fake ran as %q, exit %d; want second", out, code)
	}
}

func TestInstall_AWriteThroughTheFakeIsRefused(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root writes through any mode")
	}
	fake := filepath.Join(t.TempDir(), "tool")
	Install(t, fake, "#!/bin/sh\nexit 0\n")

	if err := os.WriteFile(fake, []byte("overwritten"), 0o755); err == nil {
		t.Fatal("a write through the fake reached the running test binary; it must be refused")
	}
}

func TestInstall_FindsAFakeOnPathByName(t *testing.T) {
	dir := t.TempDir()
	Install(t, filepath.Join(dir, "fake-tool-on-path"), "#!/bin/sh\necho on-path \"$@\"\n")
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))

	out, err := exec.Command("fake-tool-on-path", "x").CombinedOutput()

	if err != nil || string(out) != "on-path x\n" {
		t.Fatalf("PATH lookup ran %q, %v; want on-path x", out, err)
	}
}

func TestInterpreterArgv(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		want       []string
	}{
		{"sh", "#!/bin/sh\necho\n", []string{"/bin/sh", "s", "a"}},
		{"env with interpreter", "#!/usr/bin/env bash\n", []string{"/usr/bin/env", "bash", "s", "a"}},
		{"no shebang", "echo\n", []string{"/bin/sh", "s", "a"}},
		{"empty shebang", "#!\necho\n", []string{"/bin/sh", "s", "a"}},
		{"empty body", "", []string{"/bin/sh", "s", "a"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := interpreterArgv([]byte(tc.body), "s", []string{"a"}); !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("interpreterArgv = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestCopyExecutable_CopiesTheBytesAndRefusesToOverwrite(t *testing.T) {
	dir := t.TempDir()
	from := filepath.Join(dir, "from")
	if err := os.WriteFile(from, []byte("binary bytes"), 0o600); err != nil {
		t.Fatal(err)
	}
	to := filepath.Join(dir, "to")

	if err := copyExecutable(from, to); err != nil {
		t.Fatalf("copyExecutable: %v", err)
	}
	if got, _ := os.ReadFile(to); string(got) != "binary bytes" {
		t.Fatalf("copied %q, want the source bytes", got)
	}
	if err := copyExecutable(from, to); err == nil {
		t.Fatal("copyExecutable overwrote an existing file")
	}
	if err := copyExecutable(filepath.Join(dir, "missing"), filepath.Join(dir, "other")); err == nil {
		t.Fatal("copyExecutable of a missing source succeeded")
	}
}

func TestRunIfFake_ExecsTheInterpreterOnlyWhenAScriptSitsBesideTheExecutable(t *testing.T) {
	dir := t.TempDir()
	withScript := filepath.Join(dir, "tool")
	if err := os.WriteFile(withScript+scriptSuffix, []byte("#!/usr/bin/env bash\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	unreadableScript := filepath.Join(dir, "unreadable")
	if err := os.Mkdir(unreadableScript+scriptSuffix, 0o700); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name       string
		executable func() (string, error)
		execErr    error
		wantArgv   []string
		wantExit   int
	}{
		{"a test binary run as itself stays a test binary", func() (string, error) { return filepath.Join(dir, "plain"), nil }, nil, nil, -1},
		{"an unknown executable path stays a test binary", func() (string, error) { return "", errors.New("unknown") }, nil, nil, -1},
		{"a fake runs its script under the shebang", func() (string, error) { return withScript, nil }, nil, []string{"/usr/bin/env", "bash", withScript + scriptSuffix}, 127},
		{"a failed exec exits loudly", func() (string, error) { return withScript, nil }, errors.New("exec format error"), []string{"/usr/bin/env", "bash", withScript + scriptSuffix}, 127},
		{"a script that exists but cannot be read exits loudly instead of running the test binary as the fake", func() (string, error) { return unreadableScript, nil }, nil, nil, 127},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var gotArgv []string
			gotExit := -1
			runIfFake(tc.executable, func(path string, argv, _ []string) error {
				gotArgv = argv
				if path != argv[0] {
					t.Errorf("exec path %q differs from argv[0] %q", path, argv[0])
				}
				return tc.execErr
			}, func(code int) { gotExit = code })

			if tc.wantArgv == nil && gotArgv != nil {
				t.Fatalf("exec'd %v for a binary with no script beside it", gotArgv)
			}
			if tc.wantArgv != nil && (len(gotArgv) < len(tc.wantArgv) || !reflect.DeepEqual(gotArgv[:len(tc.wantArgv)], tc.wantArgv)) {
				t.Fatalf("argv = %v, want prefix %v", gotArgv, tc.wantArgv)
			}
			if gotExit != tc.wantExit {
				t.Fatalf("exit = %d, want %d", gotExit, tc.wantExit)
			}
		})
	}
}

func TestInstall_FallsBackToACopyWhenTheBinaryCannotBeLinked(t *testing.T) {
	dir := t.TempDir()
	self := filepath.Join(dir, "self")
	if err := os.WriteFile(self, []byte("test binary bytes"), 0o755); err != nil {
		t.Fatal(err)
	}
	fake := filepath.Join(dir, "tool")

	linkErr, err := install(self, fake, "#!/bin/sh\n", func(string, string) error { return errors.New("cross-device link") })

	if err != nil {
		t.Fatalf("install: %v", err)
	}
	if linkErr == nil {
		t.Fatal("install hid that it fell back to a copy; the copy is a new file and its first run may wait on a scan")
	}
	if got, _ := os.ReadFile(fake); string(got) != "test binary bytes" {
		t.Fatalf("fallback copy holds %q, want the test binary's bytes", got)
	}
	if info, _ := os.Stat(fake); info.Mode().Perm() != readOnlyExecutable {
		t.Fatalf("fallback copy mode = %v, want %v", info.Mode().Perm(), os.FileMode(readOnlyExecutable))
	}
}

func TestInstall_ReportsWhatItCouldNotDo(t *testing.T) {
	dir := t.TempDir()
	self := filepath.Join(dir, "self")
	if err := os.WriteFile(self, []byte("bytes"), 0o755); err != nil {
		t.Fatal(err)
	}
	occupied := filepath.Join(dir, "occupied")
	if err := os.MkdirAll(filepath.Join(occupied, "child"), 0o755); err != nil {
		t.Fatal(err)
	}
	noLink := func(string, string) error { return errors.New("no link") }
	for _, tc := range []struct {
		name, self, path, want string
	}{
		{"script directory missing", self, filepath.Join(dir, "missing", "tool"), "write the script"},
		{"path is a non-empty directory", self, occupied, "replace"},
		{"binary unreadable for the copy", filepath.Join(dir, "gone"), filepath.Join(dir, "tool"), "install"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := install(tc.self, tc.path, "#!/bin/sh\n", noLink)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("install error = %v, want one naming %q", err, tc.want)
			}
		})
	}
}

func TestInstall_ASymlinkToAFakeRunsTheScript(t *testing.T) {
	dir := t.TempDir()
	fake := filepath.Join(dir, "tool")
	Install(t, fake, "#!/bin/sh\necho script-ran \"$1\"\n")
	alias := filepath.Join(t.TempDir(), "alias")
	if err := os.Symlink(fake, alias); err != nil {
		t.Fatal(err)
	}

	out, code := run(t, alias, "", "-test.run=^$")

	if out != "script-ran -test.run=^$\n" || code != 0 {
		t.Fatalf("symlinked fake ran as %q, exit %d; it ran the test binary instead of the script", out, code)
	}
}
