package overlap

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
)

func gatherEvidence(in Input, idx moduleIndex, lane, peer sideView) Evidence {
	return Evidence{
		SharedPaths:     sharedPaths(lane, peer),
		EdgesLaneToPeer: edges(idx, lane.pkgs, peer.pkgs),
		EdgesPeerToLane: edges(idx, peer.pkgs, lane.pkgs),
		BuildZone:       sortedUnique(append(lane.pathsIn(zoneBuild), peer.pathsIn(zoneBuild)...)),
		GateZone:        sortedUnique(append(lane.pathsIn(zoneGate), peer.pathsIn(zoneGate)...)),
		DataEdges:       dataEdges(in.Catalogs, lane, peer),
		Derived:         sortedUnique(in.Catalogs.Fired(lane.paths, peer.paths, sortedUnique(in.Conflicted))),
		Unknown:         sortedUnique(append(append(lane.pathsIn(zoneUnknown), peer.pathsIn(zoneUnknown)...), in.Failures...)),
	}
}

func sharedPaths(lane, peer sideView) []string {
	out := []string{}
	for _, p := range lane.paths {
		pz, inPeer := peer.zones[p]
		lz := lane.zones[p]
		if !inPeer || lz == zoneBookkeeping || (lz == zoneDerived && pz == zoneDerived) {
			continue
		}
		out = append(out, p)
	}
	return out
}

func edges(idx moduleIndex, from, to map[string]bool) []Edge {
	out := []Edge{}
	for _, f := range sortedKeys(from) {
		reached := map[string]bool{}
		for _, d := range idx.closure(f) {
			if to[d] {
				reached[d] = true
			}
		}
		for _, d := range sortedKeys(reached) {
			out = append(out, Edge{From: f, To: d})
		}
	}
	return out
}

func dataEdges(cat Catalogs, lane, peer sideView) []string {
	set := map[string]bool{}
	for _, side := range []sideView{lane, peer} {
		for _, p := range side.paths {
			if side.zones[p] != zoneBookkeeping && cat.DataRead(p) {
				set[p] = true
			}
		}
	}
	return sortedKeys(set)
}

type digestBody struct {
	Evidence Evidence            `json:"evidence"`
	Blobs    map[string]BlobPair `json:"blobs"`
}

func edgePeerPaths(peer sideView, ev Evidence) []string {
	edgePkgs := map[string]bool{}
	for _, e := range ev.EdgesLaneToPeer {
		edgePkgs[e.To] = true
	}
	for _, e := range ev.EdgesPeerToLane {
		edgePkgs[e.From] = true
	}
	out := []string{}
	for _, p := range peer.pathsIn(zoneModule) {
		if anyIn(peer.owners[p], edgePkgs) {
			out = append(out, p)
		}
	}
	return out
}

func anyIn(xs []string, set map[string]bool) bool {
	for _, x := range xs {
		if set[x] {
			return true
		}
	}
	return false
}

func evidenceDigest(ev Evidence, edgePaths []string, blobs map[string]BlobPair) string {
	body := digestBody{Evidence: ev, Blobs: map[string]BlobPair{}}
	for _, group := range [][]string{ev.SharedPaths, ev.BuildZone, ev.GateZone, ev.DataEdges, ev.Unknown, edgePaths} {
		for _, p := range group {
			if b, ok := blobs[p]; ok {
				body.Blobs[p] = b
			}
		}
	}
	raw, _ := json.Marshal(body)
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}
