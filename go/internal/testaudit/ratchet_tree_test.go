package testaudit

import (
	"os"
	"testing"
)

func TestTreeRatchet(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	root, err := findModuleRoot(wd)
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := DefaultConfig()
	if err != nil {
		t.Fatal(err)
	}
	if err := CheckTree(root, cfg); err != nil {
		t.Fatal(err)
	}
}
