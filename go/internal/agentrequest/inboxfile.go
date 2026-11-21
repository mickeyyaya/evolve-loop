package agentrequest

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"slices"

	"github.com/mickeyyaya/evolve-loop/go/internal/agentclass"
)

var ErrItemRefused = errors.New("the inbox intake refused the item")

type Filed struct {
	ID   string
	Path string
}

type FileFunc func(ctx context.Context, projectRoot string, raw []byte, source string) (Filed, error)

func sourceOf(cell Cell, name string) string {
	return fmt.Sprintf("request:%d/%s", cell.Cycle, name)
}

func unknownItemArgs(args map[string]string) []string {
	var unknown []string
	for _, key := range slices.Sorted(maps.Keys(args)) {
		if _, _, ok := agentclass.ParseArg(agentclass.KindInboxAdd, key); !ok {
			unknown = append(unknown, key)
		}
	}
	return unknown
}

func numbered(args map[string]string, prefix string) []string {
	type entry struct {
		n     int
		value string
	}
	var entries []entry
	for key, value := range args {
		if name, n, ok := agentclass.ParseArg(agentclass.KindInboxAdd, key); ok && name == prefix {
			entries = append(entries, entry{n, value})
		}
	}
	slices.SortFunc(entries, func(a, b entry) int { return a.n - b.n })
	out := make([]string, len(entries))
	for i, e := range entries {
		out[i] = e.value
	}
	return out
}

func buildItem(req Request, weight float64) ([]byte, error) {
	item := map[string]any{"summary": req.Why, "weight": weight}
	for _, spec := range agentclass.ArgSpecs(agentclass.KindInboxAdd) {
		if spec.Numbered {
			item[spec.Name] = numbered(req.What.Args, spec.Name)
		} else {
			item[spec.Name] = req.What.Args[spec.Name]
		}
	}
	return json.Marshal(item)
}
