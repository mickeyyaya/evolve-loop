// Package goalhash computes a stable hash of a /evo:loop goal string.
package goalhash

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"unicode"
)

func Normalize(raw string) string {
	lower := strings.ToLower(raw)
	var b strings.Builder
	b.Grow(len(lower))
	prevSpace := false
	for _, r := range lower {
		if unicode.IsSpace(r) {
			if !prevSpace {
				b.WriteByte(' ')
				prevSpace = true
			}
			continue
		}
		b.WriteRune(r)
		prevSpace = false
	}
	return strings.TrimSpace(b.String())
}

func Compute(goal string) string {
	sum := sha256.Sum256([]byte(Normalize(goal)))
	return hex.EncodeToString(sum[:])
}

func Short(goal string) string {
	return Compute(goal)[:8]
}
