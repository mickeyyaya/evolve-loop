package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestLoad_DeliverableKinds_FromRegistry — ADR-0099 slice 2: the document
// deliverable contract is CONFIG (phase-registry.json:config.deliverable_kinds),
// consumed by ONE deterministic engine. Pins the shipped registry's shape.
func TestLoad_DeliverableKinds_FromRegistry(t *testing.T) {
	cfg, _ := Load(filepath.Join("..", "..", "..", "docs", "architecture", "phase-registry.json"), map[string]string{})
	doc, ok := cfg.DeliverableKinds["document"]
	if !ok {
		t.Fatalf("registry declares no deliverable_kinds.document: %+v", cfg.DeliverableKinds)
	}
	var _ DeliverableKindSpec = doc // apicover: the registry-declared contract type is named here
	if doc.Root != "solutions" || doc.MinOptions < 2 || doc.EvidenceFile == "" {
		t.Errorf("document spec = %+v, want root=solutions, min_options>=2, evidence_file set", doc)
	}
	for _, f := range []string{"recommendation.md", "assumptions-and-evidence.md"} {
		found := false
		for _, r := range doc.RequiredFiles {
			if r == f {
				found = true
			}
		}
		if !found {
			t.Errorf("required_files %v lacks %s", doc.RequiredFiles, f)
		}
	}
	if len(doc.RequiredSections["recommendation.md"]) == 0 || len(doc.ForbidPlaceholders) == 0 {
		t.Errorf("document spec must declare recommendation sections and placeholders: %+v", doc)
	}
}

// TestLoadDomain — the first Go reader of .evolve/domain.json (documented in
// docs/reference/configuration.md since v8, zero readers until now): the
// project's default deliverable kind when a task declares none.
func TestLoadDomain(t *testing.T) {
	root := t.TempDir()
	if _, ok, err := LoadDomain(root); ok || err != nil {
		t.Fatalf("absent domain.json must be ok=false with no error; got ok=%v err=%v", ok, err)
	}
	if err := os.MkdirAll(filepath.Join(root, ".evolve"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".evolve", "domain.json"), []byte(`{"domain": "writing",}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, ok, err := LoadDomain(root); ok || err == nil {
		t.Fatalf("malformed domain.json must be a loud error, never a silent code project; got ok=%v err=%v", ok, err)
	}
	if got := (Domain{}).DefaultDeliverableKind(); got != "code" {
		t.Errorf("zero Domain default = %q, want code", got)
	}
	for _, tc := range []struct{ domain, want string }{
		{"coding", "code"}, {"writing", "document"}, {"research", "document"}, {"design", "code"}, {"mixed", "code"}, {"", "code"},
	} {
		if err := os.MkdirAll(filepath.Join(root, ".evolve"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, ".evolve", "domain.json"), []byte(`{"domain": "`+tc.domain+`", "evalMode": "bash"}`), 0o644); err != nil {
			t.Fatal(err)
		}
		d, ok, err := LoadDomain(root)
		if !ok || err != nil || d.Domain != tc.domain {
			t.Errorf("LoadDomain(%s) = %+v ok=%v err=%v", tc.domain, d, ok, err)
		}
		if got := d.DefaultDeliverableKind(); got != tc.want {
			t.Errorf("domain %q → default kind %q, want %q", tc.domain, got, tc.want)
		}
	}
}

// TestDocumentSpec_AndKindVocabulary names the one lookup and the two kind
// constants every projection uses, and pins the load warning for a hole in
// the contract (root/min_options are registry-owned, never defaulted).
func TestDocumentSpec_AndKindVocabulary(t *testing.T) {
	if DeliverableKindCode != "code" || DeliverableKindDocument != "document" {
		t.Fatalf("kind vocabulary drifted: %q %q", DeliverableKindCode, DeliverableKindDocument)
	}
	cfg, _ := Load(filepath.Join("..", "..", "..", "docs", "architecture", "phase-registry.json"), map[string]string{})
	if spec, ok := cfg.DocumentSpec(); !ok || spec.Root != "solutions" {
		t.Errorf("DocumentSpec() = %+v ok=%v", spec, ok)
	}
	if _, ok := (RoutingConfig{}).DocumentSpec(); ok {
		t.Errorf("zero config must declare no document spec")
	}
	dir := t.TempDir()
	reg := filepath.Join(dir, "r.json")
	if err := os.WriteFile(reg, []byte(`{"config":{"deliverable_kinds":{"document":{"required_files":["x.md"]}}}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	_, ws := Load(reg, map[string]string{})
	found := false
	for _, w := range ws {
		if strings.Contains(w.Message, "deliverable_kinds[document]") {
			found = true
		}
	}
	if !found {
		t.Errorf("a contract without root/min_options must load with a warning; got %v", ws)
	}
}

// TestSignalKeys_AreTheConditionalRuleWords — ADR-0099 slice 3: an overlay
// `when` clause, core's dispatch projection and the compiled tdd rule name the
// deliverable-kind signal with ONE word, and the goal type is the scout's
// namespaced routable field.
func TestSignalKeys_AreTheConditionalRuleWords(t *testing.T) {
	named := false
	for _, c := range DefaultTddRule().Clauses() {
		if c.Field == SignalDeliverableKind {
			named = true
		}
	}
	if !named {
		t.Errorf("DefaultTddRule %q does not name SignalDeliverableKind %q", DefaultTddRuleExpr, SignalDeliverableKind)
	}
	if !strings.HasPrefix(SignalGoalType, "scout.") {
		t.Errorf("SignalGoalType = %q, want the scout-namespaced routable field", SignalGoalType)
	}
}
