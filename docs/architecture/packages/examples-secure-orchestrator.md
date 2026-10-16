# examples/secure-orchestrator

## Purpose

`examples/secure-orchestrator` is a reference program that shows four defenses against reward hacking by LLM agents. It is example code and not part of the loop. The teaching text that the program once carried as comments is now in [the README of the example](../../../go/examples/secure-orchestrator/README.md).

## Design

- **One file, stdlib only.** `main.go` is a `package main` that runs a mock cycle (scout, builder, auditor, ship). The Go port replaced an earlier Python script.
- **Named functions carry the defenses.** `runSafeCommand` and `safeGitCommit` (no shell injection), `setupSecureWorktree` (hook prevention), `hashFile` and `verifyTestIntegrity` (tamper-proofing), `stripCommentsForAuditor` (prompt-injection defense).

## Invariants

- **No shell.** Every command goes through an argv slice. The example must never build a command from a string.
- **The package doc stays short.** The explanation of each defense lives in the README, so the program stays readable as code.
