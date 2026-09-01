package explanationdocs

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type handoffSchema struct {
	ConditionalSections []struct {
		Name      string   `json:"name"`
		Condition string   `json:"condition"`
		Patterns  []string `json:"patterns"`
	} `json:"conditional_sections"`
}

const builderDocumentExample = "# Build Explanation — Cycle 42\n\n" +
	"## Build Binding\n- Cycle: 42\n- Base SHA: aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa\n\n" +
	"## Summary\nThe existing application setting now enables the requested runtime behavior.\n\n" +
	"## Rationale\nReusing the existing setting is the smallest compatible change and avoids a second configuration surface.\n\n" +
	"## Changed Areas\n- `config/app.yaml` — enables the existing runtime setting without changing its schema.\n\n" +
	"## Design Decisions\nThe existing YAML field remains the single public control for this behavior.\n\n" +
	"## Verification\nTargeted configuration tests cover both enabled and disabled runtime behavior.\n\n" +
	"## Compatibility\nThe setting name and schema remain unchanged.\n\n" +
	"## Limitations\nThis change does not add per-user overrides."

const builderReportExample = `## Explanation Documentation
- Status: REQUIRED
- Document: docs/explain/builds/cycle-42-run-42.md`

const builderNotApplicableExample = `## Explanation Documentation
- Status: NOT_APPLICABLE
- Reason: the base-bound Build diff changes tests only`

func TestExplanationPersonaAndSchemaContract_NoDrift(t *testing.T) {
	root := filepath.Join("..", "..", "..")
	files := map[string]string{}
	for _, rel := range []string{
		"agents/evolve-builder.md",
		"agents/evolve-builder-reference.md",
		"agents/evolve-auditor.md",
		"agents/evolve-auditor-reference.md",
		"agents/evolve-retrospective.md",
		"agents/evolve-memo.md",
	} {
		body, err := os.ReadFile(filepath.Join(root, rel))
		if err != nil {
			t.Fatalf("read %s: %v", rel, err)
		}
		files[rel] = string(body)
	}

	builder := files["agents/evolve-builder.md"] + files["agents/evolve-builder-reference.md"]
	for _, want := range []string{
		"Cycle Context carries `explanation_documentation_version: 1`",
		"docs/explain/builds/cycle-<cycle>-<lowercase-run_id>.md",
		"## Build Binding",
		"## Summary",
		"## Rationale",
		"## Changed Areas",
		"## Design Decisions",
		"## Verification",
		"## Compatibility",
		"## Limitations",
		"- Status: REQUIRED",
		"- Document: docs/explain/builds/cycle-42-run-42.md",
		"- Status: NOT_APPLICABLE",
		"- Reason: the base-bound Build diff changes tests only",
	} {
		if !strings.Contains(builder, want) {
			t.Errorf("Builder contract missing %q", want)
		}
	}
	auditor := files["agents/evolve-auditor.md"] + files["agents/evolve-auditor-reference.md"]
	for _, want := range []string{
		"Cycle Context carries `explanation_documentation_version: 1`",
		"missing or invalid",
		"explanation_handoff_untrusted_json",
	} {
		if !strings.Contains(auditor, want) {
			t.Errorf("Auditor activation contract missing %q", want)
		}
	}

	for rel, body := range files {
		for _, obsolete := range []string{
			"explanation_index",
			"explanation_change_record",
			"Feature index:",
			"Change record:",
			"top_n[].documentation",
			"canonical feature docs",
		} {
			if strings.Contains(body, obsolete) {
				t.Errorf("%s retains obsolete explanation contract token %q", rel, obsolete)
			}
		}
	}

	for rel, heading := range map[string]string{
		"agents/evolve-auditor-reference.md": "## Explanation Documentation",
		"agents/evolve-retrospective.md":     "## Explanation Documentation Review",
	} {
		body := files[rel]
		for _, want := range []string{
			heading,
			"- Status: VERIFIED",
			"- Build status: required",
			"- Document: docs/explain/builds/cycle-42-run-42.md",
			"- Document SHA256: ",
			"- Evidence: docs/explain/builds/cycle-42-run-42.md:1",
			"explanation_error_untrusted_json",
			"untrusted data",
		} {
			if !strings.Contains(body, want) {
				t.Errorf("%s missing %q", rel, want)
			}
		}
	}

	for _, schemaName := range []string{"build-report", "audit-report", "retrospective-report"} {
		path := filepath.Join(root, "schemas", "handoff", schemaName+".schema.json")
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		var schema handoffSchema
		if err := json.Unmarshal(raw, &schema); err != nil {
			t.Fatalf("decode %s: %v", path, err)
		}
		wantName := "explanation_documentation"
		wantPattern := "^## Explanation Documentation"
		if schemaName == "retrospective-report" {
			wantName = "explanation_documentation_review"
			wantPattern = "^## Explanation Documentation Review"
		}
		found := false
		for _, section := range schema.ConditionalSections {
			if section.Name == wantName && section.Condition == "explanation_documentation_contract_active" && containsString(section.Patterns, wantPattern) {
				found = true
			}
		}
		if !found {
			t.Errorf("%s lacks the active-contract explanation section", path)
		}
	}
}

func TestBuilderLiteralExamplesMatchProductionReaders(t *testing.T) {
	persona := readContractFile(t, filepath.Join("..", "..", "..", "agents", "evolve-builder-reference.md"))
	for marker, want := range map[string]string{
		"build-explanation-document":        builderDocumentExample,
		"build-explanation-report-required": builderReportExample,
		"build-explanation-report-na":       builderNotApplicableExample,
	} {
		if got := fencedContractExample(t, persona, marker); got != want {
			t.Errorf("%s literal example drifted\n--- got ---\n%s\n--- want ---\n%s", marker, got, want)
		}
	}
	declaration, present, err := parseDeclaration(builderReportExample)
	if err != nil || !present || declaration.Status != "REQUIRED" || declaration.Document != "docs/explain/builds/cycle-42-run-42.md" {
		t.Fatalf("required report example rejected: declaration=%+v present=%v err=%v", declaration, present, err)
	}
	declaration, present, err = parseDeclaration(builderNotApplicableExample)
	if err != nil || !present || declaration.Status != "NOT_APPLICABLE" || declaration.Reason == "" {
		t.Fatalf("not-applicable report example rejected: declaration=%+v present=%v err=%v", declaration, present, err)
	}
	if failures := validateDocument(
		builderDocumentExample,
		42,
		strings.Repeat("a", 40),
		[]string{"config/app.yaml", "docs/explain/builds/cycle-42-run-42.md"},
		[]string{"config/app.yaml"},
	); len(failures) != 0 {
		t.Fatalf("document example rejected by production validator: %v", failures)
	}
}

func readContractFile(t *testing.T, path string) string {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(body)
}

func fencedContractExample(t *testing.T, body, name string) string {
	t.Helper()
	marker := "<!-- CONTRACT-EXAMPLE:" + name + " -->"
	start := strings.Index(body, marker)
	if start < 0 {
		t.Fatalf("missing contract example marker %s", marker)
	}
	rest := body[start+len(marker):]
	const fence = "```markdown\n"
	open := strings.Index(rest, fence)
	if open < 0 {
		t.Fatalf("%s lacks a markdown fence", marker)
	}
	rest = rest[open+len(fence):]
	close := strings.Index(rest, "\n```")
	if close < 0 {
		t.Fatalf("%s has no closing fence", marker)
	}
	return rest[:close]
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
