package router

import (
	"fmt"
	"sort"
	"strings"
)

func RenderRecipeProjection(recipes map[string][]string) string {
	types := make([]string, 0, len(recipes))
	for t := range recipes {
		types = append(types, t)
	}
	sort.Strings(types)
	var b strings.Builder
	for _, t := range types {
		fmt.Fprintf(&b, "| %s | %s |\n", t, strings.Join(recipes[t], " → "))
	}
	return b.String()
}
