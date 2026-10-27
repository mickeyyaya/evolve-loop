package main

import (
	"strconv"
	"strings"
)

const (
	cliExitOK          = 0
	cliExitFailed      = 1
	cliExitIO          = 2
	cliExitRefused     = 3
	cliExitQueued      = 4
	cliExitUsage       = 10
	cliExitInput       = 11
	cliExitInterrupted = 130
)

var cliExitKinds = map[int]string{
	cliExitOK:          "ok",
	cliExitFailed:      "failed",
	cliExitIO:          "io",
	cliExitRefused:     "refused",
	cliExitQueued:      "queued",
	cliExitUsage:       "usage",
	cliExitInput:       "input",
	cliExitInterrupted: "interrupted",
}

const (
	availAlways   = "always"
	availQueued   = "queued"
	availLeased   = "leased"
	availBoundary = "boundary"
)

type coreVerb struct {
	path   string
	class  string
	major  int
	stream bool
	codes  map[int]string
}

func (v coreVerb) schema() string {
	return "evolve." + strings.ReplaceAll(v.path, " ", ".") + "/" + strconv.Itoa(max(v.major, 1))
}

func (v coreVerb) exitKind(rc int) (string, bool) {
	if kind, ok := cliExitKinds[rc]; ok {
		return kind, true
	}
	kind, ok := v.codes[rc]
	return kind, ok && rc >= 21 && rc <= 99
}

var codeIndexRefreshCodes = map[int]string{codeExitIndexDirty: "index-dirty", codeExitIndexFetch: "index-fetch"}

var coreVerbs = []coreVerb{
	{path: "status", class: availAlways},
	{path: "loop status", class: availAlways},
	{path: "wave status", class: availAlways},
	{path: "wave supply", class: availAlways},
	{path: "wave watch", class: availAlways, stream: true},
	{path: "inbox list", class: availAlways},
	{path: "inbox show", class: availAlways},
	{path: "inbox rank", class: availAlways},
	{path: "inbox lint", class: availAlways},
	{path: "inbox batches", class: availAlways},
	{path: "signals tail", class: availAlways, stream: true},
	{path: "events channels", class: availAlways},
	{path: "events kinds", class: availAlways},
	{path: "events watch", class: availAlways, stream: true},
	{path: "ledger verify", class: availAlways},
	{path: "dossier verify", class: availAlways},
	{path: "dossier show", class: availAlways},
	{path: "cli-routing show", class: availAlways},
	{path: "cli-routing check", class: availAlways},
	{path: "cli-routing explain", class: availAlways},
	{path: "routing explain", class: availAlways},
	{path: "clihealth list", class: availAlways},
	{path: "clihealth usage", class: availAlways},
	{path: "agent list", class: availAlways},
	{path: "agent capabilities", class: availAlways},
	{path: "ci classify", class: availAlways},
	{path: "scan secrets", class: availAlways},
	{path: "docs ste-lint", class: availAlways},
	{path: "quality report", class: availLeased},
	{path: "quality compare", class: availLeased},
	{path: "quality show", class: availAlways},
	{path: "agent run", class: availLeased, codes: map[int]string{21: "dispatch"}},
	{path: "inbox claim", class: availLeased},
	{path: "inbox add", class: availLeased},
	{path: "inbox consume", class: availLeased},
	{path: "inbox edit", class: availQueued},
	{path: "inbox withdraw", class: availQueued},
	{path: "inbox verify", class: availQueued},
	{path: "inbox route-console", class: availQueued},
	{path: "inbox route-lane", class: availQueued},
	{path: "cli-routing set", class: availQueued},
	{path: "cli-routing unset", class: availQueued},
	{path: "cli-routing init", class: availQueued},
	{path: "cli-routing migrate", class: availQueued},
	{path: "wave note", class: availLeased},
	{path: "checkpoint save", class: availLeased},
	{path: "checkpoint list", class: availLeased},
	{path: "checkpoint restore", class: availLeased},
	{path: "checkpoint prune", class: availLeased},
	{path: "gc", class: availLeased},
	{path: "loop-stop", class: availLeased},
	{path: "wave stop", class: availLeased},
	{path: "wave next", class: availBoundary, major: 2, codes: codeIndexRefreshCodes},
	{path: "boundary run", class: availBoundary, codes: codeIndexRefreshCodes},
	{path: "pr merge", class: availBoundary},
	{path: "sync-main", class: availBoundary, codes: codeIndexRefreshCodes},
	{path: "cycle resume", class: availBoundary},
	{path: "cycle reset", class: availBoundary},
	{path: "cli update", class: availBoundary},
	{path: "loop", class: availBoundary},
}

func lookupCoreVerb(path string) (coreVerb, bool) {
	for _, v := range coreVerbs {
		if v.path == path {
			return v, true
		}
	}
	return coreVerb{}, false
}

func coreVerbSchema(path string) string {
	if v, ok := lookupCoreVerb(path); ok {
		return v.schema()
	}
	return ""
}
