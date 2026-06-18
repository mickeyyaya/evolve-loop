# Fixture skill (flagreaders regression test only — NOT a real skill)

This file exists solely to prove the flagreaders guard scans non-Go surfaces.

It references a flag that has NO registry row, the way `skills/loop/SKILL.md`
referenced `EVOLVE_TRIAGE_ENABLED` while cycle-360 removed flags that still had
shell readers:

    EVOLVE_FAKE_NONGO_ONLY_FLAG=1 evolve loop

It also references a real, registered flag which MUST NOT be reported as an
orphan (it has a row): set `EVOLVE_SANDBOX=1` to force the sandbox.

A dynamic-prefix fragment that must be IGNORED (it ends in `_`, like a built-up
key): `EVOLVE_E2E_MODEL_${cli}`.
