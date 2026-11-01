package main

import (
	"slices"
	"strings"
)

const blockedByFlag = "--blocked-by"

func takeBlockedBy(args []string) (string, []string, bool) {
	i := slices.Index(args, blockedByFlag)
	if i < 0 {
		return "", args, true
	}
	if i+1 >= len(args) {
		return "", nil, false
	}
	blocker := strings.TrimSpace(args[i+1])
	return blocker, slices.Concat(args[:i], args[i+2:]), blocker != ""
}
