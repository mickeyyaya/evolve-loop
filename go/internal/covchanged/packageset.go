package covchanged

import (
	"fmt"
	"io/fs"
	"path"
	"strings"
)

type PackageSet map[string]bool

func ParsePackageSet(data []byte) (PackageSet, error) {
	set := PackageSet{}
	for n, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		dir := path.Clean(line)
		if !fs.ValidPath(dir) || dir == "." || strings.Contains(dir, "...") {
			return nil, fmt.Errorf("covchanged: package set line %d: %q is not a package directory under the module", n+1, line)
		}
		set[dir] = true
	}
	if len(set) == 0 {
		return nil, fmt.Errorf("covchanged: the package set names no package")
	}
	return set, nil
}

func (s PackageSet) Holds(rel string) bool { return s.HasDir(path.Dir(rel)) }

func (s PackageSet) HasDir(dir string) bool { return s == nil || s[dir] }
