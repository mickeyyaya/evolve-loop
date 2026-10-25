package faketmux

import (
	"context"
	"fmt"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

type process struct {
	pid, ppid int
	comm      string
}

var formatVar = regexp.MustCompile(`#\{([a-z_]+)\}`)

func (s *server) expand(sess session, format string) (string, error) {
	rec, err := sess.pane()
	if err != nil {
		return "", err
	}
	var ferr error
	line := formatVar.ReplaceAllStringFunc(format, func(v string) string {
		value, err := s.variable(sess, rec.pid, formatVar.FindStringSubmatch(v)[1])
		if err != nil && ferr == nil {
			ferr = err
		}
		return value
	})
	return line, ferr
}

func (s *server) variable(sess session, pid int, name string) (string, error) {
	switch name {
	case "session_name":
		return sess.name, nil
	case "pane_pid":
		return strconv.Itoa(pid), nil
	case "window_activity":
		return sess.activity(), nil
	case "pane_current_command":
		table, err := s.procs()
		if err != nil {
			return "", fmt.Errorf("list processes: %w", err)
		}
		return foreground(table, pid), nil
	}
	return "", nil
}

func foreground(table []process, root int) string {
	comm := ""
	for cur := root; cur != 0; {
		next := 0
		for _, p := range table {
			if p.pid == cur {
				comm = p.comm
			}
			if p.ppid == cur && p.pid != cur && p.pid > next {
				next = p.pid
			}
		}
		cur = next
	}
	if comm == "" {
		return ""
	}
	return filepath.Base(comm)
}

var runPS = func() (string, error) {
	return sysexec.Output(context.Background(), sysexec.DefaultRunner, "", "ps", "-A", "-o", "pid=", "-o", "ppid=", "-o", "comm=")
}

func listProcesses() ([]process, error) {
	out, err := runPS()
	if err != nil {
		return nil, err
	}
	return parseProcesses(out)
}

func parseProcesses(out string) ([]process, error) {
	var table []process
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		f := strings.Fields(line)
		if len(f) < 3 {
			return nil, fmt.Errorf("ps line %q", line)
		}
		pid, perr := strconv.Atoi(f[0])
		ppid, qerr := strconv.Atoi(f[1])
		if perr != nil || qerr != nil {
			return nil, fmt.Errorf("ps line %q", line)
		}
		table = append(table, process{pid: pid, ppid: ppid, comm: strings.Join(f[2:], " ")})
	}
	return table, nil
}
