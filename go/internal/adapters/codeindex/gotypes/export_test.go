package gotypes

import (
	"context"
	"io"

	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

var EncodeGob = encodeGob

func BuildWith(ctx context.Context, run sysexec.RunFunc, env []string, encode func(io.Writer, any) error, warn io.Writer, moduleDir, out string) error {
	return build(ctx, buildEnv{run: run, env: env, encode: encode, warn: warn}, moduleDir, out)
}

func WriteIndexVersion(path string, version int) error {
	return writeIndex(path, indexData{Version: version}, encodeGob)
}
