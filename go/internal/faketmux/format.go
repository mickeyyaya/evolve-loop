package faketmux

import (
	"context"
	"fmt"
	"path/filepath"
	"regexp"
	"strconv"

	"github.com/mickeyyaya/evolve-loop/go/internal/proctree"
)

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
	case "window_width":
		return strconv.Itoa(sess.width()), nil
	case "window_height":
		return strconv.Itoa(sess.height()), nil
	case "window_activity":
		return sess.activity(), nil
	case "pane_current_command":
		table, err := s.procs(context.Background())
		if err != nil {
			return "", fmt.Errorf("list processes: %w", err)
		}
		return foreground(table, pid), nil
	}
	return "", nil
}

func foreground(table []proctree.Process, root int) string {
	comm := ""
	for cur := root; cur != 0; {
		next := 0
		for _, p := range table {
			if p.Pid == cur {
				comm = p.Comm
			}
			if p.Ppid == cur && p.Pid != cur && p.Pid > next {
				next = p.Pid
			}
		}
		cur = next
	}
	if comm == "" {
		return ""
	}
	return filepath.Base(comm)
}

var listProcesses = proctree.ExecLister()
