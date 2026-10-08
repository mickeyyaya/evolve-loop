package stelint

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestTheLimitsMatchTheStandard(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "..", filepath.FromSlash(StandardPath)))
	if err != nil {
		t.Fatalf("read the standard: %v", err)
	}
	_, section, found := strings.Cut(string(raw), "\n## The lint\n")
	if !found {
		t.Fatal("the standard has no \"The lint\" section")
	}
	section, _, _ = strings.Cut(section, "\n## ")
	for _, want := range []string{
		fmt.Sprintf("%d words in a numbered item", maxStepWords),
		fmt.Sprintf("%d words in other text", maxSentenceWords),
		fmt.Sprintf("%d sentences", maxParagraphSentences),
	} {
		if !strings.Contains(section, want) {
			t.Errorf("\"The lint\" section of the standard does not say %q; the code and the standard disagree", want)
		}
	}
}
