package faketmux

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

type failWriter struct{}

func (failWriter) Write([]byte) (int, error) { return 0, errInjected }

func liveSession(t *testing.T, f fakeTmux, name string) session {
	t.Helper()
	f.must("new-session", "-d", "-s", name, "sleep 30")
	return f.server().session(name)
}

func wantErr(t *testing.T, err error, want string) {
	t.Helper()
	if err == nil || !strings.Contains(err.Error(), want) {
		t.Errorf("err = %v, want one naming %q", err, want)
	}
}

func TestServer_AFailingStdoutFailsEveryPrintingCommand(t *testing.T) {
	t.Parallel()
	f := newFakeTmux(t)
	liveSession(t, f, "w")
	if err := f.server().writeOptionCommand(); err != nil {
		t.Fatal(err)
	}
	s := f.server()
	s.stdout = failWriter{}
	target := map[byte]string{'t': "w"}
	for name, inv := range map[string]invocation{
		"show-options":    {flags: target, args: []string{"@o"}},
		"capture-pane":    {flags: target},
		"display-message": {flags: target, args: []string{"#{pane_pid}"}},
		"list-panes":      {flags: target},
		"list-sessions":   {flags: map[byte]string{}},
	} {
		wantErr(t, commands[name].run(s, inv), "injected")
	}
}

func (s *server) writeOptionCommand() error {
	return s.setOption(invocation{flags: map[byte]string{'t': "w"}, args: []string{"@o", "v"}})
}

func TestServer_AProcessListingFailureFailsTheForegroundQuery(t *testing.T) {
	t.Parallel()
	f := newFakeTmux(t)
	sess := liveSession(t, f, "p")
	s := f.server()
	s.procs = func() ([]process, error) { return nil, errInjected }

	_, err := s.expand(sess, "#{pane_current_command}")

	wantErr(t, err, "list processes: injected")
	wantErr(t, s.listSessions(invocation{flags: map[byte]string{'F': "#{pane_current_command}"}}), "injected")
}

func TestServer_BrokenStateFilesFailTheCommandsThatReadThem(t *testing.T) {
	t.Parallel()
	f := newFakeTmux(t)
	sess := liveSession(t, f, "b")
	s := f.server()
	if err := os.Remove(sess.path("screen")); err != nil {
		t.Fatal(err)
	}
	target := invocation{flags: map[byte]string{'t': "b", 'b': "b"}, args: []string{"x"}}

	wantErr(t, s.capturePane(target), "no such file")
	wantErr(t, s.sendKeys(target), "no such file")
	if got := sess.activity(); got != "0" {
		t.Errorf("activity without a screen = %q, want 0", got)
	}
	buf := filepath.Join(t.TempDir(), "buf")
	if err := os.WriteFile(buf, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := s.loadBuffer(invocation{flags: target.flags, args: []string{buf}}); err != nil {
		t.Fatal(err)
	}
	wantErr(t, s.pasteBuffer(target), "no such file")
}

func TestServer_AFileWhereADirectoryBelongsFailsTheWrite(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "sock"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	s := &server{dir: filepath.Join(root, "sock"), stdout: &bytes.Buffer{}, procs: listProcesses}

	wantErr(t, s.newSession(invocation{flags: map[byte]string{'s': "n"}}), "not a directory")
	wantErr(t, s.killServer(invocation{}), "not a directory")
	wantErr(t, s.listSessions(invocation{}), "not a directory")
	wantErr(t, s.loadBuffer(invocation{args: []string{filepath.Join(root, "sock")}}), "not a directory")
}

func TestServer_ABufferPathThatIsADirectoryFailsLoadBuffer(t *testing.T) {
	t.Parallel()
	s := &server{dir: t.TempDir()}
	if err := os.MkdirAll(s.bufferPath("b"), 0o700); err != nil {
		t.Fatal(err)
	}
	src := filepath.Join(t.TempDir(), "src")
	if err := os.WriteFile(src, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}

	wantErr(t, s.loadBuffer(invocation{flags: map[byte]string{'b': "b"}, args: []string{src}}), "is a directory")
}

func TestSession_ARecordWithoutAPaneFailsTheReaders(t *testing.T) {
	t.Parallel()
	sess := session{name: "x", dir: t.TempDir()}
	s := &server{dir: t.TempDir(), stdout: &bytes.Buffer{}}

	_, err := s.expand(sess, "x")
	wantErr(t, err, "no such file")
	_, err = sess.capture(0)
	wantErr(t, err, "no such file")
	for body, want := range map[string]string{"x": "pane record", "1 24": "has no process"} {
		if err := os.WriteFile(sess.path("pane"), []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
		_, _, err := sess.pane()
		wantErr(t, err, want)
	}
}

func TestSession_StartFailsCleanlyOnEachBrokenStep(t *testing.T) {
	t.Parallel()
	sess := session{name: "s", dir: t.TempDir()}
	if err := os.Mkdir(sess.path("pane.tmp"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(sess.path("pane.tmp/keep"), nil, 0o600); err != nil {
		t.Fatal(err)
	}

	wantErr(t, sess.start([]string{"sleep", "30"}, "", 24), "is a directory")
	wantErr(t, sess.start([]string{"sleep", "30"}, "", 24), "make the pane input")
}

func TestSession_WriteFileFailsWhenTheTargetIsADirectory(t *testing.T) {
	t.Parallel()
	sess := session{name: "s", dir: t.TempDir()}
	if err := os.MkdirAll(sess.path("pane/keep"), 0o700); err != nil {
		t.Fatal(err)
	}

	if err := sess.writeFile("pane", "1 1"); err == nil {
		t.Error("a rename over a non-empty directory must fail")
	}
}

func TestSession_FeedNeedsAReader(t *testing.T) {
	t.Parallel()
	sess := session{name: "f", dir: t.TempDir()}
	if err := syscall.Mkfifo(sess.path("in"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(sess.path("screen"), nil, 0o600); err != nil {
		t.Fatal(err)
	}

	wantErr(t, sess.feed("x", ""), "session f has no reader")
	if err := sess.feed("", ""); err != nil {
		t.Errorf("an empty feed writes nothing and succeeds: %v", err)
	}
}

func TestSeams_InjectedFailuresSurface(t *testing.T) {
	f := newFakeTmux(t)
	sess := liveSession(t, f, "seam")
	s := f.server()

	restore := setSeams(t)
	setBlocking = func(*os.File) error { return errInjected }
	wantErr(t, sess.feed("x", ""), "block on the pane input: injected")

	signalGroup = func(int) error { return syscall.EPERM }
	wantErr(t, s.killServer(invocation{}), "kill session seam")
	wantErr(t, sess.kill(), "operation not permitted")

	runPS = func() (string, error) { return "", errInjected }
	if _, err := listProcesses(); !errors.Is(err, errInjected) {
		t.Errorf("listProcesses = %v, want the ps failure", err)
	}
	restore()

	for _, file := range []string{"in", "screen"} {
		dir := t.TempDir()
		openFile = func(name string, flag int, perm os.FileMode) (*os.File, error) {
			if filepath.Base(name) == file {
				return nil, errInjected
			}
			return os.OpenFile(name, flag, perm)
		}
		wantErr(t, session{name: "o", dir: dir}.start([]string{"sleep", "30"}, "", 24), "injected")
		restore()
	}
}

func setSeams(t *testing.T) func() {
	t.Helper()
	open, signal, blocking, ps := openFile, signalGroup, setBlocking, runPS
	restore := func() { openFile, signalGroup, setBlocking, runPS = open, signal, blocking, ps }
	t.Cleanup(restore)
	return restore
}
