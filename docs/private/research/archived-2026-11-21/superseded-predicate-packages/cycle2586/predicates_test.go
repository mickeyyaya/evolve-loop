//go:build acs

package cycle2586

import (
	"bytes"
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/events/channel"
	"github.com/mickeyyaya/evolve-loop/go/internal/events/publisher"
	"github.com/mickeyyaya/evolve-loop/go/internal/events/rootwire"
	"github.com/mickeyyaya/evolve-loop/go/internal/paths"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

var unregisteredRoles = []string{"integration", "auth", "queue", "latency", "retire"}

var (
	buildOnce   sync.Once
	builtBinary string
	buildOutput string
	buildCode   int
)

func evolveBinary(t *testing.T) string {
	t.Helper()
	buildOnce.Do(func() {
		dir, err := os.MkdirTemp("", "evolve-cycle2586-")
		if err != nil {
			buildCode = -1
			buildOutput = err.Error()
			return
		}
		builtBinary = filepath.Join(dir, "evolve")
		stdout, stderr, code, _ := acsassert.SubprocessOutput("go", "-C", acsassert.GoModuleDir(t), "build", "-o", builtBinary, "./cmd/evolve")
		buildCode = code
		buildOutput = stdout + stderr
	})
	if buildCode != 0 {
		t.Fatalf("go build ./cmd/evolve exit %d\n%s", buildCode, acsassert.OutputTail(buildOutput, 40))
	}
	return builtBinary
}

func signalRecords(t *testing.T, evolveDir string) []channel.Record {
	t.Helper()
	log, err := channel.New(paths.EventChannelsRoot(evolveDir), "signals", channel.Config{SegmentBytes: 1 << 20, LockDeadline: time.Minute})
	if err != nil {
		t.Fatalf("channel.New(signals) = %v", err)
	}
	batch, err := log.Read(0)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		t.Fatalf("Read(signals) = %v", err)
	}
	return batch.Records
}

func emitShipLanded(role, evolveDir string, stderr *bytes.Buffer) {
	center := signalcenter.New(signalcenter.WithPID(4242))
	closeRoot := rootwire.Attach(center, role, evolveDir, stderr)
	center.Emit(signalcenter.Event{
		Cycle: 7, Module: signalcenter.ModuleShip, Origin: "Test.emit", Kind: signalcenter.KindShipLanded,
		Severity: signalcenter.SeverityInfo, Reason: "landed",
	})
	closeRoot()
}

func mainSources(t *testing.T) []*ast.File {
	t.Helper()
	dir := filepath.Join(acsassert.GoModuleDir(t), "cmd", "evolve")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var files []*ast.File
	fset := token.NewFileSet()
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") || strings.HasSuffix(e.Name(), "_test.go") {
			continue
		}
		file, err := parser.ParseFile(fset, filepath.Join(dir, e.Name()), nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", e.Name(), err)
		}
		files = append(files, file)
	}
	return files
}

func stringConstants(files []*ast.File) map[string]string {
	consts := map[string]string{}
	for _, file := range files {
		for _, decl := range file.Decls {
			gen, ok := decl.(*ast.GenDecl)
			if !ok || gen.Tok != token.CONST {
				continue
			}
			for _, spec := range gen.Specs {
				vs := spec.(*ast.ValueSpec)
				if len(vs.Names) != 1 || len(vs.Values) != 1 {
					continue
				}
				if lit, ok := vs.Values[0].(*ast.BasicLit); ok && lit.Kind == token.STRING {
					consts[vs.Names[0].Name], _ = strconv.Unquote(lit.Value)
				}
			}
		}
	}
	return consts
}

func callSiteRoles(files []*ast.File) []string {
	consts := stringConstants(files)
	found := map[string]bool{}
	for _, file := range files {
		ast.Inspect(file, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok || len(call.Args) == 0 {
				return true
			}
			callee, ok := call.Fun.(*ast.Ident)
			if !ok || (callee.Name != "newRoleSignalCenter" && callee.Name != "newRoleSignalCenterWithAttach") {
				return true
			}
			switch arg := call.Args[0].(type) {
			case *ast.BasicLit:
				role, _ := strconv.Unquote(arg.Value)
				found[role] = true
			case *ast.Ident:
				if role, ok := consts[arg.Name]; ok {
					found[role] = true
				}
			}
			return true
		})
	}
	return slices.Sorted(maps.Keys(found))
}

func TestC2586_001_EveryRoleThatACallSitePassesIsAPublisherRole(t *testing.T) {
	roles := callSiteRoles(mainSources(t))

	for _, floor := range []string{"ship", "quality", "integration"} {
		if !slices.Contains(roles, floor) {
			t.Fatalf("the call sites of cmd/evolve yield roles %v without %q: the derivation lost a root", roles, floor)
		}
	}
	var refused []string
	for _, role := range roles {
		if !slices.Contains(publisher.Roles(), role) {
			refused = append(refused, role)
		}
	}
	if len(refused) > 0 {
		t.Errorf("roles %v passed at call sites are not one of publisher.Roles() %v: Config.check refuses them and rootwire.Attach drops every event", refused, publisher.Roles())
	}
}

func TestC2586_002_RolesIsTheClosedSetOfTheTwentySpecRoots(t *testing.T) {
	acsassert.GoTests(t, acsassert.GoTestSpec{
		Dir:     acsassert.GoModuleDir(t),
		Package: "./internal/events/publisher",
		Pattern: "^TestRoles_IsTheClosedSetOfTheSpecRoots$",
		Names:   []string{"TestRoles_IsTheClosedSetOfTheSpecRoots"},
	})
	got := publisher.Roles()
	if len(got) != 20 {
		t.Errorf("publisher.Roles() lists %d roles %v, want 20: the 15 of the spec and %v", len(got), got, unregisteredRoles)
	}
	for _, role := range unregisteredRoles {
		if !slices.Contains(got, role) {
			t.Errorf("publisher.Roles() = %v lacks the role %q", got, role)
		}
	}
}

func TestC2586_003_AnIntegrationRootEmitReachesTheRootBus(t *testing.T) {
	evolveDir := t.TempDir()
	var stderr bytes.Buffer

	emitShipLanded("integration", evolveDir, &stderr)

	records := signalRecords(t, evolveDir)
	if len(records) != 1 || records[0].Source != "integration" || records[0].Signal == nil || records[0].Signal.Kind != signalcenter.KindShipLanded {
		t.Errorf("signals channel holds %+v, want one ship.landed record with source integration", records)
	}
	if strings.Contains(stderr.String(), "publishes no events") {
		t.Errorf("rootwire.Attach warned that role integration publishes no events: %s", stderr.String())
	}
}

func TestC2586_004_EveryFormerlyRefusedRoleReachesTheRootBusUnderItsOwnName(t *testing.T) {
	for _, role := range unregisteredRoles {
		evolveDir := t.TempDir()
		var stderr bytes.Buffer

		emitShipLanded(role, evolveDir, &stderr)

		records := signalRecords(t, evolveDir)
		if len(records) != 1 || records[0].Source != role {
			t.Errorf("role %s: signals channel holds %+v, want one record with source %s (stderr: %s)", role, records, role, stderr.String())
		}
	}
}

func TestC2586_005_ARoleOutsideTheClosedSetStillPublishesNothingAndWarnsOnce(t *testing.T) {
	evolveDir := t.TempDir()
	var stderr bytes.Buffer

	emitShipLanded("not-a-spec-root", evolveDir, &stderr)

	if records := signalRecords(t, evolveDir); len(records) != 0 {
		t.Errorf("an unregistered role published %+v: the set must stay closed", records)
	}
	if got := strings.Count(stderr.String(), "publishes no events"); got != 1 {
		t.Errorf("stderr carries %d refusal warnings, want exactly 1: %s", got, stderr.String())
	}
}

func TestC2586_006_SignalsCodesCheckPasses(t *testing.T) {
	out, errOut, code, err := acsassert.SubprocessOutput(evolveBinary(t), "signals", "codes", "check")
	if err != nil || code != 0 {
		t.Fatalf("evolve signals codes check exit %d (err=%v)\nstdout: %s\nstderr: %s", code, err, out, errOut)
	}
}

func TestC2586_007_TheEventChannelsSpecListsEveryRoleThatThePublisherAccepts(t *testing.T) {
	acsassert.GoTests(t, acsassert.GoTestSpec{
		Dir:     acsassert.GoModuleDir(t),
		Package: "./internal/events/publisher",
		Pattern: "^TestRoles_AreTheRolesTheEventChannelsSpecLists$",
		Names:   []string{"TestRoles_AreTheRolesTheEventChannelsSpecLists"},
	})
	raw, err := os.ReadFile(filepath.Join(acsassert.RepoRoot(t), "docs", "architecture", "event-channels.md"))
	if err != nil {
		t.Fatal(err)
	}
	_, afterLead, found := strings.Cut(string(raw), "names the root:")
	if !found {
		t.Fatal("event-channels.md has no roles bullet")
	}
	roleList, _, _ := strings.Cut(afterLead, "`. ")
	roleList += "`"
	for _, role := range publisher.Roles() {
		if !strings.Contains(roleList, "`"+role+"`") {
			t.Errorf("event-channels.md §1 does not list the role %q that publisher.Roles() accepts", role)
		}
	}
}
