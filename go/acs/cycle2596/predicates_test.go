//go:build acs

package cycle2596

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

func callSiteRoles(t *testing.T) []string {
	t.Helper()
	dir := filepath.Join(acsassert.GoModuleDir(t), "cmd", "evolve")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	fset := token.NewFileSet()
	var files []*ast.File
	consts := map[string]string{}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") || strings.HasSuffix(e.Name(), "_test.go") {
			continue
		}
		file, err := parser.ParseFile(fset, filepath.Join(dir, e.Name()), nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", e.Name(), err)
		}
		files = append(files, file)
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

func emitShipLanded(role, evolveDir string, stderr *bytes.Buffer) {
	center := signalcenter.New(signalcenter.WithPID(4242))
	closeRoot := rootwire.Attach(center, role, evolveDir, stderr)
	center.Emit(signalcenter.Event{
		Cycle: 7, Module: signalcenter.ModuleShip, Origin: "Test.emit", Kind: signalcenter.KindShipLanded,
		Severity: signalcenter.SeverityInfo, Reason: "landed",
	})
	closeRoot()
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

func TestC2596_001_EveryRoleACallSitePassesIsAPublisherRole(t *testing.T) {
	roles := callSiteRoles(t)

	for _, role := range unregisteredRoles {
		if !slices.Contains(roles, role) {
			t.Fatalf("the call sites of cmd/evolve yield roles %v without %q: the derivation lost a root", roles, role)
		}
	}
	for _, role := range roles {
		if !slices.Contains(publisher.Roles(), role) {
			t.Errorf("role %q passed at a call site is not one of publisher.Roles() %v: Config.check refuses it and rootwire.Attach drops every event of that root", role, publisher.Roles())
		}
	}
}

func TestC2596_002_RolesIsTheClosedSetOfTheSpecRootsAndNamesTheFiveRefusedRoles(t *testing.T) {
	acsassert.GoTests(t, acsassert.GoTestSpec{
		Dir:     acsassert.GoModuleDir(t),
		Package: "./internal/events/publisher",
		Pattern: "^TestRoles_IsTheClosedSetOfTheSpecRoots$",
		Names:   []string{"TestRoles_IsTheClosedSetOfTheSpecRoots"},
	})
	got := publisher.Roles()
	if len(got) != 20 {
		t.Errorf("publisher.Roles() lists %d roles %v, want 20", len(got), got)
	}
	for _, role := range unregisteredRoles {
		if !slices.Contains(got, role) {
			t.Errorf("publisher.Roles() = %v lacks the role %q", got, role)
		}
	}
}

func TestC2596_003_AnIntegrationRootEmitReachesTheRootBus(t *testing.T) {
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

func TestC2596_004_EveryFormerlyRefusedRoleReachesTheRootBusUnderItsOwnName(t *testing.T) {
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

func TestC2596_005_ARoleOutsideTheClosedSetStillPublishesNothingAndWarnsOnce(t *testing.T) {
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

func TestC2596_006_SignalsCodesCheckPasses(t *testing.T) {
	binary := filepath.Join(t.TempDir(), "evolve")
	_, buildErr, code, err := acsassert.SubprocessOutput("go", "-C", acsassert.GoModuleDir(t), "build", "-o", binary, "./cmd/evolve")
	if err != nil || code != 0 {
		t.Fatalf("go build ./cmd/evolve exit %d (err=%v)\n%s", code, err, acsassert.OutputTail(buildErr, 40))
	}
	out, errOut, code, err := acsassert.SubprocessOutput(binary, "signals", "codes", "check")
	if err != nil || code != 0 {
		t.Fatalf("evolve signals codes check exit %d (err=%v)\nstdout: %s\nstderr: %s", code, err, out, errOut)
	}
}
