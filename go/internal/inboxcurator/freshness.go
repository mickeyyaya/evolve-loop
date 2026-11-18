package inboxcurator

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/inboxbatch"
	"github.com/mickeyyaya/evolve-loop/go/internal/landedclaim"
)

var (
	shippedVerdictRE = regexp.MustCompile(`already-shipped: ([0-9a-f]{7,64})`)
	absentVerdictRE  = regexp.MustCompile(`stale: (.+?) absent at HEAD`)
	verdictTokenSep  = regexp.MustCompile(`[\s,;]+|\band\b`)
)

type Freshness struct {
	OnMain     landedclaim.Check
	PathExists func(string) bool
}

func routedReasonText(it inboxbatch.Item) string {
	var reason string
	if json.Unmarshal(it.RoutedReason, &reason) != nil {
		return ""
	}
	return reason
}

func (f Freshness) Stale(it inboxbatch.Item) string {
	reason := routedReasonText(it)
	if sha := shippedVerdictRE.FindStringSubmatch(reason); sha != nil && f.OnMain != nil && f.OnMain(sha[1], it.DeclaredPaths()) {
		return fmt.Sprintf("the lane verdict already-shipped %s is on main and touches the declared surface", sha[1])
	}
	m := absentVerdictRE.FindStringSubmatch(reason)
	if m == nil || f.PathExists == nil {
		return ""
	}
	named := f.resolveNamed(it, m[1])
	if len(named) == 0 {
		return ""
	}
	for _, p := range named {
		if f.PathExists(p) {
			return ""
		}
	}
	return "the lane verdict names " + strings.Join(named, ", ") + " absent at HEAD, and the tree still lacks every one of them"
}

func (f Freshness) resolveNamed(it inboxbatch.Item, text string) []string {
	var out []string
	for _, tok := range verdictTokenSep.Split(text, -1) {
		tok = strings.Trim(tok, "/.:()`")
		if tok == "" {
			continue
		}
		matched := false
		for _, declared := range it.DeclaredPaths() {
			d := strings.TrimSuffix(strings.TrimPrefix(declared, "./"), "/")
			if d == tok || strings.HasSuffix(d, "/"+tok) {
				out = append(out, d)
				matched = true
			}
		}
		if !matched && strings.Contains(tok, "/") {
			out = append(out, tok)
		}
	}
	return out
}
