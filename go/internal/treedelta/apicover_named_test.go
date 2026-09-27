package treedelta

import (
	"context"
	"testing"
)

func TestAPI_EveryExportIsNamed(t *testing.T) {
	var (
		_ func(string, string) []string                                                                    = Args
		_ func(context.Context, Git, string, string, string) ([]byte, error)                               = Delta
		_ func(context.Context, Git, string, string, string, string, string) ([]byte, []byte, bool, error) = Identical
		_ Git
	)
	if len(Args("a", "b")) != 8 {
		t.Errorf("Args = %v", Args("a", "b"))
	}
}
