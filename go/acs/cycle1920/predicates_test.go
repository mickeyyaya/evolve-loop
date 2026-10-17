//go:build acs

package cycle1920

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	researchDoc      = "docs/research/practice-agy-cli-changelog-page-2026-10.md"
	priorSuite       = "go/acs/cycle1915/predicates_test.go"
	newestHeading    = "### Version v2.24.0"
	maxSentenceWords = 25
)

func readFile(t *testing.T, rel string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(acsassert.RepoRoot(t), rel))
	if err != nil {
		t.Fatalf("RED: %s unreadable: %v", rel, err)
	}
	return string(b)
}

func v2_24_0Section(t *testing.T) string {
	t.Helper()
	doc := readFile(t, researchDoc)
	at := strings.Index(doc, newestHeading+"\n")
	if at < 0 {
		t.Fatalf("RED: doc has no %q section", newestHeading)
	}
	body := doc[at+len(newestHeading)+1:]
	if next := strings.Index(body, "\n#"); next >= 0 {
		body = body[:next]
	}
	return body
}

func vertexSentences(section string) []string {
	var found []string
	for _, line := range strings.Split(section, "\n") {
		for _, s := range regexp.MustCompile(`[^.]+\.`).FindAllString(line, -1) {
			if strings.Contains(s, "Vertex AI") {
				found = append(found, strings.TrimSpace(s))
			}
		}
	}
	return found
}

func TestC1920_001_V2_24_0SectionExplainsVertexAI(t *testing.T) {
	sentences := vertexSentences(v2_24_0Section(t))
	if len(sentences) == 0 {
		t.Fatalf("RED: the v2.24.0 section has no sentence that names Vertex AI")
	}
}

func TestC1920_002_VertexAISentenceNamesProjectAccessCondition(t *testing.T) {
	sentences := vertexSentences(v2_24_0Section(t))
	if len(sentences) == 0 {
		t.Fatalf("RED: the v2.24.0 section has no sentence that names Vertex AI")
	}
	for _, s := range sentences {
		lower := strings.ToLower(s)
		if strings.Contains(lower, "project") && strings.Contains(lower, "access") {
			return
		}
	}
	t.Errorf("RED: no Vertex AI sentence names the project-access condition: %q", sentences)
}

func TestC1920_003_VertexAISentenceStaysWithinSteLimit(t *testing.T) {
	sentences := vertexSentences(v2_24_0Section(t))
	if len(sentences) == 0 {
		t.Fatalf("RED: the v2.24.0 section has no sentence that names Vertex AI")
	}
	for _, s := range sentences {
		if words := len(strings.Fields(s)); words > maxSentenceWords {
			t.Errorf("RED: Vertex AI sentence has %d words, limit %d: %q", words, maxSentenceWords, s)
		}
	}
}

func TestC1920_004_VertexAIAddsNoCodeIdentifierOutsideSnapshot(t *testing.T) {
	section := v2_24_0Section(t)
	if len(vertexSentences(section)) == 0 {
		t.Fatalf("RED: the v2.24.0 section has no sentence that names Vertex AI")
	}
	known := map[string]bool{"Vertex AI": true}
	for _, m := range regexp.MustCompile("`([^`]+)`").FindAllStringSubmatch(strings.Join(vertexSentences(section), " "), -1) {
		if !known[m[1]] {
			t.Errorf("RED: Vertex AI sentence cites %q, not in the v2.24.0 snapshot", m[1])
		}
	}
}

func TestC1920_005_PriorSuiteRequiresVertexAIInSection(t *testing.T) {
	src := readFile(t, priorSuite)
	at := strings.Index(src, "func TestC1915_002_")
	if at < 0 {
		t.Fatalf("RED: %s has no TestC1915_002", priorSuite)
	}
	body := src[at:]
	if end := strings.Index(body[1:], "\nfunc "); end >= 0 {
		body = body[:end+2]
	}
	if !strings.Contains(body, `"Vertex AI"`) {
		t.Errorf("RED: TestC1915_002 item list does not require %q", "Vertex AI")
	}
}

func TestC1920_006_PriorSuiteStillPassesAndStaysCommentFree(t *testing.T) {
	root := acsassert.RepoRoot(t)
	out, _, code, err := acsassert.SubprocessOutput("go", "-C", filepath.Join(root, "go"), "test", "-tags", "acs", "-count=1", "-run", "TestC1915_00[1-4]", "./acs/cycle1915")
	if err != nil || code != 0 {
		t.Fatalf("RED: cycle1915 predicates fail (exit %d): %s", code, out)
	}
}
