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

func OwnedByDispatch(id string, tree, live []Identity) Proof {
	private := func(p Process) bool { return !SharedHelper(p) }
	attached := AnyOf(private, Recorded(live))
	return AnyOf(Recorded(tree).and(attached), TaggedWith(id).and(private))
}
