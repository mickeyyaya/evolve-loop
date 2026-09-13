package signalcenter

// registry_test.go — the code registry (ADR-0101 decision 4; design §5.4). Codes are
// MODULE_SNAKE_CASE, registered by their owning module with a one-line doc. A
// duplicate registration with the same doc is a no-op; a conflicting one is
// RECORDED (never a panic — a duplicate code is a review defect, not an
// availability event) and a named test asserts the registry is conflict-free.

import (
	"strings"
	"testing"
)

func TestRegistry_BuiltInDriftCodesAreRegisteredWithDocs(t *testing.T) {
	t.Parallel()
	docs := RegisteredCodes()[ModuleSignalCenter]
	want := map[Code]bool{
		CodeUnknownModule: true, CodeUnknownKind: true, CodeUnknownSeverity: true, CodeMissingCode: true,
		CodeUnregisteredCode: true, CodeMissingReason: true, CodeBadOrigin: true, CodeListenerPanicked: true, CodeSinkDropped: true,
	}
	for _, d := range docs {
		if want[d.Code] && strings.TrimSpace(d.Doc) != "" {
			delete(want, d.Code)
		}
	}
	if len(want) != 0 {
		t.Errorf("built-in codes missing or undocumented: %v", want)
	}
	for c := range map[Code]bool{CodeUnknownModule: true, CodeListenerPanicked: true, CodeSinkDropped: true} {
		if m, ok := IsRegistered(c); !ok || m != ModuleSignalCenter || !c.BelongsTo(ModuleSignalCenter) {
			t.Errorf("%q must be registered to signalcenter and carry its prefix", c)
		}
	}
}

func TestRegisterCode_DuplicateIdenticalIsNoOp_ConflictIsRecorded(t *testing.T) {
	t.Parallel()
	const code Code = "AUDIT_TEST_REGISTRY_FIXTURE"
	RegisterCode(ModuleAudit, code, "fixture: an audit gate refused the report")
	RegisterCode(ModuleAudit, code, "fixture: an audit gate refused the report")  // identical → no-op
	RegisterCode(ModuleAudit, code, "fixture: a DIFFERENT doc for the same code") // conflicting doc
	RegisterCode(ModuleShip, code, "fixture: another module claims the code")     // conflicting owner
	after := conflictsFor(code)
	if len(after) != 2 {
		t.Fatalf("two conflicts must be recorded for %s, got %d: %+v", code, len(after), after)
	}
	var sawDoc, sawOwner bool
	for _, c := range after {
		if c.Code != code {
			continue
		}
		if c.Module == ModuleAudit && c.OtherModule == ModuleAudit && c.Doc != c.OtherDoc {
			sawDoc = true
		}
		if c.OtherModule == ModuleShip {
			sawOwner = true
		}
	}
	if !sawDoc || !sawOwner {
		t.Errorf("conflicts name both sides (module, other module, docs): %+v", after)
	}
	if m, ok := IsRegistered(code); !ok || m != ModuleAudit {
		t.Errorf("the first registration wins: %q → %q (%v)", code, m, ok)
	}
	if got := RegisteredCodes()[ModuleAudit]; !containsCode(got, code) {
		t.Errorf("the registered code is listed under its owner: %+v", got)
	}
}

func containsCode(docs []CodeDoc, c Code) bool {
	for _, d := range docs {
		if d.Code == c {
			return true
		}
	}
	return false
}

func TestRegisterCode_RejectsMalformedAndForeignPrefixAsConflicts(t *testing.T) {
	t.Parallel()
	RegisterCode(ModuleShip, "not_a_code", "fixture")          // malformed
	RegisterCode(ModuleShip, "AUDIT_LOOKS_FOREIGN", "fixture") // prefix belongs to another module
	if got := len(conflictsFor("not_a_code")) + len(conflictsFor("AUDIT_LOOKS_FOREIGN")); got != 2 {
		t.Errorf("malformed and foreign-prefix registrations are recorded as conflicts, got %d", got)
	}
	if _, ok := IsRegistered("not_a_code"); ok {
		t.Error("a malformed code is never registered")
	}
}

// The repo-level guard: the real registry (built-ins plus every module's table)
// must be conflict-free. Test fixtures above register under names that carry the
// word FIXTURE / LOOKS_FOREIGN / not_a_code so this assertion can exclude them.
func TestRegistryConflicts_RealRegistryIsClean(t *testing.T) {
	t.Parallel()
	for _, c := range RegistryConflicts() {
		s := string(c.Code)
		if strings.Contains(s, "FIXTURE") || strings.Contains(s, "LOOKS_FOREIGN") || s == "not_a_code" {
			continue
		}
		t.Errorf("real registry conflict: %+v", c)
	}
}

func conflictsFor(code Code) []Conflict {
	var out []Conflict
	for _, c := range RegistryConflicts() {
		if c.Code == code {
			out = append(out, c)
		}
	}
	return out
}
