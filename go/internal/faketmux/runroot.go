package faketmux

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

var (
	mkdirTemp = os.MkdirTemp
	newToken  = func() (string, error) {
		b := make([]byte, 16)
		_, err := rand.Read(b)
		return hex.EncodeToString(b), err
	}
)

const (
	ownerFile  = "owner"
	markerFile = "marker"
	runPrefix  = "run-"
	claimTemp  = ".claim-"
)

func baseDir() string {
	return filepath.Join(os.TempDir(), "evolve-faketmux-"+strconv.Itoa(os.Getuid()))
}

func Claim() (string, error) { return claimIn(baseDir()) }

func Reap(root string) error { return reapIn(baseDir(), root) }

func claimIn(base string) (string, error) {
	token, err := baseToken(base)
	if err != nil {
		return "", err
	}
	if err := sweep(base, token); err != nil {
		return "", err
	}
	pid := os.Getpid()
	begun, err := processStart(pid)
	if err != nil {
		return "", fmt.Errorf("read the start of the owner: %w", err)
	}
	claim, err := mkdirTemp(base, claimTemp)
	if err == nil {
		err = os.WriteFile(filepath.Join(claim, ownerFile), []byte(fmt.Sprintf("%d %s %s", pid, begun, token)), 0o600)
	}
	if err != nil {
		return "", errors.Join(err, os.RemoveAll(claim))
	}
	root := filepath.Join(base, runPrefix+strings.TrimPrefix(filepath.Base(claim), claimTemp))
	return root, os.Rename(claim, root)
}

func baseToken(base string) (string, error) {
	if err := privateDir(base); err != nil {
		return "", err
	}
	path := filepath.Join(base, markerFile)
	token, err := newToken()
	if err == nil {
		err = publishOnce(path, token)
	}
	if err != nil {
		return "", fmt.Errorf("mark the fake tmux base: %w", err)
	}
	body, err := os.ReadFile(path)
	if err != nil || len(body) != len(token) {
		return "", fmt.Errorf("the fake tmux base %s has no valid marker %q: %v", base, body, err)
	}
	return string(body), nil
}

func publishOnce(path, content string) error {
	tmp := path + "-" + content
	if err := os.WriteFile(tmp, []byte(content), 0o600); err != nil {
		return err
	}
	defer func() { _ = os.Remove(tmp) }()
	return ignoreExist(os.Link(tmp, path))
}

func ignoreExist(err error) error {
	if errors.Is(err, fs.ErrExist) {
		return nil
	}
	return err
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

func sweep(base, token string) error {
	runs, err := os.ReadDir(base)
	if err != nil {
		return err
	}
	var errs []error
	for _, run := range runs {
		root := filepath.Join(base, run.Name())
		if owner, ok := ownerOf(root, token); ok && !startedAs(owner.pid, owner.begun) {
			if err := reapIn(base, root); !errors.Is(err, errRefused) {
				errs = append(errs, err)
			}
		}
	}
	return errors.Join(errs...)
}

var errRefused = errors.New("refuse to reap")

type ownerRecord struct {
	pid   int
	begun string
}

func ownerOf(root, token string) (ownerRecord, bool) {
	var rec ownerRecord
	var mark string
	body, err := os.ReadFile(filepath.Join(root, ownerFile))
	if err == nil {
		_, err = fmt.Sscanf(string(body), "%d %s %s", &rec.pid, &rec.begun, &mark)
	}
	return rec, err == nil && strings.HasPrefix(filepath.Base(root), runPrefix) && mark == token
}

func confinedRun(base, root string) error {
	switch {
	case root == "":
		return fmt.Errorf("%w an empty root", errRefused)
	case !filepath.IsAbs(root) || filepath.Clean(root) != root:
		return fmt.Errorf("%w %q: not a clean absolute path", errRefused, root)
	case filepath.Dir(root) != base || !strings.HasPrefix(filepath.Base(root), runPrefix):
		return fmt.Errorf("%w %s: not a run directory of the fake tmux base %s", errRefused, root, base)
	}
	if err := realDir(root); err != nil {
		return err
	}
	token, err := os.ReadFile(filepath.Join(base, markerFile))
	if _, ok := ownerOf(root, string(token)); err != nil || !ok {
		return fmt.Errorf("%w %s: its owner record does not carry the marker of %s", errRefused, root, base)
	}
	return nil
}

func realDir(path string) error {
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("%w %s: not a real directory", errRefused, path)
	}
	return nil
}

func reapIn(base, root string) error {
	if err := confinedRun(base, root); err != nil {
		return ignoreNotExist(err)
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		return ignoreNotExist(err)
	}
	var sockets []string
	for _, e := range entries {
		if e.Name() == ownerFile {
			continue
		}
		if !e.IsDir() || realDir(filepath.Join(root, e.Name(), "sessions")) != nil {
			return fmt.Errorf("%w %s: %s is not a fake tmux server", errRefused, root, e.Name())
		}
		sockets = append(sockets, filepath.Join(root, e.Name()))
	}
	var errs []error
	for _, dir := range sockets {
		errs = append(errs, (&server{dir: dir}).killServer(invocation{}))
	}
	return errors.Join(append(errs, os.Remove(filepath.Join(root, ownerFile)), os.Remove(root))...)
}
