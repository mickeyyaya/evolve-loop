# internal/logcompress

> The task: `artifact-log-compression`. The predicates: `go/acs/cycle2248/predicates_test.go`. The settings: the `log_compression` block of policy.json, decoded by `Policy.LogCompressionConfig` in [internal-policy](internal-policy.md).

## Purpose

`internal/logcompress` removes bulky, low-signal lines from a test log, a build output or a diff before a prompt carries it. The filter is pure. It reads no file, no environment and no clock. The same input and settings always give the same output.

## Design

- **One function.** `Compress` takes the text and the settings. It returns a `Result` with the text, `Applied`, and the byte and token counts of the input and the output.
- **Lines are kept or elided, never edited.** Each kept line is an unmodified input line in input order. An elided group becomes one marker line: `[logcompress: N lines elided: <reason>]`. Kept lines plus the elided counts always equal the input line count.
- **Four reasons to elide.**
  - A run of at least `pass_run_min` lines that start with a `pass_prefixes` entry.
  - Each line that starts with a `noise_prefixes` entry.
  - A run of at least `repeat_run_min` identical lines. The first line stays.
  - The body of a large diff file. The header lines stay. The marker holds the exact `+N -M` count.
- **Failure lines are never elided.** A line that contains a `failure_markers` entry is out of every run, so a repeated failure line stays in full count. A failure-only log passes through byte-exact.
- **Never-compress regions.** A line that contains a `never_compress` entry opens a region. The region ends at the next blank line. No line of a region is elided. A diff file that holds such a line is not collapsed.
- **Two refusals.** Input under `min_input_bytes` is returned untouched. A cut under `min_reduction_ratio` is refused, and the input comes back untouched. A group whose marker is not shorter than its lines stays as it is.
- **Tokens are an estimate.** `ceil(bytes / bytes_per_token)`. The value comes from policy.

## Invariants

- The package holds no numeric literal except 0 and 1. A threshold lives in policy.json (`TestC2248_014`).
- The policy decoder has no compiled default. An absent or invalid block is an error, so a missing block cannot silently change a prompt.
- A diff body ends at the first line that does not start with `+`, `-`, a space, `@` or a backslash.

## Known limits

- A repeated block of more than one line (a stack frame has two lines) is not collapsed. Only identical consecutive lines are.
- The filter has no production caller yet. The wiring at the injection boundary and the soak are separate tasks (acceptance 3 of the task, deferred by triage).
- `Compress` carries an `apicover:ignore` directive: the ACS predicate runs apicover with no `-cover` profile, where any exported function reads as 0% executed.
