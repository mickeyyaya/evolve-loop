package main

import "github.com/mickeyyaya/evolveloop/go/cmd/evolve/cmdutil"

// reorderArgs forwards to cmdutil.ReorderArgs — the implementation now lives in
// the cmd/evolve/cmdutil leaf so the decomposed internal/cli/* command groups
// share ONE definition. Thin forwarder kept so the in-package callers
// (cmd_models, cmd_phase_verify, cmd_setup) are unchanged.
func reorderArgs(args []string) []string { return cmdutil.ReorderArgs(args) }
