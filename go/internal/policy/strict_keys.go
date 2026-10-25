package policy

import (
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"reflect"
	"slices"
	"strings"
)

var ErrUnknownKey = errors.New("unknown policy key")

var metadataKeys = []string{"$schema", "description", "version"}

var keyShapes = map[reflect.Type]reflect.Type{
	reflect.TypeFor[HandoffPolicy]():      reflect.TypeFor[map[string]handoffPairJSON](),
	reflect.TypeFor[QualityPanelPolicy](): reflect.TypeFor[qualityPanelJSON](),
}

func refuseUnknownKeys(raw []byte) error {
	var doc any
	if err := json.Unmarshal(raw, &doc); err != nil {
		return err
	}
	if top, ok := doc.(map[string]any); ok {
		for _, key := range metadataKeys {
			delete(top, key)
		}
	}
	return unknownKey(doc, reflect.TypeFor[Policy](), "")
}

func unknownKey(value any, t reflect.Type, path string) error {
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	if shape, ok := keyShapes[t]; ok {
		t = shape
	}
	switch v := value.(type) {
	case map[string]any:
		if t.Kind() != reflect.Struct && t.Kind() != reflect.Map {
			return nil
		}
		return unknownObjectKey(v, t, path)
	case []any:
		if t.Kind() != reflect.Slice {
			return nil
		}
		for i, elem := range v {
			if err := unknownKey(elem, t.Elem(), fmt.Sprintf("%s[%d]", path, i)); err != nil {
				return err
			}
		}
	}
	return nil
}

func unknownObjectKey(obj map[string]any, t reflect.Type, path string) error {
	for _, key := range slices.Sorted(maps.Keys(obj)) {
		child := strings.TrimPrefix(path+"."+key, ".")
		fieldType, ok := keyType(t, key)
		if !ok {
			return fmt.Errorf("%w %q: remove it or check its spelling; docs/architecture/policy-config.md lists the keys", ErrUnknownKey, child)
		}
		if err := unknownKey(obj[key], fieldType, child); err != nil {
			return err
		}
	}
	return nil
}

func keyType(t reflect.Type, key string) (reflect.Type, bool) {
	if t.Kind() == reflect.Map {
		return t.Elem(), true
	}
	for i := range t.NumField() {
		f := t.Field(i)
		name, _, _ := strings.Cut(f.Tag.Get("json"), ",")
		if f.IsExported() && name == key {
			return f.Type, true
		}
	}
	return nil, false
}
