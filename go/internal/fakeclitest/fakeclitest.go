// Package fakeclitest stands in for a command-line tool in tests without writing a new executable file.
package fakeclitest

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

const (
	scriptSuffix       = ".fakecli"
	defaultInterpreter = "/bin/sh"
	execFailedExitCode = 127
	readOnlyExecutable = 0o555
)

type execve func(path string, argv, env []string) error

func init() {
	runIfFake(os.Executable, syscall.Exec, os.Exit)
}

func runIfFake(executable func() (string, error), exec execve, exit func(int)) {
	self, err := executable()
	if err != nil {
		return
	}
	if resolved, err := filepath.EvalSymlinks(self); err == nil {
		self = resolved
	}
	script := self + scriptSuffix
	body, err := os.ReadFile(script)
	if errors.Is(err, fs.ErrNotExist) {
		return
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "fakeclitest: read %s: %v\n", script, err)
		exit(execFailedExitCode)
		return
	}
	argv := interpreterArgv(body, script, os.Args[1:])
	err = exec(argv[0], argv, os.Environ())
	fmt.Fprintf(os.Stderr, "fakeclitest: run %s with %s: %v\n", script, argv[0], err)
	exit(execFailedExitCode)
}

func interpreterArgv(body []byte, script string, args []string) []string {
	interpreter := []string{defaultInterpreter}
	if line, _, _ := bytes.Cut(body, []byte("\n")); bytes.HasPrefix(line, []byte("#!")) {
		if fields := strings.Fields(string(line[2:])); len(fields) > 0 {
			interpreter = fields
		}
	}
	return append(append(interpreter, script), args...)
}

func Install(t testing.TB, path, body string) {
	t.Helper()
	self, err := os.Executable()
	if err != nil {
		t.Fatalf("fakeclitest: locate the test binary: %v", err)
	}
	linkErr, err := install(self, path, body, os.Link)
	if err != nil {
		t.Fatalf("fakeclitest: %v", err)
	}
	if linkErr != nil {
		t.Logf("fakeclitest: %s is a copy of the test binary, a new file the first run may wait to have scanned: %v", path, linkErr)
	}
}

func install(self, path, body string, link func(oldname, newname string) error) (linkErr, err error) {
	if err := os.WriteFile(path+scriptSuffix, []byte(body), 0o600); err != nil {
		return nil, fmt.Errorf("write the script for %s: %w", path, err)
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("replace %s: %w", path, err)
	}
	if linkErr = link(self, path); linkErr != nil {
		if err := copyExecutable(self, path); err != nil {
			return linkErr, fmt.Errorf("install %s: %w", path, err)
		}
	}
	if err := os.Chmod(path, readOnlyExecutable); err != nil {
		return linkErr, fmt.Errorf("guard %s against writes: %w", path, err)
	}
	return linkErr, nil
}

func copyExecutable(from, to string) error {
	src, err := os.Open(from)
	if err != nil {
		return err
	}
	defer func() { _ = src.Close() }()
	dst, err := os.OpenFile(to, os.O_WRONLY|os.O_CREATE|os.O_EXCL, readOnlyExecutable|0o200)
	if err != nil {
		return err
	}
	if _, err := io.Copy(dst, src); err != nil {
		_ = dst.Close()
		return err
	}
	return dst.Close()
}
