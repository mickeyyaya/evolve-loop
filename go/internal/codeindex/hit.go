// Package codeindex is the backend-neutral layer that answers code-index queries for the evolve code verb.
// See docs/architecture/packages/internal-codeindex.md.
package codeindex

import "fmt"

type Hit struct {
	File   string
	Line   int
	Symbol string
	Text   string
}

type Query string

const (
	QueryCallers Query = "callers"
	QueryAPI     Query = "api"
	QueryFind    Query = "find"
	QueryMap     Query = "map"
)

func Queries() []Query {
	return []Query{QueryCallers, QueryAPI, QueryFind, QueryMap}
}

type Direction string

const (
	DirectionIn  Direction = "in"
	DirectionOut Direction = "out"
)

func ParseDirection(raw string) (Direction, error) {
	switch d := Direction(raw); d {
	case DirectionIn, DirectionOut:
		return d, nil
	}
	return "", fmt.Errorf("direction %q: want in|out", raw)
}

type Request struct {
	Query     Query
	Arg       string
	Direction Direction
}
