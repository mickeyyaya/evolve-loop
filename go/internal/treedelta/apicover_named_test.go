package treedelta

import (
	"testing"
)

func TestAPI_EveryExportIsNamed(t *testing.T) {
	var (
		_ Git
	)
	if len(Args("a", "b")) != 8 {
		t.Errorf("Args = %v", Args("a", "b"))
	}
}
