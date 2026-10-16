# internal/kerneltest

## Purpose

`internal/kerneltest` is a rename-proof test fixture for the transition kernel. It loads the real phase configuration and exposes phases by structural role: the first anchor, the ship terminal and the verdict-branching evaluator. A test that uses these accessors survives a rename of any phase in the registry.

## Design

- **Imports only `phasespec` and `config`.** It does not import `core`, so white-box tests in `package core` can use it without an import cycle. The phase names it returns are converted to `core.Phase` by the caller, through the kernel's own name bridge.
- **Structural accessors.** `FirstAnchor` is the first mandatory anchor, `ShipTerminal` is the last, `SpineEntry` is the first spine phase. `Evaluator` is the first mandatory phase whose descriptor declares both `on_pass` and `on_fail`; it returns the empty string when none does.
- **The registry path is resolved from the source file.** `registryPath` walks up from this file's location, so the fixture works from any package's test working directory.
- **Fatal on a load error.** A kernel test cannot run without the real flow.

## Invariants

- **The reference flow declares its anchors and a verdict-branching evaluator.** `TestFixture_LoadsAndResolvesStructurally` pins this without naming a phase.

## Findings

- None.
