package inboxsplit

import (
	"fmt"
	"path"
	"regexp"
	"strings"
)

type owner int

const (
	ownerNone owner = iota
	ownerLane
	ownerConsole
)

var wordToken = regexp.MustCompile(`[A-Za-z0-9_@.-]+`)

type surface struct {
	isProtected  func(string) bool
	consoleOwned map[string]bool
}

func newSurface(isProtected func(string) bool, console, lane []string) surface {
	owned := map[string]bool{}
	note := func(entries []string, isConsole bool) {
		for _, entry := range entries {
			for _, p := range declaredPaths(entry) {
				owned[strings.TrimSuffix(p, "/")] = isConsole
				if base := path.Base(p); !strings.HasSuffix(p, "/") && path.Ext(base) != "" {
					owned[base] = isConsole
				}
			}
		}
	}
	note(lane, false)
	note(console, true)
	return surface{isProtected: isProtected, consoleOwned: owned}
}

func (s surface) ownerOf(name string) owner {
	isConsole, listed := s.consoleOwned[strings.TrimSuffix(name, "/")]
	switch {
	case !listed:
		return ownerNone
	case isConsole:
		return ownerConsole
	}
	return ownerLane
}

func (s surface) classify(text string) (owner, string) {
	found := ownerNone
	for _, name := range tokensOf(text) {
		if strings.Contains(name, "/") && s.isProtected(name) {
			return ownerConsole, name
		}
		switch s.ownerOf(name) {
		case ownerConsole:
			return ownerConsole, name
		case ownerLane:
			found = ownerLane
		}
	}
	return found, ""
}

func tokensOf(text string) []string {
	var out []string
	for _, re := range []*regexp.Regexp{pathToken, wordToken} {
		for _, tok := range re.FindAllString(text, -1) {
			out = append(out, strings.TrimRight(tok, "."))
		}
	}
	return out
}

func (s surface) divide(item Item, criteria []string) (console, lane []string, err error) {
	if name, named := s.consoleNamedBy(item); named {
		return nil, nil, fmt.Errorf("%w: %s", ErrFixNamesProtected, name)
	}
	console, lane = s.splitCriteria(criteria)
	switch {
	case len(lane) == 0:
		return nil, nil, fmt.Errorf("%w: the lane part", ErrNoCriterion)
	case len(console) == 0:
		return nil, nil, fmt.Errorf("%w: the console part", ErrNoCriterion)
	}
	return console, lane, nil
}

func (s surface) consoleNamedBy(item Item) (string, bool) {
	for _, field := range textFields {
		for _, text := range textsOf(item[field]) {
			if who, name := s.classify(text); who == ownerConsole {
				return name, true
			}
		}
	}
	return "", false
}

func (s surface) splitCriteria(criteria []string) (console, lane []string) {
	for _, criterion := range criteria {
		switch who, _ := s.classify(criterion); who {
		case ownerConsole:
			console = append(console, criterion)
		case ownerLane:
			lane = append(lane, criterion)
		default:
			console, lane = append(console, criterion), append(lane, criterion)
		}
	}
	return console, lane
}
