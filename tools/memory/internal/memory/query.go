package memory

import (
	"fmt"
	"slices"
	"sort"
	"strings"
)

// DefaultThreshold is how many records, edges or graph nodes a read will hand
// back before it stops and reports a count instead.
//
// This is the whole point of the retrieval contract. An agent that asks a broad
// question and receives two hundred records has just spent its context on
// history it did not need — which is precisely how the previous design lost
// the objective. Returning the count first forces the caller to either narrow
// the question or delegate the reading to a subagent.
const DefaultThreshold = 25

// Filter narrows a query. Zero values mean "no constraint".
type Filter struct {
	Plan        string // exact origin.plan
	ID          []string
	About       []string // match if the entry carries any of these tags (prefix match on path-like tags)
	Kinds       []Kind   // match if the entry is any of these kinds
	Slug        string   // exact slug
	Text        string   // case-insensitive substring over title and body
	Since       string
	Until       string
	CurrentOnly bool // drop entries the graph says are settled: superseded, or answered
	CountOnly   bool
	HintAdvice  []string
	Limit       int // hard cap on returned records; 0 means "use the threshold"
	Threshold   int // override DefaultThreshold; negative disables the guard
}

func (f Filter) AdmitsObjectives() bool {
	return len(f.Kinds) == 0 || slices.Contains(f.Kinds, KindObjective)
}

// Result is what every read command returns. Count is always the true number
// of matches; Records may be empty even when Count is large, and Truncated
// says which of those two situations you are in — unless the filter asked for
// the count alone, which returns no records and withholds nothing.
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
	NotFound         []string          `json:"not_found,omitempty"`
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

func QueryIn(lines, store []Line, f Filter) Result {
	matched, cur := match(lines, store, f)
	return bound(matched, cur, f)
}

// QueryEachIn answers one filter over several logs as a single result: each log is
// matched on its own and the count, the limit, the guard and NotFound are applied
// once over the union.
//
// The logs' lines are never concatenated into one QueryIn. The objectives a tree
// holds are one log's, and one Tree over two logs would join two unrelated
// hierarchies and let either one change which objective the other calls current.
// The edges are the store's in every pass, so what they say about a log's own
// objectives is the same wherever it is read from. CurrentObjective is therefore
// the first log's, exactly as QueryIn over that log alone reports it.
//
// Nothing is deduplicated: promotion copies a record with its id untouched, so the
// same id in two logs is two matches, and the first log's copy sorts ahead.
func QueryEachIn(logs [][]Line, store []Line, f Filter) Result {
	var matched []*Entry
	var cur *CurrentObjective
	for i, lines := range logs {
		m, c := match(lines, store, f)
		matched = append(matched, m...)
		if i == 0 {
			cur = c
		}
	}
	sortMostRecentFirst(matched)
	return bound(matched, cur, f)
}

// sortMostRecentFirst orders by id: an id is a ULID, so lexical order is
// chronological.
func sortMostRecentFirst(entries []*Entry) {
	sort.SliceStable(entries, func(i, j int) bool { return entries[i].ID > entries[j].ID })
}

// match is the filtering half of QueryIn: every entry the filter admits, most recent
// first, and the current objective when the filter admits objectives.
func match(lines, store []Line, f Filter) ([]*Entry, *CurrentObjective) {
	entries := Entries(lines)

	settled := map[string]bool{}
	if f.CurrentOnly {
		settled = SettledIDs(Edges(store))
	}

	kinds := map[Kind]bool{}
	for _, k := range f.Kinds {
		kinds[k] = true
	}
	ids := map[string]bool{}
	for _, id := range f.ID {
		ids[id] = true
	}
	text := strings.ToLower(strings.TrimSpace(f.Text))

	// The tree, the states and the resolved current objective are derived from
	// every entry that reached here and every edge in the store, and the filter
	// applies afterwards. Deriving from the filtered subset would let --plan
	// silently change which objective is current, and "what am I working on"
	// must not depend on what you happened to ask about.
	//
	// Built only when the query admits objectives at all, since it answers
	// nothing otherwise. What every other kind's --current drops is the settled
	// set above, which reads the store's edges rather than this log's.
	var tree *Tree
	wantsObjectives := f.AdmitsObjectives()
	if wantsObjectives {
		tree = NewTreeIn(lines, store)
	}

	var matched []*Entry
	for _, e := range entries {
		if len(ids) > 0 && !ids[e.ID] {
			continue
		}
		if f.CurrentOnly && settled[e.ID] {
			continue
		}
		// A record with no plan is the repository's rather than nobody's, so it is
		// in scope for every tree in it and a plan bound never hides it.
		if f.Plan != "" && planOf(e) != "" && planOf(e) != f.Plan {
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
		if f.Since != "" && e.At < f.Since {
			continue
		}
		if f.Until != "" && e.At > f.Until {
			continue
		}
		matched = append(matched, e)
	}

	sortMostRecentFirst(matched)

	if !wantsObjectives {
		return matched, nil
	}
	cur := tree.Current()
	// Against the matches rather than the returned records: over the
	// threshold nothing comes back at all, and "the current objective is
	// outside your filter" would then be false of every query that asked too
	// broadly.
	if cur.ID != "" && !containsID(matched, cur.ID) {
		cur.OutsideFilter = true
		cur.Why += "; it falls outside this query's filter, so it is named here rather than replaced by the runner-up"
	}
	return matched, &cur
}

// bound is the half of QueryIn that decides how much of the matches to hand back.
func bound(matched []*Entry, cur *CurrentObjective, f Filter) Result {
	res := Result{Count: len(matched), Records: []*Entry{}, CurrentObjective: cur}
	for _, id := range f.ID {
		if !containsID(matched, id) {
			res.NotFound = append(res.NotFound, id)
		}
	}

	threshold := f.Threshold
	if threshold == 0 {
		threshold = DefaultThreshold
	}

	if !f.CountOnly {
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
			res.Hint = withheldHint(f.HintAdvice)
		default:
			res.Records = matched
		}
	}
	res.Returned = len(res.Records)
	return res
}

var defaultHintAdvice = []string{"narrow the filter", "set a limit to take the most recent"}

func withheldHint(advice []string) string {
	if len(advice) == 0 {
		advice = defaultHintAdvice
	}
	return "too many matches to return safely: " + strings.Join(advice, ", ") + ", or have a subagent read them"
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
// Saying nothing used to make a record invisible to every bounded query, and the
// write path refused to produce one for exactly that reason. It now means the
// opposite — the record belongs to the repository and every plan bound admits it
// — because one log holds every tree's records and there was nowhere else for a
// fact about the repository itself to sit. What that costs is stated where it is
// paid: a bounded read returns its tree's records *and* the repository's, so it
// can hand back something the turn did not ask for.
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

const WholeBody = -1

var projectableFields = []string{"version", "id", "kind", "slug", "scope", "at",
	"about", "title", "body", "anchors", "origin"}

func ProjectableFields() []string { return slices.Clone(projectableFields) }

type Projection struct {
	fields    []string
	bodyLines int
}

func NewProjection(fields []string, bodyLines int) (Projection, error) {
	for _, f := range fields {
		if !slices.Contains(projectableFields, f) {
			return Projection{}, fmt.Errorf("%q is not a record field: name any of %s",
				f, strings.Join(projectableFields, ", "))
		}
	}
	if bodyLines < WholeBody {
		bodyLines = WholeBody
	}
	return Projection{fields: slices.Clone(fields), bodyLines: bodyLines}, nil
}

func (p Projection) Apply(records []*Entry) []any {
	out := make([]any, 0, len(records))
	for _, e := range records {
		out = append(out, p.project(e))
	}
	return out
}

func (p Projection) project(e *Entry) any {
	if len(p.fields) == 0 {
		if p.bodyLines == WholeBody {
			return e
		}
		shorter := *e
		shorter.Body = p.head(e.Body)
		return &shorter
	}
	out := make(map[string]any, len(p.fields))
	for _, f := range p.fields {
		switch f {
		case "version":
			out[f] = e.SchemaVersion()
		case "id":
			out[f] = e.ID
		case "kind":
			out[f] = e.Kind
		case "slug":
			out[f] = e.Slug
		case "scope":
			out[f] = e.Scope
		case "at":
			out[f] = e.At
		case "about":
			out[f] = e.About
		case "title":
			out[f] = e.Title
		case "body":
			if p.bodyLines != 0 {
				out[f] = p.head(e.Body)
			}
		case "anchors":
			out[f] = e.Anchors
		case "origin":
			out[f] = e.Origin
		}
	}
	return out
}

func (p Projection) head(body string) string {
	if p.bodyLines < 0 {
		return body
	}
	if p.bodyLines == 0 {
		return ""
	}
	lines := SplitLines([]byte(body))
	if len(lines) <= p.bodyLines {
		return body
	}
	return strings.Join(lines[:p.bodyLines], "\n")
}

type EdgeFilter struct {
	Types      []EdgeType
	From       string
	To         string
	Since      string
	Until      string
	CountOnly  bool
	HintAdvice []string
	Limit      int
	Threshold  int
}

type EdgeResult struct {
	Count     int     `json:"count"`
	Returned  int     `json:"returned"`
	Truncated bool    `json:"truncated"`
	Hint      string  `json:"hint,omitempty"`
	Edges     []*Edge `json:"edges"`
}

func QueryEdges(lines []Line, f EdgeFilter) EdgeResult {
	types := map[EdgeType]bool{}
	for _, t := range f.Types {
		types[t] = true
	}

	var matched []*Edge
	for _, e := range Edges(lines) {
		if len(types) > 0 && !types[e.Type] {
			continue
		}
		if f.From != "" && e.From != f.From {
			continue
		}
		if f.To != "" && e.To != f.To {
			continue
		}
		if f.Since != "" && e.At < f.Since {
			continue
		}
		if f.Until != "" && e.At > f.Until {
			continue
		}
		matched = append(matched, e)
	}
	sort.SliceStable(matched, func(i, j int) bool { return matched[i].At > matched[j].At })

	res := EdgeResult{Count: len(matched)}

	threshold := f.Threshold
	if threshold == 0 {
		threshold = DefaultThreshold
	}

	if !f.CountOnly {
		switch {
		case f.Limit > 0:
			n := min(f.Limit, len(matched))
			res.Edges = matched[:n]
			res.Truncated = n < len(matched)
			if res.Truncated {
				res.Hint = "limit reached; " + itoa(len(matched)-n) + " older matches not returned"
			}
		case threshold > 0 && len(matched) > threshold:
			res.Truncated = true
			res.Hint = withheldHint(f.HintAdvice)
		default:
			res.Edges = matched
		}
	}
	if res.Edges == nil {
		res.Edges = []*Edge{}
	}
	res.Returned = len(res.Edges)
	return res
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

type GraphResult struct {
	Count     int    `json:"count"`
	Truncated bool   `json:"truncated"`
	Hint      string `json:"hint,omitempty"`
	Nodes     []Node `json:"nodes"`
}

func GuardGraph(nodes []Node, threshold int, advice []string) GraphResult {
	res := GraphResult{Count: len(nodes), Nodes: nodes}
	if threshold == 0 {
		threshold = DefaultThreshold
	}
	if threshold > 0 && len(nodes) > threshold {
		res.Nodes = []Node{}
		res.Truncated = true
		res.Hint = withheldHint(advice)
	}
	return res
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
