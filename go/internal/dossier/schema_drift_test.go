package dossier

// schema_drift_test.go — the TestSchema_NoDrift that ADR-0055 promised
// ("The Go struct is the SSOT; schemas/cycle-dossier.schema.json is a derived
// artifact. The TestSchema_NoDrift drift test guards this in CI.") and that
// did not exist. Its absence is why the schema silently rotted: three fields
// (skipped_phases, spine_fail_opens, timing) were added to the struct and
// never to the schema, and the schema declares additionalProperties:false —
// so every real dossier would have FAILED validation by any external tool
// that took the committed schema at its word.
//
// The check is BIDIRECTIONAL on names and structural on shape.
//
// Bidirectional, because a one-way "every Go field appears in the schema"
// test still lets a removed field linger in the schema forever, and a
// schema-side-only test lets a new Go field rot exactly the way these did.
//
// Structural, because a name-only inventory is green while the schema is
// unusable. Concretely: swapping the items.$ref of skipped_phases and
// phases_run_verdict_not_adopted keeps every property name identical, yet the
// two definitions carry disjoint required keys, so every real dossier
// carrying either array is rejected. Shape is checked for the three axes that
// can produce that class of false rejection — $ref target, JSON type, and
// required — rather than by pulling in a JSON-Schema validator dependency
// into a module that deliberately has two.
//
// The `required` check is deliberately ONE-WAY: every schema-required field
// must be a Go field that is always marshalled (no omitempty). The reverse is
// not asserted, because a non-omitempty field absent from `required` is merely
// permissive, while a required field the writer may omit rejects valid
// documents. Assert the direction that can cause a false rejection.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasetiming"
)

// schemaObjects maps each object in cycle-dossier.schema.json to the Go type
// it is derived from. The empty key is the schema root. Every entry under
// "definitions" must appear here, and the test asserts that too — so adding a
// definition without naming its Go type fails rather than going unchecked.
var schemaObjects = map[string]reflect.Type{
	"":                  reflect.TypeOf(Dossier{}),
	"PhaseRecord":       reflect.TypeOf(PhaseRecord{}),
	"Defect":            reflect.TypeOf(Defect{}),
	"Lesson":            reflect.TypeOf(Lesson{}),
	"Carryover":         reflect.TypeOf(Carryover{}),
	"FailureRecord":     reflect.TypeOf(FailureRecord{}),
	"CIWatchRecord":     reflect.TypeOf(CIWatchRecord{}),
	"SkippedPhase":      reflect.TypeOf(cyclestate.SkippedPhase{}),
	"VerdictNotAdopted": reflect.TypeOf(cyclestate.VerdictNotAdopted{}),
	"SpineFailOpen":     reflect.TypeOf(cyclestate.SpineFailOpen{}),
	"TimingSummary":     reflect.TypeOf(phasetiming.Summary{}),
	"TokenUsage":        reflect.TypeOf(cyclestate.TokenUsage{}),
}

type schemaProp struct {
	Ref   string `json:"$ref"`
	Type  string `json:"type"`
	Items *struct {
		Ref  string `json:"$ref"`
		Type string `json:"type"`
	} `json:"items"`
}

type schemaObject struct {
	Required   []string              `json:"required"`
	Properties map[string]schemaProp `json:"properties"`
}

type schemaDoc struct {
	schemaObject
	Definitions map[string]schemaObject `json:"definitions"`
}

// wireField is one field as encoding/json will actually emit it.
type wireField struct {
	name      string
	typ       reflect.Type
	omitempty bool
}

// wireFields returns the fields t marshals, in declaration order. Untagged
// embedded structs are flattened the way encoding/json promotes them — the
// test's own schemaDoc relies on that promotion, so mis-modelling it here
// would be a check that cannot describe its own helper types.
func wireFields(t reflect.Type) []wireField {
	var out []wireField
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		tag := f.Tag.Get("json")
		if f.Anonymous && tag == "" {
			et := f.Type
			if et.Kind() == reflect.Pointer {
				et = et.Elem()
			}
			if et.Kind() == reflect.Struct {
				out = append(out, wireFields(et)...)
				continue
			}
		}
		if f.PkgPath != "" {
			continue // unexported: never on the wire
		}
		if tag == "-" {
			continue // deliberately not persisted (in-memory-only carriers)
		}
		name, opts, _ := strings.Cut(tag, ",")
		if name == "" {
			name = f.Name // Go's default when no tag is given
		}
		out = append(out, wireField{
			name:      name,
			typ:       f.Type,
			omitempty: strings.Contains(opts, "omitempty"),
		})
	}
	return out
}

func names(fs []wireField) []string {
	out := make([]string, 0, len(fs))
	for _, f := range fs {
		out = append(out, f.name)
	}
	return out
}

// defNameFor returns the schema definition name registered for t, if any.
func defNameFor(t reflect.Type) string {
	if t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	for name, rt := range schemaObjects {
		if name != "" && rt == t {
			return name
		}
	}
	return ""
}

// wantShape describes how the schema must spell a Go type. Returns a
// human-readable expectation and a predicate over the schema property.
func wantShape(t reflect.Type) (string, func(schemaProp) bool) {
	if def := defNameFor(t); def != "" {
		ref := "#/definitions/" + def
		return "$ref " + ref, func(p schemaProp) bool { return p.Ref == ref }
	}
	switch t.Kind() {
	case reflect.Pointer:
		return wantShape(t.Elem())
	case reflect.Slice, reflect.Array:
		elem := t.Elem()
		if def := defNameFor(elem); def != "" {
			ref := "#/definitions/" + def
			return "array of $ref " + ref, func(p schemaProp) bool {
				return p.Type == "array" && p.Items != nil && p.Items.Ref == ref
			}
		}
		ek, _ := jsonKind(elem)
		return "array of " + ek, func(p schemaProp) bool {
			return p.Type == "array" && p.Items != nil && p.Items.Type == ek
		}
	default:
		k, ok := jsonKind(t)
		if !ok {
			return "", nil // not modelled; name check still applies
		}
		return "type " + k, func(p schemaProp) bool { return p.Type == k }
	}
}

func jsonKind(t reflect.Type) (string, bool) {
	switch t.Kind() {
	case reflect.String:
		return "string", true
	case reflect.Bool:
		return "boolean", true
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return "integer", true
	case reflect.Float32, reflect.Float64:
		return "number", true
	case reflect.Map, reflect.Struct:
		return "object", true
	case reflect.Slice, reflect.Array:
		return "array", true
	default:
		return "", false
	}
}

func repoRootFromTest(t *testing.T) string {
	t.Helper()
	// This file lives at <root>/go/internal/dossier/.
	root, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatalf("resolve repo root: %v", err)
	}
	return root
}

func loadSchema(t *testing.T) schemaDoc {
	t.Helper()
	path := filepath.Join(repoRootFromTest(t), "schemas", "cycle-dossier.schema.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v — the schema is a committed artifact; a missing file is drift, not a skip", path, err)
	}
	var doc schemaDoc
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	return doc
}

func TestSchema_NoDrift(t *testing.T) {
	doc := loadSchema(t)

	// Every definition must be registered, and every registration must exist.
	for name := range doc.Definitions {
		if _, ok := schemaObjects[name]; !ok {
			t.Errorf("schema definition %q has no Go type in schemaObjects — register it so drift in it is checked too", name)
		}
	}
	for name := range schemaObjects {
		if name == "" {
			continue
		}
		if _, ok := doc.Definitions[name]; !ok {
			t.Errorf("schemaObjects names definition %q but the schema has no such definition", name)
		}
	}

	for name, goType := range schemaObjects {
		obj, ok := doc.Definitions[name]
		if name == "" {
			obj = doc.schemaObject
		} else if !ok {
			continue // already reported above
		}
		label := name
		if label == "" {
			label = "(root)"
		}

		fields := wireFields(goType)

		// 1. Name inventory, both directions.
		want := append([]string(nil), names(fields)...)
		got := make([]string, 0, len(obj.Properties))
		for k := range obj.Properties {
			got = append(got, k)
		}
		sort.Strings(want)
		sort.Strings(got)
		if !reflect.DeepEqual(want, got) {
			t.Errorf("%s (%s): schema properties drifted from the Go struct\n  missing from schema: %v\n  absent from Go:      %v",
				label, goType, missing(want, got), missing(got, want))
		}

		// 2. Shape: $ref target and JSON type. A name-only check is green
		//    while the schema rejects every real document.
		byName := make(map[string]wireField, len(fields))
		for _, f := range fields {
			byName[f.name] = f
		}
		for _, f := range fields {
			p, ok := obj.Properties[f.name]
			if !ok {
				continue // already reported by the inventory check
			}
			desc, pred := wantShape(f.typ)
			if pred == nil {
				continue
			}
			if !pred(p) {
				t.Errorf("%s.%s (Go %s): schema shape drifted — want %s, schema has %s",
					label, f.name, f.typ, desc, describe(p))
			}
		}

		// 3. required must never name a field the writer may omit.
		for _, req := range obj.Required {
			f, ok := byName[req]
			if !ok {
				continue // inventory check reports it
			}
			if f.omitempty {
				t.Errorf("%s: schema requires %q but the Go field is omitempty — valid dossiers that omit it would be rejected", label, req)
			}
		}
	}
}

func describe(p schemaProp) string {
	switch {
	case p.Ref != "":
		return "$ref " + p.Ref
	case p.Type == "array" && p.Items != nil && p.Items.Ref != "":
		return "array of $ref " + p.Items.Ref
	case p.Type == "array" && p.Items != nil:
		return "array of " + p.Items.Type
	case p.Type != "":
		return "type " + p.Type
	default:
		return "(no type or $ref)"
	}
}

// missing returns the elements of a that are not in b.
func missing(a, b []string) []string {
	in := make(map[string]bool, len(b))
	for _, s := range b {
		in[s] = true
	}
	var out []string
	for _, s := range a {
		if !in[s] {
			out = append(out, s)
		}
	}
	return out
}

// TestSchema_NoDrift_CatchesTheMutationsThatMatter is the anti-no-op twin. A
// drift test that reads both sides from the same place passes forever; these
// cases prove each axis of the comparison actually bites. The $ref-swap case
// is the specific mutation a name-only inventory could not see.
func TestSchema_NoDrift_CatchesTheMutationsThatMatter(t *testing.T) {
	t.Run("an unaccounted Go field is surfaced", func(t *testing.T) {
		type withExtra struct {
			Cycle int    `json:"cycle"`
			Novel string `json:"novel_field"`
		}
		got := names(wireFields(reflect.TypeOf(withExtra{})))
		if m := missing(got, []string{"cycle"}); len(m) != 1 || m[0] != "novel_field" {
			t.Fatalf("failed to surface an unaccounted field: got %v, missing %v", got, m)
		}
	})

	t.Run("json:- is not wire surface", func(t *testing.T) {
		// This is what keeps in-memory-only carriers such as
		// deliverable.Result.Content out of the schema.
		type withSkipped struct {
			Cycle   int    `json:"cycle"`
			Ignored string `json:"-"`
		}
		if got := names(wireFields(reflect.TypeOf(withSkipped{}))); !reflect.DeepEqual(got, []string{"cycle"}) {
			t.Errorf("json:\"-\" field leaked into the wire surface: %v", got)
		}
	})

	t.Run("an untagged embedded struct is flattened, not named", func(t *testing.T) {
		type inner struct {
			A string `json:"a"`
		}
		type outer struct {
			inner
			B string `json:"b"`
		}
		// encoding/json promotes a's key to the parent object; a check that
		// emitted "inner" would demand a property that never appears.
		if got := names(wireFields(reflect.TypeOf(outer{}))); !reflect.DeepEqual(got, []string{"a", "b"}) {
			t.Errorf("embedded struct not flattened the way encoding/json promotes it: %v", got)
		}
	})

	t.Run("a swapped $ref is caught", func(t *testing.T) {
		// The exact green-while-wrong mutation a name-only inventory misses:
		// skipped_phases pointing at VerdictNotAdopted. Both definitions
		// exist, both are registered, every name matches — but the two carry
		// disjoint required keys, so every real dossier is rejected.
		_, pred := wantShape(reflect.TypeOf([]cyclestate.SkippedPhase{}))
		if pred == nil {
			t.Fatal("no shape predicate for []cyclestate.SkippedPhase")
		}
		wrong := schemaProp{Type: "array"}
		wrong.Items = &struct {
			Ref  string `json:"$ref"`
			Type string `json:"type"`
		}{Ref: "#/definitions/VerdictNotAdopted"}
		if pred(wrong) {
			t.Error("shape check accepted skipped_phases pointing at VerdictNotAdopted")
		}
		right := schemaProp{Type: "array"}
		right.Items = &struct {
			Ref  string `json:"$ref"`
			Type string `json:"type"`
		}{Ref: "#/definitions/SkippedPhase"}
		if !pred(right) {
			t.Error("shape check rejected the correct $ref")
		}
	})

	t.Run("a flipped primitive type is caught", func(t *testing.T) {
		_, pred := wantShape(reflect.TypeOf(""))
		if pred == nil || pred(schemaProp{Type: "integer"}) || !pred(schemaProp{Type: "string"}) {
			t.Error("primitive type check does not bite")
		}
	})

	t.Run("requiring an omitempty field is caught", func(t *testing.T) {
		// The mutation: append "commit_sha" to the root required list. It is
		// omitempty, so a cycle that did not ship omits it and the document
		// is rejected.
		var found bool
		for _, f := range wireFields(reflect.TypeOf(Dossier{})) {
			if f.name == "commit_sha" {
				found = true
				if !f.omitempty {
					t.Error("commit_sha is no longer omitempty — this case needs a different field")
				}
			}
		}
		if !found {
			t.Fatal("commit_sha not found on Dossier — the required-direction case needs updating")
		}
		doc := loadSchema(t)
		for _, req := range doc.Required {
			if req == "commit_sha" {
				t.Error("schema requires commit_sha, which the writer omits")
			}
		}
	})
}

// TestSchema_EveryRegisteredTypeIsReachedFromTheRoot proves the registry is
// not a place where a definition can hide: every registered type must be
// reachable from Dossier by following fields. A definition nothing points at
// is dead schema, and dead schema is what rots.
func TestSchema_EveryRegisteredTypeIsReachedFromTheRoot(t *testing.T) {
	seen := map[reflect.Type]bool{}
	var walk func(reflect.Type)
	walk = func(rt reflect.Type) {
		for rt.Kind() == reflect.Pointer || rt.Kind() == reflect.Slice || rt.Kind() == reflect.Array || rt.Kind() == reflect.Map {
			rt = rt.Elem()
		}
		if rt.Kind() != reflect.Struct || seen[rt] {
			return
		}
		seen[rt] = true
		for _, f := range wireFields(rt) {
			walk(f.typ)
		}
	}
	walk(reflect.TypeOf(Dossier{}))

	for name, rt := range schemaObjects {
		if name == "" {
			continue
		}
		if !seen[rt] {
			t.Errorf("definition %q (%s) is registered but unreachable from Dossier — either wire it into the record or drop the definition", name, rt)
		}
	}
}
