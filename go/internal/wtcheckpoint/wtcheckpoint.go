// Package wtcheckpoint saves, lists, restores and prunes refs/checkpoints snapshots of uncommitted worktree work.
// See docs/architecture/packages/internal-wtcheckpoint.md.
package wtcheckpoint

const (
	refPrefix   = "refs/checkpoints/"
	stampLayout = "20060102T150405Z"
	remoteName  = "origin"
	labelPrefix = "label: "

	binaryNumstat = "-"
)

var snapshotExclusions = []string{":!go/evolve", ":!.evolve/ledger.*"}

type Worktree struct {
	Dir  string `json:"dir"`
	Name string `json:"worktree"`
}

type Status string

const (
	StatusSaved     Status = "saved"
	StatusUnchanged Status = "unchanged"
	StatusClean     Status = "clean"
)
