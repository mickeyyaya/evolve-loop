package stageevidence

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
)

type ChainReader interface {
	ReadChain(ctx context.Context, key string) (Chain, error)
}

type fileChainReader struct {
	path string
}

func FileChain(path string) ChainReader {
	return fileChainReader{path: path}
}

func (f fileChainReader) ReadChain(_ context.Context, key string) (Chain, error) {
	raw, err := os.ReadFile(f.path)
	if err != nil {
		return Chain{}, fmt.Errorf("stageevidence: read chain %s: %w", f.path, err)
	}
	var whole Chain
	if err := json.Unmarshal(raw, &whole); err != nil {
		return Chain{}, fmt.Errorf("stageevidence: parse chain %s: %w", f.path, err)
	}
	if err := VerifyChain(whole); err != nil {
		return Chain{}, err
	}
	filtered := Chain{preverified: true}
	scopes := map[ScopeKey]bool{}
	for _, r := range whole.Records {
		if r.Key == key {
			filtered.Records = append(filtered.Records, r)
			scopes[r.ScopeKey()] = true
		}
	}
	if len(filtered.Records) == 0 {
		return Chain{}, ErrEmptyChain
	}
	for _, o := range whole.Outcomes {
		if scopes[o.ScopeKey()] {
			filtered.Outcomes = append(filtered.Outcomes, o)
		}
	}
	return filtered, nil
}
