//go:build linux

package proctree

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"testing"
)

func procDir(t *testing.T, pid string, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	dir := filepath.Join(root, pid)
	if err := os.Mkdir(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	for name, body := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func TestReadProcArgsAt_ReadsTheCmdlineAndEnvironOfThePid(t *testing.T) {
	t.Parallel()
	root := procDir(t, "4242", map[string]string{"cmdline": "node\x00mcp.js\x00", "environ": "HOME=/h\x00EVOLVE_DISPATCH_ID=r/1/a/p9n1\x00"})

	args, env, err := readProcArgsAt(root, 4242)

	if err != nil || !reflect.DeepEqual(args, []string{"node", "mcp.js"}) || !reflect.DeepEqual(env, map[string]string{"EVOLVE_DISPATCH_ID": "r/1/a/p9n1"}) {
		t.Errorf("readProcArgsAt = %q, %v, %v, want the arguments and the tag", args, env, err)
	}
}

func TestReadProcArgsAt_AGonePidIsAnError(t *testing.T) {
	t.Parallel()
	args, _, err := readProcArgsAt(t.TempDir(), 4242)

	if !errors.Is(err, os.ErrNotExist) || args != nil {
		t.Errorf("readProcArgsAt = %q, %v, want a not-exist error", args, err)
	}
}

func TestReadProcArgsAt_AnUnreadableEnvironIsAnError(t *testing.T) {
	t.Parallel()
	root := procDir(t, "4242", map[string]string{"cmdline": "node\x00"})

	args, env, err := readProcArgsAt(root, 4242)

	if !errors.Is(err, os.ErrNotExist) || args != nil || env != nil {
		t.Errorf("readProcArgsAt = %q, %v, %v, want an error and no arguments: arguments without the environment prove nothing", args, env, err)
	}
}

func TestReadProcArgs_ReadsTheArgumentsOfThisTestProcess(t *testing.T) {
	t.Parallel()
	args, _, err := readProcArgs(os.Getpid())

	if err != nil || !slices.Equal(args, os.Args) {
		t.Errorf("readProcArgs(self) = %q, %v, want %q", args, err, os.Args)
	}
}
