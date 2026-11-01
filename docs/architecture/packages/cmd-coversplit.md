# cmd/coversplit

## Purpose

`cmd/coversplit` prints the `go test` arguments of each coverage run of the named packages. `make test-integration` builds it and reads one line for each run. The rules live in [`internal/coversplit`](internal-coversplit.md).

## Design

- The `main` package holds no logic beyond the call into `coversplit.Main`.
- The arguments are the packages of the run. `-dir` names the module directory that holds `go/.cover-split`. The default is `.`.
- The first line is the main run. Each other line is the run of one split package, and starts with its `-coverpkg` flag.
- `-merge <out>` writes the merge of the named profiles to `<out>` through `coversplit.Merge`, and plans no run. The recipe joins the profiles of its runs with it.

## Invariants

- A list that `coversplit.Parse` refuses gives exit 1 and names the list. A bad flag gives exit 2. Thus a bad list stops `make test-integration` before it runs a test.
- A merge that cannot read, join or write a profile gives exit 1 and names the cause. The merge runs after the tests, so the recipe then fails with no `coverage.txt`.
- `TestMain_PrintsThePlanOfTheNamedPackagesAndExitsWithItsCode` pins the output and the exit code.
