package yamlcodec

import (
	"errors"
	"testing"
)

func TestNewSyntaxError_KeepsTheWholeMessageOfAnErrorWithoutTheLibraryPrefix(t *testing.T) {
	t.Parallel()

	got := newSyntaxError(errors.New("boom"))

	if got.Line != 0 || got.Msg != "boom" {
		t.Errorf("newSyntaxError = %+v, want line 0 and message boom", got)
	}
}
