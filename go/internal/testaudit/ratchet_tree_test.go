package testaudit

import (
	"path/filepath"
	"testing"
)

func TestTreeRatchet(t *testing.T) {
	cfg, err := DefaultConfig()
	if err != nil {
		t.Fatal(err)
	}
	if err := CheckTree(filepath.Join("..", ".."), cfg); err != nil {
		t.Fatal(err)
	}
}
