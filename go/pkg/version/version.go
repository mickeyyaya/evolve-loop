// Package version exposes the build-stamped binary identity.
package version

import (
	"fmt"
	"runtime/debug"
	"strings"
)

var (
	version = ""
	commit  = ""
	builtAt = ""
)

var readBuildInfo = func() (*debug.BuildInfo, bool) { return debug.ReadBuildInfo() }

func Get() string {
	return composeVersion(version, commit, builtAt, readBuildInfo)
}

func Commit() string {
	return composeCommit(commit, readBuildInfo)
}

func composeCommit(c string, readBI func() (*debug.BuildInfo, bool)) string {
	if c == "" {
		if info, ok := readBI(); ok {
			for _, s := range info.Settings {
				if s.Key == "vcs.revision" {
					c = s.Value
					break
				}
			}
		}
	}
	return shortSHA(c)
}

func composeVersion(v, c, b string, readBI func() (*debug.BuildInfo, bool)) string {
	if v == "" || c == "" || b == "" {
		if info, ok := readBI(); ok {
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

func shortSHA(s string) string {
	const n = 12
	s = strings.TrimSpace(s)
	if len(s) > n {
		return s[:n]
	}
	return s
}
