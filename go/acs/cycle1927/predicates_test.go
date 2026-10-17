//go:build acs

package cycle1927

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/stelint"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

func words(n int) string {
	return strings.TrimSpace(strings.Repeat("word ", n))
}

func findings(doc string) []string {
	fs := stelint.Check([]byte(doc), stelint.Options{Words: []stelint.Substitution{{Phrase: "utilize", Replacement: "use"}}})
	out := make([]string, 0, len(fs))
	for _, f := range fs {
		out = append(out, fmt.Sprintf("%d %s", f.Line, f.Rule))
	}
	return out
}

func assertFindings(t *testing.T, name, doc string, want ...string) {
	t.Helper()
	if want == nil {
		want = []string{}
	}
	if got := findings(doc); !reflect.DeepEqual(got, want) {
		t.Errorf("%s: findings = %v, want %v\ndoc:\n%s", name, got, want, doc)
	}
}

func TestC1927_001_TheThreeF2MutantsAreKilledByNamedTests(t *testing.T) {
	names := []string{
		"TestCheck_SentenceSplitting",
		"TestCheck_AnEmptyFirstBlockquoteLineDoesNotDecideTheQuotation",
		"TestCheck_EachBlockquoteDecidesItsOwnQuotation",
	}
	acsassert.GoTests(t, acsassert.GoTestSpec{
		Dir:     filepath.Join(acsassert.RepoRoot(t), "go"),
		Package: "github.com/mickeyyaya/evolve-loop/go/internal/stelint",
		Pattern: "^(" + strings.Join(names, "|") + ")$",
		Names:   names,
	})
	assertFindings(t, "a quotation ending with ? ends the sentence", `He said "`+words(20)+`?" `+words(23)+".\n")
	assertFindings(t, "a quotation ending with ! ends the sentence", `He said "`+words(20)+`!" `+words(23)+".\n")
	assertFindings(t, "an empty first quote line", ">\n> “We utilize the first paragraph.\n>\n> We utilize the second paragraph.”\n")
	assertFindings(t, "a second blockquote decides again", "> “We utilize the quoted passage.”\n\n> We utilize it.\n", "3 STE-WORD")
}

func TestC1927_002_AnUnclosedFenceOrCommentInABlockquoteDoesNotHideTheTextAfterIt(t *testing.T) {
	assertFindings(t, "unclosed backtick fence", "> ```go\n> x\n\nWe utilize it.\n", "4 STE-WORD")
	assertFindings(t, "unclosed tilde fence", "> ~~~\n\n"+words(26)+".\n", "3 STE-SENTENCE")
	assertFindings(t, "unclosed fence then a text line", "> ```\nWe utilize it.\n", "2 STE-WORD")
	assertFindings(t, "unclosed HTML comment", "> <!-- note\n\nWe utilize it.\n", "3 STE-WORD")
	assertFindings(t, "a closed fence in a quote still hides its lines", "> ```\n> "+words(40)+" utilize\n> ```\n")
	assertFindings(t, "a closed comment in a quote still hides its lines", "> <!--\n> We utilize it.\n> -->\n")
	assertFindings(t, "a fence outside a quote still hides the rest of the file", "```\nWe utilize it.\n")
}

func TestC1927_003_AThirtyWordListItemBeforeADashRuleGivesOneSentenceFinding(t *testing.T) {
	assertFindings(t, "bullet item", "- "+words(30)+".\n---\n", "1 STE-SENTENCE")
	assertFindings(t, "numbered step", "1. "+words(21)+".\n---\n", "1 STE-SENTENCE")
	assertFindings(t, "a prose paragraph before dashes stays a heading", words(30)+"\n---\n"+words(13)+".\n")
	assertFindings(t, "a short bullet item before dashes passes", "- "+words(10)+".\n---\n")
}

func decodesDocsFloorStage(key, word string) bool {
	raw, err := json.Marshal(map[string]string{key: word})
	if err != nil {
		return false
	}
	var d policy.DocsFloorPolicy
	return json.Unmarshal(raw, &d) == nil
}

func acceptedDocsFloorStages(t *testing.T) []string {
	t.Helper()
	var accepted []string
	for _, w := range []string{"0", "off", "shadow", "enforce", "1", "on", "advisory", "true", "false", "none"} {
		stageOK, steStageOK := decodesDocsFloorStage("stage", w), decodesDocsFloorStage("ste_stage", w)
		if stageOK != steStageOK {
			t.Errorf("docs_floor decodes %q differently for stage (%v) and ste_stage (%v)", w, stageOK, steStageOK)
		}
		if stageOK {
			accepted = append(accepted, w)
		}
	}
	return accepted
}

func docsFloorSection(t *testing.T) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(acsassert.RepoRoot(t), "docs", "architecture", "policy-config.md"))
	if err != nil {
		t.Fatalf("read policy-config.md: %v", err)
	}
	doc := string(raw)
	start := strings.Index(doc, "## Documentation floors (`docs_floor`)")
	if start < 0 {
		t.Fatalf("policy-config.md has no docs_floor section")
	}
	section := doc[start+1:]
	if end := strings.Index(section, "\n## "); end >= 0 {
		section = section[:end]
	}
	return section
}

func TestC1927_004_TheDocsAndTheDecodeAgreeOnTheStageWords(t *testing.T) {
	accepted := acceptedDocsFloorStages(t)
	for _, w := range []string{"off", "shadow", "enforce"} {
		if !slices.Contains(accepted, w) {
			t.Errorf("docs_floor decode refuses canonical stage %q; accepted = %v", w, accepted)
		}
	}
	for _, w := range []string{"on", "advisory", "none"} {
		if slices.Contains(accepted, w) {
			t.Errorf("docs_floor decode accepts unknown stage %q", w)
		}
	}
	section := docsFloorSection(t)
	for _, w := range accepted {
		if !strings.Contains(section, "`"+w+"`") {
			t.Errorf("the decode accepts stage %q but the docs_floor section of policy-config.md does not name it; accepted = %v", w, accepted)
		}
	}
}
