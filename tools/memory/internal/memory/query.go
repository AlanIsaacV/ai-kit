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
	Plan        string   // exact origin.plan
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
	Count     int  `json:"count"`
	Returned  int  `json:"returned"`
	Truncated bool `json:"truncated"`
	// Hint means truncation and nothing else. It says why records were withheld
	// and what to do about it, and anything else put here would be read as that.
	Hint string `json:"hint,omitempty"`
	// CurrentObjective is which objective is being worked on, set only when the
	// filter admits objectives at all. A field of its own rather than more text
	// in Hint, because the two are unrelated answers and one is not a caveat on
	// the other.
	CurrentObjective *CurrentObjective `json:"current_objective,omitempty"`
	Records          []*Entry          `json:"records"`
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
//
// It takes the log's lines rather than the entries and edges split out of them.
// Both views are needed and both are taken here — but so is a third thing, the
// order of the file itself, which the split throws away: two slices each ordered
// internally cannot say which of one's members came before one of the other's,
// and the current-objective tie-break is stated in terms of exactly that. See
// Tree.recordActivity.
func Query(lines []Line, f Filter) Result {
	entries, edges := Entries(lines), Edges(lines)

	settled := map[string]bool{}
	if f.CurrentOnly {
		settled = SettledIDs(edges)
	}

	kinds := map[Kind]bool{}
	for _, k := range f.Kinds {
		kinds[k] = true
	}
	text := strings.ToLower(strings.TrimSpace(f.Text))

	// The tree, the states and the resolved current objective are derived from
	// every entry and every edge that reached here, and the filter applies
	// afterwards. Deriving from the filtered subset would let --plan silently
	// change which objective is current, and "what am I working on" must not
	// depend on what you happened to ask about.
	//
	// Built only when the query admits objectives at all, since it answers
	// nothing otherwise — which is also what keeps every other kind's --current
	// exactly where it was.
	var tree *Tree
	wantsObjectives := len(kinds) == 0 || kinds[KindObjective]
	if wantsObjectives {
		tree = NewTree(lines)
	}

	var matched []*Entry
	for _, e := range entries {
		if f.CurrentOnly && settled[e.ID] {
			continue
		}
		if f.Plan != "" && planOf(e) != f.Plan {
			continue
		}
		if len(kinds) > 0 && !kinds[e.Kind] {
			continue
		}
		// --current over objectives means what it means everywhere else: drop
		// what is settled, return the rest. Settled for an objective is one
		// state further — `met` or `cancelled` — and nothing else removes it.
		// Neither depth nor an open child, and not `separated` either, which
		// says the work continues elsewhere rather than that it stopped.
		//
		// Objectives only. Every other kind's --current returns exactly what it
		// returned before this existed.
		//
		// The nil check is belt and braces, not a live case: an objective that
		// got past the kind filter above means the query admits objectives, which
		// is the same condition that built the tree. Keep it anyway. This is the
		// read path, whose whole discipline is that it survives what it meets,
		// and one future filter that lets an objective through without setting
		// wantsObjectives would turn a query into a panic.
		if f.CurrentOnly && e.Kind == KindObjective && tree != nil && !tree.State(e.ID).Open() {
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

	if wantsObjectives {
		cur := tree.Current()
		// Against the matches rather than the returned records: over the
		// threshold nothing comes back at all, and "the current objective is
		// outside your filter" would then be false of every query that asked too
		// broadly.
		if cur.ID != "" && !containsID(matched, cur.ID) {
			cur.OutsideFilter = true
			cur.Why += "; it falls outside this query's filter, so it is named here rather than replaced by the runner-up"
		}
		res.CurrentObjective = &cur
	}
	return res
}

func containsID(entries []*Entry, id string) bool {
	for _, e := range entries {
		if e.ID == id {
			return true
		}
	}
	return false
}

// planOf is the plan a record says it came from, or "" when it says nothing.
//
// A record with no `origin.plan` matches no plan filter at all — not even the
// empty one, since an empty Filter.Plan means the filter is off. That is the
// point: one log holds every plan's records, so a record that cannot name its
// plan is invisible to every bounded query, which is why `add` refuses to write
// one. See ErrNoPlan.
func planOf(e *Entry) string {
	if e.Origin == nil {
		return ""
	}
	return e.Origin.Plan
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
