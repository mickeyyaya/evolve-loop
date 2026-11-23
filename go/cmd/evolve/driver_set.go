package main

import (
	"github.com/mickeyyaya/evolve-loop/go/internal/bridge"
	"github.com/mickeyyaya/evolve-loop/go/internal/modelquery"
)

var driverSet = bridge.DefaultDriverSet()

func modelRouter(capturer modelquery.ModelCapturer) modelquery.Router {
	return modelquery.DefaultRouter(capturer, bridge.NewModelLister(driverSet, bridge.ModelQueryRunner))
}
