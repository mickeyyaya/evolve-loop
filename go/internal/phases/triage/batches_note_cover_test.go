package triage

import "testing"

func TestSelectableBatchesNote_AnEmptyBacklogHasNoNote(t *testing.T) {
	t.Parallel()
	if got := selectableBatchesNote(nil, nil); got != "" {
		t.Errorf("selectableBatchesNote(nil) = %q, want no note", got)
	}
}
