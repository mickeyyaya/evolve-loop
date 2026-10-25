package ipcenv

import (
	"os"
	"strings"
)

const namespace = "EVOLVE_"

func Scrub(environ []string) []string {
	out := make([]string, 0, len(environ))
	for _, kv := range environ {
		if strings.HasPrefix(kv, namespace) {
			continue
		}
		out = append(out, kv)
	}
	return out
}

func GoTestEnv(extra ...string) []string {
	return append(Scrub(os.Environ()), extra...)
}

func Keep(environ []string, allowed func(name string) bool) []string {
	out := make([]string, 0, len(environ))
	for _, kv := range environ {
		if name, _, ok := strings.Cut(kv, "="); ok && name != "" && allowed(name) {
			out = append(out, kv)
		}
	}
	return out
}
