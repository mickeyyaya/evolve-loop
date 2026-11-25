// Package promptboundary owns the line that splits a composed prompt into a
// byte-stable cacheable prefix and a per-cycle suffix. It imports nothing
// from the repo. See docs/architecture/packages/internal-promptboundary.md.
package promptboundary

import "strings"

const Boundary = "\n\n## Cycle Context\n"

func boundaryIndex(prompt string) int {
	return strings.Index(prompt, Boundary)
}

func StaticPrefix(prompt string) string {
	if i := boundaryIndex(prompt); i >= 0 {
		return prompt[:i]
	}
	return prompt
}

func Place(prompt, block string) string {
	if block == "" {
		return prompt
	}
	i := boundaryIndex(prompt)
	if i < 0 {
		return prompt + Boundary + block
	}
	cut := i + len(Boundary)
	return prompt[:cut] + block + prompt[cut:]
}
