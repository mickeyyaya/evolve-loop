package main

import (
	"bytes"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/lanerouting"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/build"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

func TestLaneForbidden_TheRepoBuilderSandboxRoutesAProfileItemToTheConsole(t *testing.T) {
	t.Parallel()
	var warn bytes.Buffer
	forbidden := laneForbidden(findRepoRoot(t), &warn)
	if !forbidden(".evolve/profiles/historian.json") {
		t.Fatal("the builder sandbox denies .evolve/profiles, so no lane can create a profile")
	}
	if forbidden("go/internal/fleet/fleet.go") {
		t.Fatal("ordinary source stays lane work")
	}
	if warn.Len() != 0 {
		t.Fatalf("the repo's builder profile must load: %s", warn.String())
	}
}

func TestLaneForbidden_WithoutTheBuilderProfileWarnsAndJudgesProtectedSurface(t *testing.T) {
	t.Parallel()
	var warn bytes.Buffer
	forbidden := laneForbidden(t.TempDir(), &warn)
	if !forbidden("go/internal/guards/role.go") || forbidden(".evolve/profiles/historian.json") {
		t.Fatal("the fallback judges protected surface only")
	}
	if strings.Count(warn.String(), "WARN") != 1 || !strings.Contains(warn.String(), string(lanerouting.CodeRoutingFallback)) {
		t.Fatalf("a missing builder profile must be loud exactly once, with the registered code, got %q", warn.String())
	}
}

func TestLaneForbiddenTo_EmitsTheRoutingFallbackIntoTheCycleCenter(t *testing.T) {
	t.Parallel()
	signals := signalcenter.New()
	forbidden := laneForbiddenTo(t.TempDir(), signals.Emit)
	_ = forbidden("go/internal/fleet/fleet.go")
	signals.Flush()
	recent := signals.Recent()
	if len(recent) != 1 || recent[0].Code != lanerouting.CodeRoutingFallback || recent[0].Fields["profile"] != build.ProfileName {
		t.Fatalf("the cycle root's fallback must reach its Signal Center as the coded event naming the build profile, got %+v", recent)
	}
}

func TestLaneForbidden_ADirectoryThatOnlyHoldsProtectedSurfaceStaysLaneWork(t *testing.T) {
	t.Parallel()
	for name, root := range map[string]string{"the repo builder profile": findRepoRoot(t), "no builder profile": t.TempDir()} {
		var warn bytes.Buffer
		forbidden := laneForbidden(root, &warn)
		for _, dir := range []string{"go/internal/core", "go/internal/core/", "go/cmd/evolve"} {
			if forbidden(dir) {
				t.Errorf("%s: forbidden(%q) = true, want false (ADR-0139 decision 5)", name, dir)
			}
		}
		for _, protected := range []string{"go/internal/explanationdocs", "go/internal/core/orchestrator.go"} {
			if !forbidden(protected) {
				t.Errorf("%s: forbidden(%q) = false, want true", name, protected)
			}
		}
	}
}

func TestLaneForbidden_ARootThatNeverRoutesStaysSilent(t *testing.T) {
	t.Parallel()
	var warn bytes.Buffer
	_ = laneForbidden(t.TempDir(), &warn)
	if warn.Len() != 0 {
		t.Fatalf("building the predicate must not load or warn; got %q", warn.String())
	}
}

func TestRoutingRoots_JudgeWithTheLanePredicate(t *testing.T) {
	t.Parallel()
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	fset := token.NewFileSet()
	triageConfigSetsIt := false
	for _, name := range files {
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		src, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		file, err := parser.ParseFile(fset, name, src, 0)
		if err != nil {
			t.Fatal(err)
		}
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			ast.Inspect(decl, func(n ast.Node) bool {
				switch x := n.(type) {
				case *ast.SelectorExpr:
					if id, isIdent := x.X.(*ast.Ident); isIdent && id.Name == "guards" && x.Sel.Name == "IsProtectedScope" && !(ok && fn.Name.Name == "laneForbidden") {
						t.Errorf("%s: a routing root passes guards.IsProtectedScope directly; use laneForbidden", fset.Position(x.Pos()))
					}
				case *ast.CompositeLit:
					if sel, isSel := x.Type.(*ast.SelectorExpr); isSel && sel.Sel.Name == "Config" {
						if pkg, isIdent := sel.X.(*ast.Ident); isIdent && pkg.Name == "triage" {
							for _, elt := range x.Elts {
								if kv, isKV := elt.(*ast.KeyValueExpr); isKV {
									if key, isIdent := kv.Key.(*ast.Ident); isIdent && key.Name == "LaneForbidden" {
										triageConfigSetsIt = true
									}
								}
							}
						}
					}
				}
				return true
			})
		}
	}
	if !triageConfigSetsIt {
		t.Error("the production triage.Config must set LaneForbidden")
	}
}

func TestWireOrchestrator_TheCycleRootSendsTheRoutingFallbackToItsSignalCenter(t *testing.T) {
	root, evolveDir := wiringRoot(t, "")
	inbox := filepath.Join(evolveDir, "inbox")
	if err := os.MkdirAll(inbox, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(inbox, "a.json"), []byte(`{"id":"lane-work","weight":0.9,"files":["go/internal/fleet/fleet.go"]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	d := wireOrchestratorDeps(root, evolveDir, io.Discard, routingRun{})
	defer d.CloseSignals()
	composer, ok := d.Runners[core.PhaseTriage].(interface {
		ComposePrompt(string, core.PhaseRequest) string
	})
	if !ok {
		t.Fatal("the cycle root's triage runner must compose its prompt")
	}
	if prompt := composer.ComposePrompt("BODY", core.PhaseRequest{ProjectRoot: root}); !strings.Contains(prompt, "lane-work") {
		t.Fatalf("the triage prompt must judge the inbox item, so the lane predicate is asked: %s", prompt)
	}
	_ = composer.ComposePrompt("BODY", core.PhaseRequest{ProjectRoot: root})
	d.Signals.Flush()
	found := 0
	for _, e := range d.Signals.Recent() {
		if e.Code == lanerouting.CodeRoutingFallback {
			found++
		}
	}
	if found != 1 {
		t.Fatalf("a root with no builder profile must send INBOX_ROUTING_FALLBACK into the cycle's Signal Center once across two prompts, got %d in %+v", found, d.Signals.Recent())
	}
}
