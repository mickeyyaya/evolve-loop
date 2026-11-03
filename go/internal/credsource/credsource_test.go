package credsource

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
)

type call struct {
	name string
	args []string
}

type fakeRunner struct {
	calls []call
	out   map[string]string
	err   map[string]error
}

func (f *fakeRunner) run(_ context.Context, name string, args ...string) (string, error) {
	f.calls = append(f.calls, call{name: name, args: args})
	return f.out[name], f.err[name]
}

func writeSecret(t *testing.T, home, host, secret string, mode os.FileMode) string {
	t.Helper()
	dir := filepath.Join(home, ".config", Service, "credentials")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, host)
	if err := os.WriteFile(path, []byte(secret), mode); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, mode); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestFor_ChoosesTheRealizationByGOOSData(t *testing.T) {
	for _, tc := range []struct {
		goos, want string
	}{
		{"darwin", "keychain"},
		{"linux", "libsecret+file"},
		{"windows", "credential-manager"},
	} {
		src, err := For(tc.goos, Deps{})
		if err != nil {
			t.Fatalf("%s: %v", tc.goos, err)
		}
		if got := src.Name(); got != tc.want {
			t.Errorf("For(%q).Name() = %q, want %q", tc.goos, got, tc.want)
		}
	}
	if _, err := For("plan9", Deps{}); err == nil || !strings.Contains(err.Error(), "plan9") {
		t.Fatalf("For(plan9) = %v, want an error that names the OS", err)
	}
}

func TestKeychain_LookupTable(t *testing.T) {
	want := call{name: "security", args: []string{"find-generic-password", "-s", "evolve-loop", "-a", "github.com", "-w"}}
	for _, tc := range []struct {
		name    string
		out     string
		err     error
		secret  string
		wantErr string
	}{
		{name: "the keychain answers", out: "tok-1\n", secret: "tok-1"},
		{name: "the item is empty", out: "\n", wantErr: ErrNoCredential.Error()},
		{name: "security fails", err: errors.New("exit status 44"), wantErr: "keychain: exit status 44"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := &fakeRunner{out: map[string]string{"security": tc.out}, err: map[string]error{"security": tc.err}}
			var run Runner = f.run
			src, _ := For("darwin", Deps{Run: run})
			got, err := src.Lookup(context.Background(), DefaultHost)
			if len(f.calls) != 1 || f.calls[0].name != want.name || !slices.Equal(f.calls[0].args, want.args) {
				t.Fatalf("calls = %+v, want one %+v", f.calls, want)
			}
			checkAnswer(t, got, err, "keychain", tc.secret, tc.wantErr)
		})
	}
}

func checkAnswer(t *testing.T, got Answer, err error, source, secret, wantErr string) {
	t.Helper()
	if wantErr != "" {
		if err == nil || !strings.Contains(err.Error(), wantErr) {
			t.Fatalf("err = %v, want it to contain %q", err, wantErr)
		}
		if got != (Answer{}) {
			t.Fatalf("a failed lookup returned %+v", got)
		}
		return
	}
	if err != nil {
		t.Fatal(err)
	}
	if got != (Answer{Source: source, Secret: secret}) {
		t.Fatalf("answer = %+v, want source %q", got, source)
	}
}

func TestLibsecret_LookupTable(t *testing.T) {
	want := call{name: "secret-tool", args: []string{"lookup", "service", "evolve-loop", "host", "github.com"}}
	refused := errors.New("exit status 1")
	for _, tc := range []struct {
		name     string
		out      string
		err      error
		file     string
		mode     os.FileMode
		link     bool
		dir      bool
		homeFile bool
		homeErr  error
		source   string
		secret   string
		wantErr  string
	}{
		{name: "libsecret answers", out: "tok-1\n", source: "libsecret", secret: "tok-1"},
		{name: "libsecret has no item and the file answers", out: "", file: "tok-2\n", mode: 0o600, source: "file", secret: "tok-2"},
		{name: "secret-tool fails and the file answers", err: refused, file: "tok-3", mode: 0o600, source: "file", secret: "tok-3"},
		{name: "the file is readable by the group", err: refused, file: "tok-4", mode: 0o640, wantErr: "mode 0640, want 0600"},
		{name: "the file is a symlink", err: refused, file: "tok-5", mode: 0o600, link: true, wantErr: "not a regular file"},
		{name: "the file is empty", err: refused, file: "\n", mode: 0o600, wantErr: ErrNoCredential.Error()},
		{name: "no file", err: refused, wantErr: ErrNoCredential.Error()},
		{name: "libsecret has no item and no file", out: "", wantErr: ErrNoCredential.Error()},
		{name: "the path is a directory", err: refused, dir: true, wantErr: "is not a regular file"},
		{name: "the home is a file", err: refused, homeFile: true, wantErr: "not a directory"},
		{name: "no home", err: refused, homeErr: errors.New("no passwd entry"), wantErr: "no passwd entry"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			home := t.TempDir()
			if tc.homeFile {
				home = filepath.Join(home, "file")
				if err := os.WriteFile(home, nil, 0o600); err != nil {
					t.Fatal(err)
				}
			}
			if tc.dir {
				if err := os.MkdirAll(filepath.Join(home, ".config", Service, "credentials", "github.com"), 0o700); err != nil {
					t.Fatal(err)
				}
			}
			if tc.file != "" {
				path := writeSecret(t, home, "real-host", tc.file, tc.mode)
				target := filepath.Join(filepath.Dir(path), "github.com")
				if tc.link {
					if err := os.Symlink(path, target); err != nil {
						t.Fatal(err)
					}
				} else if err := os.Rename(path, target); err != nil {
					t.Fatal(err)
				}
			}
			f := &fakeRunner{out: map[string]string{"secret-tool": tc.out}, err: map[string]error{"secret-tool": tc.err}}
			src, _ := For("linux", Deps{Run: f.run, Home: func() (string, error) { return home, tc.homeErr }})
			got, err := src.Lookup(context.Background(), "github.com")
			if len(f.calls) != 1 || f.calls[0].name != want.name || !slices.Equal(f.calls[0].args, want.args) {
				t.Fatalf("calls = %+v, want one %+v", f.calls, want)
			}
			checkAnswer(t, got, err, tc.source, tc.secret, tc.wantErr)
			if tc.wantErr != "" && tc.err != nil && !strings.Contains(err.Error(), "libsecret: exit status 1") {
				t.Fatalf("err = %v, want the libsecret cause kept", err)
			}
		})
	}
}

func TestCredentialManager_LookupTable(t *testing.T) {
	for _, tc := range []struct {
		name    string
		out     string
		err     error
		secret  string
		wantErr string
	}{
		{name: "the credential manager answers", out: "tok-1\r\n", secret: "tok-1"},
		{name: "the target is empty", out: "", wantErr: ErrNoCredential.Error()},
		{name: "powershell fails", err: errors.New("exit status 2"), wantErr: "credential-manager: exit status 2"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := &fakeRunner{out: map[string]string{"powershell.exe": tc.out}, err: map[string]error{"powershell.exe": tc.err}}
			src, _ := For("windows", Deps{Run: f.run})
			got, err := src.Lookup(context.Background(), "github.com")
			if len(f.calls) != 1 || f.calls[0].name != "powershell.exe" {
				t.Fatalf("calls = %+v, want one powershell.exe", f.calls)
			}
			args := f.calls[0].args
			if !slices.Equal(args[:3], []string{"-NoProfile", "-NonInteractive", "-Command"}) || len(args) != 4 {
				t.Fatalf("args = %q", args)
			}
			for _, part := range []string{"CredReadW", "'evolve-loop:github.com'", "CredFree", "[IntPtr]::Size", "if ($blobSize -eq 0) { exit 3 }"} {
				if !strings.Contains(args[3], part) {
					t.Errorf("the script does not contain %q", part)
				}
			}
			checkAnswer(t, got, err, "credential-manager", tc.secret, tc.wantErr)
		})
	}
}

func TestGuarded_RefusesAHostThatIsNotAHostName(t *testing.T) {
	for _, goos := range []string{"darwin", "linux", "windows"} {
		for _, host := range []string{"", "github.com'; Remove-Item x; '", "-w", "a/b", "a b"} {
			f := &fakeRunner{}
			src, _ := For(goos, Deps{Run: f.run})
			if _, err := src.Lookup(context.Background(), host); err == nil || !strings.Contains(err.Error(), "not a host name") {
				t.Errorf("%s: Lookup(%q) = %v, want a host refusal", goos, host, err)
			}
			if len(f.calls) != 0 {
				t.Errorf("%s: Lookup(%q) ran %+v", goos, host, f.calls)
			}
		}
	}
}

func TestDefault_UsesTheRealizationOfThisOS(t *testing.T) {
	src, err := Default()
	if err != nil {
		t.Fatal(err)
	}
	want, _ := For(runtime.GOOS, Deps{})
	if src.Name() == "" || src.Name() != want.Name() {
		t.Fatalf("Default().Name() = %q, want %q", src.Name(), want.Name())
	}
}

func TestExecRunner_ReturnsStdoutAndTheExitError(t *testing.T) {
	out, err := execRunner(context.Background(), "sh", "-c", "printf tok; printf noise >&2")
	if err != nil || out != "tok" {
		t.Fatalf("execRunner = %q, %v; want tok", out, err)
	}
	if _, err := execRunner(context.Background(), "sh", "-c", "exit 44"); err == nil || !strings.Contains(err.Error(), "exit status 44") {
		t.Fatalf("execRunner = %v, want exit status 44", err)
	}
}

func TestPasswdHome_IsTheHomeOfThisUIDAndIgnoresHOME(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	home, err := passwdHome()
	if runtime.GOOS == "windows" {
		t.Skip("windows has no passwd entry")
	}
	if err != nil || home == "" || home == os.Getenv("HOME") {
		t.Fatalf("passwdHome = %q, %v", home, err)
	}
}

func TestPasswdHome_ReportsALookupFailure(t *testing.T) {
	old := lookupHome
	t.Cleanup(func() { lookupHome = old })
	lookupHome = func(string) (string, error) { return "", errors.New("unknown uid") }
	if _, err := passwdHome(); err == nil || !strings.Contains(err.Error(), "unknown uid") {
		t.Fatalf("passwdHome = %v, want the lookup error", err)
	}
	lookupHome = func(string) (string, error) { return "", nil }
	if _, err := passwdHome(); err == nil || !strings.Contains(err.Error(), "no home directory") {
		t.Fatalf("passwdHome = %v, want a no-home error", err)
	}
}

func TestLookupHome_ReportsAnUnknownUID(t *testing.T) {
	if _, err := lookupHome("not-a-uid"); err == nil {
		t.Fatal("lookupHome(not-a-uid) returned no error")
	}
}

func TestChildVariables_ListsEachTokenVariableThatGhReads(t *testing.T) {
	want := []string{"GH_ENTERPRISE_TOKEN", "GH_TOKEN", "GITHUB_ENTERPRISE_TOKEN", "GITHUB_TOKEN"}
	got := slices.Sorted(slices.Values(ChildVariables))
	if !slices.Equal(got, want) {
		t.Fatalf("ChildVariables = %q, want %q", got, want)
	}
}

func TestLibsecretFile_ChecksTheTypeAndModeBeforeItReads(t *testing.T) {
	home := t.TempDir()
	path := writeSecret(t, home, "github.com", "tok-1", 0o600)
	fifo := filepath.Join(filepath.Dir(path), "fifo.example")
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(fifo, path); err != nil {
		t.Fatal(err)
	}
	var reads []string
	l := libsecretOrFile{
		home: func() (string, error) { return home, nil },
		read: func(p string) ([]byte, error) { reads = append(reads, p); return nil, errors.New("read fault") },
	}
	if _, err := l.file("github.com"); err == nil || !strings.Contains(err.Error(), "not a regular file") {
		t.Fatalf("file() = %v, want a refusal of the link", err)
	}
	if len(reads) != 0 {
		t.Fatalf("file() read %v before it checked the type", reads)
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	writeSecret(t, home, "github.com", "tok-1", 0o600)
	if _, err := l.file("github.com"); err == nil || !strings.Contains(err.Error(), "read fault") {
		t.Fatalf("file() = %v, want the read fault", err)
	}
}
