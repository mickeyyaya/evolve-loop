package core

import (
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/phasespec"
)

func TestIsInfraTeardownError_UnionSemantics(t *testing.T) {
	for _, tc := range []struct {
		name string
		err  error
		want bool
	}{
		{"bare artifact timeout", ErrArtifactTimeout, true},
		{"bare transient bridge failure", ErrTransientBridgeFailure, true},
		{"wrapped artifact timeout", fmt.Errorf("bridge dispatch: %w", ErrArtifactTimeout), true},
		{"wrapped transient bridge failure", fmt.Errorf("driver bounce: %w", ErrTransientBridgeFailure), true},
		{"unrelated logic error", errors.New("nil pointer dereference"), false},
		{"nil error", nil, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := IsInfraTeardownError(tc.err); got != tc.want {
				t.Errorf("IsInfraTeardownError(%v) = %v, want %v", tc.err, got, tc.want)
			}
		})
	}
}

func TestIsTransientBridgeError_StaysTransientOnly(t *testing.T) {
	if !isTransientBridgeError(ErrTransientBridgeFailure) {
		t.Error("isTransientBridgeError must still match ErrTransientBridgeFailure — it is the reusable transient-only component")
	}
	if isTransientBridgeError(ErrArtifactTimeout) {
		t.Error("isTransientBridgeError(ErrArtifactTimeout) = true — the transient-only component was WIDENED into the union; " +
			"this is the exact blind-widen failure mode the item warns about")
	}
	if isTransientBridgeError(errors.New("logic bug")) {
		t.Error("isTransientBridgeError must never match a non-sentinel error")
	}
}

func TestOptionalInfraSkip_InfraGateUnchangedAfterConsolidation(t *testing.T) {
	o := amplNewSkipOrchestrator(t, nil, nil, optionalSpecFor("learn"))

	for _, tc := range []struct {
		name string
		err  error
		want bool
	}{
		{"artifact timeout is infra-shaped", ErrArtifactTimeout, true},
		{"transient bridge failure is infra-shaped", ErrTransientBridgeFailure, true},
		{"wrapped transient is infra-shaped", fmt.Errorf("relaunch: %w", ErrTransientBridgeFailure), true},
		{"logic error is NOT infra-shaped", errors.New("index out of range"), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := o.optionalInfraSkip(Phase("learn"), tc.err); got != tc.want {
				t.Errorf("optionalInfraSkip(learn, %v) = %v, want %v — the consolidation must be behavior-preserving",
					tc.err, got, tc.want)
			}
		})
	}
}

func TestOptionalInfraSkip_GateAgreesWithIsOptionalSkippableError(t *testing.T) {
	o := amplNewSkipOrchestrator(t, nil, nil, optionalSpecFor("learn"))
	for _, err := range []error{
		ErrArtifactTimeout,
		ErrTransientBridgeFailure,
		ErrAgentDocMissing,
		fmt.Errorf("wrapped: %w", ErrArtifactTimeout),
		fmt.Errorf("phase learn: load agent: %w", ErrAgentDocMissing),
		errors.New("plain logic error"),
	} {
		want := IsOptionalSkippableError(err)
		if got := o.optionalInfraSkip(Phase("learn"), err); got != want {
			t.Errorf("for err=%v: optionalInfraSkip=%v but IsOptionalSkippableError=%v — the site's error gate "+
				"must be exactly the single-source predicate for the adoption to be sound", err, got, want)
		}
	}
}

func TestInfraTeardownUnion_SpelledExactlyOnce(t *testing.T) {
	sites, err := findInfraTeardownUnionSpellings(".")
	if err != nil {
		t.Fatalf("scan internal/core: %v", err)
	}
	sort.Strings(sites)

	const canonical = "errors.go:IsInfraTeardownError"
	if len(sites) != 1 || !strings.HasPrefix(sites[0], canonical) {
		t.Fatalf("the (timeout OR transient) union is spelled at %d site(s): %v\n"+
			"want EXACTLY one — %s. Every other site that means the same concept must call "+
			"IsInfraTeardownError; sites that are timeout-ONLY or transient-ONLY are different "+
			"predicates and must NOT be collapsed into it.",
			len(sites), sites, canonical)
	}
}

// optionalSpecFor is a tiny local helper so the table tests above read cleanly;
// it mirrors the catalog shape amplNewSkipOrchestrator expects.
func optionalSpecFor(name string) []phasespec.PhaseSpec {
	return []phasespec.PhaseSpec{{Name: name, Optional: true}}
}

// findInfraTeardownUnionSpellings walks dir's non-test Go files and returns
// "file:funcName" for each function whose body contains a binary expression
// joining the artifact-timeout sentinel and the transient-bridge sentinel
// (either as `||` of positives or `&&` of negations).
func findInfraTeardownUnionSpellings(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var sites []string
	fset := token.NewFileSet()
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		f, perr := parser.ParseFile(fset, filepath.Join(dir, name), nil, 0)
		if perr != nil {
			return nil, fmt.Errorf("parse %s: %w", name, perr)
		}
		ast.Inspect(f, func(n ast.Node) bool {
			fn, ok := n.(*ast.FuncDecl)
			if !ok || fn.Body == nil {
				return true
			}
			ast.Inspect(fn.Body, func(inner ast.Node) bool {
				be, ok := inner.(*ast.BinaryExpr)
				if !ok || (be.Op != token.LOR && be.Op != token.LAND) {
					return true
				}
				src := exprText(fset, be)
				if mentionsTimeoutSentinel(src) && mentionsTransientSentinel(src) {
					sites = append(sites, fmt.Sprintf("%s:%s", name, fn.Name.Name))
					return false // one report per spelling, not per nested node
				}
				return true
			})
			return true
		})
	}
	return dedupeStrings(sites), nil
}

func exprText(fset *token.FileSet, e ast.Expr) string {
	start := fset.Position(e.Pos())
	end := fset.Position(e.End())
	data, err := os.ReadFile(start.Filename)
	if err != nil || end.Offset > len(data) {
		return ""
	}
	return string(data[start.Offset:end.Offset])
}

func mentionsTimeoutSentinel(src string) bool {
	return strings.Contains(src, "ErrArtifactTimeout")
}

// The transient half may appear either as the sentinel itself or via its
// transient-ONLY component helper — both spell "transient bridge failure".
func mentionsTransientSentinel(src string) bool {
	return strings.Contains(src, "ErrTransientBridgeFailure") || strings.Contains(src, "isTransientBridgeError")
}

func dedupeStrings(in []string) []string {
	seen := make(map[string]bool, len(in))
	out := make([]string, 0, len(in))
	for _, s := range in {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}
