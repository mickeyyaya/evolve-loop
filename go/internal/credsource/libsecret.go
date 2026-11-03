package credsource

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

const fileMode fs.FileMode = 0o600

type libsecretOrFile struct {
	run  Runner
	home func() (string, error)
	read func(string) ([]byte, error)
}

func (libsecretOrFile) Name() string { return "libsecret+file" }

func (l libsecretOrFile) Lookup(ctx context.Context, host string) (Answer, error) {
	out, err := l.run(ctx, "secret-tool", "lookup", "service", Service, "host", host)
	if err == nil {
		if answer, empty := secretAnswer("libsecret", out); empty == nil {
			return answer, nil
		}
	}
	answer, ferr := l.file(host)
	if ferr == nil {
		return answer, nil
	}
	if err != nil {
		return Answer{}, errors.Join(fmt.Errorf("libsecret: %w", err), ferr)
	}
	return Answer{}, ferr
}

func (l libsecretOrFile) file(host string) (Answer, error) {
	home, err := l.home()
	if err != nil {
		return Answer{}, fmt.Errorf("file: %w", err)
	}
	path := filepath.Join(home, ".config", Service, "credentials", host)
	info, err := os.Lstat(path)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return Answer{}, ErrNoCredential
	case err != nil:
		return Answer{}, fmt.Errorf("file: %w", err)
	case !info.Mode().IsRegular():
		return Answer{}, fmt.Errorf("file: %s is not a regular file", path)
	case info.Mode().Perm() != fileMode:
		return Answer{}, fmt.Errorf("file: %s has mode %04o, want %04o", path, info.Mode().Perm(), fileMode)
	}
	raw, err := l.read(path)
	if err != nil {
		return Answer{}, fmt.Errorf("file: %w", err)
	}
	return secretAnswer("file", string(raw))
}
