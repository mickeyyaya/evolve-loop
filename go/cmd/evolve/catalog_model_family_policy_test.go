package main

import (
	"path/filepath"
	"reflect"
	"runtime"
	"testing"

	gobridge "github.com/mickeyyaya/evolve-loop/go/internal/bridge"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

func TestTheCheckedInCatalogFiltersEachTargetToItsModelFamily(t *testing.T) {
	_, thisFile, _, _ := runtime.Caller(0)
	checkedIn := filepath.Join(filepath.Dir(thisFile), "..", "..", "..", ".evolve", "policy.json")
	pol, err := policy.Load(checkedIn)
	if err != nil {
		t.Fatalf("load %s: %v", checkedIn, err)
	}
	filters := pol.CatalogConfig().AllowedFamilies
	for _, target := range []string{"agy-tmux", "agy-claude-tmux", "claude-tmux", "codex-tmux"} {
		entry := policy.BaseCLI(target)
		if want := []string{gobridge.ModelFamily(target)}; !reflect.DeepEqual(filters[entry], want) {
			t.Errorf("catalog.allowed_families[%s] = %v, want %v: one agy listing feeds both agy entries, and each must keep to the models its target dispatches", entry, filters[entry], want)
		}
	}
}
