package main

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/agentclass"
	"github.com/mickeyyaya/evolve-loop/go/internal/paths"
	"github.com/mickeyyaya/evolve-loop/go/internal/runlease"
	"github.com/mickeyyaya/evolve-loop/go/internal/seatlogin"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

const seatCanary = "sk-ant-oat01-CANARY-0123456789"

var seatFixedNow = time.Date(2026, 11, 6, 9, 0, 0, 0, time.UTC)

type seatTokenRig struct {
	home, root string
	env        authEnv
}

type explodingReader struct{ t *testing.T }

func (r explodingReader) Read([]byte) (int, error) {
	r.t.Helper()
	r.t.Error("stdin was read")
	return 0, io.EOF
}

func newSeatTokenRig(t *testing.T, stdin string) seatTokenRig {
	t.Helper()
	home := t.TempDir()
	root := t.TempDir()
	if err := os.MkdirAll(paths.EvolveDirOf(root), 0o755); err != nil {
		t.Fatal(err)
	}
	return seatTokenRig{home: home, root: root, env: authEnv{
		Home:  home,
		Stdin: strings.NewReader(stdin),
		Live:  func(string) []runlease.LiveRun { return nil },
		Now:   func() time.Time { return seatFixedNow },
	}}
}

func (r seatTokenRig) run(t *testing.T, args ...string) (int, string, string) {
	t.Helper()
	var out, errb bytes.Buffer
	rc := runAuthEnv(r.env, args, &out, &errb)
	return rc, out.String(), errb.String()
}

func (r seatTokenRig) tokenPath() string { return paths.SeatTokenPath(r.home) }

func captureAuthEvents(t *testing.T) func() []signalcenter.Event {
	t.Helper()
	var mu sync.Mutex
	var events []signalcenter.Event
	prev := attachRootEventsFn
	t.Cleanup(func() { attachRootEventsFn = prev })
	attachRootEventsFn = func(c *signalcenter.Center, role, _ string, _ io.Writer) func() {
		if role == roleAuth {
			c.Subscribe(func(e signalcenter.Event) {
				mu.Lock()
				defer mu.Unlock()
				events = append(events, e)
			})
		}
		return c.Flush
	}
	return func() []signalcenter.Event {
		mu.Lock()
		defer mu.Unlock()
		return slices.Clone(events)
	}
}

func liveLane() []runlease.LiveRun {
	return []runlease.LiveRun{{Dir: "/r/runs/lane-a", Lease: runlease.Lease{OwnerPID: 4242}}}
}

func TestSeatTokenSet_WritesTheTokenAtMode0600AndNeverPrintsIt(t *testing.T) {
	t.Parallel()
	r := newSeatTokenRig(t, seatCanary+"\n")
	rc, out, errText := r.run(t, "seat-token", "set", "--project-root", r.root)
	if rc != 0 {
		t.Fatalf("rc=%d err=%q", rc, errText)
	}
	info, err := os.Stat(r.tokenPath())
	if err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("info=%v err=%v", info, err)
	}
	body, _ := os.ReadFile(r.tokenPath())
	if string(body) != seatCanary+"\n" {
		t.Fatalf("body=%q", body)
	}
	if strings.Contains(out+errText, seatCanary) || !strings.Contains(out, r.tokenPath()) {
		t.Fatalf("out=%q err=%q", out, errText)
	}
	if !strings.Contains(out, "each claude seat now runs isolated on it") {
		t.Fatalf("out=%q", out)
	}
}

func TestSeatTokenSet_ReplacesAWideFileWithMode0600(t *testing.T) {
	t.Parallel()
	r := newSeatTokenRig(t, seatCanary)
	if err := os.MkdirAll(filepath.Dir(r.tokenPath()), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(r.tokenPath(), []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	if rc, _, errText := r.run(t, "seat-token", "set", "--project-root", r.root); rc != 0 {
		t.Fatalf("rc=%d err=%q", rc, errText)
	}
	info, _ := os.Stat(r.tokenPath())
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("mode=%v", info.Mode())
	}
}

func TestSeatTokenSet_ThenResolveReadsTheSameToken(t *testing.T) {
	t.Parallel()
	r := newSeatTokenRig(t, seatCanary)
	if rc, _, errText := r.run(t, "seat-token", "set", "--project-root", r.root); rc != 0 {
		t.Fatalf("rc=%d err=%q", rc, errText)
	}
	got := seatlogin.Resolve(r.home, "", seatlogin.OS())
	if got.Refusal != "" || got.SeatToken != seatCanary || !got.HostIsolated {
		t.Fatalf("got %+v", got)
	}
}

func TestSeatTokenVerbs_RefuseWhileALaneIsLiveBeforeReadingStdin(t *testing.T) {
	t.Parallel()
	for _, verb := range []string{"set", "clear"} {
		r := newSeatTokenRig(t, "")
		r.env.Stdin = explodingReader{t}
		r.env.Live = func(string) []runlease.LiveRun { return liveLane() }
		if err := os.MkdirAll(filepath.Dir(r.tokenPath()), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(r.tokenPath(), []byte("keep"), 0o600); err != nil {
			t.Fatal(err)
		}
		rc, out, errText := r.run(t, "seat-token", verb, "--project-root", r.root)
		want := "evolve auth seat-token " + verb + ": refused — a loop lane is live (/r/runs/lane-a, pid 4242); run it at a wave boundary (`evolve loop-stop --wait`)\n"
		if rc != cliExitRefused || out != "" || errText != want {
			t.Fatalf("%s: rc=%d out=%q err=%q", verb, rc, out, errText)
		}
		body, _ := os.ReadFile(r.tokenPath())
		if string(body) != "keep" {
			t.Fatalf("%s: the file changed: %q", verb, body)
		}
	}
}

func TestSeatTokenVerbs_NeedAProjectRootAndAKnownVerb(t *testing.T) {
	t.Parallel()
	cases := [][]string{{"seat-token"}, {"seat-token", "bogus"}, {"seat-token", "set"}, {"seat-token", "clear"}, {"seat-token", "set", "--project-root", "/x", "extra"}, {"seat-token", "set", "--project-root", "/x", "--token=" + seatCanary}, {"seat-token", "set", "--bogus"}}
	for _, args := range cases {
		r := newSeatTokenRig(t, seatCanary)
		rc, out, errText := r.run(t, args...)
		if rc != exitUsage || out != "" || !strings.HasPrefix(errText, "evolve auth: ") || !strings.HasSuffix(errText, authUsage+"\n") {
			t.Fatalf("%v: rc=%d out=%q err=%q", args, rc, out, errText)
		}
		if _, err := os.Stat(r.tokenPath()); err == nil {
			t.Fatalf("%v: a file was written", args)
		}
	}
}

func TestSeatTokenSet_RefusesAValueWithoutThePrefixWithThePinnedText(t *testing.T) {
	t.Parallel()
	for _, stdin := range []string{"", "  \n", "not-a-token-CANARY"} {
		r := newSeatTokenRig(t, stdin)
		rc, out, errText := r.run(t, "seat-token", "set", "--project-root", r.root)
		want := "evolve auth seat-token set: the value on stdin was refused: it does not start with sk-ant-oat01-. The holder at " + r.tokenPath() + " is unchanged. Next: run claude setup-token, then evolve auth seat-token set again.\n"
		if rc != cliExitInput || out != "" || errText != want {
			t.Fatalf("%q: rc=%d out=%q err=%q", stdin, rc, out, errText)
		}
		if _, err := os.Stat(r.tokenPath()); err == nil {
			t.Fatalf("%q: a file was written", stdin)
		}
	}
}

func TestSeatTokenSet_AWriteFailureExitsTwoWithoutTheToken(t *testing.T) {
	t.Parallel()
	r := newSeatTokenRig(t, seatCanary)
	if err := os.WriteFile(paths.SeatRoot(r.home), []byte("a file, not a directory"), 0o600); err != nil {
		t.Fatal(err)
	}
	rc, out, errText := r.run(t, "seat-token", "set", "--project-root", r.root)
	if rc != cliExitIO || out != "" || strings.Contains(errText, seatCanary) || !strings.HasPrefix(errText, "evolve auth seat-token set: ") {
		t.Fatalf("rc=%d out=%q err=%q", rc, out, errText)
	}
}

func TestSeatTokenSet_ATempFileFailureExitsTwoWithoutTheToken(t *testing.T) {
	t.Parallel()
	if os.Geteuid() == 0 {
		t.Skip("root ignores directory modes")
	}
	r := newSeatTokenRig(t, seatCanary)
	dir := filepath.Dir(r.tokenPath())
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(dir, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o700) })
	rc, out, errText := r.run(t, "seat-token", "set", "--project-root", r.root)
	if rc != cliExitIO || out != "" || strings.Contains(errText, seatCanary) || !strings.HasPrefix(errText, "evolve auth seat-token set: write the token: ") {
		t.Fatalf("rc=%d out=%q err=%q", rc, out, errText)
	}
}

func TestSeatTokenSet_AStdinFailureExitsTwo(t *testing.T) {
	t.Parallel()
	r := newSeatTokenRig(t, "")
	r.env.Stdin = io.MultiReader(strings.NewReader("sk-ant"), errReader{})
	rc, out, errText := r.run(t, "seat-token", "set", "--project-root", r.root)
	if rc != cliExitIO || out != "" || !strings.Contains(errText, "read stdin: boom") {
		t.Fatalf("rc=%d out=%q err=%q", rc, out, errText)
	}
}

type errReader struct{}

func (errReader) Read([]byte) (int, error) { return 0, errors.New("boom") }

func TestSeatTokenVerbs_AnEmptyHomeExitsTwo(t *testing.T) {
	t.Parallel()
	for _, verb := range []string{"set", "clear"} {
		r := newSeatTokenRig(t, seatCanary)
		r.env.Home = ""
		rc, out, errText := r.run(t, "seat-token", verb, "--project-root", r.root)
		if rc != cliExitIO || out != "" || errText != "evolve auth seat-token "+verb+": the home directory is empty\n" {
			t.Fatalf("%s: rc=%d out=%q err=%q", verb, rc, out, errText)
		}
	}
}

func TestSeatTokenClear_RemovesTheFileAndIsIdempotent(t *testing.T) {
	t.Parallel()
	r := newSeatTokenRig(t, seatCanary)
	if rc, _, _ := r.run(t, "seat-token", "set", "--project-root", r.root); rc != 0 {
		t.Fatal("set failed")
	}
	for i := 0; i < 2; i++ {
		rc, out, errText := r.run(t, "seat-token", "clear", "--project-root", r.root)
		if rc != 0 || errText != "" || !strings.Contains(out, r.tokenPath()) {
			t.Fatalf("pass %d: rc=%d out=%q err=%q", i, rc, out, errText)
		}
		if _, err := os.Stat(r.tokenPath()); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("pass %d: err=%v", i, err)
		}
	}
	if got := seatlogin.Resolve(r.home, "", seatlogin.OS()); got != (seatlogin.Decision{}) {
		t.Fatalf("got %+v", got)
	}
}

func TestSeatTokenClear_ARemoveFailureExitsTwo(t *testing.T) {
	t.Parallel()
	r := newSeatTokenRig(t, "")
	if err := os.MkdirAll(filepath.Join(r.tokenPath(), "inner"), 0o700); err != nil {
		t.Fatal(err)
	}
	rc, out, errText := r.run(t, "seat-token", "clear", "--project-root", r.root)
	if rc != cliExitIO || out != "" || !strings.HasPrefix(errText, "evolve auth seat-token clear: ") {
		t.Fatalf("rc=%d out=%q err=%q", rc, out, errText)
	}
}

func TestSeatTokenVerbs_EmitOneSignalWithTheVerbAndTheTimeAndNoToken(t *testing.T) {
	events := captureAuthEvents(t)
	r := newSeatTokenRig(t, seatCanary)
	r.run(t, "seat-token", "set", "--project-root", r.root)
	r.run(t, "seat-token", "clear", "--project-root", r.root)
	got := events()
	if len(got) != 2 {
		t.Fatalf("events=%+v", got)
	}
	for i, verb := range []string{"set", "clear"} {
		e := got[i]
		if e.Module != signalcenter.ModuleCLI || e.Fields["verb"] != verb || e.Fields["at"] != seatFixedNow.Format(time.RFC3339) {
			t.Fatalf("%s: %+v", verb, e)
		}
		if strings.Contains(e.Reason+e.Fields["verb"]+e.Fields["at"]+string(e.Code), seatCanary) {
			t.Fatalf("%s: token in the signal", verb)
		}
	}
}

func TestSeatTokenVerbs_ARefusedVerbEmitsNoSignal(t *testing.T) {
	events := captureAuthEvents(t)
	r := newSeatTokenRig(t, "bad")
	r.run(t, "seat-token", "set", "--project-root", r.root)
	r.env.Live = func(string) []runlease.LiveRun { return liveLane() }
	r.run(t, "seat-token", "clear", "--project-root", r.root)
	if got := events(); len(got) != 0 {
		t.Fatalf("events=%+v", got)
	}
}

func TestAuthSeatToken_BothVerbsAreDeniedToAgentsAndTheUsageNamesThem(t *testing.T) {
	t.Parallel()
	for _, verb := range []string{"set", "clear"} {
		m := agentclass.Of([]string{"auth", "seat-token", verb, "--project-root", "/x"})
		if m.Name != "auth seat-token" || m.Class != agentclass.Denied {
			t.Fatalf("%s: match=%+v", verb, m)
		}
		if !strings.Contains(authUsage, "seat-token "+verb) {
			t.Fatalf("%s: usage lacks the verb", verb)
		}
	}
}

func TestRunAuth_PassesStdinToTheSeatTokenVerb(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	root := t.TempDir()
	if err := os.MkdirAll(paths.EvolveDirOf(root), 0o755); err != nil {
		t.Fatal(err)
	}
	var out, errb bytes.Buffer
	rc := runAuth([]string{"seat-token", "set", "--project-root", root}, strings.NewReader(seatCanary), &out, &errb)
	if rc != 0 {
		t.Fatalf("rc=%d err=%q", rc, errb.String())
	}
	if body, _ := os.ReadFile(paths.SeatTokenPath(home)); string(body) != seatCanary+"\n" {
		t.Fatalf("body=%q", body)
	}
}

type countingReader struct {
	r io.Reader
	n int
}

func (c *countingReader) Read(p []byte) (int, error) {
	n, err := c.r.Read(p)
	c.n += n
	return n, err
}

func (r seatTokenRig) signalsPath() string {
	return filepath.Join(paths.EvolveDirOf(r.root), "signals.ndjson")
}

func TestSeatTokenVerbs_ProductionGateRefusesOnARealLease(t *testing.T) {
	t.Parallel()
	home, root := t.TempDir(), t.TempDir()
	workspace := filepath.Join(paths.EvolveDirOf(root), "runs", "cycle-7")
	if err := os.MkdirAll(workspace, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := runlease.Write(workspace, runlease.Lease{RunID: "r", OwnerPID: os.Getpid()}, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	env := productionAuthEnv(home)
	env.Stdin = strings.NewReader(seatCanary)
	var out, errb bytes.Buffer
	rc := runAuthEnv(env, []string{"seat-token", "set", "--project-root", root}, &out, &errb)
	if rc != cliExitRefused || !strings.Contains(errb.String(), "a loop lane is live") {
		t.Fatalf("rc=%d err=%q", rc, errb.String())
	}
	if _, err := os.Stat(paths.SeatTokenPath(home)); err == nil {
		t.Fatal("the holder was written under a live lease")
	}
}

func TestProductionAuthEnv_NowIsTheClock(t *testing.T) {
	t.Parallel()
	if d := time.Since(productionAuthEnv("/h").Now()); d < 0 || d > time.Minute {
		t.Fatalf("d=%v", d)
	}
}

func TestSeatTokenSet_SignalLandsInTheRootWithTheCode(t *testing.T) {
	t.Parallel()
	r := newSeatTokenRig(t, seatCanary)
	if rc, _, e := r.run(t, "seat-token", "set", "--project-root", r.root); rc != 0 {
		t.Fatalf("rc=%d %q", rc, e)
	}
	body, err := os.ReadFile(r.signalsPath())
	if err != nil || !strings.Contains(string(body), "CLI_SEAT_TOKEN_MOVED") || strings.Contains(string(body), seatCanary) {
		t.Fatalf("err=%v body=%q", err, body)
	}
}

func TestSeatTokenSet_AFailedWriteEmitsNoSignal(t *testing.T) {
	t.Parallel()
	r := newSeatTokenRig(t, seatCanary)
	if err := os.WriteFile(paths.SeatRoot(r.home), []byte("a file"), 0o600); err != nil {
		t.Fatal(err)
	}
	if rc, _, _ := r.run(t, "seat-token", "set", "--project-root", r.root); rc != cliExitIO {
		t.Fatalf("rc=%d", rc)
	}
	if _, err := os.Stat(r.signalsPath()); err == nil {
		t.Fatal("a failed set emitted a signal")
	}
}

func TestSeatTokenClear_ASettingsRemoveFailureExitsTwoAndEmitsNoSignal(t *testing.T) {
	t.Parallel()
	r := newSeatTokenRig(t, "")
	if err := os.MkdirAll(filepath.Join(paths.SeatSettingsPath(r.home), "inner"), 0o700); err != nil {
		t.Fatal(err)
	}
	rc, out, errText := r.run(t, "seat-token", "clear", "--project-root", r.root)
	if rc != cliExitIO || out != "" || !strings.Contains(errText, "remove the seat settings") {
		t.Fatalf("rc=%d out=%q err=%q", rc, out, errText)
	}
	if _, err := os.Stat(r.signalsPath()); err == nil {
		t.Fatal("a clear that left the seat settings emitted a signal")
	}
}

func TestSeatTokenClear_AFailedRemoveEmitsNoSignal(t *testing.T) {
	t.Parallel()
	r := newSeatTokenRig(t, "")
	if err := os.MkdirAll(filepath.Join(r.tokenPath(), "inner"), 0o700); err != nil {
		t.Fatal(err)
	}
	if rc, _, _ := r.run(t, "seat-token", "clear", "--project-root", r.root); rc != cliExitIO {
		t.Fatalf("rc=%d", rc)
	}
	if _, err := os.Stat(r.signalsPath()); err == nil {
		t.Fatal("a failed clear emitted a signal")
	}
}

func TestSeatTokenSet_StdinReadIsBoundedAndAnOverlongValueIsRefused(t *testing.T) {
	t.Parallel()
	r := newSeatTokenRig(t, "")
	c := &countingReader{r: strings.NewReader(seatlogin.TokenPrefix + strings.Repeat("x", 3*seatTokenMaxBytes))}
	r.env.Stdin = c
	rc, _, errText := r.run(t, "seat-token", "set", "--project-root", r.root)
	if c.n > seatTokenMaxBytes+2 {
		t.Fatalf("read %d bytes", c.n)
	}
	if rc != cliExitInput || !strings.Contains(errText, "it is longer than 4096 bytes") {
		t.Fatalf("rc=%d err=%q", rc, errText)
	}
	if _, err := os.Stat(r.tokenPath()); err == nil {
		t.Fatal("a cut token was written")
	}
}

func TestSeatTokenSet_RefusesAValueWithInnerWhitespaceAndNeverEchoesIt(t *testing.T) {
	t.Parallel()
	for _, stdin := range []string{seatCanary + "\nsecond-SECRET", seatlogin.TokenPrefix + "a b-SECRET"} {
		r := newSeatTokenRig(t, stdin)
		rc, out, errText := r.run(t, "seat-token", "set", "--project-root", r.root)
		if rc != cliExitInput || strings.Contains(out+errText, "SECRET") || !strings.Contains(errText, "it holds whitespace or a control byte inside") {
			t.Fatalf("rc=%d out=%q err=%q", rc, out, errText)
		}
		if _, err := os.Stat(r.tokenPath()); err == nil {
			t.Fatal("a file was written")
		}
	}
}

func TestSeatTokenSet_APositionalOrFlagTokenIsNeverEchoed(t *testing.T) {
	t.Parallel()
	for _, args := range [][]string{
		{"seat-token", "set", "{root}", seatCanary},
		{"seat-token", "set", seatCanary, "{root}"},
		{"seat-token", "set", "{root}", "--token=" + seatCanary},
		{"seat-token", "set", "{root}", "-" + seatCanary},
		{"seat-token", "set", "{root}", "--" + seatCanary},
		{"seat-token", seatCanary},
	} {
		r := newSeatTokenRig(t, "")
		full := []string{}
		for _, a := range args {
			if a == "{root}" {
				full = append(full, "--project-root", r.root)
				continue
			}
			full = append(full, a)
		}
		rc, out, errText := r.run(t, full...)
		if rc != cliExitUsage || strings.Contains(out+errText, seatCanary) || strings.Contains(out+errText, "CANARY") {
			t.Fatalf("%v: rc=%d leak=%q", args, rc, out+errText)
		}
	}
}

func TestSeatTokenUsage_SaysTheTokenComesOnStdinInYourOwnTerminal(t *testing.T) {
	t.Parallel()
	for _, want := range []string{"on stdin", "the post-mortem and the spend readers read the transcripts of a seat under the seat root", "in your own terminal", "never paste a token into a chat or an agent session", "a ! command in a session lands in the transcript"} {
		if !strings.Contains(authUsage, want) {
			t.Errorf("usage lacks %q", want)
		}
	}
	if strings.Contains(authUsage, "no launch reads it") {
		t.Error("usage still says no launch reads the token")
	}
}

func TestSeatTokenVerbs_ARootWithoutAnEvolveDirectoryIsRefusedBeforeTheGate(t *testing.T) {
	t.Parallel()
	r := newSeatTokenRig(t, seatCanary)
	live := false
	r.env.Live = func(string) []runlease.LiveRun { live = true; return nil }
	root := filepath.Join(t.TempDir(), "typo")
	rc, out, errText := r.run(t, "seat-token", "set", "--project-root", root)
	if rc != cliExitUsage || out != "" || !strings.Contains(errText, "is not a loop root") || live {
		t.Fatalf("rc=%d out=%q err=%q live=%v", rc, out, errText, live)
	}
	if _, err := os.Stat(root); err == nil {
		t.Fatal("the typo root was created")
	}
}

func TestSeatTokenClear_RemovesTheArmedMarkerSoTheHostInspectsAsNone(t *testing.T) {
	t.Parallel()
	r := newSeatTokenRig(t, "")
	if err := os.MkdirAll(filepath.Dir(r.tokenPath()), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(r.tokenPath(), []byte(seatCanary), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(paths.SeatSettingsPath(r.home), []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	if rc, _, e := r.run(t, "seat-token", "clear", "--project-root", r.root); rc != 0 {
		t.Fatalf("rc=%d %q", rc, e)
	}
	if got := seatlogin.Inspect(r.home, seatlogin.OS()); got.State != seatlogin.StateNone {
		t.Fatalf("state=%v: %s", got.State, got.Message)
	}
}

func TestSeatTokenSet_TheHolderDirectoryIsPrivate(t *testing.T) {
	t.Parallel()
	r := newSeatTokenRig(t, seatCanary)
	if rc, _, e := r.run(t, "seat-token", "set", "--project-root", r.root); rc != 0 {
		t.Fatalf("rc=%d %q", rc, e)
	}
	info, err := os.Stat(filepath.Dir(r.tokenPath()))
	if err != nil || info.Mode().Perm() != 0o700 {
		t.Fatalf("mode=%v err=%v", info, err)
	}
}
