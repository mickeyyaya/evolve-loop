package main

import (
	"bytes"
	"cmp"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"slices"

	"github.com/mickeyyaya/evolve-loop/go/internal/atomicwrite"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

var retiredEffortKeys = []string{"effort_level", "effort_overrides"}

type profileEffort struct {
	Name             string            `json:"name"`
	ModelTierDefault string            `json:"model_tier_default"`
	EffortLevel      *string           `json:"effort_level"`
	EffortOverrides  map[string]string `json:"effort_overrides"`
}

func moveProfileEfforts(dir string, block policy.CLIRouting, rewrites map[string][]byte) (policy.CLIRouting, error) {
	paths, err := filepath.Glob(filepath.Join(dir, "*.json"))
	if err != nil {
		return block, err
	}
	slices.Sort(paths)
	for _, path := range paths {
		raw, err := os.ReadFile(path)
		if err != nil {
			return block, err
		}
		var p profileEffort
		if json.Unmarshal(raw, &p) != nil || p.Name == "" || (p.EffortLevel == nil && p.EffortOverrides == nil) {
			continue
		}
		file := filepath.Base(path)
		if block, err = moveOneProfileEffort(block, p, file); err != nil {
			return block, err
		}
		if rewrites[file], err = removeTopLevelKeys(raw, retiredEffortKeys...); err != nil {
			return block, fmt.Errorf("%s: %w", file, err)
		}
	}
	return block, nil
}

func moveOneProfileEffort(block policy.CLIRouting, p profileEffort, file string) (policy.CLIRouting, error) {
	if p.EffortLevel != nil && *p.EffortLevel != "" {
		level := *p.EffortLevel
		inherited, _ := efforts(withoutAgentEffort(block, p.Name)).Resolve(cmp.Or(p.ModelTierDefault, unsetDispatchTier))
		switch current := block.Agents[p.Name].Effort; {
		case current != "" && current != level:
			return block, fmt.Errorf("%s: effort_level %q and cli_routing.agents.%s.effort %q differ: keep one", file, level, p.Name, current)
		case current == "" && level != inherited:
			block.Agents = withEntry(maps.Clone(block.Agents), p.Name, agentRuleWith(block.Agents[p.Name], nil, "", level))
		}
	}
	table := efforts(block)
	for _, tier := range slices.Sorted(maps.Keys(p.EffortOverrides)) {
		if got, _ := table.Resolve(tier, p.Name); got != p.EffortOverrides[tier] {
			return block, fmt.Errorf("%s: effort_overrides.%s = %q has no table form (the table gives %q): set cli_routing.tiers.%s or agents.%s with --effort, then remove the key", file, tier, p.EffortOverrides[tier], got, tier, p.Name)
		}
	}
	return block, nil
}

func efforts(block policy.CLIRouting) policy.EffortTable {
	return policy.Policy{CLIRouting: &block}.Efforts()
}

func withoutAgentEffort(block policy.CLIRouting, agent string) policy.CLIRouting {
	rule, ok := block.Agents[agent]
	if !ok {
		return block
	}
	rule.Effort = ""
	block.Agents = withEntry(maps.Clone(block.Agents), agent, rule)
	return block
}

func removeTopLevelKeys(raw []byte, keys ...string) ([]byte, error) {
	out := raw
	for _, key := range keys {
		next, err := removeTopLevelKey(out, key)
		if err != nil {
			return nil, err
		}
		out = next
	}
	return out, nil
}

type memberSpan struct {
	key        string
	start, end int64
}

func removeTopLevelKey(raw []byte, key string) ([]byte, error) {
	members, err := topLevelMembers(raw)
	if err != nil {
		return nil, err
	}
	i := slices.IndexFunc(members, func(m memberSpan) bool { return m.key == key })
	switch {
	case i < 0:
		return raw, nil
	case i > 0:
		return slices.Concat(raw[:members[i].start], raw[members[i].end:]), nil
	case len(members) == 1:
		return slices.Concat(raw[:members[0].start], raw[members[0].end:]), nil
	}
	comma := members[1].start + int64(bytes.IndexByte(raw[members[1].start:], ',')) + 1
	return slices.Concat(raw[:members[0].start], raw[comma:]), nil
}

func topLevelMembers(raw []byte) ([]memberSpan, error) {
	dec := json.NewDecoder(bytes.NewReader(raw))
	if tok, err := dec.Token(); err != nil || tok != json.Delim('{') {
		return nil, fmt.Errorf("want a JSON object")
	}
	var members []memberSpan
	for dec.More() {
		start := dec.InputOffset()
		tok, err := dec.Token()
		if err != nil {
			return nil, err
		}
		var value json.RawMessage
		if err := dec.Decode(&value); err != nil {
			return nil, err
		}
		members = append(members, memberSpan{key: fmt.Sprint(tok), start: start, end: dec.InputOffset()})
	}
	return members, nil
}

func reportProfileRewrites(w io.Writer, dir string, rewrites map[string][]byte, dryRun bool) {
	verb := "rewrote"
	if dryRun {
		verb = "would rewrite"
	}
	for _, file := range slices.Sorted(maps.Keys(rewrites)) {
		fmt.Fprintf(w, "%s %s: removed %v (moved to cli_routing)\n", verb, filepath.Join(dir, file), retiredEffortKeys)
	}
}

func writeProfileRewrites(dir string, rewrites map[string][]byte) error {
	for _, file := range slices.Sorted(maps.Keys(rewrites)) {
		if err := atomicwrite.Bytes(filepath.Join(dir, file), rewrites[file]); err != nil {
			return err
		}
	}
	return nil
}

type profileOverlay struct {
	base  fs.FS
	files map[string][]byte
}

func (o profileOverlay) Open(name string) (fs.File, error) { return o.base.Open(name) }

func (o profileOverlay) ReadFile(name string) ([]byte, error) {
	if data, ok := o.files[name]; ok {
		return slices.Clone(data), nil
	}
	return fs.ReadFile(o.base, name)
}

func (o profileOverlay) ReadDir(name string) ([]fs.DirEntry, error) { return fs.ReadDir(o.base, name) }
