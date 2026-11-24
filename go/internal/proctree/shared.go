package proctree

import (
	"path/filepath"
	"slices"
	"strings"
)

type sharedMatch int

const (
	commIs sharedMatch = iota
	commHas
	argIs
	argThen
	progWith
)

type sharedRule struct {
	match  sharedMatch
	needle string
	next   string
}

var sharedHelpers = []sharedRule{
	{match: commIs, needle: "tmux"},
	{match: argThen, needle: "daemon", next: "run"},
	{match: argIs, needle: "bg-pty-host"},
	{match: argIs, needle: "bg-spare"},
	{match: progWith, needle: "ollama", next: "serve"},
	{match: commHas, needle: "chrome"},
	{match: commHas, needle: "chromium"},
}

func SharedHelper(p Process) bool {
	return slices.ContainsFunc(sharedHelpers, func(r sharedRule) bool { return r.matches(p) })
}

func (r sharedRule) matches(p Process) bool {
	names := programNames(p)
	switch r.match {
	case commIs:
		return slices.Contains(names, r.needle)
	case commHas:
		return slices.ContainsFunc(names, func(n string) bool { return strings.Contains(n, r.needle) })
	case argIs:
		return len(p.Args) > 1 && slices.Contains(p.Args[1:], r.needle)
	case progWith:
		return slices.Contains(names, r.needle) && len(p.Args) > 1 && slices.Contains(p.Args[1:], r.next)
	default:
		i := slices.Index(p.Args, r.needle)
		return i > 0 && i+1 < len(p.Args) && p.Args[i+1] == r.next
	}
}

func programNames(p Process) []string {
	names := []string{strings.ToLower(filepath.Base(p.Comm))}
	if len(p.Args) > 0 {
		names = append(names, strings.ToLower(filepath.Base(p.Args[0])))
	}
	return names
}

var launcherNames = []string{"node", "npx", "npm", "uvx", "uv", "python", "python3", "deno", "bun", "sh", "bash", "zsh"}

func Launchers() []string { return slices.Clone(launcherNames) }

func ProgramIn(names []string) Proof {
	return func(p Process) bool {
		return slices.ContainsFunc(allowNames(p), func(have string) bool {
			return slices.ContainsFunc(names, func(want string) bool { return versionOf(have, want) })
		})
	}
}

func versionOf(have, want string) bool {
	rest, ok := strings.CutPrefix(have, strings.ToLower(want))
	return ok && strings.Trim(rest, "0123456789.") == ""
}

func allowNames(p Process) []string {
	comm := strings.ToLower(filepath.Base(p.Comm))
	if len(p.Args) == 0 {
		return []string{comm}
	}
	arg0 := strings.ToLower(filepath.Base(p.Args[0]))
	if comm == "." || comm == "" || (comm != arg0 && strings.HasPrefix(arg0, comm)) {
		return []string{comm, arg0}
	}
	return []string{comm}
}

type Dispatch struct {
	ID       string
	Tree     []Identity
	Live     []Identity
	Killable Proof
}

func NotShared(p Process) bool { return !SharedHelper(p) }

func (d Dispatch) Owned() Proof {
	killable := d.Killable
	if killable == nil {
		killable = func(Process) bool { return false }
	}
	attached := AnyOf(NotShared, Recorded(d.Live))
	return AnyOf(Recorded(d.Tree).and(attached), TaggedWith(d.ID).and(killable).and(NotShared))
}
