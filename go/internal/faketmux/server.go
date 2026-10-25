package faketmux

import (
	"cmp"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type server struct {
	dir    string
	stdout io.Writer
	procs  func() ([]process, error)
}

func (s *server) startServer(invocation) error {
	return os.MkdirAll(s.sessionsDir(), 0o700)
}

func (s *server) killServer(invocation) error {
	names, err := s.sessionNames()
	if err != nil {
		return err
	}
	for _, name := range names {
		if err := s.session(name).kill(); err != nil {
			return err
		}
	}
	return ignoreNotExist(errors.Join(os.RemoveAll(s.sessionsDir()), os.RemoveAll(s.buffersDir()), os.Remove(s.dir)))
}

func ignoreNotExist(err error) error {
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	return err
}

func (s *server) sessionsDir() string { return filepath.Join(s.dir, "sessions") }

func (s *server) session(name string) session {
	return session{name: name, dir: filepath.Join(s.sessionsDir(), name)}
}

func (s *server) sessionNames() ([]string, error) {
	entries, err := os.ReadDir(s.sessionsDir())
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var names []string
	for _, e := range entries {
		if s.session(e.Name()).alive() {
			names = append(names, e.Name())
		}
	}
	return names, nil
}

func (s *server) target(inv invocation) (session, error) {
	name := targetName(inv.flags['t'])
	if name == "" {
		return session{}, errors.New("no target session")
	}
	if !validName(name) {
		return session{}, fmt.Errorf("bad session name %q", name)
	}
	sess := s.session(name)
	if !sess.alive() {
		return session{}, fmt.Errorf("can't find session: %s", name)
	}
	return sess, nil
}

func (s *server) newSession(inv invocation) error {
	name := inv.flags['s']
	if !validName(name) {
		return fmt.Errorf("bad session name %q", name)
	}
	height, err := strconv.Atoi(cmp.Or(inv.flags['y'], "24"))
	if err != nil {
		return fmt.Errorf("bad height: %w", err)
	}
	sess := s.session(name)
	if sess.alive() {
		return fmt.Errorf("duplicate session: %s", name)
	}
	if err := errors.Join(os.RemoveAll(sess.dir), os.MkdirAll(sess.dir, 0o700)); err != nil {
		return err
	}
	return sess.start(paneArgv(inv.args), inv.flags['c'], height)
}

func validName(name string) bool {
	return name != "" && !strings.ContainsAny(name, `/\:.`)
}

func paneArgv(args []string) []string {
	switch len(args) {
	case 0:
		return []string{"/bin/sh"}
	case 1:
		return []string{"/bin/sh", "-c", args[0]}
	}
	return args
}

func (s *server) hasSession(inv invocation) error {
	_, err := s.target(inv)
	return err
}

func (s *server) killSession(inv invocation) error {
	sess, err := s.target(inv)
	if err != nil {
		return err
	}
	return sess.kill()
}

func (s *server) setOption(inv invocation) error {
	if inv.flags['t'] == "" {
		return nil
	}
	sess, err := s.target(inv)
	if err != nil {
		return err
	}
	if len(inv.args) != 2 {
		return fmt.Errorf("set-option wants a name and a value, got %q", inv.args)
	}
	return sess.writeOption(inv.args[0], inv.args[1])
}

func (s *server) showOptions(inv invocation) error {
	sess, err := s.target(inv)
	if err != nil {
		return err
	}
	if len(inv.args) != 1 {
		return fmt.Errorf("show-options wants one name, got %q", inv.args)
	}
	value, ok := sess.option(inv.args[0])
	if !ok {
		return nil
	}
	_, err = fmt.Fprintln(s.stdout, value)
	return err
}

func (s *server) sendKeys(inv invocation) error {
	sess, err := s.target(inv)
	if err != nil {
		return err
	}
	_, literal := inv.flags['l']
	input, echo := translateKeys(inv.args, literal)
	return sess.feed(input, echo)
}

func (s *server) loadBuffer(inv invocation) error {
	if len(inv.args) != 1 {
		return fmt.Errorf("load-buffer wants one file, got %q", inv.args)
	}
	body, err := os.ReadFile(inv.args[0])
	if err != nil {
		return err
	}
	if err := os.MkdirAll(s.buffersDir(), 0o700); err != nil {
		return err
	}
	return os.WriteFile(s.bufferPath(inv.flags['b']), body, 0o600)
}

func (s *server) buffersDir() string { return filepath.Join(s.dir, "buffers") }

func (s *server) bufferPath(name string) string {
	return filepath.Join(s.buffersDir(), "buffer-"+hex.EncodeToString([]byte(cmp.Or(name, "0"))))
}

func (s *server) pasteBuffer(inv invocation) error {
	sess, err := s.target(inv)
	if err != nil {
		return err
	}
	path := s.bufferPath(inv.flags['b'])
	body, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("no buffer %s: %w", inv.flags['b'], err)
	}
	if err := sess.feed(string(body), string(body)); err != nil {
		return err
	}
	if _, del := inv.flags['d']; del {
		return os.Remove(path)
	}
	return nil
}

func (s *server) capturePane(inv invocation) error {
	sess, err := s.target(inv)
	if err != nil {
		return err
	}
	scrollback, err := scrollbackOf(inv.flags['S'])
	if err != nil {
		return err
	}
	screen, err := sess.capture(scrollback)
	if err != nil {
		return err
	}
	_, err = io.WriteString(s.stdout, screen)
	return err
}

func scrollbackOf(start string) (int, error) {
	switch start {
	case "":
		return 0, nil
	case "-":
		return -1, nil
	}
	n, err := strconv.Atoi(start)
	if err != nil {
		return 0, fmt.Errorf("bad start line %q: %w", start, err)
	}
	return max(-n, 0), nil
}

func (s *server) displayMessage(inv invocation) error {
	sess, err := s.target(inv)
	if err != nil {
		return err
	}
	if len(inv.args) != 1 {
		return fmt.Errorf("display-message wants one format, got %q", inv.args)
	}
	return s.printFormat(sess, inv.args[0])
}

func (s *server) listPanes(inv invocation) error {
	sess, err := s.target(inv)
	if err != nil {
		return err
	}
	return s.printFormat(sess, cmp.Or(inv.flags['F'], "#{pane_pid}"))
}

func (s *server) listSessions(inv invocation) error {
	names, err := s.sessionNames()
	if err != nil {
		return err
	}
	if len(names) == 0 {
		return fmt.Errorf("no server running on %s", s.dir)
	}
	for _, name := range names {
		if err := s.printFormat(s.session(name), cmp.Or(inv.flags['F'], "#{session_name}")); err != nil {
			return err
		}
	}
	return nil
}

func (s *server) resizeWindow(inv invocation) error {
	_, err := s.target(inv)
	return err
}

func (s *server) printFormat(sess session, format string) error {
	line, err := s.expand(sess, format)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(s.stdout, line)
	return err
}
