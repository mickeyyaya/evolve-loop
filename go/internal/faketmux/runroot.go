package faketmux

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"syscall"
)

var mkdirTemp = os.MkdirTemp

const (
	ownerFile = "owner"
	runPrefix = "run-"
)

func Claim(base string) (string, error) {
	if err := privateDir(base); err != nil {
		return "", err
	}
	if err := sweep(base); err != nil {
		return "", err
	}
	pid := os.Getpid()
	begun, err := processStart(pid)
	if err != nil {
		return "", fmt.Errorf("read the start of the owner: %w", err)
	}
	claim, err := mkdirTemp(base, ".claim-")
	if err == nil {
		err = os.WriteFile(filepath.Join(claim, ownerFile), []byte(fmt.Sprintf("%d %s", pid, begun)), 0o600)
	}
	if err != nil {
		return "", errors.Join(err, os.RemoveAll(claim))
	}
	root := filepath.Join(base, runPrefix+strings.TrimPrefix(filepath.Base(claim), ".claim-"))
	return root, os.Rename(claim, root)
}

func privateDir(dir string) error {
	made := os.MkdirAll(dir, 0o700)
	info, err := os.Lstat(dir)
	if err = errors.Join(made, err); err != nil {
		return err
	}
	st, ok := info.Sys().(*syscall.Stat_t)
	if !info.IsDir() || !ok || int(st.Uid) != os.Getuid() || info.Mode().Perm()&0o077 != 0 {
		return fmt.Errorf("the fake tmux base %s must be a directory that only this user can use (mode %v)", dir, info.Mode())
	}
	return nil
}

func sweep(base string) error {
	runs, err := os.ReadDir(base)
	if err != nil {
		return err
	}
	var errs []error
	for _, run := range runs {
		root := filepath.Join(base, run.Name())
		if strings.HasPrefix(run.Name(), runPrefix) && !ownerLives(root) {
			errs = append(errs, Reap(root))
		}
	}
	return errors.Join(errs...)
}

func ownerLives(root string) bool {
	body, err := os.ReadFile(filepath.Join(root, ownerFile))
	var pid int
	var begun string
	if err == nil {
		_, err = fmt.Sscanf(string(body), "%d %s", &pid, &begun)
	}
	return err == nil && startedAs(pid, begun)
}

func Reap(root string) error {
	sockets, err := os.ReadDir(root)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	var errs []error
	for _, socket := range sockets {
		if socket.IsDir() {
			errs = append(errs, (&server{dir: filepath.Join(root, socket.Name())}).killServer(invocation{}))
		}
	}
	return errors.Join(append(errs, os.RemoveAll(root))...)
}
