// Package yamlcodec is the single home of the gopkg.in/yaml.v3 dependency.
package yamlcodec

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

func Marshal(v any) ([]byte, error) {
	return yaml.Marshal(v)
}

func Unmarshal(data []byte, v any) error {
	return yaml.Unmarshal(data, v)
}

func UnmarshalStrict(data []byte, v any) error {
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	return dec.Decode(v)
}

type Kind int

const (
	KindScalar Kind = iota
	KindMapping
	KindSequence
)

type Node struct{ node *yaml.Node }

type SyntaxError struct {
	Line int
	Msg  string
}

func (e *SyntaxError) Error() string {
	if e.Line == 0 {
		return e.Msg
	}
	return fmt.Sprintf("line %d: %s", e.Line, e.Msg)
}

var syntaxLine = regexp.MustCompile(`^yaml: (?:line (\d+): )?(.*)$`)

func DecodeAll(data []byte) ([]Node, *SyntaxError) {
	dec := yaml.NewDecoder(bytes.NewReader(data))
	var docs []Node
	for {
		var doc yaml.Node
		err := dec.Decode(&doc)
		if errors.Is(err, io.EOF) {
			return docs, nil
		}
		if err != nil {
			return nil, newSyntaxError(err)
		}
		docs = append(docs, Node{node: doc.Content[0]})
	}
}

func newSyntaxError(err error) *SyntaxError {
	m := syntaxLine.FindStringSubmatch(strings.SplitN(err.Error(), "\n", 2)[0])
	if m == nil {
		return &SyntaxError{Msg: err.Error()}
	}
	line, _ := strconv.Atoi(m[1])
	return &SyntaxError{Line: line, Msg: m[2]}
}

func (n Node) Line() int { return n.node.Line }

func (n Node) Kind() Kind {
	switch deref(n.node).Kind {
	case yaml.MappingNode:
		return KindMapping
	case yaml.SequenceNode:
		return KindSequence
	}
	return KindScalar
}

func (n Node) Len() int {
	switch n.Kind() {
	case KindMapping:
		return len(mappingPairs(deref(n.node)))
	case KindSequence:
		return len(deref(n.node).Content)
	}
	return 0
}

func (n Node) Keys() []string {
	if n.Kind() != KindMapping {
		return nil
	}
	pairs := mappingPairs(deref(n.node))
	keys := make([]string, len(pairs))
	for i, p := range pairs {
		keys[i] = p.key
	}
	return keys
}

func (n Node) Get(key string) (Node, bool) {
	if n.Kind() != KindMapping {
		return Node{}, false
	}
	for _, p := range mappingPairs(deref(n.node)) {
		if p.key == key {
			return Node{node: p.value}, true
		}
	}
	return Node{}, false
}

type pair struct {
	key   string
	value *yaml.Node
}

const maxMergeDepth = 32

func deref(n *yaml.Node) *yaml.Node {
	for n.Kind == yaml.AliasNode {
		n = n.Alias
	}
	return n
}

func mappingPairs(m *yaml.Node) []pair {
	return mergedPairs(m, 0)
}

func mergedPairs(m *yaml.Node, depth int) []pair {
	var own, inherited []pair
	for i := 0; i+1 < len(m.Content); i += 2 {
		key, value := m.Content[i], m.Content[i+1]
		if key.Tag == "!!merge" {
			if depth < maxMergeDepth {
				inherited = append(inherited, mergeSources(value, depth+1)...)
			}
			continue
		}
		own = append(own, pair{key: key.Value, value: value})
	}
	return withMissing(own, inherited)
}

func mergeSources(value *yaml.Node, depth int) []pair {
	value = deref(value)
	switch value.Kind {
	case yaml.MappingNode:
		return mergedPairs(value, depth)
	case yaml.SequenceNode:
		var all []pair
		for _, item := range value.Content {
			if item = deref(item); item.Kind == yaml.MappingNode {
				all = append(all, mergedPairs(item, depth)...)
			}
		}
		return all
	}
	return nil
}

func withMissing(base, extra []pair) []pair {
	have := map[string]bool{}
	for _, p := range base {
		have[p.key] = true
	}
	for _, p := range extra {
		if !have[p.key] {
			have[p.key] = true
			base = append(base, p)
		}
	}
	return base
}
