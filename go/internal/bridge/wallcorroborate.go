package bridge

// wallcorroborate.go — out-of-band corroboration for the exhaustion fast-fail
// (2026-08-15 false-wall incident; inbox exhaustion-scan-needs-corroboration).
//
// The pane exhaustion scan can never distinguish STATE from SUBJECT MATTER: a
// lane fixing the exhaustion regexes renders the true wall phrases from its
// own test fixtures, the file persists on-pane exactly like a real wall, both
// existing guards (prompt-echo strip, persistence gate) are structurally
// defeated, and every fallback family "walls" on the same content — a false
// all-families quota checkpoint while the plans have headroom.
//
// Pattern: Strategy via DI. WallCorroborator is a Deps seam (like Runner/Now):
// on a persistence-gate cross the site asks the corroborator; only a
// corroborated wall escalates rc 85. The default strategy sends ONE cheap
// headless request to the family — a truth the pane's content cannot forge:
// a provider that answers is not walled, whatever the terminal shows. nil
// corroborator = legacy behavior byte-identical (every pre-existing test and
// caller unaffected); the production composition root wires
// DefaultWallCorroborator explicitly.

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"
)

// WallCorroborator reports whether the CLI family is REALLY walled, via a
// check independent of pane content. true = wall corroborated (escalate);
// false = provider answered (suppress the pane match as content-induced).
type WallCorroborator func(ctx context.Context, cli string) bool

// wallProbeTimeout bounds one corroboration probe. Var: the hung-probe
// contract is test-pinned. A provider that cannot answer one token within
// this budget is walled in every sense that matters to a dispatch.
var wallProbeTimeout = 60 * time.Second

// wallProbeRecipes is the per-family probe DATA: the cheapest one-shot
// headless request each CLI supports. Families without an entry cannot be
// corroborated and stay conservative (walled=true — legacy behavior). Argv
// forms verified live 2026-08-15 (CODEX-ALIVE / CLAUDE-ALIVE probes).
var wallProbeRecipes = map[string]struct {
	argv  []string
	stdin string
}{
	"claude": {argv: []string{"claude", "-p", "reply with exactly: OK", "--model", "haiku"}},
	"codex":  {argv: []string{"codex", "exec", "-"}, stdin: "reply with exactly: OK"},
}

// DefaultWallCorroborator builds the production strategy over the given
// runner. One probe per call; success (rc=0) is proof of service. Probe
// errors, non-zero exits, and deadline expiry all corroborate the wall —
// the conservative direction (a missed suppression merely fails over, the
// pre-fix behavior; a missed wall would burn the artifact window).
func DefaultWallCorroborator(run CmdRunner, log io.Writer) WallCorroborator {
	if run == nil {
		run = execRunner // production subprocess seam (withDefaults precedent)
	}
	if log == nil {
		log = io.Discard
	}
	return func(ctx context.Context, cli string) bool {
		family := strings.TrimSuffix(cli, "-tmux")
		recipe, ok := wallProbeRecipes[family]
		if !ok {
			return true // no recipe — cannot corroborate, stay conservative
		}
		pctx, cancel := context.WithTimeout(ctx, wallProbeTimeout)
		defer cancel()
		var stdin io.Reader
		if recipe.stdin != "" {
			stdin = strings.NewReader(recipe.stdin)
		}
		rc, err := run(pctx, recipe.argv[0], "", recipe.argv[1:], nil, stdin, io.Discard, io.Discard)
		if err != nil || rc != 0 {
			fmt.Fprintf(log, "[bridge] wall corroborated for %s: probe rc=%d err=%v\n", family, rc, err)
			return true
		}
		return false
	}
}

// wallCorroborated is the shared site decision: nil corroborator preserves
// the legacy verdict (pane match = wall); otherwise the strategy decides.
// Both fast-fail sites (the autorespond tick and the stop-review checkpoint)
// route through here so their semantics can never drift.
func wallCorroborated(ctx context.Context, corroborate WallCorroborator, cli string) bool {
	if corroborate == nil {
		return true
	}
	return corroborate(ctx, cli)
}

// checkpointWallState carries the stop-review site's one-probe latch. Extracted
// from runTmuxREPL's checkpoint block (review HIGH-2: the site's logic needs a
// direct test; inline it was untestable without contriving a pane the fast
// poll cannot see first).
type checkpointWallState struct {
	suppressed bool
	probed     bool
	confirmed  bool
}

// decide consumes one gate observation. escalate=true means the wall is
// corroborated (site returns rc 85); suppressNow=true exactly once, on the
// healthy verdict, so the caller logs the suppression a single time. The
// probe runs at most ONCE for the lifetime of the state, whatever it answers.
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
