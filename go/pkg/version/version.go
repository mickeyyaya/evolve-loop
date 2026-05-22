// Package version exposes the build-stamped binary identity.
//
// Values are set at link time via -ldflags '-X .version=… -X .commit=… -X .builtAt=…'
// (see Makefile). When unset, Get() falls back to runtime/debug.BuildInfo so
// `go run`, `go install`, and `go test` all return a useful string.
package version

import (
	"fmt"
	"runtime/debug"
	"strings"
)

// Linker-injected; do not read directly — call Get().
var (
	version = ""
	commit  = ""
	builtAt = ""
)

// Get returns a human-readable build identity.
//
// Preference order:
//  1. ldflag-injected values (release / make build path).
//  2. runtime/debug.BuildInfo VCS revision (go install path).
//  3. The literal "dev" / "unknown" (go run / go test path).
func Get() string {
	v, c, b := version, commit, builtAt
	if v == "" || c == "" {
		if info, ok := debug.ReadBuildInfo(); ok {
			if v == "" {
				v = info.Main.Version
			}
			for _, s := range info.Settings {
				if c == "" && s.Key == "vcs.revision" {
					c = shortSHA(s.Value)
				}
				if b == "" && s.Key == "vcs.time" {
					b = s.Value
				}
			}
		}
	}
	return formatVersion(v, c, b)
}

// formatVersion is the pure formatter — broken out so tests can pin the
// shape without depending on link-time injection or BuildInfo.
func formatVersion(v, c, b string) string {
	if v == "" {
		v = "dev"
	}
	c = shortSHA(c)
	if c == "" {
		c = "unknown"
	}
	if b == "" {
		return fmt.Sprintf("evolve %s (%s)", v, c)
	}
	return fmt.Sprintf("evolve %s (%s, built %s)", v, c, b)
}

// shortSHA trims whitespace and truncates to 12 chars (matches git's
// default short-sha length, and the Makefile's --short=12 flag).
func shortSHA(s string) string {
	const n = 12
	s = strings.TrimSpace(s)
	if len(s) > n {
		return s[:n]
	}
	return s
}
