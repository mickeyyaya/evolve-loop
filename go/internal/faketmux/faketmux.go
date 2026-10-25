package faketmux

import (
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strings"
)

const Flag = "--fake-tmux"

const defaultSocket = "default"

type invocation struct {
	flags map[byte]string
	args  []string
}

type command struct {
	valued string
	run    func(*server, invocation) error
}

var commands = map[string]command{
	"start-server":    {run: (*server).startServer},
	"kill-server":     {run: (*server).killServer},
	"new-session":     {valued: "cstxyFn", run: (*server).newSession},
	"has-session":     {valued: "t", run: (*server).hasSession},
	"kill-session":    {valued: "t", run: (*server).killSession},
	"set-option":      {valued: "t", run: (*server).setOption},
	"show-options":    {valued: "t", run: (*server).showOptions},
	"send-keys":       {valued: "tN", run: (*server).sendKeys},
	"load-buffer":     {valued: "bt", run: (*server).loadBuffer},
	"paste-buffer":    {valued: "bst", run: (*server).pasteBuffer},
	"capture-pane":    {valued: "bEStN", run: (*server).capturePane},
	"display-message": {valued: "cdtF", run: (*server).displayMessage},
	"list-panes":      {valued: "tFf", run: (*server).listPanes},
	"list-sessions":   {valued: "Ff", run: (*server).listSessions},
	"resize-window":   {valued: "txy", run: (*server).resizeWindow},
}

func Run(root string, args []string, stdout, stderr io.Writer) int {
	if err := run(root, args, stdout); err != nil {
		fmt.Fprintf(stderr, "faketmux: %v\n", err)
		return 1
	}
	return 0
}

func run(root string, args []string, stdout io.Writer) error {
	if root == "" {
		return errors.New("no state root")
	}
	socket, rest, err := globalFlags(args)
	if err != nil {
		return err
	}
	s := &server{dir: filepath.Join(root, socket), stdout: stdout, procs: listProcesses}
	for _, words := range splitCommands(rest) {
		if err := s.exec(words); err != nil {
			return err
		}
	}
	return nil
}

func globalFlags(args []string) (string, []string, error) {
	socket := defaultSocket
	for len(args) > 0 && strings.HasPrefix(args[0], "-") {
		if len(args) < 2 || (args[0] != "-L" && args[0] != "-f") {
			return "", nil, fmt.Errorf("unsupported global flag %q", args[0])
		}
		if args[0] == "-L" {
			socket = args[1]
		}
		args = args[2:]
	}
	if socket == "" || strings.ContainsAny(socket, `/\`) || socket == "." || socket == ".." {
		return "", nil, fmt.Errorf("bad socket name %q", socket)
	}
	return socket, args, nil
}

func splitCommands(args []string) [][]string {
	var out [][]string
	var cur []string
	for _, a := range append(args, ";") {
		if a != ";" && a != `\;` {
			cur = append(cur, a)
			continue
		}
		if len(cur) > 0 {
			out = append(out, cur)
		}
		cur = nil
	}
	return out
}

func (s *server) exec(words []string) error {
	cmd, ok := commands[words[0]]
	if !ok {
		return fmt.Errorf("unknown command: %s", words[0])
	}
	inv, err := parseInvocation(words[1:], cmd.valued)
	if err != nil {
		return fmt.Errorf("%s: %w", words[0], err)
	}
	return cmd.run(s, inv)
}

func parseInvocation(words []string, valued string) (invocation, error) {
	inv := invocation{flags: map[byte]string{}}
	for i := 0; i < len(words); i++ {
		w := words[i]
		if w == "--" {
			inv.args = words[i+1:]
			return inv, nil
		}
		if len(w) < 2 || w[0] != '-' {
			inv.args = words[i:]
			return inv, nil
		}
		next, err := parseFlagCluster(inv.flags, w[1:], valued, words[i+1:])
		if err != nil {
			return inv, err
		}
		i += next
	}
	return inv, nil
}

func parseFlagCluster(flags map[byte]string, cluster, valued string, rest []string) (int, error) {
	for j := 0; j < len(cluster); j++ {
		c := cluster[j]
		if !strings.ContainsRune(valued, rune(c)) {
			flags[c] = ""
			continue
		}
		if j+1 < len(cluster) {
			flags[c] = cluster[j+1:]
			return 0, nil
		}
		if len(rest) == 0 {
			return 0, fmt.Errorf("flag -%c needs a value", c)
		}
		flags[c] = rest[0]
		return 1, nil
	}
	return 0, nil
}

func targetName(target string) string {
	name := strings.TrimPrefix(target, "=")
	name, _, _ = strings.Cut(name, ":")
	return name
}
