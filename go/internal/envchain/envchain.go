// Package envchain encodes the canonical four-tier env-precedence chain used
// across evolve-loop.
package envchain

import "os"

func Resolve(key string, reqEnv map[string]string, profile, def string) string {
	if v, ok := reqEnv[key]; ok && v != "" {
		return v
	}
	if v := os.Getenv(key); v != "" {
		return v
	}
	if profile != "" {
		return profile
	}
	return def
}
