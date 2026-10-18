package signalcenter

import (
	"strconv"
	"strings"
	"sync/atomic"
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

func TestRegistry_TheLoopLostCodeIsOwnedByTheLoopModuleWithADoc(t *testing.T) {
	t.Parallel()

	owner, registered := IsRegistered(CodeLoopLost)

	if !registered || owner != ModuleLoop || !CodeLoopLost.BelongsTo(ModuleLoop) {
		t.Errorf("LOOP_LOST owner = %q (registered %v), want the loop module: every build must hold the code of the synthetic loop.lost record, so the generated code list never depends on which packages a binary links", owner, registered)
	}
	var doc string
	for _, d := range RegisteredCodes()[ModuleLoop] {
		if d.Code == CodeLoopLost {
			doc = d.Doc
		}
	}
	if strings.TrimSpace(doc) == "" {
		t.Error("LOOP_LOST has no doc in the registry")
	}
}

func TestRegisterCode_DuplicateIdenticalIsNoOp_ConflictIsRecorded(t *testing.T) {
	t.Parallel()
	code := fixtureCode("AUDIT_TEST_REGISTRY_FIXTURE")
	RegisterCode(ModuleAudit, code, "fixture: an audit gate refused the report")
	RegisterCode(ModuleAudit, code, "fixture: an audit gate refused the report")
	RegisterCode(ModuleAudit, code, "fixture: a DIFFERENT doc for the same code")
	RegisterCode(ModuleShip, code, "fixture: another module claims the code")
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
	malformed, foreign := fixtureCode("not_a_code"), fixtureCode("AUDIT_LOOKS_FOREIGN")
	RegisterCode(ModuleShip, malformed, "fixture")
	RegisterCode(ModuleShip, foreign, "fixture")
	if got := len(conflictsFor(malformed)) + len(conflictsFor(foreign)); got != 2 {
		t.Errorf("malformed and foreign-prefix registrations are recorded as conflicts, got %d", got)
	}
	if _, ok := IsRegistered(malformed); ok {
		t.Error("a malformed code is never registered")
	}
}

func TestRegistryConflicts_RealRegistryIsClean(t *testing.T) {
	t.Parallel()
	for _, c := range RegistryConflicts() {
		s := string(c.Code)
		if strings.Contains(s, "FIXTURE") || strings.Contains(s, "LOOKS_FOREIGN") || strings.HasPrefix(s, "not_a_code") {
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

var fixtureSeq uint64

func fixtureCode(base string) Code {
	return Code(base + "_" + strconv.FormatUint(atomic.AddUint64(&fixtureSeq, 1), 10))
}
