package bridge

import (
	"encoding/json"
	"fmt"
	"maps"
	"os"
	"path/filepath"
)

const manifestBaseKey = "base"

func manifestBytes(cli string) ([]byte, error) {
	if data, err := os.ReadFile(filepath.Join(bridgeManifestDir(), cli+".json")); err == nil {
		return data, nil
	}
	data, err := manifestFS.ReadFile("manifests/" + cli + ".json")
	if err != nil {
		return nil, fmt.Errorf("bridge:manifest: no manifest for cli=%s", cli)
	}
	return data, nil
}

func resolvedManifestBytes(cli string) ([]byte, error) {
	data, err := manifestBytes(cli)
	if err != nil {
		return nil, err
	}
	return resolveManifestBase(cli, data)
}

func resolveManifestBase(cli string, data []byte) ([]byte, error) {
	var target map[string]any
	if json.Unmarshal(data, &target) != nil {
		return data, nil
	}
	raw, hasBase := target[manifestBaseKey]
	if !hasBase {
		return data, nil
	}
	baseName, isName := raw.(string)
	if !isName || baseName == "" {
		return nil, fmt.Errorf("bridge:manifest: cli=%s: base must name the manifest of the binary it drives, got %v", cli, raw)
	}
	base, err := loadBaseObject(cli, baseName)
	if err != nil {
		return nil, err
	}
	delete(target, manifestBaseKey)
	return json.Marshal(mergePatch(base, target))
}

func loadBaseObject(cli, baseName string) (map[string]any, error) {
	data, err := manifestBytes(baseName)
	if err != nil {
		return nil, fmt.Errorf("bridge:manifest: cli=%s base=%q: %w", cli, baseName, err)
	}
	var base map[string]any
	if err := json.Unmarshal(data, &base); err != nil {
		return nil, fmt.Errorf("bridge:manifest: cli=%s base=%q: invalid JSON: %w", cli, baseName, err)
	}
	if _, chained := base[manifestBaseKey]; chained {
		return nil, fmt.Errorf("bridge:manifest: cli=%s base=%q itself names a base; a target names the manifest of its binary", cli, baseName)
	}
	return base, nil
}

func mergePatch(base any, patch any) any {
	patchObject, isObject := patch.(map[string]any)
	if !isObject {
		return patch
	}
	baseObject, _ := base.(map[string]any)
	merged := maps.Clone(baseObject)
	if merged == nil {
		merged = make(map[string]any, len(patchObject))
	}
	for key, value := range patchObject {
		if value == nil {
			delete(merged, key)
			continue
		}
		merged[key] = mergePatch(merged[key], value)
	}
	return merged
}
