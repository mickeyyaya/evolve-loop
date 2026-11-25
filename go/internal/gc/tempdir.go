package gc

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"
)

type tempShape struct {
	id      string
	creator string
	pattern *regexp.Regexp
}

var tempShapes = []tempShape{
	{id: "go-build", creator: "go toolchain", pattern: regexp.MustCompile(`^go-build\d+$`)},
	{id: "go-test-tempdir", creator: "testing.T.TempDir", pattern: regexp.MustCompile(`^Test[\w.-]*\d{6,}$`)},
	{id: "gittest", creator: "go/internal/gittest", pattern: regexp.MustCompile(`^gittest\d{6,}$`)},
	{id: "release-dryrun", creator: "go/cmd/evolve/cmd_release.go", pattern: regexp.MustCompile(`^release-pipeline-dryrun-.+\.json$`)},
	{id: "acs-temp", creator: "go/acs", pattern: regexp.MustCompile(`^acs(?:(?:-cycle|-c)?\d+(?:-.+|\d{6,})|-gitprobe-.+)$`)},
	{id: "cycle-temp", creator: "go/acs", pattern: regexp.MustCompile(`^cycle\d+-.+$`)},
	{id: "evolve-temp", creator: "go/cmd/evolve;go/internal;go/acs", pattern: regexp.MustCompile(`^evolve-[A-Za-z0-9_.-]+\d{6,}$`)},
	{id: "ev-isolated-home", creator: "go/cmd/evolve/main_test.go;go/internal/bridge/tmuxtest_main_test.go;go/internal/gittest/credentials.go", pattern: regexp.MustCompile(`^ev(?:codexhome|codeusage|tmux|ghconfig)\d{6,}$`)},
	{id: "buildfloor-cover", creator: "go/internal/core/build_floor_reviewer.go", pattern: regexp.MustCompile(`^buildfloor-cover-\d{6,}$`)},
	{id: "audit-generator-builds", creator: "go/internal/phases/audit/generator_builds_test.go", pattern: regexp.MustCompile(`^audit-generator-builds-\d{6,}$`)},
	{id: "boundary-latency", creator: "go/acs/cycle2483/predicates_test.go", pattern: regexp.MustCompile(`^boundary-latency-c\d+-\d{6,}$`)},
	{id: "cycle-short-c", creator: "go/acs/cycle1791;go/acs/cycle2237;go/acs/cycle2376;go/acs/cycle2616", pattern: regexp.MustCompile(`^c\d+(?:tmux)?\d{6,}$`)},
	{id: "ciwatch-testjson", creator: "go/internal/ciwatch/classify_testjson.go", pattern: regexp.MustCompile(`^ciwatch-testjson-\d{6,}$`)},
	{id: "core-seat-ws", creator: "go/internal/core/orchestrator_main_test.go", pattern: regexp.MustCompile(`^core-seat-ws-\d{6,}$`)},
	{id: "e2gc", creator: "go/cmd/evolve/cmd_gc_tmux_backfill_e2_test.go", pattern: regexp.MustCompile(`^e2gc\d{6,}$`)},
	{id: "gcfuzz", creator: "go/internal/gc/discover_fuzz_test.go", pattern: regexp.MustCompile(`^gcfuzz\d{6,}$`)},
	{id: "gcsock", creator: "go/internal/gc/salvage_backfill_f1_test.go", pattern: regexp.MustCompile(`^gcsock\d{6,}$`)},
	{id: "gitexec-index", creator: "go/internal/gitexec", pattern: regexp.MustCompile(`^gitexec-(?:patch-)?index-\d{6,}$`)},
	{id: "gotypes-fixture", creator: "go/internal/adapters/codeindex/gotypes/helpers_test.go", pattern: regexp.MustCompile(`^gotypes-fixture-\d{6,}$`)},
	{id: "l2-checkout", creator: "go/internal/integration/train_l2_test.go", pattern: regexp.MustCompile(`^l2-checkout-\d{6,}$`)},
	{id: "loop-filed-lint", creator: "go/acs/cycle2588/predicates_test.go", pattern: regexp.MustCompile(`^loop-filed-lint-c\d+-\d{6,}$`)},
	{id: "modelquery-pin-acs", creator: "go/acs/cycle1882/predicates_test.go", pattern: regexp.MustCompile(`^modelquery-pin-acs-\d{6,}$`)},
	{id: "runner-home", creator: "go/internal/phases/runner/runner_test.go", pattern: regexp.MustCompile(`^runner-home-\d{6,}$`)},
	{id: "salvage-repack", creator: "go/internal/gc/salvage_repack.go", pattern: regexp.MustCompile(`^salvage-repack-\d{6,}$`)},
	{id: "sandbox-git-fixture", creator: "go/internal/bridge/isolation_policy_integration_test.go", pattern: regexp.MustCompile(`^sandbox-git-fixture-\d{6,}$`)},
	{id: "sbc-shared", creator: "go/internal/bridge/sandbox_contract_integration_test.go", pattern: regexp.MustCompile(`^sbc-shared\d{6,}$`)},
	{id: "shiptest", creator: "go/internal/phases/ship/shiptest/fixture.go", pattern: regexp.MustCompile(`^shiptest-\d{6,}$`)},
	{id: "swx", creator: "go/internal/swarm/kill_exact_test.go", pattern: regexp.MustCompile(`^swx\d{6,}$`)},
	{id: "testlatency", creator: "go/acs/cycle2064;go/acs/cycle2360;go/acs/cycle2384", pattern: regexp.MustCompile(`^testlatency-(?:acs|bin|c\d+)-\d{6,}$`)},
	{id: "tmt", creator: "go/internal/tmuxtest/tmuxtest_test.go", pattern: regexp.MustCompile(`^tmt\d{6,}$`)},
	{id: "u12", creator: "go/internal/observerengine/tail_test.go", pattern: regexp.MustCompile(`^u12\d{6,}$`)},
	{id: "vulnfilter-acs", creator: "go/acs/cycle1881/predicates_test.go", pattern: regexp.MustCompile(`^vulnfilter-acs-\d{6,}$`)},
}

func matchTempShape(name string) (tempShape, bool) {
	for _, s := range tempShapes {
		if s.pattern.MatchString(name) {
			return s, true
		}
	}
	return tempShape{}, false
}

type tempShapeMatcher struct{}

func (tempShapeMatcher) MatchString(name string) bool {
	_, ok := matchTempShape(name)
	return ok
}

var pipelineTempRe tempShapeMatcher

func UnregisteredTempPatterns(root string) ([]string, error) {
	unregistered := map[string]struct{}{}
	fset := token.NewFileSet()
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() || !strings.HasSuffix(d.Name(), ".go") {
			return nil
		}
		return collectUnregisteredPatterns(fset, path, unregistered)
	})
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(unregistered))
	for pat := range unregistered {
		out = append(out, pat)
	}
	slices.Sort(out)
	return out, nil
}

func collectUnregisteredPatterns(fset *token.FileSet, path string, unregistered map[string]struct{}) error {
	f, err := parser.ParseFile(fset, path, nil, parser.SkipObjectResolution)
	if err != nil {
		return err
	}
	osNames := osImportNames(f)
	if len(osNames) == 0 {
		return nil
	}
	ast.Inspect(f, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok || len(call.Args) != 2 {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || sel.Sel.Name != "MkdirTemp" {
			return true
		}
		id, ok := sel.X.(*ast.Ident)
		if !ok || !osNames[id.Name] {
			return true
		}
		pat, ok := leadingStringLiteral(call.Args[1])
		if !ok {
			return true
		}
		if _, registered := matchTempShape(sampleTempName(pat)); !registered {
			unregistered[pat] = struct{}{}
		}
		return true
	})
	return nil
}

func osImportNames(f *ast.File) map[string]bool {
	names := map[string]bool{}
	for _, imp := range f.Imports {
		importPath, err := strconv.Unquote(imp.Path.Value)
		if err != nil || importPath != "os" {
			continue
		}
		name := "os"
		if imp.Name != nil {
			name = imp.Name.Name
		}
		if name != "_" && name != "." {
			names[name] = true
		}
	}
	return names
}

func leadingStringLiteral(expr ast.Expr) (string, bool) {
	if s, ok := fullStringLiteral(expr); ok {
		return s, true
	}
	expr = unwrapParens(expr)
	bin, ok := expr.(*ast.BinaryExpr)
	if !ok || bin.Op != token.ADD {
		return "", false
	}
	return leadingStringLiteral(bin.X)
}

func fullStringLiteral(expr ast.Expr) (string, bool) {
	expr = unwrapParens(expr)
	switch v := expr.(type) {
	case *ast.BasicLit:
		if v.Kind != token.STRING {
			return "", false
		}
		s, err := strconv.Unquote(v.Value)
		return s, err == nil
	case *ast.BinaryExpr:
		if v.Op != token.ADD {
			return "", false
		}
		left, lok := fullStringLiteral(v.X)
		right, rok := fullStringLiteral(v.Y)
		if !lok || !rok {
			return "", false
		}
		return left + right, true
	default:
		return "", false
	}
}

func unwrapParens(expr ast.Expr) ast.Expr {
	for {
		p, ok := expr.(*ast.ParenExpr)
		if !ok {
			return expr
		}
		expr = p.X
	}
}

func sampleTempName(pattern string) string {
	const digits = "1234567890"
	if idx := strings.LastIndexByte(pattern, '*'); idx >= 0 {
		return pattern[:idx] + digits + pattern[idx+1:]
	}
	return pattern + digits
}

type TempReapReport struct {
	Entries int      `json:"entries"`
	Held    int      `json:"held,omitempty"`
	Bytes   int64    `json:"bytes"`
	Errors  []string `json:"errors,omitempty"`
}

type TempOption func(*tempConfig)

type tempConfig struct {
	held func(path string) (bool, error)
}

func SparingHeld(held func(path string) (bool, error)) TempOption {
	return func(c *tempConfig) { c.held = held }
}

type tempAction string

const (
	tempActionRemove       tempAction = "remove"
	tempActionKeepFresh    tempAction = "keep_fresh"
	tempActionSpareHeld    tempAction = "spare_held"
	tempActionSpareUnknown tempAction = "spare_unknown"
)

type tempClassification struct {
	path     string
	action   tempAction
	rule     string
	evidence string
}

func classifyTempDir(dir string, cutoff time.Time, cfg tempConfig) ([]tempClassification, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read temp dir %s: %w", dir, err)
	}
	var items []tempClassification
	for _, e := range entries {
		shape, ok := matchTempShape(e.Name())
		if !ok {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		items = append(items, classifyTempEntry(filepath.Join(dir, e.Name()), shape, info.IsDir(), info.ModTime(), cutoff, cfg.held))
	}
	return items, nil
}

func classifyTempEntry(path string, shape tempShape, isDir bool, modTime, cutoff time.Time, heldFn func(string) (bool, error)) tempClassification {
	if !modTime.Before(cutoff) {
		return tempClassification{
			path:     path,
			action:   tempActionKeepFresh,
			rule:     "temp.fresh",
			evidence: fmt.Sprintf("shape %s (%s); mtime %s is at or after cutoff %s", shape.id, shape.creator, modTime.UTC().Format(time.RFC3339), cutoff.UTC().Format(time.RFC3339)),
		}
	}
	if heldFn != nil {
		held, err := heldFn(path)
		if err != nil {
			return tempClassification{
				path:     path,
				action:   tempActionSpareUnknown,
				rule:     "temp.holder_unknown",
				evidence: fmt.Sprintf("spared %s: liveness unknown: %v", path, err),
			}
		}
		if held {
			return tempClassification{
				path:     path,
				action:   tempActionSpareHeld,
				rule:     "temp.held",
				evidence: fmt.Sprintf("shape %s (%s); held by a live process", shape.id, shape.creator),
			}
		}
	}
	if isDir && hasLiveSocket(path) {
		return tempClassification{
			path:     path,
			action:   tempActionSpareHeld,
			rule:     "temp.held",
			evidence: fmt.Sprintf("shape %s (%s); holds a live unix socket", shape.id, shape.creator),
		}
	}
	return tempClassification{
		path:     path,
		action:   tempActionRemove,
		rule:     "temp.ttl_hours",
		evidence: fmt.Sprintf("shape %s (%s); mtime %s is before cutoff %s; no live holder", shape.id, shape.creator, modTime.UTC().Format(time.RFC3339), cutoff.UTC().Format(time.RFC3339)),
	}
}

func hasLiveSocket(root string) bool {
	found := false
	_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.Type()&fs.ModeSocket == 0 {
			return nil
		}
		conn, dialErr := net.DialTimeout("unix", path, 200*time.Millisecond)
		if dialErr == nil {
			_ = conn.Close()
			found = true
			return fs.SkipAll
		}
		return nil
	})
	return found
}

func ReapPipelineTemp(dir string, cutoff time.Time, removeAll func(string) error, opts ...TempOption) TempReapReport {
	var cfg tempConfig
	for _, o := range opts {
		o(&cfg)
	}
	var rep TempReapReport
	items, err := classifyTempDir(dir, cutoff, cfg)
	if err != nil {
		rep.Errors = append(rep.Errors, err.Error())
		return rep
	}
	for _, it := range items {
		switch it.action {
		case tempActionKeepFresh:
			continue
		case tempActionSpareUnknown:
			rep.Errors = append(rep.Errors, it.evidence)
		case tempActionSpareHeld:
			rep.Held++
		case tempActionRemove:
			size := treeSize(it.path)
			if err := removeAll(it.path); err != nil {
				rep.Errors = append(rep.Errors, err.Error())
				continue
			}
			rep.Entries++
			rep.Bytes += size
		}
	}
	return rep
}

func treeSize(root string) int64 {
	var n int64
	_ = filepath.WalkDir(root, func(_ string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		if info, err := d.Info(); err == nil {
			n += info.Size()
		}
		return nil
	})
	return n
}
