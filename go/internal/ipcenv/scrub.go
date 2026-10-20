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
