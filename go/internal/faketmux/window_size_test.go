package faketmux

import (
	"os"
	"testing"
)

func TestNewSession_ABadWidthIsRefused(t *testing.T) {
	t.Parallel()
	f := newFakeTmux(t)
	f.refuses("width", "new-session", "-d", "-s", "w", "-x", "abc", "sleep 30")
}

func TestNewSession_AFailingPaneStartIsReported(t *testing.T) {
	f := newFakeTmux(t)
	saved := openFile
	t.Cleanup(func() { openFile = saved })
	openFile = func(string, int, os.FileMode) (*os.File, error) { return nil, errInjected }
	f.refuses("injected", "new-session", "-d", "-s", "w", "sleep 30")
}

func TestResizeWindow_ANudgePastTheLimitIsRefused(t *testing.T) {
	t.Parallel()
	f := newFakeTmux(t)
	liveSession(t, f, "r")
	f.refuses("out of range", "resize-window", "-t", "=r:", "-L", "80")
	f.refuses("bad adjustment", "resize-window", "-t", "=r:", "-U", "-1")
}

func TestSessionSize_FallsBackToTheDefaultsWithoutRecords(t *testing.T) {
	t.Parallel()
	sess := session{name: "n", dir: t.TempDir()}
	if got := sess.width(); got != defaultWidth {
		t.Errorf("width = %d, want %d", got, defaultWidth)
	}
	if got := sess.height(); got != defaultHeight {
		t.Errorf("height = %d, want %d", got, defaultHeight)
	}
	wantErr(t, sess.writeSize(10, 10), "no such file")
}

func TestSessionWriteSize_ABlockedWidthFileIsReported(t *testing.T) {
	t.Parallel()
	f := newFakeTmux(t)
	sess := liveSession(t, f, "b")
	if err := os.Mkdir(sess.path("width.tmp"), 0o700); err != nil {
		t.Fatal(err)
	}
	wantErr(t, sess.writeSize(10, 10), "directory")
}

func TestSessionSize_ReadsWhatWriteSizeStored(t *testing.T) {
	t.Parallel()
	f := newFakeTmux(t)
	sess := liveSession(t, f, "s")
	if err := sess.writeSize(33, 11); err != nil {
		t.Fatal(err)
	}
	if sess.width() != 33 || sess.height() != 11 {
		t.Errorf("size = %dx%d, want 33x11", sess.width(), sess.height())
	}
}
