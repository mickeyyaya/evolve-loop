# Measured sandbox capability and session hints

The first two-wave recovery verification attempt on 2026-09-09 (Taipei)
halted during readiness, before allocating any cycle. The host report claimed
Darwin nested-Claude `sandbox_apply` would return EPERM because PATH contained
a Codex session marker. In the same environment the native capability probe
(`sandbox-exec` running `/usr/bin/true`) succeeded. This attempt is not a
completed verification wave.

## Issue and correction

Preflight guessed capability from the session marker while the bridge refused
all nested hints before considering the measured result. The session detector
can identify a managed CLI context; it cannot prove either an outer sandbox or
inability to apply an inner one.

Both consumers now use `sandbox.ShouldWrap`. Unsupported platforms and missing
binaries are rejected. A completed capability measurement governs whether the
actual profile wrapper may be attempted. Measured failure is rejected; a nested
hint without a completed measurement remains rejected. Standalone unmeasured
availability retains its existing behavior. Preflight reports the same decision
and explanation, and does not advertise a startup fallback when wrapping works.

The capability probe is only a startup measurement, not an attestation that a
profile enforces all of its paths. Required launches still apply their actual
profile and fail closed if it cannot be applied. This change does not authorize
an unwrapped retry or trust an outer CLI's confinement. Named-session reuse and
unsupported Linux denial/linked-worktree policies retain their existing gates.

The separately configured nested canary may still halt an explicitly enforced
canary policy. It is a diagnostic of one write, not permission to omit a required
wrapper; its default remains off.

## Profile startup contracts

Applying real profiles exposed two additional defects. An empty optional
repository root rendered an invalid SBPL empty subpath; the generator now skips
that absent read root, matching its other optional path fields. The shared real
tmux fixture also supplies its actual project root. A native empty-root profile
test permits a declared write while retaining an explicit protected-file denial.

`AllowNetwork=true` previously emitted no network rule under `deny default`, so
macOS still rejected connections. The generator now explicitly allows networking
when requested and keeps the explicit denial when disabled. Native tests connect
to a local fixture server in the true case and require a connection denial in
the false case; startup controls run before and after. Filesystem rules are
unchanged by this network setting. The bridge already requests network access
for model-reaching phases.

Interactive macOS clients also need to reopen their controlling terminal and
configure raw mode. The old profile denied both, leaving Claude's trust dialog
malformed. After creating a tmux session, the bridge resolves its exact terminal,
validates the clean numeric `/dev/ttys...` character-device path, and passes it
through the existing wrapper request. The grant permits data writes and only `TIOCGETA`, the three termios setters
(`TIOCSETA`, `TIOCSETAW`, `TIOCSETAF`), and `TIOCGWINSZ` on the assigned device and `/dev/tty`; the ioctl command filter is
combined with the exact paths using `require-all`. A broader ioctl grant was
rejected in review because an owned-PTY probe showed it permits input injection.
The filtered policy denied that operation. The durable fixture uses a harmless
excluded line-discipline query, with an unwrapped positive control, to prove
unlisted ioctls are denied. Headless launches receive no terminal grant. Lookup
failures stop before launching the child. Custom controllers
without terminal lookup retain the existing profile without extra grants.
The native fixture independently identifies its assigned terminal, exercises raw
mode and writes, denies a separate terminal's write/ioctl access and a protected
file write, and completes a delivered task. The initial Node-only setter
allowance still blocked Agy and Codex during
real boot checks. A native helper now exercises all three termios update modes;
it failed on the restricted policy before the additional setters were allowed.
Claude, Codex and Agy all subsequently passed real sandboxed boot checks.

## Validation

Regression tests retain the Codex PATH hint and cover successful, failed, and
unchecked measurements on both supported platforms. The real macOS child fixture
also retains that hint, applies a mandatory production profile, permits its
worktree reads/writes, and denies protected reads (including a symlink alias)
and evaluation writes. Those cases failed before the correction and pass after it.

Broader tmux launch validation and the two actual live waves are tracked in the
recovery validation report; a passing capability measurement alone is not proof
that interactive CLIs boot or that the loop produces useful changes.
