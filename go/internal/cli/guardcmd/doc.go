// Package guardcmd holds the `evolve` CLI handlers for the trust-kernel GUARD
// and pre-commit-gate subcommands — guard, commit-gate, commit-prefix-gate,
// eval, preflight-environment, and postedit-validate.
//
// HOW: each exported Run* function has the standard subcommand signature
// (args []string, stdin io.Reader, stdout, stderr io.Writer) int and is wired
// into the dispatcher table in cmd/evolve/registry.go. Handlers parse their own
// flags and delegate the real work to the internal/* packages (commitgate,
// commitprefixgate, evalgate, preflight, posteditvalidate, guards); shared CLI
// helpers come from cmd/evolve/cmdutil. The package holds no business logic.
//
// WHY: cmd/evolve was a 77-handler package main; grouping the guard/gate
// handlers into their own importable package (SRP) shrinks the composition root
// and lets these handlers be tested directly. cmd is a fan-in-0 sink, so the
// extraction carries no import-cycle risk; registry.go now references
// guardcmd.Run* instead of package-main-private funcs.
//
// Key exported symbols:
//   - [RunGuard], [RunCommitGate], [RunCommitPrefixGate] — trust-kernel + commit gates
//   - [RunEval] — eval-quality / verify subcommands
//   - [RunPreflight], [RunPostEditValidate] — environment probe + PostToolUse validator
//
// Depends on: cmd/evolve/cmdutil + the internal/* gate implementations.
package guardcmd
