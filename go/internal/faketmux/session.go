package faketmux

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"github.com/mickeyyaya/evolve-loop/go/internal/proctree"
	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

const paneTerm = "TERM=screen"

var (
	openFile     = os.OpenFile
	signalGroup  = func(pid int) error { return syscall.Kill(-pid, syscall.SIGKILL) }
	setBlocking  = func(f *os.File) error { return syscall.SetNonblock(int(f.Fd()), false) }
	processStart = proctree.StartOf
)

type session struct {
	name, dir string
}

func (s session) path(file string) string { return filepath.Join(s.dir, file) }

func (s session) start(argv []string, workdir string, height int) error {
	if err := syscall.Mkfifo(s.path("in"), 0o600); err != nil {
		return fmt.Errorf("make the pane input: %w", err)
	}
	in, err := openFile(s.path("in"), os.O_RDWR, 0)
	if err != nil {
		return fmt.Errorf("open the pane input: %w", err)
	}
	defer func() { _ = in.Close() }()
	screen, err := openFile(s.path("screen"), os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0o600)
	if err != nil {
		return fmt.Errorf("open the pane screen: %w", err)
	}
	defer func() { _ = screen.Close() }()
	cmd := paneCommand(argv, workdir, in, screen)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start the pane: %w", err)
	}
	go func() { _ = cmd.Wait() }()
	pid := cmd.Process.Pid
	begun, err := processStart(pid)
	if err == nil {
		err = s.writeFile("pane", fmt.Sprintf("%d %d %s", pid, height, begun))
	}
	if err != nil {
		return errors.Join(fmt.Errorf("record the pane: %w", err), signalGroup(pid))
	}
	return nil
}

func paneCommand(argv []string, workdir string, in, screen *os.File) *exec.Cmd {
	cmd := sysexec.Command(context.Background(), argv[0], argv[1:]...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = in, screen, screen
	cmd.Dir = workdir
	cmd.Env = append(paneEnv(os.Environ()), paneTerm)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	return cmd
}

func paneEnv(env []string) []string {
	out := make([]string, 0, len(env))
	for _, kv := range env {
		name, _, _ := strings.Cut(kv, "=")
		if name != "TMUX" && name != "TMUX_PANE" && name != "TERM" {
			out = append(out, kv)
		}
	}
	return out
}

func (s session) writeFile(file, body string) error {
	tmp := s.path(file + ".tmp")
	if err := os.WriteFile(tmp, []byte(body), 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, s.path(file))
}

type paneRecord struct {
	pid, height int
	begun       string
}

func (s session) pane() (paneRecord, error) {
	var rec paneRecord
	body, err := os.ReadFile(s.path("pane"))
	if err != nil {
		return rec, err
	}
	if _, err := fmt.Sscanf(string(body), "%d %d %s", &rec.pid, &rec.height, &rec.begun); err != nil {
		return paneRecord{}, fmt.Errorf("pane record %q: %w", body, err)
	}
	if rec.pid <= 1 {
		return paneRecord{}, fmt.Errorf("pane record %q has no process", body)
	}
	return rec, nil
}

func startedAs(pid int, begun string) bool {
	now, err := processStart(pid)
	return err == nil && now == begun
}

func (s session) alive() bool {
	rec, err := s.pane()
	return err == nil && startedAs(rec.pid, rec.begun)
}

func (s session) kill() error {
	if rec, err := s.pane(); err == nil && startedAs(rec.pid, rec.begun) {
		if err := signalGroup(rec.pid); err != nil && !errors.Is(err, syscall.ESRCH) {
			return fmt.Errorf("kill session %s: %w", s.name, err)
		}
	}
	return os.RemoveAll(s.dir)
}

func (s session) feed(input, echo string) error {
	if err := appendFile(s.path("screen"), echo); err != nil {
		return err
	}
	if input == "" {
		return nil
	}
	in, err := openFile(s.path("in"), os.O_WRONLY|syscall.O_NONBLOCK, 0)
	if err != nil {
		return fmt.Errorf("session %s has no reader: %w", s.name, err)
	}
	if err := setBlocking(in); err != nil {
		return errors.Join(fmt.Errorf("block on the pane input: %w", err), in.Close())
	}
	return writeAndClose(in, input)
}

func appendFile(path, text string) error {
	if text == "" {
		return nil
	}
	f, err := openFile(path, os.O_WRONLY|os.O_APPEND, 0)
	if err != nil {
		return err
	}
	return writeAndClose(f, text)
}

func writeAndClose(f *os.File, text string) error {
	_, err := f.WriteString(text)
	return errors.Join(err, f.Close())
}

func (s session) capture(scrollback int) (string, error) {
	rec, err := s.pane()
	if err != nil {
		return "", err
	}
	body, err := os.ReadFile(s.path("screen"))
	if err != nil {
		return "", err
	}
	text := strings.ReplaceAll(strings.ReplaceAll(string(body), "\r\n", "\n"), "\r", "")
	lines := strings.Split(strings.TrimSuffix(text, "\n"), "\n")
	if rows := rec.height + scrollback; scrollback >= 0 && len(lines) > rows {
		lines = lines[len(lines)-rows:]
	}
	return strings.Join(lines, "\n") + "\n", nil
}

func (s session) optionPath(name string) string {
	return s.path("option-" + hex.EncodeToString([]byte(name)))
}

func (s session) writeOption(name, value string) error {
	return os.WriteFile(s.optionPath(name), []byte(value), 0o600)
}

func (s session) option(name string) (string, bool) {
	body, err := os.ReadFile(s.optionPath(name))
	return string(body), err == nil
}

func (s session) activity() string {
	info, err := os.Stat(s.path("screen"))
	if err != nil {
		return "0"
	}
	return strconv.FormatInt(info.ModTime().Unix(), 10)
}
