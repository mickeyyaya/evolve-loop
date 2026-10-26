# Profile isolation policy recovery

Status: implemented in the September 2026 recovery changes. This document
supersedes earlier claims that a nested CLI marker establishes confinement.

## Issue and gap

Profiles declared filesystem denials, but the bridge parsed only the network
field. The sandbox wrapper omitted denials, and the Linux generator did not
apply its existing write-denial field. Consequently a profile declaration was
not evidence that a child process received that restriction.

Investigation also exposed three connected failures:

- Before the fix, the Linux launch prefix omitted the `bwrap` executable.
- Before the fix, macOS path aliases bypassed the repository write denial.
- Before the fix, a broad scratch-directory write grant reopened a repository located under that scratch directory.

Before the fix, a nested-session marker was accepted as satisfying mandatory sandboxing. The optional preflight
canary treated any setup error as verified outer confinement.

## Supported policy

The bridge now shares `profiles.SandboxConfig` with the canonical loader.
`sandbox.enabled=true` requires a successfully constructed OS sandbox wrapper
for the launch. An explicit `EVOLVE_SANDBOX=off` remains an operator opt-out,
reported as UNCONFINED. An absent or disabled profile sandbox does not introduce
a new mandatory requirement; an explicit launch requirement still applies.

The filesystem fields have separate meanings:

| Field | Meaning |
|---|---|
| `deny_subpaths` | Deny writes; inputs remain readable. |
| `deny_read_subpaths` | Deny reads; use this for private document directories. |

The existing Scout, Auditor, and Orchestrator profiles that named
`docs/private` now explicitly declare its read denial as well. This change does
not infer more private paths for unrelated profiles. Eval sources remain
readable for roles with write denials, including Builder.

TDD authors permanent
`.evolve/evals/<task-slug>.md` files inside its active worktree, as required by
its persona. Its profile permits that authoring. Builder retains its eval write
denial. The bridge independently keeps the main repository read-only for both
roles. This is a role boundary, not an append-only historical-eval contract.

Network access retains the existing model-CLI
behavior. This change does not supply an isolation guarantee for network access.

The bridge retains its existing worktree/workspace/scratch write allowlist.
This recovery does not implement the profile `write_subpaths` pattern language
or use `read_only_repo=false` to widen that allowlist.

Paths must be literal paths. Unknown placeholders, glob syntax, relative parent
escapes, and unresolved relative paths without a root are rejected. Relative
denials cover both the project and active worktree. Existing symlink targets
and canonical ancestors of missing descendants are included without reading
the protected contents. Canonical repository/write roots prevent macOS
`/var`/`/private/var` aliases from changing the effective boundary. Broad scratch
grants precede the repository restriction, and explicit denials follow writable
grants.

### Git worktree exception

For a linked worktree, the relative `.git` write denial protects that checkout's
pointer file. It is not expanded into a denial of the common repository object
store.

Source worktree staging and checkpoint commits require writes outside
the checkout:

- its index and lock
- HEAD and lock
- HEAD reflog
- commit message
- common content-addressed objects
- current branch refs/reflogs

The system derives these grants
from the actual linked-worktree metadata. The gitdir itself is never
granted writable. Explicit denials protect `commondir`, `gitdir`, configuration,
hooks, `info`, shallow/packed-ref controls, and object-store `info` (including
alternates), even when broad scratch permissions otherwise cover their parent.

macOS grants the required files and current branch lock paths precisely.
Bubblewrap needs coarse writable directory mounts to create and rename Git
lockfiles. Suppose the sandbox grants those mounts. Then they also expose routing/configuration metadata. Linked-worktree
Git metadata confinement is therefore explicitly unsupported on Linux. A
mandatory linked-worktree launch fails closed.

The explicit sandbox-off opt-out
still exists and is reported as unconfined. Existing phase/ship gates remain
responsible for the Git operation contract. The filesystem sandbox is not a
complete Git authorization system.

## Platform and capability limits

macOS emits explicit SBPL read/write denials after its allow rules. Linux emits
write-denial read-only mounts after writable mounts; directory read denials use
empty, mode-000, read-only tmpfs mounts with capabilities dropped. The production
Linux wrapper requires denial targets to exist, and read-denial targets to be
directories. Missing targets and file read-denial requests are explicitly
unsupported and fail mandatory launches closed. They are never silently
discarded through `--ro-bind-try`. This restriction may require provisioning
empty protected directories before enabling a profile on Linux.

A missing sandbox binary, measured inability to apply the sandbox, unsupported
policy, or nested environment without an applied wrapper cannot satisfy a
mandatory launch.

A successful capability measurement outranks a session hint
(such as a Codex PATH entry). It permits a try of the actual profile wrapper,
whose successful application is still required. An unchecked nested hint does
not establish capability or outer confinement. The preflight and dispatch gate
use the same decision. See [measured capability](measured-sandbox-capability.md).

The
tmux bridge also refuses to reuse an existing named session for a mandatory
profile: it cannot establish which policy confined that process. A new session
can apply the requested profile. Explicit sandbox-off remains the opt-out.

The optional nested canary remains a diagnostic of one try to write, not an
attestation of the requested profile's read/write policy. Missing-parent and
other setup failures are inconclusive. Even a permission-denied write can be
ordinary filesystem permissions. No canary result waives mandatory launch
controls. The default canary-off setting remains unchanged.

The existing broad HOME read permission and CLI state-directory writes remain.
This is protection of declared paths and the source worktree boundary. It does not
claim that these items are isolated: credentials, arbitrary hardlinks, concurrent host filesystem
changes, network endpoints, or every shared Git ref. The system resolves canonical
paths before launch. Host-side changes to symlink
targets during launch are outside this contract.

## Verification

The first regression tests failed on these defects:

- actual missing SBPL rules
- acceptance of unverified nested confinement
- a canary setup error called verified
- missing Linux write-denial mounts
- a Linux prefix without an executable

Other native negative probes reproduced main-checkout writes through a
path alias and through a broad scratch grant before the fix of those boundaries.

Run:

```sh
cd go
go test ./internal/adapters/sandbox ./internal/bridge ./internal/looppreflight ./internal/profiles -count=1
go test -tags=integration ./internal/adapters/sandbox ./internal/bridge -run 'TestSandboxFixtureChildEnforcesReadAndWritePolicy|TestLaunchProfilePolicyWithFixtureChild|TestSandboxPreservesLinkedWorktreeGitIndex|TestNativeRoleEvalAuthoringBoundary' -count=1 -v
```

Native child tests are behind the `integration` build tag.

`TestLaunchProfilePolicyWithFixtureChild` drives real profile parsing, launch
composition, and a harmless shell child under the native sandbox. It proves
that allowed work and eval reading succeed. Then it asserts that private reads (including
a symlink alias) and eval writes fail. A setup failure fails the test. The test does not
interpret it as confinement.

`TestSandboxPreservesLinkedWorktreeGitIndex`
uses a temporary Git repository under `/tmp` to prove that staging and checkpoint
commits succeed while main-checkout/shared-config writes fail. On macOS the test also checks
that a sibling ref cannot be created. Harmless zero-byte append-open probes
verify that routing/configuration metadata and object alternates are not writable
while staging and checkpoint commits continue to work. The tests read no real private
documentation.

Native child tests explicitly skip with UNVERIFIED capability diagnostics on a
host that cannot apply the sandbox. The recovery session executed these native
tests successfully on macOS. Linux argument construction and unsupported-path
behavior were tested deterministically; native Linux enforcement still requires
running the same fixture-child tests on a capable Linux host.

`TestNativeRoleEvalAuthoringBoundary` (macOS integration) loads the checked-in
TDD and Builder profiles through `Engine.LaunchArgs` and runs a harmless provider
fixture under the actual OS sandbox. It reproduces TDD authoring denial before
the profile repair. Then it proves these results:

- TDD creates a worktree eval.
- Both roles read existing evals.
- Builder cannot create or change evals.
- Neither role changes main-repository evals or protected profile files.

Denied writes preserve fixture
contents. The test uses no live provider or private document.
