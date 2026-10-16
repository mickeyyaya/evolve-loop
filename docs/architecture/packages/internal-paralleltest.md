# internal/paralleltest

## Purpose

`internal/paralleltest` is the one guard that keeps test trees parallel by default. Each package test reads its own sources and lists its serial tests. The guard flags a test that is neither parallel nor listed. It also flags a parallel test that changes process state, directly, through a helper or through a method on a package-level value. Background: [the cycle 1869 build explainer](../../explain/builds/cycle-1869-01m4rvyfk91nmt5wksrgbwsq88.md).

## Design

- The guard reads sources, so it is a pure static check with no build of the package under test.

## Invariants

- A new serial test enters the package's list with its reason, never by dropping `t.Parallel()` silently.
