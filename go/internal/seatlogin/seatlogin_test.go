package seatlogin

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/paths"
)

const canary = "sk-ant-oat01-CANARY-0123456789"

func plant(t *testing.T, mode fs.FileMode, body string) string {
	t.Helper()
	home := t.TempDir()
	path := paths.SeatTokenPath(home)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), mode); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, mode); err != nil {
		t.Fatal(err)
	}
	return home
}

func TestResolve_AbsentFileIsTheStoredLogin(t *testing.T) {
	t.Parallel()
	got := Resolve(t.TempDir(), "", OS())
	if got != (Decision{}) {
		t.Fatalf("got %+v", got)
	}
}

func TestResolve_ValidTokenIsolatesTheHostAndTrimsTheNewline(t *testing.T) {
	t.Parallel()
	home := plant(t, 0o600, canary+"\n")
	got := Resolve(home, "", OS())
	want := Decision{HostIsolated: true, SeatToken: canary, Dir: paths.SeatRoot(home)}
	if got != want {
		t.Fatalf("got %+v want %+v", got, want)
	}
}

func TestResolve_EmptyFileRefusesAndStillIsolatesTheHost(t *testing.T) {
	t.Parallel()
	home := plant(t, 0o600, " \n")
	got := Resolve(home, "", OS())
	want := "bridge: the seat token at " + paths.SeatTokenPath(home) + " is empty, so no claude seat can start. Next: run claude setup-token, then evolve auth seat-token set, or run evolve auth seat-token clear to go back to the stored login."
	if got.Refusal != want || !got.HostIsolated || got.SeatToken != "" {
		t.Fatalf("got %+v", got)
	}
}

func TestResolve_WideModeRefusesWithoutReadingTheFile(t *testing.T) {
	t.Parallel()
	home := plant(t, 0o644, canary)
	reads := 0
	deps := OS()
	read := deps.ReadFile
	deps.ReadFile = func(p string) ([]byte, error) { reads++; return read(p) }
	got := Resolve(home, "", deps)
	want := "bridge: the seat token at " + paths.SeatTokenPath(home) + " has mode 0644, not 0600, so no claude seat can start. Next: run evolve auth seat-token set again."
	if got.Refusal != want || !got.HostIsolated || got.SeatToken != "" || reads != 0 {
		t.Fatalf("got %+v reads=%d", got, reads)
	}
}

func TestResolve_AnUnreadableFileRefusesAndHoldsNoToken(t *testing.T) {
	t.Parallel()
	home := plant(t, 0o600, canary)
	deps := OS()
	deps.ReadFile = func(string) ([]byte, error) { return []byte(canary), errors.New("boom") }
	got := Resolve(home, "", deps)
	want := "bridge: the seat token at " + paths.SeatTokenPath(home) + " cannot be read: boom. Next: run evolve auth seat-token set again, or run evolve auth seat-token clear to go back to the stored login."
	if got.Refusal != want || !got.HostIsolated || got.SeatToken != "" {
		t.Fatalf("got %+v", got)
	}
}

func TestResolve_AStatFailureOtherThanAbsentRefuses(t *testing.T) {
	t.Parallel()
	home := t.TempDir()
	deps := OS()
	deps.Stat = func(string) (fs.FileInfo, error) { return nil, errors.New("denied") }
	got := Resolve(home, "", deps)
	if !strings.Contains(got.Refusal, "cannot be read: denied") || !got.HostIsolated {
		t.Fatalf("got %+v", got)
	}
}

func TestResolve_ADirectoryInPlaceOfTheFileRefuses(t *testing.T) {
	t.Parallel()
	home := t.TempDir()
	if err := os.MkdirAll(paths.SeatTokenPath(home), 0o700); err != nil {
		t.Fatal(err)
	}
	got := Resolve(home, "", OS())
	if !strings.Contains(got.Refusal, "is not a regular file") || !got.HostIsolated {
		t.Fatalf("got %+v", got)
	}
}

func TestResolve_EmptyHomeRefuses(t *testing.T) {
	t.Parallel()
	got := Resolve("", "", OS())
	want := "bridge: the home directory is empty, so the seat token cannot be found and no claude seat can start."
	if got.Refusal != want || got.HostIsolated {
		t.Fatalf("got %+v", got)
	}
}

func TestResolve_NoRefusalHoldsTheToken(t *testing.T) {
	t.Parallel()
	home := plant(t, 0o644, canary)
	if strings.Contains(Resolve(home, "", OS()).Refusal, canary) {
		t.Fatal("token in the mode refusal")
	}
	home = plant(t, 0o600, canary+" tail")
	got := Resolve(home, "", OS())
	if got.Refusal == "" || strings.Contains(got.Refusal, canary) {
		t.Fatalf("got %+v", got)
	}
}

func TestResolve_Mode0400Refuses(t *testing.T) {
	t.Parallel()
	got := Resolve(plant(t, 0o400, canary), "", OS())
	if got.SeatToken != "" || !strings.Contains(got.Refusal, "has mode 0400, not 0600") {
		t.Fatalf("refusal %q, token set=%v", got.Refusal, got.SeatToken != "")
	}
}

func TestParseToken_TakesOnlyAPrefixedTokenWithoutInnerWhitespaceWithinTheCap(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name, in, want, reason string
	}{
		{"plain", canary, canary, ""},
		{"trailing newline", canary + "\r\n", canary, ""},
		{"no prefix", "abc", "", "it does not start with " + TokenPrefix},
		{"empty", "  \n", "", "it does not start with " + TokenPrefix},
		{"second line", canary + "\nsecond", "", "it holds whitespace or a control byte inside"},
		{"space inside", TokenPrefix + "a b", "", "it holds whitespace or a control byte inside"},
		{"control byte", TokenPrefix + "a\x00b", "", "it holds whitespace or a control byte inside"},
		{"over the cap", TokenPrefix + strings.Repeat("x", MaxTokenBytes), "", "it is longer than 4096 bytes"},
		{"at the cap", TokenPrefix + strings.Repeat("x", MaxTokenBytes-len(TokenPrefix)), "ok", ""},
	}
	for _, c := range cases {
		got, reason := ParseToken(c.in)
		if c.want == "ok" {
			if reason != "" || len(got) != MaxTokenBytes {
				t.Errorf("%s: reason=%q len=%d", c.name, reason, len(got))
			}
			continue
		}
		if got != c.want || reason != c.reason {
			t.Errorf("%s: got %q reason %q", c.name, got, reason)
		}
		if strings.Contains(reason, "CANARY") {
			t.Errorf("%s: the reason holds the value", c.name)
		}
	}
}

func TestResolve_ABodyThatIsNotATokenRefusesWithoutTheValue(t *testing.T) {
	t.Parallel()
	home := plant(t, 0o600, canary+"\nsecond-line-SECRET")
	got := Resolve(home, "", OS())
	want := "bridge: the seat token at " + paths.SeatTokenPath(home) + " is not a seat token: it holds whitespace or a control byte inside, so no claude seat can start. Next: run claude setup-token, then evolve auth seat-token set, or run evolve auth seat-token clear to go back to the stored login."
	if got.Refusal != want || !got.HostIsolated || got.SeatToken != "" {
		t.Fatalf("got %+v", got)
	}
}

func TestResolve_ASymlinkToTheTokenRefuses(t *testing.T) {
	t.Parallel()
	home := plant(t, 0o600, canary)
	real := paths.SeatTokenPath(home) + ".real"
	if err := os.Rename(paths.SeatTokenPath(home), real); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(real, paths.SeatTokenPath(home)); err != nil {
		t.Fatal(err)
	}
	got := Resolve(home, "", OS())
	if !strings.Contains(got.Refusal, "is not a regular file") || got.SeatToken != "" || !got.HostIsolated {
		t.Fatalf("got %+v", got)
	}
}

func TestResolve_AParameterFileWinsOverTheHolderAndLeavesItUntouched(t *testing.T) {
	t.Parallel()
	home := plant(t, 0o600, "sk-ant-oat01-HOLDER")
	param := filepath.Join(t.TempDir(), "tok")
	if err := os.WriteFile(param, []byte(canary+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	reads := []string{}
	deps := OS()
	read := deps.ReadFile
	deps.ReadFile = func(p string) ([]byte, error) { reads = append(reads, p); return read(p) }
	got := Resolve(home, param, deps)
	want := Decision{HostIsolated: true, SeatToken: canary, Dir: paths.SeatRoot(home)}
	if got != want || len(reads) != 1 || reads[0] != param {
		t.Fatalf("got %+v reads=%v", got, reads)
	}
}

func TestResolve_AParameterFileWithAWrongModeOrNoBodyRefusesNamingTheFile(t *testing.T) {
	t.Parallel()
	home := t.TempDir()
	param := filepath.Join(t.TempDir(), "tok")
	if err := os.WriteFile(param, []byte(canary), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(param, 0o644); err != nil {
		t.Fatal(err)
	}
	got := Resolve(home, param, OS())
	if !strings.Contains(got.Refusal, "the seat token file at "+param+" has mode 0644, not 0600") || got.SeatToken != "" || !got.HostIsolated {
		t.Fatalf("got %+v", got)
	}
	got = Resolve(home, filepath.Join(t.TempDir(), "absent"), OS())
	if !strings.Contains(got.Refusal, "cannot be read") || got.SeatToken != "" || !got.HostIsolated {
		t.Fatalf("an absent parameter file must refuse, not fall back: %+v", got)
	}
}

func TestResolve_AParameterWithAnEmptyHomeRefuses(t *testing.T) {
	t.Parallel()
	got := Resolve("", "/some/file", OS())
	if got.Refusal == "" || got.SeatToken != "" {
		t.Fatalf("got %+v", got)
	}
}

func TestResolve_ATokenGivenAsTheParameterIsRefusedWithoutAnEcho(t *testing.T) {
	t.Parallel()
	probe := "/cwd/" + TokenPrefix + "PROBECANARY-abc"
	d := Resolve(t.TempDir(), probe, OS())
	if d.Refusal == "" || !d.HostIsolated || strings.Contains(d.Refusal, "PROBECANARY") || strings.Contains(d.Refusal, TokenPrefix) {
		t.Fatalf("got %+v", d)
	}
}

func TestTokenMode_OnlyThatExactModeIsAccepted(t *testing.T) {
	t.Parallel()
	if TokenMode != 0o600 {
		t.Fatalf("TokenMode %04o, want 0600", uint32(TokenMode))
	}
	ok := Resolve(plant(t, TokenMode, canary), "", OS())
	if ok.Refusal != "" || ok.SeatToken != canary {
		t.Fatalf("a file at TokenMode: %+v", ok)
	}
	for _, mode := range []fs.FileMode{TokenMode | 0o040, TokenMode &^ 0o200} {
		got := Resolve(plant(t, mode, canary), "", OS())
		if got.SeatToken != "" || !strings.Contains(got.Refusal, "not 0600") {
			t.Errorf("mode %04o differs from TokenMode but resolved: %+v", uint32(mode), got)
		}
	}
}
