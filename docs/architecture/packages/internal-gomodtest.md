# internal/gomodtest

## Purpose

`internal/gomodtest` writes a throwaway Go module tree for a test. It is the one home of this helper, so that no test package holds a copy.

## Design

- **`WriteModule`.** It takes a map from a relative path to a file body. It makes a new temporary directory, makes the parent directories, writes each file and returns the directory. A write that fails stops the test.
