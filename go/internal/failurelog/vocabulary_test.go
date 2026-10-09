package failurelog

import (
	"strings"
	"testing"
)

func TestVocabularyList_NamesEveryKnownClassificationOnce(t *testing.T) {
	list := VocabularyList()
	for _, c := range KnownClassifications() {
		if strings.Count(list, string(c)) < 1 {
			t.Errorf("vocabulary omits %q: %s", c, list)
		}
	}
	if strings.Contains(list, string(UnknownClassification)) {
		t.Errorf("unknown-classification is the absence of a class, not a choice: %s", list)
	}
	if NormalizeLegacy("superseded-predicate-contradiction") != UnknownClassification {
		t.Error("an invented class normalizes to unknown — the gate's refusal predicate")
	}
}
