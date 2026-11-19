package bridge

import (
	"context"
	"fmt"
	"io"
	"strconv"
	"strings"
)

type modelGit struct {
	commits []string
}

func newModelGit(commits ...string) CmdRunner {
	m := &modelGit{commits: commits}
	return m.run
}

func (m *modelGit) sha(i int) string {
	return fmt.Sprintf("%040x", i+1)
}

func (m *modelGit) resolve(ref string) int {
	n := len(m.commits)
	if ref == "HEAD" && n > 0 {
		return n - 1
	}
	if rest, ok := strings.CutPrefix(ref, "HEAD~"); ok {
		back, err := strconv.Atoi(rest)
		if err == nil && n-1-back >= 0 {
			return n - 1 - back
		}
		return -1
	}
	for i := range m.commits {
		if ref == m.sha(i) {
			return i
		}
	}
	return -1
}

func (m *modelGit) run(_ context.Context, name, _ string, args, _ []string, _ io.Reader, stdout, stderr io.Writer) (int, error) {
	if name != "git" || len(args) < 3 || args[0] != "-C" {
		return 0, nil
	}
	sub, rest := args[2], args[3:]
	switch sub {
	case "rev-parse":
		return m.revParse(rest, stdout, stderr)
	case "rev-list":
		return m.revList(rest, stdout, stderr)
	case "log":
		return m.log(rest, stdout, stderr)
	}
	return 0, nil
}

func (m *modelGit) fatal(stderr io.Writer, what string) (int, error) {
	_, _ = io.WriteString(stderr, "fatal: "+what+"\n")
	return 128, nil
}

func (m *modelGit) revParse(rest []string, stdout, stderr io.Writer) (int, error) {
	ref := rest[len(rest)-1]
	verify := len(rest) > 1
	i := m.resolve(ref)
	switch {
	case i >= 0:
		_, _ = io.WriteString(stdout, m.sha(i)+"\n")
		return 0, nil
	case verify:
		return 1, nil
	}
	_, _ = io.WriteString(stdout, ref+"\n")
	return m.fatal(stderr, "ambiguous argument '"+ref+"': unknown revision")
}

func (m *modelGit) revList(rest []string, stdout, stderr io.Writer) (int, error) {
	base, head, _ := strings.Cut(rest[0], "..")
	from, to := m.resolve(base), m.resolve(head)
	if from < 0 || to < 0 {
		return m.fatal(stderr, "bad revision '"+rest[0]+"'")
	}
	for i := to; i > from; i-- {
		_, _ = io.WriteString(stdout, m.sha(i)+"\n")
	}
	return 0, nil
}

func (m *modelGit) log(rest []string, stdout, stderr io.Writer) (int, error) {
	rev := rest[len(rest)-1]
	i := m.resolve(rev)
	if i < 0 {
		return m.fatal(stderr, "bad revision '"+rev+"'")
	}
	_, _ = io.WriteString(stdout, m.commits[i]+"\n")
	return 0, nil
}
