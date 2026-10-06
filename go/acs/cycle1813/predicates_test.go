//go:build acs

package cycle1813

import (
	"context"
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/core/advisor"
	"github.com/mickeyyaya/evolve-loop/go/internal/phaseio"
	"github.com/mickeyyaya/evolve-loop/go/internal/router"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

type scriptedLauncher struct {
	stdout string
	err    error
	calls  int
}

func (l *scriptedLauncher) Launch(_ context.Context, _ advisor.LaunchRequest) (advisor.LaunchResponse, error) {
	l.calls++
	if l.err != nil {
		return advisor.LaunchResponse{ExitCode: 1}, l.err
	}
	return advisor.LaunchResponse{Stdout: l.stdout}, nil
}

type failingProposer struct {
	proposal *router.Proposal
	err      error
	calls    int
}

func (p *failingProposer) Propose(router.RouteInput) (*router.Proposal, error) {
	p.calls++
	return p.proposal, p.err
}

func recordingCenter() (*signalcenter.Center, func() []signalcenter.Event) {
	c := signalcenter.New()
	var got []signalcenter.Event
	c.Subscribe(func(e signalcenter.Event) { got = append(got, e) })
	return c, func() []signalcenter.Event {
		c.Flush()
		return append([]signalcenter.Event(nil), got...)
	}
}

func codedWarnings(events []signalcenter.Event) []signalcenter.Event {
	var warns []signalcenter.Event
	for _, e := range events {
		if e.Severity == signalcenter.SeverityWarn && e.Code != "" {
			warns = append(warns, e)
		}
	}
	return warns
}

func llmRoutingConfig() config.RoutingConfig {
	return config.RoutingConfig{
		Stage:         config.StageEnforce,
		Mode:          config.ModeDynamicLLM,
		Mandatory:     []string{"scout", "build", "audit", "ship"},
		MaxInsertions: 4,
		PhaseEnable:   map[string]config.Enable{},
		Triggers:      map[string]config.RoutingBlock{},
	}
}

func buildCompletedInput(t *testing.T) router.RouteInput {
	t.Helper()
	return router.RouteInput{
		Current:     "build",
		Verdict:     "PASS",
		Cfg:         llmRoutingConfig(),
		Completed:   []string{"scout", "build"},
		Workspace:   t.TempDir(),
		ProjectRoot: t.TempDir(),
		Cycle:       1813,
		Env:         map[string]string{"EVOLVE_CLI": "claude-tmux"},
	}
}

func routerAdvisor(launcher advisor.Launcher, center *signalcenter.Center) *advisor.Advisor {
	return advisor.New(launcher, advisor.Identity{CLI: "claude-tmux", Model: "opus", AgentLabel: "router"}, nil,
		advisor.WithSignals(func() *signalcenter.Center { return center }))
}

func TestC1813_001_FailingProposeEmitsOneCodedWarnAndTheStaticDecisionStillApplies(t *testing.T) {
	cases := []struct {
		name     string
		launcher *scriptedLauncher
		wantCode signalcenter.Code
	}{
		{"bridge launch fails", &scriptedLauncher{err: errors.New("every CLI in the chain failed")}, advisor.CodeLaunchFailed},
		{"response carries no proposal", &scriptedLauncher{stdout: "the advisor rambled and returned no JSON"}, advisor.CodeResponseUnparseable},
		{"response is malformed JSON", &scriptedLauncher{stdout: `{"next_phase": "ship",`}, advisor.CodeResponseUnparseable},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			center, events := recordingCenter()
			in := buildCompletedInput(t)

			got := router.Select(in.Cfg, routerAdvisor(tc.launcher, center)).Decide(in)
			static := router.StaticPreset{}.Decide(in)

			if tc.launcher.calls == 0 {
				t.Fatalf("the production strategy never consulted the proposer; the failure path was not exercised")
			}
			if !reflect.DeepEqual(got, static) {
				t.Errorf("a failed Propose must leave the static decision:\n got    %+v\n static %+v", got, static)
			}
			warns := codedWarnings(events())
			if len(warns) != 1 {
				t.Fatalf("a failing Propose must emit exactly one coded WARN, got %d: %+v", len(warns), warns)
			}
			if warns[0].Code != tc.wantCode {
				t.Errorf("WARN code = %q, want %q (%+v)", warns[0].Code, tc.wantCode, warns[0])
			}
		})
	}

	t.Run("a successful Propose emits no WARN", func(t *testing.T) {
		center, events := recordingCenter()
		launcher := &scriptedLauncher{stdout: `{"next_phase":"audit","justification":"audit follows build"}`}
		in := buildCompletedInput(t)
		router.Select(in.Cfg, routerAdvisor(launcher, center)).Decide(in)
		if launcher.calls == 0 {
			t.Fatalf("the production strategy never consulted the proposer")
		}
		if warns := codedWarnings(events()); len(warns) != 0 {
			t.Errorf("a healthy proposal must not warn, got %+v", warns)
		}
	})

	t.Run("a proposal returned alongside an error is discarded", func(t *testing.T) {
		in := buildCompletedInput(t)
		static := router.StaticPreset{}.Decide(in)
		movingProposal := &router.Proposal{NextPhase: "audit", InsertPhases: []string{"adversarial-review"}, Justification: "insert a review"}

		healthy := &failingProposer{proposal: movingProposal}
		if accepted := router.Select(in.Cfg, healthy).Decide(in); reflect.DeepEqual(accepted, static) {
			t.Fatalf("fixture proposal %+v does not move the decision even without an error, so the error path below proves nothing: %+v", movingProposal, accepted)
		}

		broken := &failingProposer{proposal: movingProposal, err: errors.New("proposer failed after a partial decode")}
		got := router.Select(in.Cfg, broken).Decide(in)
		if broken.calls == 0 {
			t.Fatalf("the production strategy never consulted the proposer")
		}
		if !reflect.DeepEqual(got, static) {
			t.Errorf("a proposal returned with a non-nil error must be ignored and the static decision kept:\n got    %+v\n static %+v", got, static)
		}
	})
}

func writeRegistry(t *testing.T, root string, body []byte) string {
	t.Helper()
	path := config.RegistryPath(root)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if body != nil {
		if err := os.WriteFile(path, body, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return path
}

func policyForProject(t *testing.T, root string, env map[string]string, opts ...config.Option) router.PhasePolicy {
	t.Helper()
	fn := reflect.ValueOf(router.PolicyForProject)
	ft := fn.Type()
	acceptsOptions := ft.IsVariadic() && ft.In(ft.NumIn()-1) == reflect.TypeOf([]config.Option(nil))
	if len(opts) > 0 && !acceptsOptions {
		t.Fatalf("router.PolicyForProject is %s; it must take a trailing ...config.Option so a caller can inject the Center its registry WARN rides", ft)
	}
	args := []reflect.Value{reflect.ValueOf(root), reflect.ValueOf(env)}
	for _, o := range opts {
		args = append(args, reflect.ValueOf(o))
	}
	out := fn.Call(args)
	if len(out) != 1 {
		t.Fatalf("router.PolicyForProject returns %d values, want the PhasePolicy alone", len(out))
	}
	p, ok := out[0].Interface().(router.PhasePolicy)
	if !ok {
		t.Fatalf("router.PolicyForProject returns %s, want router.PhasePolicy", out[0].Type())
	}
	return p
}

func withCenter(c *signalcenter.Center) config.Option {
	return config.WithSignals(func() *signalcenter.Center { return c })
}

func TestC1813_002_MalformedConfigInPolicyForProjectEmitsOneCodedWarnAndFailsOpen(t *testing.T) {
	env := map[string]string{}
	baseline := policyForProject(t, t.TempDir(), env)

	malformedRoot := t.TempDir()
	writeRegistry(t, malformedRoot, []byte(`{"phases": [{"name": "scout",}]`))

	unreadableRoot := t.TempDir()
	if err := os.MkdirAll(writeRegistry(t, unreadableRoot, nil), 0o755); err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name     string
		root     string
		wantCode signalcenter.Code
	}{
		{"registry is not valid JSON", malformedRoot, config.CodeRegistryMalformed},
		{"registry path cannot be read", unreadableRoot, config.CodeRegistryUnreadable},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			center, events := recordingCenter()
			got := policyForProject(t, tc.root, env, withCenter(center))

			warns := codedWarnings(events())
			if len(warns) != 1 {
				t.Fatalf("a malformed config must emit exactly one coded WARN, got %d: %+v", len(warns), warns)
			}
			if warns[0].Code != tc.wantCode || warns[0].Module != signalcenter.ModuleConfig {
				t.Errorf("WARN = %s/%q, want %s/%q", warns[0].Module, warns[0].Code, signalcenter.ModuleConfig, tc.wantCode)
			}
			if !reflect.DeepEqual(got.Cfg, baseline.Cfg) {
				t.Errorf("a malformed config must fail open to the defaults:\n got      %+v\n defaults %+v", got.Cfg, baseline.Cfg)
			}
			for _, phase := range []string{"scout", "build", "audit", "ship"} {
				if !got.ShouldRunPhase(phase) {
					t.Errorf("fail-open policy refuses mandatory phase %q", phase)
				}
			}
		})
	}

	t.Run("an absent registry stays silent", func(t *testing.T) {
		center, events := recordingCenter()
		policyForProject(t, t.TempDir(), env, withCenter(center))
		if warns := codedWarnings(events()); len(warns) != 0 {
			t.Errorf("absence is not a fault and must not warn, got %+v", warns)
		}
	})

	t.Run("a disabled registry is not read and stays silent", func(t *testing.T) {
		center, events := recordingCenter()
		policyForProject(t, malformedRoot, map[string]string{"EVOLVE_USE_PHASE_REGISTRY": "0"}, withCenter(center))
		if warns := codedWarnings(events()); len(warns) != 0 {
			t.Errorf("EVOLVE_USE_PHASE_REGISTRY=0 skips the registry, so its malformation must not warn, got %+v", warns)
		}
	})

	t.Run("the two-argument production call shape still fails open without a Center", func(t *testing.T) {
		if got := policyForProject(t, malformedRoot, env); !reflect.DeepEqual(got.Cfg, baseline.Cfg) {
			t.Errorf("PolicyForProject(root, env) on a malformed registry = %+v, want the defaults", got.Cfg)
		}
	})
}

type errorReturnScan struct {
	funcs  map[string]*ast.FuncDecl
	where  map[string]token.Position
	always map[string]bool
}

func scanPackageSources(t *testing.T, fset *token.FileSet, files []*ast.File) *errorReturnScan {
	t.Helper()
	s := &errorReturnScan{funcs: map[string]*ast.FuncDecl{}, where: map[string]token.Position{}, always: map[string]bool{}}
	for _, f := range files {
		for _, decl := range f.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Body == nil || !lastResultIsError(fn.Type) {
				continue
			}
			key := funcKey(fn)
			s.funcs[key] = fn
			s.where[key] = fset.Position(fn.Pos())
		}
	}
	for changed := true; changed; {
		changed = false
		for key, fn := range s.funcs {
			if !s.always[key] && s.errorAlwaysNil(fn) {
				s.always[key] = true
				changed = true
			}
		}
	}
	return s
}

func (s *errorReturnScan) offenders() []string {
	var out []string
	for key := range s.always {
		out = append(out, key+" ("+s.where[key].String()+")")
	}
	sort.Strings(out)
	return out
}

func funcKey(fn *ast.FuncDecl) string {
	if fn.Recv == nil || len(fn.Recv.List) == 0 {
		return fn.Name.Name
	}
	recv := fn.Recv.List[0].Type
	if star, ok := recv.(*ast.StarExpr); ok {
		recv = star.X
	}
	if id, ok := recv.(*ast.Ident); ok {
		return id.Name + "." + fn.Name.Name
	}
	return fn.Name.Name
}

func lastResultIsError(ft *ast.FuncType) bool {
	if ft.Results == nil || len(ft.Results.List) == 0 {
		return false
	}
	id, ok := ft.Results.List[len(ft.Results.List)-1].Type.(*ast.Ident)
	return ok && id.Name == "error"
}

func namedErrorResult(ft *ast.FuncType) string {
	last := ft.Results.List[len(ft.Results.List)-1]
	if len(last.Names) == 0 {
		return ""
	}
	return last.Names[len(last.Names)-1].Name
}

func returnStatements(body *ast.BlockStmt) []*ast.ReturnStmt {
	var rets []*ast.ReturnStmt
	ast.Inspect(body, func(n ast.Node) bool {
		switch v := n.(type) {
		case *ast.FuncLit:
			return false
		case *ast.ReturnStmt:
			rets = append(rets, v)
		}
		return true
	})
	return rets
}

func (s *errorReturnScan) errorAlwaysNil(fn *ast.FuncDecl) bool {
	rets := returnStatements(fn.Body)
	if len(rets) == 0 {
		return false
	}
	for _, ret := range rets {
		if !s.returnsNilError(fn, ret) {
			return false
		}
	}
	return true
}

func (s *errorReturnScan) returnsNilError(fn *ast.FuncDecl, ret *ast.ReturnStmt) bool {
	if len(ret.Results) == 0 {
		named := namedErrorResult(fn.Type)
		return named != "" && s.identAlwaysNil(fn, named)
	}
	if len(ret.Results) == 1 && resultCount(fn.Type) > 1 {
		return s.isAlwaysNilCall(ret.Results[0])
	}
	return s.exprAlwaysNil(fn, ret.Results[len(ret.Results)-1])
}

func resultCount(ft *ast.FuncType) int {
	n := 0
	for _, field := range ft.Results.List {
		if len(field.Names) == 0 {
			n++
			continue
		}
		n += len(field.Names)
	}
	return n
}

func (s *errorReturnScan) exprAlwaysNil(fn *ast.FuncDecl, e ast.Expr) bool {
	switch v := e.(type) {
	case *ast.Ident:
		if v.Name == "nil" {
			return true
		}
		return s.identAlwaysNil(fn, v.Name)
	case *ast.CallExpr:
		return s.isAlwaysNilCall(v)
	case *ast.ParenExpr:
		return s.exprAlwaysNil(fn, v.X)
	}
	return false
}

func (s *errorReturnScan) isAlwaysNilCall(e ast.Expr) bool {
	call, ok := e.(*ast.CallExpr)
	if !ok {
		return false
	}
	id, ok := call.Fun.(*ast.Ident)
	return ok && s.always[id.Name]
}

func isNilIdent(e ast.Expr) bool {
	id, ok := e.(*ast.Ident)
	return ok && id.Name == "nil"
}

func fieldListNames(fl *ast.FieldList, name string) bool {
	if fl == nil {
		return false
	}
	for _, field := range fl.List {
		for _, n := range field.Names {
			if n.Name == name {
				return true
			}
		}
	}
	return false
}

func (s *errorReturnScan) assignmentKeepsNil(v *ast.AssignStmt, i int) bool {
	if len(v.Rhs) == len(v.Lhs) {
		return isNilIdent(v.Rhs[i]) || s.isAlwaysNilCall(v.Rhs[i])
	}
	return len(v.Rhs) == 1 && i == len(v.Lhs)-1 && s.isAlwaysNilCall(v.Rhs[0])
}

func (s *errorReturnScan) identAlwaysNil(fn *ast.FuncDecl, name string) bool {
	if fieldListNames(fn.Type.Params, name) || fieldListNames(fn.Recv, name) {
		return false
	}
	declared := fieldListNames(fn.Type.Results, name)
	safe := true
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		switch v := n.(type) {
		case *ast.FuncLit:
			return false
		case *ast.UnaryExpr:
			if id, ok := v.X.(*ast.Ident); ok && v.Op == token.AND && id.Name == name {
				safe = false
			}
		case *ast.RangeStmt:
			for _, bound := range []ast.Expr{v.Key, v.Value} {
				if id, ok := bound.(*ast.Ident); ok && id.Name == name {
					safe = false
				}
			}
		case *ast.AssignStmt:
			for i, lhs := range v.Lhs {
				id, ok := lhs.(*ast.Ident)
				if !ok || id.Name != name {
					continue
				}
				declared = true
				if !s.assignmentKeepsNil(v, i) {
					safe = false
				}
			}
		case *ast.ValueSpec:
			for i, n := range v.Names {
				if n.Name != name {
					continue
				}
				declared = true
				if len(v.Values) == len(v.Names) && !isNilIdent(v.Values[i]) && !s.isAlwaysNilCall(v.Values[i]) {
					safe = false
				}
				if len(v.Values) != 0 && len(v.Values) != len(v.Names) && !(i == len(v.Names)-1 && s.isAlwaysNilCall(v.Values[0])) {
					safe = false
				}
			}
		}
		return true
	})
	return declared && safe
}

func parseSources(t *testing.T, fset *token.FileSet, sources map[string]string) []*ast.File {
	t.Helper()
	var files []*ast.File
	for name, src := range sources {
		f, err := parser.ParseFile(fset, name, src, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", name, err)
		}
		files = append(files, f)
	}
	return files
}

func parseRouterPackage(t *testing.T, dir string) (*token.FileSet, []*ast.File) {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read %s: %v", dir, err)
	}
	fset := token.NewFileSet()
	var files []*ast.File
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, filepath.Join(dir, name), nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", name, err)
		}
		files = append(files, f)
	}
	if len(files) == 0 {
		t.Fatalf("no non-test Go sources under %s", dir)
	}
	return fset, files
}

func TestC1813_003_TheAlwaysNilErrorScannerDetectsDirectAndTransitiveCases(t *testing.T) {
	fset := token.NewFileSet()
	files := parseSources(t, fset, map[string]string{"fixture.go": `package p

import "errors"

type T struct{}

func literalNil() error { return nil }

func pairNil() (int, error) { return 1, nil }

func viaDeadCheck() (int, error) {
	v, err := pairNil()
	if err != nil {
		return 0, err
	}
	return v, nil
}

func forwardsDead() (int, error) { return pairNil() }

func (T) method() (string, error) { return "", nil }

func zeroVar() (int, error) {
	var err error
	return 1, err
}

func namedBare() (n int, err error) {
	n = 1
	return
}

func namedReturnedByName() (n int, err error) {
	n = 2
	return n, err
}

func realFailure(ok bool) (int, error) {
	if !ok {
		return 0, errors.New("refused")
	}
	return 1, nil
}

func forwardsReal() (int, error) {
	v, err := realFailure(false)
	if err != nil {
		return 0, err
	}
	return v, nil
}

func namedAssigned(ok bool) (err error) {
	if !ok {
		err = errors.New("refused")
	}
	return
}

func varAssignedLater(ok bool) error {
	var err error
	if !ok {
		err = errors.New("refused")
	}
	return err
}

func viaPointer() error {
	var err error
	fill(&err)
	return err
}

func fill(p *error) { *p = errors.New("filled") }

func rangeBound(errs []error) (err error) {
	for _, err = range errs {
		if err != nil {
			return err
		}
	}
	return nil
}

func closureOnly() error {
	f := func() error { return errors.New("inner") }
	_ = f
	return nil
}

func passesParam(err error) error { return err }

func noError() int { return 1 }
`})
	got := scanPackageSources(t, fset, files).always
	for _, want := range []string{"literalNil", "pairNil", "viaDeadCheck", "forwardsDead", "T.method", "zeroVar", "namedBare", "namedReturnedByName", "closureOnly"} {
		if !got[want] {
			t.Errorf("scanner missed always-nil error return in %s", want)
		}
	}
	for _, clean := range []string{"realFailure", "forwardsReal", "namedAssigned", "varAssignedLater", "viaPointer", "rangeBound", "passesParam", "noError"} {
		if got[clean] {
			t.Errorf("scanner flagged %s, whose error can be non-nil", clean)
		}
	}
}

// acs-predicate: source-structure
func TestC1813_004_NoFunctionInRouterHasAnAlwaysNilErrorReturn(t *testing.T) {
	root := acsassert.RepoRoot(t)
	fset, files := parseRouterPackage(t, filepath.Join(root, "go", "internal", "router"))
	if offenders := scanPackageSources(t, fset, files).offenders(); len(offenders) > 0 {
		t.Errorf("internal/router has %d function(s) whose error return is always nil — drop the error or give it a real failure:\n  %s",
			len(offenders), strings.Join(offenders, "\n  "))
	}
}

func callFailOpen(t *testing.T, fn any, workspace string, completed []string) reflect.Value {
	t.Helper()
	out := reflect.ValueOf(fn).Call([]reflect.Value{reflect.ValueOf(workspace), reflect.ValueOf(completed)})
	if len(out) == 2 && !out[1].IsNil() {
		t.Fatalf("an unreadable handoff must fail open, not fail: %v", out[1].Interface())
	}
	return out[0]
}

func TestC1813_005_DigestAndAssembleHandoffsStillFailOpenOnAnUnreadableHandoff(t *testing.T) {
	ws := t.TempDir()
	if err := os.Mkdir(filepath.Join(ws, "handoff-scout.json"), 0o755); err != nil {
		t.Fatal(err)
	}

	sig, ok := callFailOpen(t, router.Digest, ws, []string{"scout"}).Interface().(router.RoutingSignals)
	if !ok {
		t.Fatalf("Digest's first result is not router.RoutingSignals")
	}
	if len(sig.DigestDegraded) == 0 {
		t.Errorf("an unreadable handoff-scout.json must be recorded in DigestDegraded, got %+v", sig)
	}

	handoffs, ok := callFailOpen(t, router.AssembleHandoffs, ws, []string{"scout"}).Interface().(phaseio.Handoffs)
	if !ok {
		t.Fatalf("AssembleHandoffs' first result is not phaseio.Handoffs")
	}
	if !reflect.DeepEqual(handoffs.Degraded(), sig.DigestDegraded) {
		t.Errorf("AssembleHandoffs must carry Digest's degraded reads: got %v, want %v", handoffs.Degraded(), sig.DigestDegraded)
	}

	clean, ok := callFailOpen(t, router.Digest, t.TempDir(), []string{"scout"}).Interface().(router.RoutingSignals)
	if !ok {
		t.Fatalf("Digest's first result is not router.RoutingSignals")
	}
	if len(clean.DigestDegraded) != 0 {
		t.Errorf("an absent handoff is not degraded, got %v", clean.DigestDegraded)
	}
}
