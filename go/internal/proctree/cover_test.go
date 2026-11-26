package proctree

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestParseProcArgs2_RefusesAnExecPathWithoutItsTerminator(t *testing.T) {
	t.Parallel()
	args, env, err := parseProcArgs2([]byte("\x01\x00\x00\x00/bin/x"))

	if !errors.Is(err, errProcArgsTruncated) || args != nil || env != nil {
		t.Errorf("parseProcArgs2 = %q, %v, %v, want %v and nothing else", args, env, err, errProcArgsTruncated)
	}
}

func TestParseProcFiles_AnEmptyCmdlineHasNoArguments(t *testing.T) {
	t.Parallel()
	args, env := parseProcFiles([]byte("\x00\x00"), nil)

	if args != nil || len(env) != 0 {
		t.Errorf("parseProcFiles = %q, %v, want nil arguments and no environment (a kernel thread has an empty cmdline)", args, env)
	}
}

func TestSaveTree_AStartTimeJSONCannotHoldIsAnEncodeErrorAndWritesNothing(t *testing.T) {
	t.Parallel()
	dir := filepath.Join(t.TempDir(), "dispatch-trees")

	err := SaveTree(dir, "01R/1/build/p5n1", []Identity{{Pid: 10, Started: time.Date(10000, 1, 1, 0, 0, 0, 0, time.UTC)}})

	if err == nil || !strings.HasPrefix(err.Error(), "encode dispatch tree: ") {
		t.Fatalf("SaveTree = %v, want an encode error", err)
	}
	if _, serr := os.Stat(dir); !os.IsNotExist(serr) {
		t.Errorf("the tree dir exists after an encode error: %v", serr)
	}
}

func TestSaveTree_ADirUnderAFileIsAMakeDirError(t *testing.T) {
	t.Parallel()
	file := filepath.Join(t.TempDir(), "plain")
	if err := os.WriteFile(file, nil, 0o600); err != nil {
		t.Fatal(err)
	}

	err := SaveTree(filepath.Join(file, "dispatch-trees"), "01R/1/build/p5n1", nil)

	if err == nil || !strings.HasPrefix(err.Error(), "make dispatch tree dir: ") {
		t.Errorf("SaveTree = %v, want a make-dir error", err)
	}
}

func TestSaveTree_ACreateFailureIsAWriteErrorAndLeavesNoTree(t *testing.T) {
	t.Parallel()
	dir, id := t.TempDir(), "01R/1/build/p5n1"
	cause := errors.New("no space left on device")
	create := func(string, string) (*os.File, error) { return nil, cause }

	err := saveTree(create, dir, id, []Identity{{Pid: 10, Started: t0}})

	if !errors.Is(err, cause) || err.Error() != "write dispatch tree: no space left on device" {
		t.Errorf("saveTree = %v, want a write error wrapping the cause", err)
	}
	if _, serr := os.Stat(TreeFile(dir, id)); !os.IsNotExist(serr) {
		t.Errorf("a tree file exists after a failed write: %v", serr)
	}
}

func TestWriteTemp_AFailedWriteRemovesTheTempFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	create := func(d, pattern string) (*os.File, error) {
		f, err := os.CreateTemp(d, pattern)
		if err != nil {
			return nil, err
		}
		return f, f.Close()
	}

	name, err := writeTemp(create, dir, []byte("{}"))

	if !errors.Is(err, os.ErrClosed) || !strings.HasPrefix(err.Error(), "write dispatch tree: ") || name != "" {
		t.Fatalf("writeTemp = %q, %v, want a write error wrapping %v", name, err, os.ErrClosed)
	}
	if entries, rerr := os.ReadDir(dir); rerr != nil || len(entries) != 0 {
		t.Errorf("dir entries = %v, %v, want the half-written temp file removed", entries, rerr)
	}
}

func TestLoadTree_AnUnreadableTreeIsAReadError(t *testing.T) {
	t.Parallel()
	dir, id := t.TempDir(), "01R/1/build/p5n1"
	if err := os.Mkdir(TreeFile(dir, id), 0o700); err != nil {
		t.Fatal(err)
	}

	got, err := LoadTree(dir, id)

	if err == nil || !strings.HasPrefix(err.Error(), "read dispatch tree: ") || got != nil {
		t.Errorf("LoadTree = %v, %v, want a read error and no tree", got, err)
	}
}

func TestLoadTree_ACorruptTreeIsADecodeError(t *testing.T) {
	t.Parallel()
	dir, id := t.TempDir(), "01R/1/build/p5n1"
	if err := os.WriteFile(TreeFile(dir, id), []byte("{"), 0o600); err != nil {
		t.Fatal(err)
	}

	got, err := LoadTree(dir, id)

	if err == nil || !strings.HasPrefix(err.Error(), "decode dispatch tree: ") || got != nil {
		t.Errorf("LoadTree = %v, %v, want a decode error and no tree", got, err)
	}
}

func TestLoadTree_ATreeOfAnotherDispatchIsRefused(t *testing.T) {
	t.Parallel()
	dir, id := t.TempDir(), "01R/1/build/p5n1"
	data := []byte(`{"dispatch":"01R/1/build/p5n2","members":[{"pid":10,"started":"2026-10-08T14:00:00Z"}]}`)
	if err := os.WriteFile(TreeFile(dir, id), data, 0o600); err != nil {
		t.Fatal(err)
	}

	got, err := LoadTree(dir, id)

	want := `names dispatch "01R/1/build/p5n2", want "01R/1/build/p5n1"`
	if err == nil || !strings.HasSuffix(err.Error(), want) || got != nil {
		t.Errorf("LoadTree = %v, %v, want no tree and an error ending %q: a file name collision must never hand out another dispatch's pids", got, err, want)
	}
}

func TestRemoveTree_AnUnremovableTreeIsAnError(t *testing.T) {
	t.Parallel()
	dir, id := t.TempDir(), "01R/1/build/p5n1"
	path := TreeFile(dir, id)
	if err := os.Mkdir(path, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(path, "keep"), nil, 0o600); err != nil {
		t.Fatal(err)
	}

	err := RemoveTree(dir, id)

	if err == nil || !strings.HasPrefix(err.Error(), "remove dispatch tree: ") {
		t.Errorf("RemoveTree = %v, want a remove error", err)
	}
	if _, serr := os.Stat(path); serr != nil {
		t.Errorf("the tree path is gone: %v", serr)
	}
}
