package committedset

import (
	"path/filepath"

	"github.com/mickeyyaya/evolve-loop/go/internal/atomicwrite"
)

const ProtectedRouteFile = "tdd-protected-route.json"

const ProtectedPredicatesDir = "tdd-protected-predicates"

type ProtectedRoute struct {
	Cycle  int    `json:"cycle"`
	ItemID string `json:"item_id"`
}

func WriteProtectedRoute(workspace string, route ProtectedRoute) error {
	return atomicwrite.JSON(filepath.Join(workspace, ProtectedRouteFile), route)
}

func ReadProtectedRoute(workspace string) (ProtectedRoute, bool) {
	var route ProtectedRoute
	if !readJSON(workspace, ProtectedRouteFile, &route) {
		return ProtectedRoute{}, false
	}
	return route, route.ItemID != ""
}
