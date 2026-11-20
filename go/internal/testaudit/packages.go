package testaudit

import (
	"errors"
	"io/fs"
	"path/filepath"
	"slices"
	"strings"
)

func CheckPackages(root string, pkgs []string, cfg Config) error {
	base := cleanPath(root)
	var problems []error
	for _, pkg := range normalizePackages(pkgs) {
		if err := checkPackage(base, pkg, cfg); err != nil {
			problems = append(problems, err)
		}
	}
	return errors.Join(problems...)
}

func checkPackage(base, pkg string, cfg Config) error {
	found, err := packageFindings(base, pkg, cfg)
	if err != nil {
		return err
	}
	baseline, err := packageBaseline(base, pkg)
	if err != nil {
		return err
	}
	return diffFindings(found, baseline)
}

func normalizePackages(pkgs []string) []string {
	var out []string
	for _, p := range pkgs {
		if p == "" {
			p = "."
		}
		p = filepath.ToSlash(filepath.Clean(p))
		if !skippedPackage(p) && !slices.Contains(out, p) {
			out = append(out, p)
		}
	}
	slices.Sort(out)
	return out
}

func skippedPackage(pkg string) bool {
	if pkg == "." {
		return false
	}
	for _, seg := range strings.Split(pkg, "/") {
		if skippedDir(seg) {
			return true
		}
	}
	return false
}

func packageFindings(base, pkg string, cfg Config) ([]Finding, error) {
	p, err := loadPackage(resolvePkgDir(base, pkg))
	if errors.Is(err, fs.ErrNotExist) {
		p, err = &pkgInfo{}, nil
	}
	if err != nil {
		return nil, err
	}
	p.dir = pkg
	found := auditPackage(p, cfg)
	for _, row := range cfg.Fakes {
		if row.SuiteDir != "" && cleanPath(row.SuiteDir) == pkg && !suiteDeclaredPkg(p, row.Tool) {
			found = append(found, Finding{Package: pkg, Test: "fakes:" + row.Tool, Rule: ruleUnverifiedFake})
		}
	}
	return finish(found, cfg), nil
}

func packageBaseline(base, pkg string) ([]Finding, error) {
	local, _, err := readShardFile(filepath.Join(resolvePkgDir(base, pkg), filepath.FromSlash(shardRelPath)))
	if err != nil {
		return nil, err
	}
	cb, _, err := readCentralBaselines(filepath.Join(base, filepath.FromSlash(centralBaselinesRelPath)))
	if err != nil {
		return nil, err
	}
	for _, r := range cb.Rows {
		if r.Package == pkg {
			local = append(local, r)
		}
	}
	return local, nil
}
