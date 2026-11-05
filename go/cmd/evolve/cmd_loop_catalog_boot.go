package main

import (
	"context"
	"fmt"
	"io"
)

func refreshCatalogAtLoopBoot(ctx context.Context, refresh func(context.Context) error, stderr io.Writer) {
	if refresh == nil {
		return
	}
	if err := refresh(ctx); err != nil {
		fmt.Fprintf(stderr, "[loop] WARN catalog refresh at boot failed: %v\n", err)
	}
}
