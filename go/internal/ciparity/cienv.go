package ciparity

import "strings"

var CIEnvAllowlist = []string{
	"PATH", "HOME", "TMPDIR", "USER", "SHELL",
	"GOROOT", "GOPATH", "GOCACHE", "GOMODCACHE", "GOFLAGS", "GOTOOLCHAIN", "CC",
}

func CIEnv(environ []string) []string {
	values := make(map[string]string, len(environ))
	for _, kv := range environ {
		if k, v, ok := strings.Cut(kv, "="); ok {
			values[k] = v
		}
	}
	env := make([]string, 0, len(CIEnvAllowlist))
	for _, k := range CIEnvAllowlist {
		if v, ok := values[k]; ok {
			env = append(env, k+"="+v)
		}
	}
	return env
}
