package continuation

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/acssuite"
	"github.com/mickeyyaya/evolve-loop/go/internal/acsverdict"
)

const repairFirstHeader = "FIRST ACTION: the prior attempt left these predicates RED, and they are still in the tree you resume. " +
	"Repair or retract each one BEFORE any new work. A predicate that cannot hold is yours to fix or retract on a continuation; " +
	"record the change in your report.\n"

var (
	predicatePackage = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)
	predicateTest    = regexp.MustCompile(`^Test[A-Za-z0-9_]*$`)
	predicateFile    = regexp.MustCompile(`^[A-Za-z0-9_.-]+_test\.go$`)
)

func RepairFirst(priorWorkspace, worktree string) (string, error) {
	raw, err := os.ReadFile(filepath.Join(priorWorkspace, acsverdict.Filename))
	if os.IsNotExist(err) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("continuation: read prior %s: %w", acsverdict.Filename, err)
	}
	var v acssuite.Verdict
	if err := json.Unmarshal(raw, &v); err != nil {
		return "", fmt.Errorf("continuation: parse prior %s: %w", acsverdict.Filename, err)
	}
	var lines []string
	seen := map[string]bool{}
	for _, r := range v.Results {
		pkg, test, ok := livePredicate(r, worktree)
		if !ok || seen[pkg+"/"+test] {
			continue
		}
		seen[pkg+"/"+test] = true
		file, err := declaringFile(worktree, pkg, test)
		if err != nil {
			return "", err
		}
		lines = append(lines, fmt.Sprintf("- %s: %s", file, test))
	}
	if len(lines) == 0 {
		return "", nil
	}
	return repairFirstHeader + strings.Join(lines, "\n"), nil
}

func livePredicate(r acssuite.Result, worktree string) (pkg, test string, ok bool) {
	if r.ResultStr != "red" || strings.HasPrefix(r.ACID, acsverdict.SyntheticRedPrefix) {
		return "", "", false
	}
	pkg, rest, found := strings.Cut(r.ACID, "/")
	test, _, _ = strings.Cut(rest, "/")
	if !found || !predicateTest.MatchString(test) || !predicatePackage.MatchString(pkg) {
		return "", "", false
	}
	info, err := os.Stat(filepath.Join(worktree, "go", "acs", pkg))
	return pkg, test, err == nil && info.IsDir()
}

func declaringFile(worktree, pkg, test string) (string, error) {
	files, err := filepath.Glob(filepath.Join(worktree, "go", "acs", pkg, "*_test.go"))
	if err != nil {
		return "", fmt.Errorf("continuation: list predicate files of %s: %w", pkg, err)
	}
	declares := regexp.MustCompile(`(?m)^func ` + test + `\(`)
	for _, f := range files {
		body, err := os.ReadFile(f)
		if err != nil {
			return "", fmt.Errorf("continuation: read predicate file %s: %w", f, err)
		}
		if base := filepath.Base(f); predicateFile.MatchString(base) && declares.Match(body) {
			return "go/acs/" + pkg + "/" + base, nil
		}
	}
	return "go/acs/" + pkg + "/", nil
}
