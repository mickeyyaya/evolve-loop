package core

// infra_teardown_single_source_test.go — RED contract for cycle-1166 Task 2
// (infra-teardown-predicate-single-source, inbox weight 0.86).
//
// The union predicate "is this a bridge infra teardown?" —
//
//	errors.Is(err, ErrArtifactTimeout) || errors.Is(err, ErrTransientBridgeFailure)
//
// — now has a single-source home (core.IsInfraTeardownError, errors.go:81) but
// is STILL hand-spelled at call sites (most notably optionalInfraSkip in
// orchestrator.go, which spells the De Morgan negation
// `!errors.Is(err, ErrArtifactTimeout) && !isTransientBridgeError(err)`).
// That is the 7th spelling of one concept: if a THIRD teardown sentinel is ever
// added, every hand-spelled site must be found and updated or the definitions
// silently diverge.
//
// This is a BEHAVIOR-PRESERVING refactor, so most assertions here are pins that
// must stay green before AND after (they encode "you did not change semantics").
// The one genuinely RED assertion is the UNIQUENESS scan.
//
// The item's own loudest warning is the anti-goal: "this item's whole risk is a
// blind widen of a timeout-only or transient-only site into the union." The
// negative tests below encode exactly that.

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

// TestIsInfraTeardownError_UnionSemantics pins the single source's meaning:
// BOTH sentinels (wrapped or bare) are infra teardowns; nothing else is.
// Must stay green across the refactor — it is the contract every adopting site
// inherits.
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

// TestIsTransientBridgeError_StaysTransientOnly is the ANTI-WIDEN negative test
// the item calls its whole risk. isTransientBridgeError is a COMPONENT of the
// union, not the union: it must keep returning false for ErrArtifactTimeout.
// A "consolidation" that aliases it to IsInfraTeardownError fails here.
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

// TestOptionalInfraSkip_InfraGateUnchangedAfterConsolidation pins the
// behavior of the single loudest adoption site named by the item
// (orchestrator.go optionalInfraSkip, whose guard is exactly
// !IsInfraTeardownError(err)). Adopting the helper must not change ANY of these
// verdicts — including the negative, which forbids collapsing the infra gate
// into "always skip".
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

// TestOptionalInfraSkip_InfraGateAgreesWithIsInfraTeardownError is the
// EQUIVALENCE proof that justifies the replacement: for a phase that clears
// every OTHER guard (optional, off-floor, non-mandatory), optionalInfraSkip's
// verdict must equal IsInfraTeardownError's verdict for every error shape. If
// these ever disagree, the site does NOT mean the same concept and must not
// adopt the helper.
func TestOptionalInfraSkip_InfraGateAgreesWithIsInfraTeardownError(t *testing.T) {
	o := amplNewSkipOrchestrator(t, nil, nil, optionalSpecFor("learn"))
	for _, err := range []error{
		ErrArtifactTimeout,
		ErrTransientBridgeFailure,
		fmt.Errorf("wrapped: %w", ErrArtifactTimeout),
		errors.New("plain logic error"),
	} {
		want := IsInfraTeardownError(err)
		if got := o.optionalInfraSkip(Phase("learn"), err); got != want {
			t.Errorf("for err=%v: optionalInfraSkip=%v but IsInfraTeardownError=%v — the site's infra gate "+
				"must be exactly the union predicate for the adoption to be sound", err, got, want)
		}
	}
}

// TestInfraTeardownUnion_SpelledExactlyOnce is the RED uniqueness scan (AC-2:
// "a grep-style assertion documents that (timeout OR transient) is spelled
// exactly ONCE via IsInfraTeardownError").
//
// It parses every non-test .go file in this package and counts boolean
// expressions that combine the two sentinels — in either polarity:
//
//	errors.Is(e, ErrArtifactTimeout) || errors.Is(e, ErrTransientBridgeFailure)   (union)
//	!errors.Is(e, ErrArtifactTimeout) && !isTransientBridgeError(e)               (De Morgan)
//
// Exactly one such expression may exist: the body of IsInfraTeardownError.
// This is a UNIQUENESS assertion, not a "source contains string X" check — it
// cannot be satisfied by ADDING text, only by REMOVING the duplicate spellings,
// so it is not a degenerate source-grep predicate.
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
