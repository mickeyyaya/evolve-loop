package main

import (
	"bytes"
	"context"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/agentclass"
	"github.com/mickeyyaya/evolve-loop/go/internal/bridge"
	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

const (
	authFixtureOutput = `keychain: "/Users/x/Library/Keychains/login.keychain-db"
version: 512
class: "genp"
attributes:
    0x00000007 <blob>="Claude Code-credentials"
    "acct"<blob>="planted-account-name"
    "cdat"<timedate>=0x32303236313130313030303030305A00  "20261101000000Z\000"
    "mdat"<timedate>=0x32303236313130353139313633305A00  "20261105191630Z\000"
    "svce"<blob>="Claude Code-credentials"
password: "planted-secret-token"
`
	authFixtureTime = "2026-11-05T19:16:30Z"
)

var authFileTime = time.Date(2026, 11, 4, 8, 30, 0, 0, time.UTC)

func authKeychain(out string, rc int, err error, gotArgs *[]string, gotStdin *io.Reader) keychainTimeFunc {
	run := func(ctx context.Context, name, dir string, args, env []string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
		if gotArgs != nil {
			*gotArgs = append([]string{name}, args...)
		}
		if gotStdin != nil {
			*gotStdin = stdin
		}
		_, _ = io.WriteString(stdout, out)
		return rc, err
	}
	return newKeychainTime(run, time.Second)
}

func authEnvWith(t *testing.T, home string, kc keychainTimeFunc, goos string) authEnv {
	t.Helper()
	return authEnv{Home: home, Stat: os.Stat, Keychain: kc, GOOS: goos}
}

func runAuthText(t *testing.T, env authEnv, args ...string) (int, string, string) {
	t.Helper()
	var out, errb bytes.Buffer
	rc := runAuthEnv(env, args, &out, &errb)
	return rc, out.String(), errb.String()
}

func writeCredentials(t *testing.T, mode os.FileMode) string {
	t.Helper()
	home := t.TempDir()
	dir := filepath.Join(home, ".claude")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(dir, ".credentials.json")
	if err := os.WriteFile(p, []byte(`{"secret":"planted-file-secret"}`), mode); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(p, mode); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(p, authFileTime, authFileTime); err != nil {
		t.Fatal(err)
	}
	return home
}

func TestAuthStoreTimes_ARealFileAndAFoundKeychainItemPrintBothTimes(t *testing.T) {
	t.Parallel()
	home := writeCredentials(t, 0o600)
	env := authEnvWith(t, home, authKeychain(authFixtureOutput, 0, nil, nil, nil), "darwin")
	rc, out, errText := runAuthText(t, env, "store-times")
	want := "credentials file: 2026-11-04T08:30:00Z\nkeychain item: " + authFixtureTime + "\n"
	if rc != 0 || out != want || errText != "" {
		t.Fatalf("rc=%d out=%q err=%q want out=%q", rc, out, errText, want)
	}
}

func TestAuthStoreTimes_AFileWithMode000StillPrintsItsTime(t *testing.T) {
	t.Parallel()
	home := writeCredentials(t, 0o000)
	env := authEnvWith(t, home, authKeychain("", 44, nil, nil, nil), "darwin")
	rc, out, _ := runAuthText(t, env, "store-times")
	want := "credentials file: 2026-11-04T08:30:00Z\nkeychain item: absent\n"
	if rc != 0 || out != want {
		t.Fatalf("rc=%d out=%q want %q", rc, out, want)
	}
}

func TestAuthStoreTimes_AMissingFileIsAbsent(t *testing.T) {
	t.Parallel()
	env := authEnvWith(t, t.TempDir(), authKeychain("", 44, nil, nil, nil), "darwin")
	rc, out, _ := runAuthText(t, env, "store-times")
	want := "credentials file: absent\nkeychain item: absent\n"
	if rc != 0 || out != want {
		t.Fatalf("rc=%d out=%q want %q", rc, out, want)
	}
}

func TestAuthStoreTimes_AStatFailureIsUnreadable(t *testing.T) {
	t.Parallel()
	env := authEnvWith(t, t.TempDir(), authKeychain("", 44, nil, nil, nil), "darwin")
	env.Stat = func(string) (fs.FileInfo, error) { return nil, fs.ErrPermission }
	rc, out, _ := runAuthText(t, env, "store-times")
	want := "credentials file: unreadable\nkeychain item: absent\n"
	if rc != 0 || out != want {
		t.Fatalf("rc=%d out=%q want %q", rc, out, want)
	}
}

func TestAuthStoreTimes_ATimeoutPrintsUnreadableNeverAnEmptyTime(t *testing.T) {
	t.Parallel()
	run := func(ctx context.Context, _, _ string, _, _ []string, _ io.Reader, _, _ io.Writer) (int, error) {
		<-ctx.Done()
		return -1, ctx.Err()
	}
	env := authEnvWith(t, t.TempDir(), newKeychainTime(run, 20*time.Millisecond), "darwin")
	rc, out, _ := runAuthText(t, env, "store-times")
	want := "credentials file: absent\nkeychain item: unreadable\n"
	if rc != 0 || out != want {
		t.Fatalf("rc=%d out=%q want %q", rc, out, want)
	}
}

func TestAuthStoreTimes_AFailureOrAnOutputWithoutMdatIsUnreadable(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		out  string
		rc   int
		err  error
	}{
		{"start failure", "", -1, errors.New("no security binary")},
		{"other exit code", "", 1, nil},
		{"no mdat", "class: \"genp\"\n", 0, nil},
		{"bad mdat", "\"mdat\"<timedate>=0x00  \"2026\"\n", 0, nil},
		{"impossible mdat", "\"mdat\"<timedate>=0x00  \"20261399999999Z\\000\"\n", 0, nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			env := authEnvWith(t, t.TempDir(), authKeychain(c.out, c.rc, c.err, nil, nil), "darwin")
			_, out, _ := runAuthText(t, env, "store-times")
			if want := "credentials file: absent\nkeychain item: unreadable\n"; out != want {
				t.Fatalf("out=%q want %q", out, want)
			}
		})
	}
}

func TestAuthStoreTimes_AnotherOSPrintsTheFileTimeAndNoKeychain(t *testing.T) {
	t.Parallel()
	home := writeCredentials(t, 0o600)
	called := false
	kc := func(context.Context, string) (time.Time, storeState, error) {
		called = true
		return time.Time{}, storeUnreadable, nil
	}
	env := authEnvWith(t, home, kc, "linux")
	rc, out, _ := runAuthText(t, env, "store-times")
	want := "credentials file: 2026-11-04T08:30:00Z\nkeychain item: no keychain\n"
	if rc != 0 || out != want || called {
		t.Fatalf("rc=%d out=%q called=%v want %q", rc, out, called, want)
	}
}

func TestAuthStoreTimes_TheKeychainCallIsExactAndNeverAsksForASecret(t *testing.T) {
	t.Parallel()
	var args []string
	var stdin io.Reader = strings.NewReader("sentinel")
	env := authEnvWith(t, t.TempDir(), authKeychain(authFixtureOutput, 0, nil, &args, &stdin), "darwin")
	runAuthText(t, env, "store-times")
	want := []string{"security", "find-generic-password", "-s", "Claude Code-credentials"}
	if !slices.Equal(args, want) {
		t.Fatalf("args=%q want %q", args, want)
	}
	if stdin != nil {
		t.Fatalf("stdin=%v want nil", stdin)
	}
	for _, a := range args {
		if a == "-w" || a == "-g" {
			t.Fatalf("argument %q asks for the secret", a)
		}
	}
	if bridge.ClaudeKeychainService != "Claude Code-credentials" {
		t.Fatalf("service=%q", bridge.ClaudeKeychainService)
	}
}

func TestAuthStoreTimes_NoAccountTokenOrPasswordReachesTheOutput(t *testing.T) {
	t.Parallel()
	home := writeCredentials(t, 0o600)
	env := authEnvWith(t, home, authKeychain(authFixtureOutput, 0, nil, nil, nil), "darwin")
	for _, args := range [][]string{{"store-times"}, {"store-times", "--json"}} {
		rc, out, errText := runAuthText(t, env, args...)
		all := out + errText
		for _, planted := range []string{"planted-account-name", "planted-secret-token", "planted-file-secret", "password", "acct", "svce", "Claude Code-credentials"} {
			if strings.Contains(all, planted) {
				t.Fatalf("%v: output holds %q: %q", args, planted, all)
			}
		}
		if rc != 0 {
			t.Fatalf("%v: rc=%d", args, rc)
		}
	}
}

func TestAuthStoreTimes_JSONShapeIsPinnedWhole(t *testing.T) {
	t.Parallel()
	home := writeCredentials(t, 0o600)
	env := authEnvWith(t, home, authKeychain(authFixtureOutput, 0, nil, nil, nil), "darwin")
	rc, out, _ := runAuthText(t, env, "store-times", "--json")
	want := `{"schema":"evolve.auth.store-times/1","credentials_file":{"state":"present","time":"2026-11-04T08:30:00Z"},"keychain":{"state":"present","time":"2026-11-05T19:16:30Z"}}` + "\n"
	if rc != 0 || out != want {
		t.Fatalf("rc=%d out=%q want %q", rc, out, want)
	}
	env = authEnvWith(t, t.TempDir(), nil, "linux")
	_, out, _ = runAuthText(t, env, "store-times", "--json")
	want = `{"schema":"evolve.auth.store-times/1","credentials_file":{"state":"absent"},"keychain":{"state":"no keychain"}}` + "\n"
	if out != want {
		t.Fatalf("out=%q want %q", out, want)
	}
}

func TestAuthStoreTimes_HelpPrintsTheUsageAndExitsZero(t *testing.T) {
	t.Parallel()
	rc, out, errText := runAuthText(t, authEnv{}, "store-times", "--help")
	if rc != 0 || out != authUsage+"\n" || errText != "" {
		t.Fatalf("rc=%d out=%q err=%q", rc, out, errText)
	}
	rc, out, _ = runAuthText(t, authEnv{}, "--help")
	if rc != 0 || out != authUsage+"\n" {
		t.Fatalf("rc=%d out=%q", rc, out)
	}
}

func TestAuthStoreTimes_AUsageErrorExitsTen(t *testing.T) {
	t.Parallel()
	cases := [][]string{{}, {"login"}, {"store-times", "--bogus"}, {"store-times", "extra"}}
	for _, args := range cases {
		rc, out, errText := runAuthText(t, authEnv{}, args...)
		if rc != exitUsage || out != "" || !strings.HasPrefix(errText, "evolve auth: ") || !strings.HasSuffix(errText, authUsage+"\n") {
			t.Fatalf("%v: rc=%d out=%q err=%q", args, rc, out, errText)
		}
	}
}

func TestRunAuthAt_AHomeFailureExitsTwoAndAHelpCallNeedsNoLogin(t *testing.T) {
	t.Parallel()
	var out, errb bytes.Buffer
	rc := runAuthAt(func() (string, error) { return "", errors.New("no home") }, []string{"store-times"}, &out, &errb)
	if rc != exitIO || errb.String() != "evolve auth: read the home directory: no home\n" {
		t.Fatalf("rc=%d err=%q", rc, errb.String())
	}
	out.Reset()
	errb.Reset()
	rc = runAuthAt(func() (string, error) { return t.TempDir(), nil }, []string{"--help"}, &out, &errb)
	if rc != 0 || out.String() != authUsage+"\n" {
		t.Fatalf("rc=%d out=%q", rc, out.String())
	}
}

func TestRunAuth_UsesTheRealHome(t *testing.T) {
	t.Setenv("HOME", "")
	var out, errb bytes.Buffer
	rc := runAuth([]string{"--help"}, nil, &out, &errb)
	if rc != exitIO || !strings.HasPrefix(errb.String(), "evolve auth: read the home directory: ") {
		t.Fatalf("rc=%d err=%q", rc, errb.String())
	}
}

func TestProductionAuthEnv_HoldsTheRealSeams(t *testing.T) {
	t.Parallel()
	env := productionAuthEnv("/h")
	if env.Home != "/h" || env.Stat == nil || env.Keychain == nil || env.GOOS == "" {
		t.Fatalf("env=%+v", env)
	}
}

func TestAuthStoreTimes_TheVerbIsClassAAndRegistered(t *testing.T) {
	t.Parallel()
	m := agentclass.Of([]string{"auth", "store-times", "--json"})
	if m.Name != "auth store-times" || m.Class != agentclass.Allowed {
		t.Fatalf("match=%+v", m)
	}
	found := false
	for _, v := range commands {
		if v.Name == "auth" {
			found = true
		}
	}
	if !found {
		t.Fatal("auth is not registered")
	}
}

func TestParseMdat_ReadsOnlyTheMdatLine(t *testing.T) {
	t.Parallel()
	got, ok := parseKeychainMdat(authFixtureOutput)
	if !ok || got.Format(time.RFC3339) != authFixtureTime {
		t.Fatalf("got=%v ok=%v", got, ok)
	}
	if _, ok := parseKeychainMdat("\"cdat\"<timedate>=0x00  \"20261101000000Z\\000\"\n"); ok {
		t.Fatal("cdat was read as mdat")
	}
}

func TestAuthStoreTimes_AnUnreadableKeychainNamesItsCauseOnStderrAndInJSON(t *testing.T) {
	t.Parallel()
	env := authEnvWith(t, t.TempDir(), func(context.Context, string) (time.Time, storeState, error) {
		return time.Time{}, storeUnreadable, errors.New("security exited 36")
	}, "darwin")
	var out, errb bytes.Buffer
	if rc := runAuthEnv(env, []string{"store-times"}, &out, &errb); rc != 0 {
		t.Fatalf("rc=%d", rc)
	}
	if got, want := errb.String(), "evolve auth: keychain item: security exited 36\n"; got != want {
		t.Errorf("stderr = %q, want %q", got, want)
	}
	out.Reset()
	if rc := runAuthEnv(env, []string{"store-times", "--json"}, &out, &errb); rc != 0 {
		t.Fatalf("rc=%d", rc)
	}
	if !strings.Contains(out.String(), `"keychain":{"state":"unreadable","detail":"security exited 36"}`) {
		t.Errorf("json = %q", out.String())
	}
}

func TestProductionAuthEnv_ReadsTheRealSecurityBinaryOnPath(t *testing.T) {
	dir := t.TempDir()
	script := "#!/bin/sh\necho '    \"mdat\"<timedate>=0x32303236  \"20261105161319Z\\000\"'\n"
	if err := os.WriteFile(filepath.Join(dir, "security"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	env := productionAuthEnv(t.TempDir())
	if env.GOOS != runtime.GOOS {
		t.Fatalf("GOOS = %q, want %q", env.GOOS, runtime.GOOS)
	}
	env.GOOS = "darwin"
	var out, errb bytes.Buffer
	if rc := runAuthEnv(env, []string{"store-times"}, &out, &errb); rc != 0 {
		t.Fatalf("rc=%d err=%q", rc, errb.String())
	}
	if want := "credentials file: absent\nkeychain item: 2026-11-05T16:13:19Z\n"; out.String() != want {
		t.Errorf("out = %q, want %q", out.String(), want)
	}
}

func TestAuthStoreTimes_ARealTimeoutNamesTheTimeout(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "security"), []byte("#!/bin/sh\nexec sleep 20\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	env := authEnvWith(t, t.TempDir(), newKeychainTime(sysexec.DefaultRunner, 100*time.Millisecond), "darwin")
	var out, errb bytes.Buffer
	if rc := runAuthEnv(env, []string{"store-times"}, &out, &errb); rc != 0 {
		t.Fatalf("rc=%d", rc)
	}
	if !strings.Contains(out.String(), "keychain item: unreadable\n") {
		t.Errorf("out = %q", out.String())
	}
	if !strings.Contains(errb.String(), "keychain item: read the Keychain item time: context deadline exceeded") {
		t.Errorf("stderr = %q", errb.String())
	}
}

func TestAuthStoreTimes_AnUnreadableFileNamesItsCauseOnStderrAndInJSON(t *testing.T) {
	t.Parallel()
	env := authEnvWith(t, t.TempDir(), func(context.Context, string) (time.Time, storeState, error) {
		return time.Time{}, storeAbsent, nil
	}, "darwin")
	env.Stat = func(string) (fs.FileInfo, error) { return nil, errors.New("permission denied") }
	var out, errb bytes.Buffer
	if rc := runAuthEnv(env, []string{"store-times"}, &out, &errb); rc != 0 {
		t.Fatalf("rc=%d", rc)
	}
	if got, want := out.String(), "credentials file: unreadable\nkeychain item: absent\n"; got != want {
		t.Errorf("out = %q, want %q", got, want)
	}
	if got, want := errb.String(), "evolve auth: credentials file: permission denied\n"; got != want {
		t.Errorf("stderr = %q, want %q", got, want)
	}
	out.Reset()
	if rc := runAuthEnv(env, []string{"store-times", "--json"}, &out, &errb); rc != 0 {
		t.Fatalf("rc=%d", rc)
	}
	if !strings.Contains(out.String(), `"credentials_file":{"state":"unreadable","detail":"permission denied"}`) {
		t.Errorf("json = %q", out.String())
	}
}
