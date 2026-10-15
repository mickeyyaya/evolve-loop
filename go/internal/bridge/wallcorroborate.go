package bridge

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"
)

type WallCorroborator func(ctx context.Context, cli string) bool

var wallProbeTimeout = 60 * time.Second

var wallProbeRecipes = map[string]struct {
	argv  []string
	stdin string
}{
	"claude": {argv: []string{"claude", "-p", "reply with exactly: OK", "--model", "haiku"}},
	"codex":  {argv: []string{"codex", "exec", "-"}, stdin: "reply with exactly: OK"},
}

func DefaultWallCorroborator(run CmdRunner, log io.Writer) WallCorroborator {
	if run == nil {
		run = execRunner
	}
	if log == nil {
		log = io.Discard
	}
	return func(ctx context.Context, cli string) bool {
		binary := driverBinary(cli)
		recipe, ok := wallProbeRecipes[binary]
		if !ok {
			return true
		}
		pctx, cancel := context.WithTimeout(ctx, wallProbeTimeout)
		defer cancel()
		var stdin io.Reader
		if recipe.stdin != "" {
			stdin = strings.NewReader(recipe.stdin)
		}
		rc, err := run(pctx, recipe.argv[0], "", recipe.argv[1:], nil, stdin, io.Discard, io.Discard)
		if err != nil || rc != 0 {
			fmt.Fprintf(log, "[bridge] wall corroborated for %s: probe rc=%d err=%v\n", binary, rc, err)
			return true
		}
		return false
	}
}

func wallCorroborated(ctx context.Context, corroborate WallCorroborator, cli string) bool {
	if corroborate == nil {
		return true
	}
	return corroborate(ctx, cli)
}

type checkpointWallState struct {
	suppressed bool
	probed     bool
	confirmed  bool
}

func (st *checkpointWallState) decide(ctx context.Context, corroborate WallCorroborator, cli string, gateCrossed bool) (escalate, suppressNow bool) {
	if st.suppressed || !gateCrossed {
		return false, false
	}
	if !st.probed {
		st.probed = true
		st.confirmed = wallCorroborated(ctx, corroborate, cli)
		if !st.confirmed {
			st.suppressed = true
			return false, true
		}
	}
	return st.confirmed, false
}
