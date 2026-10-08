package inboxbatch_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/inboxbatch"
)

var loopStampKeys = []string{"consumed", "continuation", "failure_count", "git_sha", "last_failure_reason", "released_continuations",
	"retired_cycle", "retired_reason", "route", "routed_at", "routed_cycle", "routed_reason", "unbacked"}

var operatorStampKeys = []string{"premise_verified_at", "premise_verified_evidence", "premise_verified_sha"}

func TestRoleOf_EveryItemFieldHasOneOwner(t *testing.T) {
	for key, want := range map[string]inboxbatch.FieldRole{
		"title":                     {Owner: inboxbatch.CuratorOwned, Shape: inboxbatch.TextValue},
		"id":                        {Owner: inboxbatch.CuratorOwned, Shape: inboxbatch.TextValue},
		"weight":                    {Owner: inboxbatch.CuratorOwned, Shape: inboxbatch.NumberValue},
		"deps":                      {Owner: inboxbatch.CuratorOwned, Shape: inboxbatch.ListValue},
		"route":                     {Owner: inboxbatch.LoopStampOwned},
		"routed_reason":             {Owner: inboxbatch.LoopStampOwned},
		"failure_count":             {Owner: inboxbatch.LoopStampOwned, Shape: inboxbatch.CounterValue},
		"premise_verified_evidence": {Owner: inboxbatch.OperatorStampOwned},
		"kind":                      {Owner: inboxbatch.AuthorOwned},
		"created_at":                {Owner: inboxbatch.AuthorOwned},
		"injected_by":               {Owner: inboxbatch.AuthorOwned},
		"routed_note":               {Owner: inboxbatch.AuthorOwned},
		"premise_verified_by":       {Owner: inboxbatch.AuthorOwned},
	} {
		if got := inboxbatch.RoleOf(key); got != want {
			t.Errorf("RoleOf(%q) = %+v, want %+v", key, got, want)
		}
	}
}

func TestFieldRole_IsCounterNamesOnlyTheMonotonicLoopCounters(t *testing.T) {
	for key, want := range map[string]bool{"failure_count": true, "routed_cycle": false, "last_failure_reason": false, "weight": false, "kind": false} {
		if got := inboxbatch.RoleOf(key).IsCounter(); got != want {
			t.Errorf("RoleOf(%q).IsCounter() = %v, want %v", key, got, want)
		}
	}
}

func TestRoleOf_EachCuratedFieldCarriesTheShapeAnEditParses(t *testing.T) {
	for shape, keys := range map[inboxbatch.ValueShape][]string{
		inboxbatch.TextValue:   {"fix", "id", "priority_class", "summary", "title"},
		inboxbatch.NumberValue: {"weight"},
		inboxbatch.ListValue:   {"acceptance", "connects_to", "deps", "files"},
	} {
		for _, key := range keys {
			if got := inboxbatch.RoleOf(key).Shape; got != shape {
				t.Errorf("RoleOf(%q).Shape = %q, want %q", key, got, shape)
			}
		}
	}
}

func TestFieldsOwnedBy_ListsEachOwnersExactKeysSorted(t *testing.T) {
	for owner, want := range map[inboxbatch.FieldOwner][]string{
		inboxbatch.CuratorOwned:       {"acceptance", "connects_to", "deps", "files", "fix", "id", "priority_class", "summary", "title", "weight"},
		inboxbatch.LoopStampOwned:     loopStampKeys,
		inboxbatch.OperatorStampOwned: operatorStampKeys,
		inboxbatch.AuthorOwned:        nil,
	} {
		if got := inboxbatch.FieldsOwnedBy(owner); !slices.Equal(got, want) {
			t.Errorf("FieldsOwnedBy(%s) = %v, want %v", owner, got, want)
		}
	}
	for _, key := range append(append([]string{}, loopStampKeys...), operatorStampKeys...) {
		if !inboxbatch.RoleOf(key).IsStamp() {
			t.Errorf("RoleOf(%q).IsStamp() = false", key)
		}
	}
	if inboxbatch.RoleOf("weight").IsStamp() || inboxbatch.RoleOf("kind").IsStamp() {
		t.Error("a curated or authored field is no stamp")
	}
}

func TestConsoleRouted_NoStampIsTheItemsText(t *testing.T) {
	const protected = "go/internal/bridge/x.go:12 is where it breaks"
	for _, key := range append(append([]string{}, loopStampKeys...), operatorStampKeys...) {
		if key == "route" {
			continue
		}
		if ok, reason := routed(t, `{"id":"s","kind":"bug","`+key+`":{"note":"`+protected+`"}}`); ok {
			t.Errorf("a %s stamp naming a protected file routed the item: %q", key, reason)
		}
	}
	for _, key := range []string{"routed_note", "premise_verified_by", "summary"} {
		if ok, _ := routed(t, `{"id":"a","kind":"bug","`+key+`":{"note":"`+protected+`"}}`); !ok {
			t.Errorf("%s is authored text, but the classifier skipped it", key)
		}
	}
}

func TestStampFieldConstants_NameEveryStampAndNothingElse(t *testing.T) {
	file, err := parser.ParseFile(token.NewFileSet(), "fieldowner.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	var named []string
	for _, decl := range file.Decls {
		gen, ok := decl.(*ast.GenDecl)
		if !ok || gen.Tok != token.CONST {
			continue
		}
		for _, spec := range gen.Specs {
			vs := spec.(*ast.ValueSpec)
			for i, name := range vs.Names {
				lit, isString := vs.Values[i].(*ast.BasicLit)
				if !name.IsExported() || !strings.HasSuffix(name.Name, "Field") || !isString {
					continue
				}
				key, _ := strconv.Unquote(lit.Value)
				if !inboxbatch.RoleOf(key).IsStamp() {
					t.Errorf("%s = %q is a field constant but no stamp in the owner table", name.Name, key)
				}
				named = append(named, key)
			}
		}
	}
	slices.Sort(named)
	stamps := append(append([]string{}, loopStampKeys...), operatorStampKeys...)
	slices.Sort(stamps)
	if !slices.Equal(named, stamps) {
		t.Errorf("field constants name %v, want every stamp %v: a writer spells a stamp key only through its constant", named, stamps)
	}
}

func TestStampFieldConstants_SpellTheKeysTheInboxHolds(t *testing.T) {
	for _, pair := range [][2]string{
		{inboxbatch.RouteField, "route"},
		{inboxbatch.RoutedReasonField, "routed_reason"},
		{inboxbatch.RoutedCycleField, "routed_cycle"},
		{inboxbatch.RoutedAtField, "routed_at"},
		{inboxbatch.FailureCountField, "failure_count"},
		{inboxbatch.LastFailureReasonField, "last_failure_reason"},
		{inboxbatch.ContinuationField, "continuation"},
		{inboxbatch.ReleasedContinuationsField, "released_continuations"},
		{inboxbatch.ConsumedField, "consumed"},
		{inboxbatch.UnbackedField, "unbacked"},
		{inboxbatch.RetiredReasonField, "retired_reason"},
		{inboxbatch.RetiredCycleField, "retired_cycle"},
		{inboxbatch.GitSHAField, "git_sha"},
		{inboxbatch.PremiseVerifiedAtField, "premise_verified_at"},
		{inboxbatch.PremiseVerifiedSHAField, "premise_verified_sha"},
		{inboxbatch.PremiseVerifiedEvidenceField, "premise_verified_evidence"},
	} {
		if pair[0] != pair[1] {
			t.Errorf("a stamp constant spells %q, but items on disk carry %q", pair[0], pair[1])
		}
	}
}
