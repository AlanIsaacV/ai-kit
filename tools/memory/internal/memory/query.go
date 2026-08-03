package memory

import (
	"sort"
	"strings"
)

// DefaultThreshold is how many records a query will hand back before it stops
// and reports a count instead.
//
// This is the whole point of the retrieval contract. An agent that asks a broad
// question and receives two hundred records has just spent its context on
// history it did not need — which is precisely how the previous design lost
// the objective. Returning the count first forces the caller to either narrow
// the question or delegate the reading to a subagent.
const DefaultThreshold = 25

// Filter narrows a query. Zero values mean "no constraint".
type Filter struct {
	About       []string // match if the entry carries any of these tags (prefix match on path-like tags)
	Kinds       []Kind   // match if the entry is any of these kinds
	Slug        string   // exact slug
	Text        string   // case-insensitive substring over title and body
	CurrentOnly bool     // drop entries the graph says are settled: superseded, or answered
	Limit       int      // hard cap on returned records; 0 means "use the threshold"
	Threshold   int      // override DefaultThreshold; negative disables the guard
}

// Result is what every read command returns. Count is always the true number
// of matches; Records may be empty even when Count is large, and Truncated
// says which of those two situations you are in.
type Result struct {
	Count     int      `json:"count"`
	Returned  int      `json:"returned"`
	Truncated bool     `json:"truncated"`
	Hint      string   `json:"hint,omitempty"`
	Records   []*Entry `json:"records"`
}

// SupersededIDs returns the set of entry ids that some later entry replaces.
// Validity is derived from the graph rather than stored on the entry, because
// a stored flag would have to be rewritten and this log never rewrites.
func SupersededIDs(edges []*Edge) map[string]bool {
	return targetsOf(edges, EdgeSupersedes)
}

// AnsweredIDs returns the set of entry ids that something else has settled.
func AnsweredIDs(edges []*Edge) map[string]bool {
	return targetsOf(edges, EdgeAnswers)
}

// SettledIDs is what --current filters out: a record replaced by a newer one,
// or a question or assumption that something has since answered.
//
// Both belong here for the same reason. The point of --current is "what do I
// still have to hold in mind", and an answered question is exactly as
// irrelevant to that as a superseded decision — leaving it in would mean every
// session re-reads questions that were closed turns ago.
func SettledIDs(edges []*Edge) map[string]bool {
	out := SupersededIDs(edges)
	for id := range AnsweredIDs(edges) {
		out[id] = true
	}
	return out
}

func targetsOf(edges []*Edge, t EdgeType) map[string]bool {
	out := map[string]bool{}
	for _, e := range edges {
		if e.Type == t && e.To != "" {
			out[e.To] = true
		}
	}
	return out
}

// Query applies a filter and enforces the count-before-dump contract.
func Query(entries []*Entry, edges []*Edge, f Filter) Result {
	settled := map[string]bool{}
	if f.CurrentOnly {
		settled = SettledIDs(edges)
	}

	kinds := map[Kind]bool{}
	for _, k := range f.Kinds {
		kinds[k] = true
	}
	text := strings.ToLower(strings.TrimSpace(f.Text))

	var matched []*Entry
	for _, e := range entries {
		if f.CurrentOnly && settled[e.ID] {
			continue
		}
		if len(kinds) > 0 && !kinds[e.Kind] {
			continue
		}
		if f.Slug != "" && e.Slug != f.Slug {
			continue
		}
		if len(f.About) > 0 && !matchesAbout(e.About, f.About) {
			continue
		}
		if text != "" && !containsFold(e.Title, text) && !containsFold(e.Body, text) {
			continue
		}
		matched = append(matched, e)
	}

	// Most recent first: an id is a ULID, so lexical order is chronological.
	sort.SliceStable(matched, func(i, j int) bool { return matched[i].ID > matched[j].ID })

	res := Result{Count: len(matched), Records: []*Entry{}}

	threshold := f.Threshold
	if threshold == 0 {
		threshold = DefaultThreshold
	}

	switch {
	case f.Limit > 0:
		n := min(f.Limit, len(matched))
		res.Records = matched[:n]
		res.Truncated = n < len(matched)
		if res.Truncated {
			res.Hint = "limit reached; " + itoa(len(matched)-n) + " older matches not returned"
		}
	case threshold > 0 && len(matched) > threshold:
		// Deliberately return nothing. Handing back a partial slice would
		// read as "here is the answer" when it is a fragment of one.
		res.Truncated = true
		res.Hint = "too many matches to return safely: narrow with --about/--kind/--current, pass --limit N to take the most recent, or have a subagent read them"
	default:
		res.Records = matched
	}
	res.Returned = len(res.Records)
	return res
}

// matchesAbout is an OR over the wanted tags. A wanted tag ending in "/" is
// treated as a path prefix, so --about pkg/auth/ catches pkg/auth/token.go.
func matchesAbout(have, want []string) bool {
	for _, w := range want {
		lw := strings.ToLower(w)
		for _, h := range have {
			lh := strings.ToLower(h)
			if lh == lw {
				return true
			}
			if strings.HasSuffix(lw, "/") && strings.HasPrefix(lh, lw) {
				return true
			}
			if strings.HasSuffix(lh, "/") && strings.HasPrefix(lw, lh) {
				return true
			}
		}
	}
	return false
}

func containsFold(haystack, needleLower string) bool {
	return strings.Contains(strings.ToLower(haystack), needleLower)
}

// Node is one step of a graph walk.
type Node struct {
	Entry    *Entry `json:"entry"`
	Depth    int    `json:"depth"`
	Via      string `json:"via,omitempty"`      // edge type that reached it
	From     string `json:"from,omitempty"`     // id it was reached from
	Backward bool   `json:"backward,omitempty"` // traversed against the edge direction
}

// Graph walks outward from an id in both directions up to depth.
//
// Both directions matters: from an ADR you want the decisions it derives from,
// and from a decision you want the ADR that consumed it. A one-way walk would
// answer only half the questions anyone actually asks of this graph.
func Graph(entries []*Entry, edges []*Edge, from string, depth int) []Node {
	byID := map[string]*Entry{}
	for _, e := range entries {
		byID[e.ID] = e
	}
	root, ok := byID[from]
	if !ok {
		return nil
	}

	out := []Node{{Entry: root, Depth: 0}}
	seen := map[string]bool{from: true}
	frontier := []string{from}

	for d := 1; d <= depth && len(frontier) > 0; d++ {
		var next []string
		for _, cur := range frontier {
			for _, e := range edges {
				var otherID string
				backward := false
				switch {
				case e.From == cur:
					otherID = e.To
				case e.To == cur:
					otherID = e.From
					backward = true
				default:
					continue
				}
				if seen[otherID] {
					continue
				}
				other, ok := byID[otherID]
				if !ok {
					continue // dangling; `validate` reports it
				}
				seen[otherID] = true
				out = append(out, Node{Entry: other, Depth: d, Via: string(e.Type), From: cur, Backward: backward})
				next = append(next, otherID)
			}
		}
		frontier = next
	}
	return out
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}
