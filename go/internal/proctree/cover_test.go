package proctree

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestNewLister_AnUnrunnablePSIsAnErrorThatNamesPS(t *testing.T) {
	t.Parallel()
	cause := errors.New("executable file not found")
	run := func(context.Context, string, string, []string, []string, io.Reader, io.Writer, io.Writer) (int, error) {
		return -1, cause
	}

	got, err := NewLister(run, nil, 501)(context.Background())

	if !errors.Is(err, cause) || err.Error() != "ps: executable file not found" || got != nil {
		t.Errorf("list = %v, %v, want no table and the error %q wrapping the cause", got, err, "ps: executable file not found")
	}
}

func TestNewLister_APSExitNamesTheCodeAndTheStderr(t *testing.T) {
	t.Parallel()
	run := func(_ context.Context, _, _ string, _, _ []string, _ io.Reader, _, stderr io.Writer) (int, error) {
		_, _ = io.WriteString(stderr, "ps: illegal option\n")
		return 2, nil
	}

	_, err := NewLister(run, nil, 501)(context.Background())

	if err == nil || err.Error() != "ps: exit 2: ps: illegal option" {
		t.Errorf("list error = %v, want %q", err, "ps: exit 2: ps: illegal option")
	}
}

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

func TestParsePSRow_RefusesANonNumericIDColumn(t *testing.T) {
	t.Parallel()
	valid := "  100     1   100   501 Thu Oct  8 14:00:00 2026     node"
	if _, ok := parsePSRow(valid, 501); !ok {
		t.Fatalf("parsePSRow(%q) refused a valid row", valid)
	}
	for _, line := range []string{
		"  10x     1   100   501 Thu Oct  8 14:00:00 2026     node",
		"  100     1   100   5o1 Thu Oct  8 14:00:00 2026     node",
	} {
		if p, ok := parsePSRow(line, 501); ok {
			t.Errorf("parsePSRow(%q) = %+v, true, want a refusal", line, p)
		}
	}
}

func TestParsePSRow_RefusesAStartTimeOutsideTheLstartFormat(t *testing.T) {
	t.Parallel()
	line := "  100     1   100   501 Thx Oct  8 14:00:00 2026     node"

	if p, ok := parsePSRow(line, 501); ok {
		t.Errorf("parsePSRow(%q) = %+v, true, want a refusal: a row without a start time cannot prove an identity", line, p)
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
