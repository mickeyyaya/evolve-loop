package profiles

import (
	"path"
	"path/filepath"
	"strings"
)

// Denies reports whether a repo-relative path lies under a denied subpath, or as a
// directory spelling ("dir/") contains one; the deny list is the one the OS sandbox enforces.
func (s SandboxConfig) Denies(rel string) bool {
	p := cleanRepoPath(rel)
	if p == "" {
		return false
	}
	isDir := strings.HasSuffix(strings.TrimSpace(rel), "/")
	for _, raw := range s.DenySubpaths {
		d := cleanRepoPath(raw)
		switch {
		case d == "":
		case p == d, strings.HasPrefix(p, d+"/"):
			return true
		case isDir && strings.HasPrefix(d, p+"/"):
			return true
		}
	}
	return false
}

func cleanRepoPath(p string) string {
	p = strings.TrimSpace(p)
	if p == "" {
		return ""
	}
	if p = path.Clean(filepath.ToSlash(p)); p == "." {
		return ""
	}
	return p
}
