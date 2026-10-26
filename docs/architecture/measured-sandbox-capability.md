# Measured sandbox capability and session hints

The first two-wave recovery verification try on 2026-09-09 (Taipei)
halted during readiness, before it allocated any cycle. The host report claimed
that Darwin nested-Claude `sandbox_apply` returns EPERM because PATH contained
a Codex session marker. In the same environment the native capability probe
(`sandbox-exec` running `/usr/bin/true`) succeeded. This try is not a
completed verification wave.

## Issue and correction

Preflight guessed capability from the session marker. The bridge refused
all nested hints before it considered the measured result. The session detector
can identify a managed CLI context. It cannot prove an outer sandbox, and it cannot prove
that an inner sandbox fails to apply.

Both consumers now use `sandbox.ShouldWrap`. The rules are:

- Unsupported platforms and missing binaries are rejected.
- A completed capability measurement governs if the actual profile wrapper can be tried.
- Measured failure is rejected.
- A nested hint without a completed measurement remains rejected.
- Standalone unmeasured availability keeps its existing behavior.

Preflight reports the same decision and explanation. It does not advertise a startup fallback when wrapping works.

The capability probe is only a startup measurement. It is not an attestation that a
profile enforces all of its paths. Required launches still apply their actual
profile and fail closed if it cannot be applied. This change does not authorize
an unwrapped retry. It does not trust the confinement of an outer CLI. Named-session reuse and
unsupported Linux denial/linked-worktree policies keep their existing gates.

The separately configured nested canary can still halt an explicitly enforced
canary policy. It is a diagnostic of one write. It is not permission to omit a required
wrapper. Its default remains off.

## Profile startup contracts

Applying real profiles exposed two more defects. An empty optional
repository root rendered an invalid SBPL empty subpath. The generator now skips
that absent read root, as it does for its other optional path fields. The shared real
tmux fixture also supplies its actual project root. A native empty-root profile
test permits a declared write and keeps an explicit protected-file denial.

`AllowNetwork=true` previously emitted no network rule under `deny default`, so
macOS still rejected connections. The generator now explicitly allows networking
when requested. It keeps the explicit denial when networking is disabled.

In the true case, native tests connect
to a local fixture server. In the false case, the tests require a connection denial.
Startup controls run before and after. Filesystem rules are
unchanged by this network setting. The bridge already requests network access
for model-reaching phases.

Interactive macOS clients also need to reopen their controlling terminal and
configure raw mode. The old profile denied both, and this left Claude's trust dialog
malformed.

After the bridge creates a tmux session, it resolves the exact terminal of the session.
It validates the clean numeric `/dev/ttys...` character-device path. It passes the path
through the existing wrapper request.

The grant permits data writes. It permits only these ioctl commands on the assigned device and `/dev/tty`:

- `TIOCGETA`;
- the three termios setters (`TIOCSETA`, `TIOCSETAW`, `TIOCSETAF`);
- `TIOCGWINSZ`.

The ioctl command filter is
combined with the exact paths with `require-all`. A review rejected a broader ioctl grant
because an owned-PTY probe showed that it permits input injection.
The filtered policy denied that operation. The durable fixture uses a harmless
excluded line-discipline query, with an unwrapped positive control, to prove
that unlisted ioctls are denied.

Headless launches receive no terminal grant. Lookup
failures stop before the bridge launches the child. Custom controllers
without terminal lookup keep the existing profile without extra grants.

The native fixture independently identifies its assigned terminal and exercises raw
mode and writes. It denies the write/ioctl access of a separate terminal and a protected
file write. It completes a delivered task.

The first Node-only setter
allowance still blocked Agy and Codex during
real boot checks. A native helper now exercises all three termios update modes.
It failed on the restricted policy before the other setters were allowed.
After that, Claude, Codex and Agy all passed real sandboxed boot checks.

## Validation

Regression tests keep the Codex PATH hint. They cover successful, failed and
unchecked measurements on both supported platforms. The real macOS child fixture
also keeps that hint and applies a mandatory production profile. It permits its
worktree reads/writes. It denies protected reads (including a symlink alias)
and evaluation writes. Those cases failed before the correction and pass after it.

The recovery validation report tracks broader tmux launch validation and the two actual live waves.
A passing capability measurement alone is not proof
that interactive CLIs boot or that the loop produces useful changes.
