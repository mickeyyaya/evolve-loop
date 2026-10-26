package dispatchcred

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridge/channel"
)

const (
	heldDir        = "held"
	credentialFile = "credential"
	stageFile      = "stage"
	tokenBytes     = 32
	tokenHexLen    = 2 * tokenBytes
	stageEnforce   = "enforce"
	stageOff       = "off"
)

var (
	root       string
	lookupID   = user.LookupId
	readRandom = rand.Read
)

type Store struct {
	Dir string
}

func Default() (Store, error) {
	if root != "" {
		return Store{Dir: root}, nil
	}
	uid := strconv.Itoa(os.Getuid())
	u, err := lookupID(uid)
	if err != nil {
		return Store{}, fmt.Errorf("dispatch credential: the passwd entry of uid %s: %w", uid, err)
	}
	if u.HomeDir == "" {
		return Store{}, fmt.Errorf("dispatch credential: uid %s has no home directory", uid)
	}
	return Store{Dir: filepath.Join(u.HomeDir, ".evolve", "dispatch")}, nil
}

func (s Store) HeldDir() string { return filepath.Join(s.Dir, heldDir) }

func (s Store) credentialPath() string { return filepath.Join(s.HeldDir(), credentialFile) }

func (s Store) Ensure(stage string) error {
	if err := s.ensureCredential(); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(s.Dir, stageFile+"-*")
	if err != nil {
		return fmt.Errorf("dispatch credential: write the stage: %w", err)
	}
	_, werr := tmp.WriteString(stage + "\n")
	cerr := tmp.Close()
	if err := errors.Join(werr, cerr, os.Chmod(tmp.Name(), 0o644), os.Rename(tmp.Name(), filepath.Join(s.Dir, stageFile))); err != nil {
		_ = os.Remove(tmp.Name())
		return fmt.Errorf("dispatch credential: write the stage: %w", err)
	}
	return nil
}

func (s Store) Hold() error {
	err := s.read()
	if !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	if werr := s.ensureCredential(); werr != nil {
		return werr
	}
	return s.read()
}

func (s Store) Stage() string {
	raw, err := os.ReadFile(filepath.Join(s.Dir, stageFile))
	if errors.Is(err, fs.ErrNotExist) {
		return channel.ResolveStage("")
	}
	word := strings.TrimSpace(string(raw))
	if err != nil || channel.ResolveStage(word) != word {
		return stageEnforce
	}
	return word
}

func (s Store) stageOfAMissedHold(cause error) string {
	_, err := os.Lstat(filepath.Join(s.Dir, stageFile))
	if errors.Is(err, fs.ErrNotExist) && errors.Is(cause, fs.ErrPermission) {
		return stageEnforce
	}
	return s.Stage()
}

func (s Store) read() error {
	path := s.credentialPath()
	f, err := os.OpenFile(path, os.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0)
	if err != nil {
		return fmt.Errorf("dispatch credential: %w", err)
	}
	info, err := f.Stat()
	if err == nil && (!info.Mode().IsRegular() || info.Mode().Perm()&0o077 != 0) {
		err = fmt.Errorf("%s is %v, want a 0600 regular file", path, info.Mode())
	}
	var raw []byte
	if err == nil {
		raw, err = io.ReadAll(io.LimitReader(f, 4*tokenHexLen))
	}
	err = errors.Join(err, f.Close())
	if err != nil {
		return fmt.Errorf("dispatch credential: %w", err)
	}
	if token := strings.TrimSpace(string(raw)); len(token) != tokenHexLen || !isHex(token) {
		return fmt.Errorf("dispatch credential: %s holds no token of the pipeline", path)
	}
	return nil
}

func isHex(s string) bool {
	_, err := hex.DecodeString(s)
	return err == nil
}

func (s Store) ensureCredential() error {
	if err := os.MkdirAll(s.HeldDir(), 0o700); err != nil {
		return fmt.Errorf("dispatch credential: create %s: %w", s.HeldDir(), err)
	}
	tmp, err := os.CreateTemp(s.HeldDir(), credentialFile+"-*")
	if err != nil {
		return fmt.Errorf("dispatch credential: create a credential in %s: %w", s.HeldDir(), err)
	}
	defer func() { _ = os.Remove(tmp.Name()) }()
	buf := make([]byte, tokenBytes)
	_, err = readRandom(buf)
	if err == nil {
		_, err = tmp.WriteString(hex.EncodeToString(buf) + "\n")
	}
	err = errors.Join(err, tmp.Close())
	if err == nil {
		err = os.Link(tmp.Name(), s.credentialPath())
	}
	if err != nil && !errors.Is(err, fs.ErrExist) {
		return fmt.Errorf("dispatch credential: write %s: %w", s.credentialPath(), err)
	}
	return nil
}

type Verdict struct {
	Held  bool
	Stage string
	Cause error
}

func (v Verdict) Violation() bool { return !v.Held && v.Stage != stageOff }

func (v Verdict) Refused() bool { return !v.Held && v.Stage == stageEnforce }

func Admit(open func() (Store, error)) Verdict {
	s, err := open()
	if err != nil {
		return Verdict{Stage: channel.ResolveStage(""), Cause: err}
	}
	if err := s.Hold(); err != nil {
		return Verdict{Stage: s.stageOfAMissedHold(err), Cause: err}
	}
	return Verdict{Held: true}
}
