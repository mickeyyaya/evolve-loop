# internal/workflowlint

> Caller: `evolve yaml check` (`go/cmd/evolve/cmd_yaml.go`), the floor tier `yaml`. Inbox item: `console-checkin-workflow-lint`.

## Purpose

`internal/workflowlint` checks YAML files before they reach CI. The repository had no workflow lint, so a broken workflow was found only after a push. `Check(files)` takes `File{Path, Data}` values and returns `Finding{Path, Line, Msg}` values. It reads no file and runs no git command.

## The rules

- **Parse.** Each file must parse in full, with all its documents. A parse error gives the finding `invalid YAML: <message>` with the line that the YAML library reports. Pinned by `TestCheck_ReportsTheRulesThatAWorkflowBreaks` (case `a syntax error`).
- **Shape.** A file under `.github/workflows/` must hold a mapping in each document, and the file must have a document. Pinned by the cases `a list at the top` and `an empty file`.
- **Keys.** Each workflow document needs the keys `on` and `jobs`. `jobs` must be a mapping that is not empty. Pinned by the cases `no on key`, `no jobs key`, `empty jobs` and `jobs as a list`, and by `TestCheck_ChecksEveryDocumentOfAWorkflow`.
- **Runner.** Each job needs `runs-on` or `uses`. Pinned by the cases `a job with no runner` and `a job that is not a mapping`.
- A file outside `.github/workflows/` gets the parse rule only. Pinned by `TestCheck_AcceptsAValidWorkflowAndAPlainYAMLFile`.

## Non-goal

The package does not check the full GitHub Actions schema: it does not check step keys, expressions or action inputs. This is why `actionlint` was considered and not chosen for the first build: it needs a new external binary on each host and seat. The parser stays in `internal/adapters/yamlcodec`, the one home of the YAML library.
