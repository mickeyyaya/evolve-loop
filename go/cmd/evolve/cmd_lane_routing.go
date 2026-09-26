package main

import (
	"fmt"
	"io"
	"sync"

	"github.com/mickeyyaya/evolve-loop/go/internal/guards"
	"github.com/mickeyyaya/evolve-loop/go/internal/lanerouting"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/build"
)

// laneForbidden is the one routing predicate every routing root uses: protected surface, or a
// path the build profile's sandbox denies. The profile loads on the first routing question, so a
// root that never routes stays silent; a profile that will not load leaves protected surface only, loudly.
func laneForbidden(projectRoot string, warn io.Writer) func(string) bool {
	var once sync.Once
	judge := guards.IsProtectedScope
	return func(path string) bool {
		once.Do(func() {
			forbidden, err := lanerouting.Forbidden(projectRoot, build.ProfileName)
			if err != nil {
				fmt.Fprintf(warn, "[routing] WARN %v; routing judges protected surface only\n", err)
				return
			}
			judge = forbidden
		})
		return judge(path)
	}
}
