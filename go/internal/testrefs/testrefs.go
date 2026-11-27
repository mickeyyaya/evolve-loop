// Package testrefs proves that each test a doc of record names is declared in
// Go. A doc that cites a test nobody wrote is a false proof, so each citation
// that resolves to no declared test fails unless a shrink-only baseline lists it.
package testrefs

import (
	"encoding/json"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/stelint"
)

const BaselineRelPath = "internal/testrefs/baseline.json"

type Cite struct {
	Doc  string
	Line int
	Name string
}

var (
	spanPattern = regexp.MustCompile("`([^`\n]+)`")
	namePattern = regexp.MustCompile(`^(Test[A-Z0-9_][A-Za-z0-9_]*)(/\S*)?(\*?)$`)
	scopes      = []scope{
		{"docs/architecture", walkRecursive},
		{"docs/incidents", walkRecursive},
		{"docs/conventions", walkRecursive},
		{"docs/operations", walkRecursive},
		{"skills", walkRecursive},
		{"agents", walkFlat},
	}
)

type walkMode int

const (
	walkRecursive walkMode = iota
	walkFlat
)

type scope struct {
	dir  string
	mode walkMode
}

func Defined(goRoot string) ([]string, error) {
	fset := token.NewFileSet()
	seen := map[string]bool{}
	err := filepath.WalkDir(goRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if path != goRoot && skipDir(d.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(d.Name(), "_test.go") {
			return nil
		}
		return collectTests(fset, path, seen)
	})
	if err != nil {
		return nil, fmt.Errorf("testrefs: walk %s: %w", goRoot, err)
	}
	return sortedKeys(seen), nil
}

func skipDir(name string) bool {
	return name == "vendor" || name == "testdata" || strings.HasPrefix(name, ".") || strings.HasPrefix(name, "_")
}

func collectTests(fset *token.FileSet, path string, seen map[string]bool) error {
	file, err := parser.ParseFile(fset, path, nil, parser.SkipObjectResolution)
	if err != nil {
		return err
	}
	for _, decl := range file.Decls {
		if fn, ok := decl.(*ast.FuncDecl); ok && fn.Recv == nil && isTestName(fn.Name.Name) {
			seen[fn.Name.Name] = true
		}
	}
	return nil
}

func isTestName(name string) bool {
	return namePattern.MatchString(name)
}

func Cites(root string) ([]Cite, error) {
	var cites []Cite
	for _, sc := range scopes {
		found, err := citesUnder(root, sc)
		if err != nil {
			return nil, err
		}
		cites = append(cites, found...)
	}
	return cites, nil
}

func citesUnder(root string, sc scope) ([]Cite, error) {
	var cites []Cite
	base := filepath.Join(root, sc.dir)
	err := filepath.WalkDir(base, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if path != base && sc.mode == walkFlat {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(d.Name(), ".md") {
			return nil
		}
		rel := strings.TrimPrefix(filepath.ToSlash(path), filepath.ToSlash(root)+"/")
		found, err := citesIn(path, rel)
		cites = append(cites, found...)
		return err
	})
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	return cites, err
}

func citesIn(path, rel string) ([]Cite, error) {
	body, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cites []Cite
	generated := false
	for i, line := range strings.Split(string(body), "\n") {
		n := i + 1
		if region := stelint.RegionEdge(line); region != 0 {
			generated = region > 0
			continue
		}
		if generated {
			continue
		}
		for _, name := range namesIn(line) {
			cites = append(cites, Cite{Doc: rel, Line: n, Name: name})
		}
	}
	return cites, nil
}

func namesIn(line string) []string {
	var names []string
	for _, span := range spanPattern.FindAllStringSubmatch(line, -1) {
		if m := namePattern.FindStringSubmatch(span[1]); m != nil {
			names = append(names, m[1]+m[3])
		}
	}
	return names
}

func Missing(defined []string, cites []Cite) map[string][]string {
	missing := map[string][]string{}
	for _, c := range cites {
		if !resolves(defined, c.Name) {
			missing[c.Doc] = append(missing[c.Doc], c.Name)
		}
	}
	for doc, names := range missing {
		missing[doc] = sortedKeys(setOf(names))
	}
	return missing
}

func resolves(defined []string, name string) bool {
	if prefix, ok := strings.CutSuffix(name, "*"); ok {
		return hasPrefix(defined, prefix)
	}
	if strings.HasSuffix(name, "_") {
		return hasPrefix(defined, name)
	}
	i := sort.SearchStrings(defined, name)
	return i < len(defined) && defined[i] == name
}

func hasPrefix(defined []string, prefix string) bool {
	i := sort.SearchStrings(defined, prefix)
	return i < len(defined) && strings.HasPrefix(defined[i], prefix)
}

func Check(missing, baseline map[string][]string) []string {
	var problems []string
	for doc, names := range missing {
		listed := setOf(baseline[doc])
		for _, name := range names {
			if !listed[name] {
				problems = append(problems, fmt.Sprintf("%s cites %s, which no test declares; write the test or fix the name, never add a baseline row", doc, name))
			}
		}
	}
	for doc, names := range baseline {
		current := setOf(missing[doc])
		for _, name := range names {
			if !current[name] {
				problems = append(problems, fmt.Sprintf("%s: baseline row %s is stale; remove it from %s", doc, name, BaselineRelPath))
			}
		}
	}
	sort.Strings(problems)
	return problems
}

func Scan(root string) ([]string, error) {
	missing, err := scanMissing(root)
	if err != nil {
		return nil, err
	}
	body, err := os.ReadFile(filepath.Join(root, "go", BaselineRelPath))
	if err != nil {
		return nil, fmt.Errorf("testrefs: read baseline: %w", err)
	}
	baseline, err := parseBaseline(body, BaselineRelPath)
	if err != nil {
		return nil, err
	}
	return Check(missing, baseline), nil
}

func parseBaseline(body []byte, label string) (map[string][]string, error) {
	var baseline map[string][]string
	if err := json.Unmarshal(body, &baseline); err != nil {
		return nil, fmt.Errorf("testrefs: parse %s: %w", label, err)
	}
	return baseline, nil
}

func CheckAgainstBase(prefix string, atBase, onDisk func(string) ([]byte, error)) (string, error) {
	listing, err := atBase(prefix + BaselineRelPath)
	if errors.Is(err, fs.ErrNotExist) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	before, err := parseBaseline(listing, BaselineRelPath+" at the base")
	if err != nil {
		return "", err
	}
	body, err := onDisk(prefix + BaselineRelPath)
	if err != nil {
		return "", err
	}
	after, err := parseBaseline(body, BaselineRelPath)
	if err != nil {
		return "", err
	}
	return strings.Join(grownRows(before, after), "\n"), nil
}

func grownRows(before, after map[string][]string) []string {
	var grown []string
	for doc, names := range after {
		listed := setOf(before[doc])
		for _, name := range names {
			if !listed[name] {
				grown = append(grown, fmt.Sprintf("%s: baseline row %s is not in the baseline at the base; a baseline only shrinks, so write the test or fix the name", doc, name))
			}
		}
	}
	sort.Strings(grown)
	return grown
}

func scanMissing(root string) (map[string][]string, error) {
	defined, err := Defined(filepath.Join(root, "go"))
	if err != nil {
		return nil, err
	}
	cites, err := Cites(root)
	if err != nil {
		return nil, err
	}
	return Missing(defined, cites), nil
}

func setOf(names []string) map[string]bool {
	set := make(map[string]bool, len(names))
	for _, n := range names {
		set[n] = true
	}
	return set
}

func sortedKeys(set map[string]bool) []string {
	out := make([]string, 0, len(set))
	for k := range set {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
