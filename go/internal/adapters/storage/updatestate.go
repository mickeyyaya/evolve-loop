package storage

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/flock"
	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/statemap"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

func (s *FilesystemStorage) UpdateState(_ context.Context, mutate func(*core.State)) (core.State, error) {
	path := statemap.ResolveWriteTarget(filepath.Join(s.evolveDir, "state.json"))
	release, err := flock.PathLock(path)
	if err != nil {
		return core.State{}, fmt.Errorf("update state: %w", err)
	}
	defer release()

	obj := map[string]json.RawMessage{}
	var st core.State
	raw, err := os.ReadFile(path)
	switch {
	case err == nil && len(raw) > 0:
		if uerr := json.Unmarshal(raw, &obj); uerr != nil {
			return core.State{}, fmt.Errorf("update state: state.json malformed (%w); refusing to clobber", uerr)
		}
		if uerr := json.Unmarshal(raw, &st); uerr != nil {
			return core.State{}, fmt.Errorf("update state: decode: %w", uerr)
		}
	case err != nil && !errors.Is(err, os.ErrNotExist):
		return core.State{}, fmt.Errorf("update state: read: %w", err)
	}

	mutate(&st)
	st.StateRevision++

	typed, err := marshalState(st)
	if err != nil {
		return core.State{}, fmt.Errorf("update state: marshal: %w", err)
	}
	typedMap := map[string]json.RawMessage{}
	if err := json.Unmarshal(typed, &typedMap); err != nil {
		return core.State{}, fmt.Errorf("update state: remap: %w", err)
	}
	for _, k := range modeledKeys {
		delete(obj, k)
	}
	for k, v := range typedMap {
		obj[k] = v
	}
	if err := writeJSONAtomic(path, obj); err != nil {
		return core.State{}, fmt.Errorf("update state: write: %w", err)
	}
	return st, nil
}

var marshalState = json.Marshal

var modeledKeys = modeledStateKeys()

func modeledStateKeys() []string {
	t := reflect.TypeOf(core.State{})
	keys := make([]string, 0, t.NumField())
	for i := 0; i < t.NumField(); i++ {
		tag := t.Field(i).Tag.Get("json")
		if tag == "" || tag == "-" {
			continue
		}
		if name, _, _ := strings.Cut(tag, ","); name != "" {
			keys = append(keys, name)
		}
	}
	return keys
}
