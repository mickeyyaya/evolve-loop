package testaudit

import (
	"path"
	"slices"
	"strings"
)

const (
	moduleDirPrefix = "go/"
	auditDir        = "internal/testaudit"
)

type Scope struct {
	Packages []string
	Whole    bool
}

func AuditScope(changed []string) Scope {
	var s Scope
	for _, p := range changed {
		rel, ok := strings.CutPrefix(p, moduleDirPrefix)
		if !ok {
			continue
		}
		if rewritesAuditRules(rel) {
			s.Whole = true
		}
		if pkg, ok := scopedPackage(rel); ok && !slices.Contains(s.Packages, pkg) {
			s.Packages = append(s.Packages, pkg)
		}
	}
	slices.Sort(s.Packages)
	return s
}

func rewritesAuditRules(rel string) bool {
	switch rel {
	case centralBaselinesRelPath, auditDir + "/" + guardsFile, auditDir + "/" + fakesFile:
		return true
	}
	return path.Dir(rel) == auditDir && strings.HasSuffix(rel, ".go") && !strings.HasSuffix(rel, "_test.go")
}

func scopedPackage(rel string) (string, bool) {
	if dir, ok := strings.CutSuffix(rel, shardRelPath); ok && (dir == "" || strings.HasSuffix(dir, "/")) {
		return path.Clean(dir + "."), true
	}
	if strings.HasSuffix(rel, ".go") {
		return path.Dir(rel), true
	}
	return "", false
}
