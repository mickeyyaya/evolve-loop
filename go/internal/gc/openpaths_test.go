package gc

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

func lsofReturning(out string, code int, err error) func(context.Context, string, string, []string, []string, io.Reader, io.Writer, io.Writer) (int, error) {
	return func(_ context.Context, name, _ string, _ []string, _ []string, _ io.Reader, stdout io.Writer, _ io.Writer) (int, error) {
		if name != "lsof" {
			return -1, errors.New("unexpected program " + name)
		}
		_, _ = io.WriteString(stdout, out)
		return code, err
	}
}

func TestListOpenPaths_KeepsOnlyAbsoluteFilePaths(t *testing.T) {
	t.Parallel()
	listing := "p10\nfcwd\nn/tmp/a\nf3\nn*:8080\nn/tmp/b/payload\nnlocalhost:1->2\n"

	got, err := ListOpenPaths(context.Background(), lsofReturning(listing, 0, nil))

	if want := []string{"/tmp/a", "/tmp/b/payload"}; err != nil || !reflect.DeepEqual(got, want) {
		t.Errorf("ListOpenPaths = %v, %v, want %v", got, err, want)
	}
}

func TestListOpenPaths_AnUnknownStateIsAnError(t *testing.T) {
	t.Parallel()
	cases := map[string]func(context.Context, string, string, []string, []string, io.Reader, io.Writer, io.Writer) (int, error){
		"lsof cannot run":     lsofReturning("", -1, errors.New("not found")),
		"lsof prints nothing": lsofReturning("", 1, nil),
	}
	for name, run := range cases {
		if got, err := ListOpenPaths(context.Background(), run); err == nil || got != nil {
			t.Errorf("%s: ListOpenPaths = %v, %v, want an error and no paths", name, got, err)
		}
	}
}

func TestLiveHolderProbe_HoldsADirWhoseSubtreeIsOpenAndAsksLsofOnce(t *testing.T) {
	t.Parallel()
	calls := 0
	run := func(ctx context.Context, name, dir string, args, env []string, in io.Reader, out, errw io.Writer) (int, error) {
		calls++
		return lsofReturning("p1\nn/work/held/001\np2\nn/work/heldother/x\n", 0, nil)(ctx, name, dir, args, env, in, out, errw)
	}
	probe := LiveHolderProbe(context.Background(), run)

	for path, want := range map[string]bool{"/work/held": true, "/work/heldo": false, "/work/free": false, "/work/heldother": true} {
		if got, err := probe(path); err != nil || got != want {
			t.Errorf("probe(%s) = %v, %v, want %v", path, got, err, want)
		}
	}
	if calls != 1 {
		t.Errorf("lsof ran %d times, want 1", calls)
	}
}

func TestLiveHolderProbe_ResolvesASymlinkedDirToItsRealPath(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	real := filepath.Join(root, "real")
	link := filepath.Join(root, "link")
	if err := os.MkdirAll(real, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(real, link); err != nil {
		t.Fatal(err)
	}
	resolved, err := filepath.EvalSymlinks(real)
	if err != nil {
		t.Fatal(err)
	}

	got, perr := LiveHolderProbe(context.Background(), lsofReturning("n"+resolved+"/f\n", 0, nil))(link)

	if perr != nil || !got {
		t.Errorf("probe(link) = %v, %v, want held", got, perr)
	}
}

func TestReapPipelineTemp_SparesAHeldDirAndReportsIt(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	now := time.Date(2026, 11, 14, 12, 0, 0, 0, time.UTC)
	stale := now.Add(-30 * time.Hour)
	for _, name := range []string{"TestHeld1234567890", "TestFree1234567890", "gittest9876543210"} {
		p := filepath.Join(dir, name)
		writeFile(t, filepath.Join(p, "payload"), "0123456789")
		if err := os.Chtimes(p, stale, stale); err != nil {
			t.Fatal(err)
		}
	}
	held := func(p string) (bool, error) { return filepath.Base(p) == "TestHeld1234567890", nil }

	rep := ReapPipelineTemp(dir, now.Add(-24*time.Hour), os.RemoveAll, SparingHeld(held))

	if rep.Entries != 2 || rep.Held != 1 || len(rep.Errors) != 0 {
		t.Errorf("report = %+v, want 2 reaped and 1 held", rep)
	}
	if _, err := os.Stat(filepath.Join(dir, "TestHeld1234567890")); err != nil {
		t.Errorf("held dir removed: %v", err)
	}
}

func TestReapPipelineTemp_AnUnknownHolderStateSparesTheDirAndReportsAnError(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	p := filepath.Join(dir, "go-build1234567")
	writeFile(t, filepath.Join(p, "payload"), "x")
	old := time.Now().Add(-48 * time.Hour)
	if err := os.Chtimes(p, old, old); err != nil {
		t.Fatal(err)
	}
	unknown := func(string) (bool, error) { return false, errors.New("lsof gone") }

	rep := ReapPipelineTemp(dir, time.Now().Add(-24*time.Hour), os.RemoveAll, SparingHeld(unknown))

	if rep.Entries != 0 || len(rep.Errors) != 1 {
		t.Errorf("report = %+v, want nothing reaped and one error", rep)
	}
	if _, err := os.Stat(p); err != nil {
		t.Errorf("dir removed on an unknown state: %v", err)
	}
}

func TestPipelineTempRe_TakesOnlyGoTempDirShapes(t *testing.T) {
	t.Parallel()
	for name, want := range map[string]bool{
		"TestBuildShip1234567890":  true,
		"TestGC_Journey2718281828": true,
		"gittest3141592653":        true,
		"TestRawFixture99":         false,
		"Testing":                  false,
		"gittest":                  false,
		"MyTest1234567890":         false,
	} {
		if got := pipelineTempRe.MatchString(name); got != want {
			t.Errorf("pipelineTempRe(%q) = %v, want %v", name, got, want)
		}
	}
}

func TestTempOption_AnOptionConfiguresTheReap(t *testing.T) {
	t.Parallel()
	var opt TempOption = SparingHeld(func(string) (bool, error) { return true, nil })
	dir := t.TempDir()
	stale := time.Now().Add(-48 * time.Hour)
	p := filepath.Join(dir, "TestOpt1234567890")
	writeFile(t, filepath.Join(p, "payload"), "x")
	if err := os.Chtimes(p, stale, stale); err != nil {
		t.Fatal(err)
	}
	rep := ReapPipelineTemp(dir, time.Now().Add(-24*time.Hour), os.RemoveAll, opt)
	if rep.Held != 1 || rep.Entries != 0 {
		t.Errorf("report = %+v, want 1 held, 0 reaped", rep)
	}
}
