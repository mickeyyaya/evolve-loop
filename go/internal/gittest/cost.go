package gittest

import "maps"

var startCosts = map[string]int{
	"Fixture":          5,
	"Bare":             5,
	"At":               5,
	"Clone":            1,
	"Repo.CloneBare":   1,
	"BranchExists":     1,
	"Repo.Git":         1,
	"FromTemplate":     0,
	"BareFromTemplate": 0,
}

func StartCosts() map[string]int {
	return maps.Clone(startCosts)
}
