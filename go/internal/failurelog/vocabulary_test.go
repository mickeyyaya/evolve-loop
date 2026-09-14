package failurelog

import (
	"strings"
	"testing"
)

// VocabularyList is the ONE rendering of the failure-class vocabulary that the
// audit contract block (prompt) and the deliverables gate (correction) hand an
// agent — cycle 1684's invented class showed the auditor had never been told
// the set. Every known classification appears exactly once; unknown does not.
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
