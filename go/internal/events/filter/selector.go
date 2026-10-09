package filter

import (
	"fmt"
	"regexp"
	"strings"
)

var channelTokenRE = regexp.MustCompile(`^[a-z0-9_-]+$`)

func ResolveChannels(selectors, channels []string) ([]string, error) {
	if len(selectors) == 0 {
		return nil, fmt.Errorf("%w: no channel selector", ErrUsage)
	}
	selected := map[string]bool{}
	for _, sel := range selectors {
		tokens, err := selectorTokens(sel)
		if err != nil {
			return nil, err
		}
		matched := false
		for _, ch := range channels {
			if tokensMatch(tokens, strings.Split(ch, ".")) {
				selected[ch], matched = true, true
			}
		}
		if !matched {
			return nil, fmt.Errorf("%w: selector %q matches no channel", ErrRefused, sel)
		}
	}
	var out []string
	for _, ch := range channels {
		if selected[ch] {
			out = append(out, ch)
		}
	}
	return out, nil
}

func selectorTokens(sel string) ([]string, error) {
	tokens := strings.Split(sel, ".")
	for i, tok := range tokens {
		isLast := i == len(tokens)-1
		if channelTokenRE.MatchString(tok) || tok == "*" || (tok == ">" && isLast) {
			continue
		}
		return nil, fmt.Errorf("%w: selector %q is not a channel name or pattern", ErrUsage, sel)
	}
	return tokens, nil
}

func tokensMatch(sel, name []string) bool {
	for i, tok := range sel {
		if tok == ">" {
			return len(name) > i
		}
		if i >= len(name) || (tok != "*" && tok != name[i]) {
			return false
		}
	}
	return len(sel) == len(name)
}

func ValidChannelName(name string) bool {
	for _, tok := range strings.Split(name, ".") {
		if !channelTokenRE.MatchString(tok) {
			return false
		}
	}
	return true
}
