# internal/posteditvalidate

## Purpose

`internal/posteditvalidate` ports `legacy/scripts/verification/postedit-validate.sh`. It is the PostToolUse hook for `Edit` and `Write` tool calls. It checks the file that was just edited, by extension.

| Extension | Check |
|---|---|
| `.json` | parse check (`jq empty`, with an `encoding/json` fallback) |
| `.sh` | `bash -n` |
| `.py` | `python3 -m py_compile` |
| other | silent no-op |

## Design

- **The hook never blocks.** PostToolUse fires after the tool ran. When it finds a problem, it writes a WARN to `Options.LLMStderr`. Claude Code shows that text to the model, which then re-edits the file. `Run` always returns a nil error, and the command layer always exits 0.
- **Bypass is explicit.** `Options.Bypass` is wired from the `--bypass` flag of the command.
- **`Result.Kind` names the path taken:** `skip`, `json`, `sh`, `py`, `noop` or `bypass`. It is for tests and observation.
- **The validators are seams.** Each takes the file path and returns `(ok, errMsg)`. The defaults call out to `jq`, `bash` and `python3`.
- **A missing tool never gives a false positive.** If `bash` is absent, or neither `python3` nor `python` is on PATH, the validator returns `(true, "")`. A check that cannot run must not block or warn.
- **`jq` first for byte parity with bash.** If `jq` is absent, the validator uses `encoding/json`.
- **`py_compile` leaves a stamped pyc.** `cleanPyCache` removes `__pycache__/<name>.*.pyc` for the checked file only, as the bash `rm -rf` did. It leaves other files in the directory.
- **The guards log is best effort.** `appendGuardsLog` appends through `guardslog.Append`, which returns the close error so it is testable. This caller drops the error on purpose at this boundary. A read-only sandbox is expected to fail here, and that failure must never show. The default path is `<ProjectRoot>/.evolve/guards.log`.

## Invariants

- A tool that is absent from PATH gives `(true, "")` (`TestDefaultValidateBash_NoBash_NoFalsePositive`, `TestDefaultValidatePy_NoPython_NoFalsePositive`).
- An unopenable guards log writes nothing and does not panic (`TestAppendGuardsLog_OpenError_NoOp`).
- `cleanPyCache` removes only the pyc files of the checked source (`TestCleanPyCache_RemovesMatchingFiles`).

## Findings

- None recorded.
