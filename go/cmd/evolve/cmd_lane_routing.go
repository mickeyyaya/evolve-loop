package main

import (
	"fmt"
	"io"
	"sync"

	"github.com/mickeyyaya/evolve-loop/go/internal/guards"
	"github.com/mickeyyaya/evolve-loop/go/internal/lanerouting"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/build"
)

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
