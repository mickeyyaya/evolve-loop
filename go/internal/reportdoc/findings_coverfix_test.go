package reportdoc

import (
	"strings"
	"testing"
)

func TestCoverfixFindingKeyIsCappedAtMaxLen(t *testing.T) {
	t.Parallel()
	title := strings.Repeat("abcdefghij", 12)
	got := FindingKey(title)
	if got != title[:findingKeyMaxLen] {
		t.Errorf("FindingKey length = %d (%q), want the first %d characters", len(got), got, findingKeyMaxLen)
	}
}
