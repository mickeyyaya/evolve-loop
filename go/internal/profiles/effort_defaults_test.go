package profiles

import (
	"errors"
	"testing"
	"testing/fstest"
)

func TestLoader_RefusesTheRetiredEffortFields(t *testing.T) {
	loader := NewFromFS(fstest.MapFS{
		"level.json":    {Data: []byte(`{"name":"level","effort_level":"low"}`)},
		"override.json": {Data: []byte(`{"name":"override","effort_overrides":{"deep":"high"}}`)},
		"clean.json":    {Data: []byte(`{"name":"clean","model_tier_default":"deep"}`)},
	})
	for _, name := range []string{"level", "override"} {
		if _, err := loader.Get(name); !errors.Is(err, ErrRetiredEffortField) {
			t.Errorf("Get(%s) err = %v, want ErrRetiredEffortField", name, err)
		}
	}
	if _, err := loader.Get("clean"); err != nil {
		t.Errorf("Get(clean) err = %v, want a clean load", err)
	}
}

func TestEveryTrackedProfileLoadsWithoutAnEffortField(t *testing.T) {
	loader, names := RealTreeProfiles(t)
	if len(names) == 0 {
		t.Fatal("matched NO tracked profiles — the selector is broken and this guard is vacuous")
	}
	for _, name := range names {
		if _, err := loader.Get(name); errors.Is(err, ErrRetiredEffortField) {
			t.Errorf("profile %s still carries an effort field: run evolve cli-routing migrate", name)
		}
	}
}
