package policy

import (
	"errors"
	"reflect"
	"slices"
	"testing"
)

func TestResolveStages_NameTheKeyThatHasNoReader(t *testing.T) {
	t.Parallel()
	_, err := resolveStages([]string{"a.stage", "b.stage"}, map[string]stageReader{"a.stage": func(Policy) (string, error) { return "shadow", nil }}, Policy{})
	var keyErr *stageKeyError
	if !errors.As(err, &keyErr) || keyErr.Key != "b.stage" || !errors.Is(err, errNoStageReader) {
		t.Errorf("error = %v, want a stageKeyError for b.stage that wraps errNoStageReader", err)
	}
}

func TestResolveStages_NameTheKeyWhoseReaderFailsAndKeepTheCause(t *testing.T) {
	t.Parallel()
	cause := errors.New("bad value")
	_, err := resolveStages([]string{"a.stage"}, map[string]stageReader{"a.stage": func(Policy) (string, error) { return "", cause }}, Policy{})
	var keyErr *stageKeyError
	if !errors.As(err, &keyErr) || keyErr.Key != "a.stage" || !errors.Is(err, cause) {
		t.Errorf("error = %v, want a stageKeyError for a.stage that wraps the cause", err)
	}
	if got, want := err.Error(), "policy: stage key a.stage: bad value"; got != want {
		t.Errorf("error text = %q, want %q", got, want)
	}
}

func TestStageReaders_HaveExactlyTheKeysOfStageKeys(t *testing.T) {
	t.Parallel()
	keys := map[string]bool{}
	for _, key := range StageKeys() {
		keys[key] = true
		if stageReaders[key] == nil {
			t.Errorf("stage key %s has no reader", key)
		}
	}
	for key := range stageReaders {
		if !keys[key] {
			t.Errorf("reader %s is not a stage key", key)
		}
	}
}

func TestWalkStageFields_FindsNestedKeysAndStopsAtACycle(t *testing.T) {
	t.Parallel()
	type node struct {
		Stage string `json:"stage"`
		Next  *node  `json:"next"`
		Skip  struct {
			Stage string `json:"stage"`
		} `json:"-"`
		Bare string
	}
	var got []StageField
	walkStageFields(reflect.TypeOf(node{}), "", map[reflect.Type]bool{}, &got)
	if want := []StageField{{"stage", "string"}}; !slices.Equal(got, want) {
		t.Errorf("walk = %v, want %v", got, want)
	}
}
