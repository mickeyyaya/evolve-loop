// Package coversplit plans the coverage runs of a test package that was split from the package it tests.
// See docs/architecture/packages/internal-coversplit.md.
package coversplit

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
)

const File = ".cover-split"

type Split struct {
	Package, Target string
}

type Run struct {
	Packages []string
	CoverPkg string
}

func Parse(data []byte) ([]Split, error) {
	var splits []Split
	seen := map[string]bool{}
	for n, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) != 2 || !isPackageDir(fields[0]) || !isPackageDir(fields[1]) || fields[0] == fields[1] {
			return nil, fmt.Errorf("coversplit: %s line %d: %q is not `./<split package> ./<package it tests>`", File, n+1, line)
		}
		if seen[fields[0]] {
			return nil, fmt.Errorf("coversplit: %s line %d: %s is listed twice", File, n+1, fields[0])
		}
		seen[fields[0]] = true
		splits = append(splits, Split{Package: fields[0], Target: fields[1]})
	}
	return splits, nil
}

func isPackageDir(p string) bool {
	rel, ok := strings.CutPrefix(p, "./")
	return ok && fs.ValidPath(rel) && rel != "." && path.Clean(rel) == rel && !strings.Contains(rel, "...")
}

func Load(moduleDir string) ([]Split, error) {
	data, err := os.ReadFile(filepath.Join(moduleDir, File))
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("coversplit: read %s: %w", File, err)
	}
	return Parse(data)
}

func names(pkg, dir string) bool {
	return strings.HasSuffix(pkg, "/"+strings.TrimPrefix(dir, "./"))
}

func selected(pkgs []string, dir string) bool {
	for _, p := range pkgs {
		if names(p, dir) {
			return true
		}
	}
	return false
}

func Plan(pkgs []string, splits []Split) []Run {
	var active []Split
	for _, s := range splits {
		if selected(pkgs, s.Target) {
			active = append(active, s)
		}
	}
	var rest []string
	for _, p := range pkgs {
		if !splitOf(p, active) {
			rest = append(rest, p)
		}
	}
	var runs []Run
	if len(rest) > 0 {
		runs = append(runs, Run{Packages: rest})
	}
	for _, s := range active {
		runs = append(runs, Run{Packages: []string{s.Package}, CoverPkg: s.Target + "," + s.Package})
	}
	return runs
}

func splitOf(pkg string, active []Split) bool {
	for _, s := range active {
		if names(pkg, s.Package) {
			return true
		}
	}
	return false
}

func (r Run) Args() []string {
	if r.CoverPkg == "" {
		return r.Packages
	}
	return append([]string{"-coverpkg=" + r.CoverPkg}, r.Packages...)
}

func Merge(profiles [][]byte) ([]byte, error) {
	if len(profiles) == 0 {
		return nil, errors.New("coversplit: no coverage profile to merge")
	}
	var out bytes.Buffer
	var mode string
	for i, p := range profiles {
		head, body, _ := bytes.Cut(p, []byte("\n"))
		if !bytes.HasPrefix(head, []byte("mode: ")) {
			return nil, fmt.Errorf("coversplit: coverage profile %d does not start with a mode line", i+1)
		}
		if i == 0 {
			mode = string(head)
			out.Write(head)
			out.WriteByte('\n')
		} else if string(head) != mode {
			return nil, fmt.Errorf("coversplit: coverage profile %d has %q, want %q", i+1, head, mode)
		}
		out.Write(body)
		if len(body) > 0 && !bytes.HasSuffix(body, []byte("\n")) {
			out.WriteByte('\n')
		}
	}
	return out.Bytes(), nil
}
