package signalcenter

import (
	"slices"
	"strings"
	"testing"
)

func TestRegistry_NewKindsAndCodesAreRegistered(t *testing.T) {
	t.Parallel()
	newKinds := map[Kind]Module{
		KindWaveStarted:   ModuleLoop,
		KindWaveEnded:     ModuleLoop,
		KindInboxClaimed:  ModuleInbox,
		KindInboxReleased: ModuleInbox,
		KindCICompleted:   ModuleCI,
	}
	wantNames := map[Kind]string{
		KindWaveStarted: "wave.started", KindWaveEnded: "wave.ended", KindInboxClaimed: "inbox.claimed",
		KindInboxReleased: "inbox.released", KindCICompleted: "ci.completed",
	}
	listed := Kinds()
	for kind, module := range newKinds {
		if string(kind) != wantNames[kind] {
			t.Errorf("kind constant = %q, want %q", kind, wantNames[kind])
		}
		if !kind.Known() || !slices.Contains(listed, kind) {
			t.Errorf("%s is not in the closed kind set", kind)
		}
		if kind.Terminal() {
			t.Errorf("%s must not be terminal: it reports progress, not a stop", kind)
		}
		if !module.Known() {
			t.Errorf("module %q of %s is not in the closed module set", module, kind)
		}
		clean, drift := Normalize(Event{Module: module, Origin: "Test.emit", Kind: kind, Severity: SeverityInfo, Reason: "reason"})
		if len(drift) != 0 || clean.Severity != SeverityInfo {
			t.Errorf("an INFO %s event from module %s drifts: %v (severity %s)", kind, module, drift, clean.Severity)
		}
	}
	if string(ModuleCI) != "ci" || !slices.Contains(Modules(), ModuleCI) {
		t.Errorf("ModuleCI = %q, listed %v; want the closed module ci", ModuleCI, slices.Contains(Modules(), ModuleCI))
	}
}

func TestRegistry_TheRedRunCodeIsOwnedByTheCIModuleWithADoc(t *testing.T) {
	t.Parallel()
	if string(CodeCIRunRed) != "CI_RUN_RED" {
		t.Fatalf("CodeCIRunRed = %q, want CI_RUN_RED", CodeCIRunRed)
	}
	owner, registered := IsRegistered(CodeCIRunRed)
	if !registered || owner != ModuleCI || !CodeCIRunRed.BelongsTo(ModuleCI) {
		t.Errorf("CI_RUN_RED owner = %q (registered %v), want the ci module: the code must exist in every build, whatever packages the binary links", owner, registered)
	}
	var doc string
	for _, d := range RegisteredCodes()[ModuleCI] {
		if d.Code == CodeCIRunRed {
			doc = d.Doc
		}
	}
	if strings.TrimSpace(doc) == "" {
		t.Error("CI_RUN_RED has no doc in the registry")
	}
	warned, drift := Normalize(Event{Module: ModuleCI, Origin: "Test.emit", Kind: KindCICompleted, Severity: SeverityWarn, Code: CodeCIRunRed, Reason: "red"})
	if len(drift) != 0 || warned.Code != CodeCIRunRed || warned.Severity != SeverityWarn {
		t.Errorf("a WARN ci.completed with CI_RUN_RED drifts: %v (code %q, severity %s)", drift, warned.Code, warned.Severity)
	}
	for _, c := range RegistryConflicts() {
		if c.Code == CodeCIRunRed {
			t.Errorf("CI_RUN_RED has a registry conflict: %+v", c)
		}
	}
}
