package memory

import "strings"

const SentriiPrefix = "sentrii:"

var sentriiEdgeTypes = map[EdgeType]bool{EdgeRelatesTo: true, EdgeCausedBy: true}

func SentriiID(ref string) (string, bool) {
	return strings.CutPrefix(ref, SentriiPrefix)
}

func SentriiEdgeTypes() []EdgeType { return sortedEdgeTypes(sentriiEdgeTypes) }
