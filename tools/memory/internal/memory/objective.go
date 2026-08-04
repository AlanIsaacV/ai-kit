package memory

import (
	"fmt"
	"strings"
	"time"
)

// This file is the objective tree and everything read off it: what state a
// record's edges add up to, which objective derives from which, whether a parent
// is waiting on a child, how far along a parent is, and which single objective
// is being worked on.
//
// Two things are true of all of it and are the reason it looks the way it does.
//
// It reads the edge list directly and never walks Graph. Graph deduplicates by
// *node* and reports only the first edge that reaches one, so a typed edge can
// be shadowed by an older relates_to between the same pair and simply not be
// seen — wont-fix 01KZ4ZXEPR07E7VH06JTG8E6J0, decided not worth fixing. State
// and parentage resolved from the edges themselves are unaffected by it.
//
// And it is derivation, not enforcement. The tree rules are refused on the way
// in by CheckParent at the bottom of this file; nothing here condemns a record
// already on disk, because a log can hold a shape the write path refuses —
// hand-edited, or copied in by `promote`, which passes no gate. Every walk here
// therefore carries a visited set and reports what it met rather than trusting
// the ground it stands on.

// ------------------------------------------------------------------ state ---

// State is the resolved state of a record: what the state edges pointing out of
// it add up to, rather than a field anything stores.
//
// It is a type of its own rather than an EdgeType because one of its values is
// not an edge and never can be. `planned` is the *absence* of every state edge,
// and an edge cannot assert an absence — see the README's "Which state edge
// wins". Making the two one type would mean putting a `planned` member into a
// closed set of edge types that no record may ever carry.
type State string

const (
	// StatePlanned is the default: no state edge at all. It costs nothing to be
	// in and one appended line to leave.
	StatePlanned State = "planned"
	// StateApprovedForExecution is the freeze gate: the only state work runs
	// from.
	StateApprovedForExecution State = "approved_for_execution"
	// StateMet counts toward the parent's completion.
	StateMet State = "met"
	// StateCancelled does not.
	StateCancelled State = "cancelled"
	// StateSeparated says the work continues, but no longer under this parent.
	StateSeparated State = "separated"
	// StateBlocked says something external is being waited on. Blocked by an
	// open child is a different thing, derived from the tree and never written;
	// see Tree.BlockedByOpenChild.
	StateBlocked State = "blocked"
)

// stateEdges maps each state edge type onto the state that transition leaves a
// record in. A type absent from this map is a relation rather than a transition
// and moves no record anywhere.
//
// It is spelled out rather than read off the justification allowlist, which
// happens to have the same five keys today. The two answer different questions —
// "what may justify this transition" and "what state does this transition leave
// the record in" — and tying one to the other would let a change to either
// silently change the other. TestEveryStateEdgeTypeResolvesToAState holds them in
// step instead, which is a check rather than a coupling.
var stateEdges = map[EdgeType]State{
	EdgeApprovedForExecution: StateApprovedForExecution,
	EdgeMet:                  StateMet,
	EdgeCancelled:            StateCancelled,
	EdgeSeparated:            StateSeparated,
	EdgeBlocked:              StateBlocked,
}

// Open reports whether work on a record in this state is still live.
//
// Terminal is `met` and `cancelled`, and nothing else. `separated` is open on
// purpose: it says the work goes on somewhere other than under the parent it was
// lifted out of, not that it stopped — so a separated objective is still a
// candidate for --current, while counting toward no parent and blocking none.
// `blocked` is open for the plainer reason that waiting is not finishing.
func (s State) Open() bool { return s != StateMet && s != StateCancelled }

// ResolveState is the rule the README states normatively, as a function at last:
//
//	a record's state is the most recent state edge by `at`, ties broken by
//	order in the file — the later line wins.
//
// No state edge at all resolves to `planned`. There is no retraction and none is
// needed: `approved_for_execution` → `blocked` → `approved_for_execution` →
// `met` resolves to `met`, and the three edges under it are a true trace of a
// state machine rather than a contradiction to repair.
//
// The tie-break is exact here and nowhere better, and the seq below is why: an
// edge carries no line number, so the position in `edges` stands in for one.
// That is only "later in the file" for as long as the slice really is in file
// order — which is this function's contract with its caller, not something it
// can check. Edges is what keeps it, and every caller in this package goes
// through it.
func ResolveState(edges []*Edge, id string) State {
	state := StatePlanned
	var best time.Time
	bestSeq := 0
	found := false
	for seq, e := range edges {
		if e.From != id {
			continue
		}
		s, isTransition := stateEdges[e.Type]
		if !isTransition {
			continue
		}
		at := parseAt(e.At)
		if found && !beats(at, seq, best, bestSeq) {
			continue
		}
		state, best, bestSeq, found = s, at, seq, true
	}
	return state
}

// beats reports whether a challenger displaces the best record seen so far,
// under the one recency rule this file has:
//
//	more recent by `at` wins, and a tie on `at` goes to whichever sits later
//	in the file.
//
// Four places resolve recency — the state of a record, the latest approval, the
// most recently touched objective, and the latest thing to touch one — and they
// all mean this. Written four times, in three shapes, a change to the rule would
// have reached one of them and quietly missed the rest.
//
// seq is how far into the file a record sits. Every caller walks its input
// forwards, so seq only ever grows — but *what* it is differs, and the call sites
// say which they hold rather than being made to look alike. Two of them have the
// real line number, from Line.Num. The other two have an index into a slice that
// is in file order because Edges put it there, which is a contract with the
// caller rather than something this function could check.
//
// A challenger that ties on both wins, and that is the rule rather than
// sloppiness about `>=`. It is what makes walking forwards mean "later wins".
// And at one call site two records genuinely can share a seq — one edge bumps
// both of its endpoints to the same line — where it makes the tie fall through
// to the order of the walk itself. See recordActivity.
func beats(at time.Time, seq int, bestAt time.Time, bestSeq int) bool {
	if at.Equal(bestAt) {
		return seq >= bestSeq
	}
	return at.After(bestAt)
}

// parseAt is a record's `at` as a time, and the zero time when the field is
// absent or not RFC3339.
//
// Zero rather than an error, because this is the read path: a hand-edited line
// with a broken timestamp is a line `validate` reports, not a line that should
// make the state of everything around it unanswerable. The zero time sorts
// before every real one, so such an edge can only win when there is nothing else
// to lose to.
func parseAt(s string) time.Time {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return time.Time{}
	}
	return t
}

// ------------------------------------------------------------------- tree ---

// Anomaly is a shape the derivation met on disk that the write path refuses: an
// objective with two parents, or a cycle.
//
// It is reported rather than repaired, and rather than treated as a failure.
// The asymmetry is A1's discipline: requirements live on the write path, and the
// read path stays tolerant so a record already written is never condemned by a
// rule that arrived after it. What the reader is owed instead is a deterministic
// answer plus a statement of what was odd about it and which way the tie went.
type Anomaly struct {
	ID      string `json:"id"`
	Message string `json:"message"`
}

// Tree is the objective forest one log describes, derived once from every entry
// and every edge in it.
//
// The parent relation is *positional*, not typed: a `derives_from` edge with an
// objective at both ends. The same edge type also points a decision at the
// objective it was taken under and an ADR at the decisions it came from, and
// both keep their meaning — only objective → objective is parentage. That
// overload was accepted deliberately over a new `child_of` type, which would be
// unambiguous but would widen a closed set a second time and leave two edges
// asserting one relation.
//
// A forest, not a tree, whatever the name says: several roots are normal and
// already exist, because nothing requires an objective to have a parent.
type Tree struct {
	objectives []*Entry          // objectives in file order
	byID       map[string]*Entry // objectives only
	edges      []*Edge           // every edge in the log, in file order
	parent     map[string]string
	children   map[string][]string
	superseded map[string]bool
	settled    map[string]bool
	touched    map[string]activity
	anomalies  []Anomaly
}

// NewTree derives the forest from a whole log.
//
// Whole, always. A caller that is about to narrow by plan narrows *afterwards*:
// deriving from a filtered subset would make --plan quietly change which
// objective is current and which parent has children, and the answer to "what am
// I working on" must not depend on what you happened to ask about.
//
// It takes lines rather than the entries and edges split out of them because one
// rule here needs the order of the file itself, and that order does not survive
// the split: two slices each ordered internally cannot say which of one's
// members came before one of the other's. See recordActivity. Everything else
// reads the two views, which is why they are taken here once and kept.
func NewTree(lines []Line) *Tree {
	edges := Edges(lines)
	t := &Tree{
		byID:       map[string]*Entry{},
		edges:      edges,
		superseded: SupersededIDs(edges),
		settled:    SettledIDs(edges),
		touched:    map[string]activity{},
	}
	entries := Entries(lines)
	kinds := make(map[string]Kind, len(entries))
	for _, e := range entries {
		if _, dup := kinds[e.ID]; dup {
			// A duplicate id is a problem `validate` reports with both line
			// numbers. Here the first one wins, so the reading is at least
			// deterministic.
			continue
		}
		kinds[e.ID] = e.Kind
		if e.Kind != KindObjective {
			continue
		}
		t.byID[e.ID] = e
		t.objectives = append(t.objectives, e)
	}

	t.parent, t.children, t.anomalies = parentRelation(edges, kinds)
	t.anomalies = append(t.anomalies, findCycles(t.objectives, t.parent)...)
	t.recordActivity(lines)
	return t
}

// parentRelation reads the parent relation out of a log's edges: child id →
// parent id, plus the reverse.
//
// Where a child carries more than one parent edge — which `link` refuses, and
// which a hand-edited or promoted log can still hold — the first edge in the file
// wins and the rest are reported. Picking silently would make the reading depend
// on an accident; refusing to answer would make one bad line hide a whole tree.
func parentRelation(edges []*Edge, kinds map[string]Kind) (parent map[string]string, children map[string][]string, anomalies []Anomaly) {
	parent = map[string]string{}
	children = map[string][]string{}
	for _, e := range edges {
		if e.Type != EdgeDerivesFrom {
			continue
		}
		// The positional rule. A dangling endpoint has no kind here either, so
		// an edge pointing at a record this log does not hold is not parentage
		// — `validate` is what reports it.
		if kinds[e.From] != KindObjective || kinds[e.To] != KindObjective {
			continue
		}
		if have, taken := parent[e.From]; taken {
			anomalies = append(anomalies, Anomaly{ID: e.From, Message: fmt.Sprintf(
				"an objective has one parent, and this one has two: %s is the first in the file and is the one used; the edge to %s is ignored",
				have, e.To)})
			continue
		}
		parent[e.From] = e.To
		children[e.To] = append(children[e.To], e.From)
	}
	return parent, children, anomalies
}

// findCycles reports every cycle in an already-built parent map, once each.
//
// Three colours, which is what makes it terminate and what keeps a cycle from
// being reported once per member: unvisited, on the current path, settled. A
// walk that reaches a node already on its own path has found the cycle, and
// everything from that node onward is it; a walk that reaches a settled node has
// found nothing new.
//
// The links are left exactly as they are. Cutting one to restore a tree would be
// the reader inventing a parentage nobody wrote, so every walk that follows a
// parent chain carries its own visited set instead — see Depth and Ancestors.
func findCycles(objectives []*Entry, parent map[string]string) []Anomaly {
	const (
		onPath  = 1
		settled = 2
	)
	colour := map[string]int{}
	var out []Anomaly
	for _, o := range objectives {
		var path []string
		for id := o.ID; ; {
			if colour[id] == settled {
				break
			}
			if colour[id] == onPath {
				cycle := path
				for i, p := range path {
					if p == id {
						cycle = path[i:]
						break
					}
				}
				out = append(out, Anomaly{ID: id, Message: fmt.Sprintf(
					"the parent relation is a tree and this is a cycle: %s",
					strings.Join(append(append([]string{}, cycle...), id), " -> "))})
				break
			}
			colour[id] = onPath
			path = append(path, id)
			next, ok := parent[id]
			if !ok {
				break
			}
			id = next
		}
		for _, p := range path {
			colour[p] = settled
		}
	}
	return out
}

// Objectives returns every objective in the log, in file order.
func (t *Tree) Objectives() []*Entry { return t.objectives }

// Anomalies returns what the derivation met that the write path would have
// refused. Empty for every log this tool wrote on its own.
func (t *Tree) Anomalies() []Anomaly { return t.anomalies }

// State is the resolved state of any record in the log, objective or not.
func (t *Tree) State(id string) State { return ResolveState(t.edges, id) }

// Parent is the objective this one derives from, or "" for a root.
//
// A `separated` objective keeps the parent edge that is on disk, and this
// reports it. Separation detaches it from the parent's *accounting* — it counts
// toward no completion and blocks nothing — rather than erasing where it came
// from, and the log never rewrites, so the edge is still the honest answer to
// "where did this come from".
func (t *Tree) Parent(id string) string { return t.parent[id] }

// Children are the objectives that derive from this one, in file order.
//
// A copy, because the slice inside the Tree is the derivation's own and a caller
// that appended to it would be editing the tree. Same trap as the map aliasing
// record.go warns about, one container along.
func (t *Tree) Children(id string) []string {
	return append([]string{}, t.children[id]...)
}

// Depth is how many parents stand above an objective; a root is 0.
//
// Advisory, and reported for exactly that reason. **Depth suggests; it does not
// decide.** Nothing filters on it and nothing resolves the current objective by
// it: an urgent fix arrives, or a sibling suddenly matters more, or the draft you
// are editing turns out to be the real work, and a rule that forced the deepest
// node would make every one of those a fight with the tool.
func (t *Tree) Depth(id string) int {
	seen := map[string]bool{id: true}
	d := 0
	for {
		p, ok := t.parent[id]
		if !ok || seen[p] {
			return d
		}
		seen[p] = true
		id, d = p, d+1
	}
}

// BlockedByOpenChild is the ids of the children holding this objective up, and
// it is derived rather than written on purpose.
//
// A `blocked` edge pointing at another objective is refused by the justification
// allowlist, so "blocked on a sibling" and "blocked by my own child" can never
// end up indistinguishable on disk. This is the derivation that stands in its
// place.
//
// It is informational and nothing acts on it. Such an objective is still
// returned by --current and is still eligible to be the resolved current one,
// because a parent whose child is open is very often exactly the parent you are
// about to work on.
//
// A `separated` child does not block: it was lifted out from under this parent,
// so its progress is no longer this parent's business. Neither does a superseded
// one — the work was absorbed and the record that absorbed it carries it now.
func (t *Tree) BlockedByOpenChild(id string) []string {
	out := []string{}
	for _, child := range t.children[id] {
		s := t.State(child)
		if s == StateSeparated || !s.Open() || t.settled[child] {
			continue
		}
		out = append(out, child)
	}
	return out
}

// Completion is a parent's children sorted into what each does to its progress.
type Completion struct {
	CountedDone []string `json:"counted_done"`
	NotCounted  []string `json:"not_counted"`
	Outstanding []string `json:"outstanding"`
}

// Completion sorts an objective's children into those three buckets.
//
// A superseded child counts as done, and that is decision
// 01KZ30RRADAT4FM5DF3QJRCT55 rather than an oversight: absorbed work exists,
// someone else did it, and not counting it would leave the parent permanently
// short of a child nobody is ever going to do again.
//
// The order of the tests decides the case where a child is both, and it is asked
// counted-done first: a child that was superseded and then cancelled still
// counts, because the record that replaced it carries the work and cancelling
// the husk afterwards does not undo it.
func (t *Tree) Completion(id string) Completion {
	c := Completion{CountedDone: []string{}, NotCounted: []string{}, Outstanding: []string{}}
	for _, child := range t.children[id] {
		switch s := t.State(child); {
		case s == StateMet || t.superseded[child]:
			c.CountedDone = append(c.CountedDone, child)
		case s == StateCancelled || s == StateSeparated:
			c.NotCounted = append(c.NotCounted, child)
		default:
			c.Outstanding = append(c.Outstanding, child)
		}
	}
	return c
}

// --------------------------------------------------- the current objective ---

// The three rules Current can resolve by, named in the answer so the reading is
// never a guess presented as a fact.
const (
	// RuleApproved: an open objective carries the latest approved_for_execution
	// edge in the log. The mark, deliberately placed.
	RuleApproved = "approved_for_execution"
	// RuleActivity: none does, so the most recently touched open objective wins.
	RuleActivity = "activity"
	// RuleNone: there is no open objective to name.
	RuleNone = "none"
)

// CurrentObjective is the single answer to "which objective is being worked on",
// together with the rule that produced it.
//
// One answer, and separate from --current, which returns every open objective
// and filters on nothing else. The two are not the same question: --current is
// "what is still live", this is "what am I on right now". Reported side by side
// so a skill can offer the resolved one and let the user switch, rather than
// having to ask every session including when it is obvious.
type CurrentObjective struct {
	ID    string `json:"id,omitempty"`
	Slug  string `json:"slug,omitempty"`
	Title string `json:"title,omitempty"`
	Rule  string `json:"rule"`
	Why   string `json:"why"`
	// OutsideFilter says the objective named here is not among the records this
	// query returned. The derivation reads the whole log and the filter applies
	// afterwards, so this is the honest case rather than a bug: naming the
	// runner-up instead would answer a question nobody asked.
	OutsideFilter bool `json:"outside_filter,omitempty"`
}

// Current resolves the one objective being worked on: the open objective whose
// most recent approved_for_execution edge is the latest in the log, or — when no
// open objective carries one — the open objective with the most recent activity.
//
// Hybrid, and both halves are load-bearing. Activity alone is implicit, moves on
// its own and cannot be pinned. The mark alone costs a decision plus an edge on
// every context switch, which is exactly the ceremony this loop exists to stop
// paying. Together, the normal case costs nothing and pinning the focus stays
// available at the moment you are writing a decision anyway — which is what the
// allowlist demands behind any state edge.
//
// The candidates are every *open* objective that nothing has superseded or
// answered. Depth is not involved, and neither is having an open child: every
// open objective is potentially the current one.
//
// An objective that was approved and later blocked is still a candidate under
// the first half. Its state is `blocked`, which is open, and the approval it
// carries is still the latest mark anyone placed — the block says work stopped
// for a reason, not that the focus moved somewhere else.
func (t *Tree) Current() CurrentObjective {
	candidate := map[string]bool{}
	for _, o := range t.objectives {
		if t.settled[o.ID] {
			continue
		}
		if !t.State(o.ID).Open() {
			continue
		}
		candidate[o.ID] = true
	}
	if len(candidate) == 0 {
		return CurrentObjective{Rule: RuleNone, Why: "no open objective in this log"}
	}

	// The mark. Later line wins a shared `at`, exactly as it does for state —
	// and, exactly as there, the seq is a position in a file-ordered slice
	// standing in for a line number an edge does not carry.
	var markID string
	var markAt time.Time
	markSeq := 0
	for seq, e := range t.edges {
		if e.Type != EdgeApprovedForExecution || !candidate[e.From] {
			continue
		}
		at := parseAt(e.At)
		if markID != "" && !beats(at, seq, markAt, markSeq) {
			continue
		}
		markID, markAt, markSeq = e.From, at, seq
	}
	if markID != "" {
		return t.answer(markID, RuleApproved,
			"the open objective carrying the most recent approved_for_execution edge in the log")
	}

	// Activity, which is what is left. The seq here is the real line number, and
	// it is the one place where two candidates can genuinely carry the same one —
	// see recordActivity for what happens then and why it is left as it is.
	var bestID string
	var best activity
	for _, o := range t.objectives {
		if !candidate[o.ID] {
			continue
		}
		a, touched := t.touched[o.ID]
		if !touched {
			continue
		}
		if bestID != "" && !beats(a.at, a.line, best.at, best.line) {
			continue
		}
		bestID, best = o.ID, a
	}
	if bestID == "" {
		// Unreachable: activity gives every objective at least its own entry's
		// `at`, so a candidate always has one. Stated as a refusal to answer
		// rather than left to fall through into a nameless "current objective".
		return CurrentObjective{Rule: RuleNone, Why: "no open objective in this log"}
	}
	return t.answer(bestID, RuleActivity,
		"no open objective is approved for execution, so this is the one most recently touched")
}

func (t *Tree) answer(id, rule, why string) CurrentObjective {
	c := CurrentObjective{ID: id, Rule: rule, Why: why}
	if e := t.byID[id]; e != nil {
		c.Slug, c.Title = e.Slug, e.Title
	}
	return c
}

// activity is the latest `at` touching one objective, and the line of the log
// that supplied it.
type activity struct {
	at   time.Time
	line int
}

// recordActivity fills, per objective, the latest `at` among the objective's own
// entry and every edge with it at *either* endpoint — and the line that carried
// it, which is what breaks a tie.
//
// Either endpoint, because most of what happens to an objective is written
// pointing at it: the unit that implements it, the decision taken under it, the
// child that derives from it. An edge touching a *child*, though, is activity on
// the child and not on the parent — this walks edges, not the tree.
//
// The line number is why NewTree takes lines. `at` is written to the second, and
// this loop appends a batch of records at the end of a turn, so sharing an `at`
// is the ordinary case rather than the exotic one: on this repo's own log, most
// lines share their second with another record, and some of those seconds mix an
// entry with an edge. Entries and edges split into two slices cannot say which of
// one's members came before one of the other's, so the answer would have had to
// be guessed. It is read off the file instead, and the tie-break is the rule as
// stated — the later line in the file — and not an approximation of it.
//
// One pass, in file order, through beats: a record no earlier than the best so
// far replaces it, which is what makes walking forwards mean "the later line
// wins". The line passed here is Line.Num, the real one.
//
// # Where the rule runs out, and what happens then
//
// One line can touch two objectives. An edge bumps *both* of its endpoints, so a
// `derives_from` between two objectives leaves them tied on `at` and tied on the
// line as well — and "the later line" cannot separate two records that share a
// line. There is nothing left for the stated rule to say.
//
// The tie then falls through to the order the objectives themselves sit in.
// Current walks t.objectives in file order and beats lets a challenger that ties
// on both win, so the objective whose own *entry* is later in the file takes it.
// Deterministic and stable — the same log always gives the same answer — and it
// decides anything at all only when those two objectives are the two most
// recently touched open ones.
//
// Left as it is rather than fixed, deliberately. Any rule that separated them
// would have to invent a reason to prefer the parent or the child over the other,
// and there is not one: the edge is a single fact about both of them, written
// once. Documenting it is the whole of the fix.
func (t *Tree) recordActivity(lines []Line) {
	bump := func(id string, at time.Time, line int) {
		if _, isObjective := t.byID[id]; !isObjective {
			return
		}
		if cur, seen := t.touched[id]; seen && !beats(at, line, cur.at, cur.line) {
			return
		}
		t.touched[id] = activity{at: at, line: line}
	}
	for _, l := range lines {
		switch {
		case l.Entry != nil:
			bump(l.Entry.ID, parseAt(l.Entry.At), l.Num)
		case l.Edge != nil:
			at := parseAt(l.Edge.At)
			bump(l.Edge.From, at, l.Num)
			bump(l.Edge.To, at, l.Num)
		}
	}
}

// ----------------------------------------------------------- state report ---

// ObjectiveState is one objective as the tree sees it.
type ObjectiveState struct {
	ID                 string     `json:"id"`
	Slug               string     `json:"slug"`
	Title              string     `json:"title"`
	Plan               string     `json:"plan,omitempty"`
	State              State      `json:"state"`
	Open               bool       `json:"open"`
	Depth              int        `json:"depth"`
	Parent             string     `json:"parent,omitempty"`
	Children           []string   `json:"children"`
	BlockedByOpenChild []string   `json:"blocked_by_open_child"`
	Completion         Completion `json:"completion"`
}

// ExcludedObjective is an objective the report leaves out of the live tree, and
// why.
//
// Listed rather than dropped. An objective that has simply vanished from the
// report looks like a bug in the tool or a missing record; an objective listed
// with the reason it was set aside is an answer.
type ExcludedObjective struct {
	ID     string `json:"id"`
	Slug   string `json:"slug"`
	Title  string `json:"title"`
	State  State  `json:"state"`
	Reason string `json:"reason"`
}

// StateReport is what `memory state` answers with: every live objective, what
// state its edges put it in, where it sits in the tree, what it is waiting on,
// how far along it is — and the one objective the current-resolution rule picks
// out of all of them.
type StateReport struct {
	Objectives []ObjectiveState    `json:"objectives"`
	Excluded   []ExcludedObjective `json:"excluded"`
	Current    CurrentObjective    `json:"current"`
	Anomalies  []Anomaly           `json:"anomalies"`
}

// Report builds the whole reading, in file order.
//
// A superseded or answered objective is moved to Excluded rather than reported
// as live. That is the same rule --current applies to every other kind: a record
// something replaced is not something you still have to hold in mind. It is
// still named, with its reason, because silence would be indistinguishable from
// a record that was never there.
func (t *Tree) Report() StateReport {
	rep := StateReport{
		Objectives: []ObjectiveState{},
		Excluded:   []ExcludedObjective{},
		Current:    t.Current(),
		Anomalies:  t.anomalies,
	}
	if rep.Anomalies == nil {
		rep.Anomalies = []Anomaly{}
	}
	for _, o := range t.objectives {
		state := t.State(o.ID)
		if t.settled[o.ID] {
			reason := "answered"
			if t.superseded[o.ID] {
				reason = "superseded"
			}
			rep.Excluded = append(rep.Excluded, ExcludedObjective{
				ID: o.ID, Slug: o.Slug, Title: o.Title, State: state, Reason: reason,
			})
			continue
		}
		rep.Objectives = append(rep.Objectives, ObjectiveState{
			ID: o.ID, Slug: o.Slug, Title: o.Title, Plan: planOf(o),
			State:              state,
			Open:               state.Open(),
			Depth:              t.Depth(o.ID),
			Parent:             t.Parent(o.ID),
			Children:           t.Children(o.ID),
			BlockedByOpenChild: t.BlockedByOpenChild(o.ID),
			Completion:         t.Completion(o.ID),
		})
	}
	return rep
}

// -------------------------------------------------------------- write path ---

// CheckParent reports whether a new edge may join the objective tree. A nil
// error means it may.
//
// This is a write-path rule and only a write-path rule, for the reasons
// CheckJustification states and one more of its own. A rule set judges a record
// in isolation and cannot see either endpoint's kind, let alone the shape of the
// whole graph; the read sweep stays tolerant so an edge already on disk is never
// condemned by a requirement that arrived after it; and `link` has already loaded
// every entry to refuse a dangling edge, so the kinds are free at the one moment
// the answer can still be acted on. The log is append-only: there is no taking a
// second parent back afterwards.
//
// It applies to nothing but objective → objective. `decision → objective`,
// `objective → decision` and an ADR deriving from several decisions are all
// untouched, because `derives_from` means "came out of" everywhere else and only
// means "child of" between two objectives.
//
// The errors name both halves — the rule and what is actually on disk — for the
// same reason CheckJustification's do: "not allowed" without the parent it
// already has, or without the cycle it would close, sends the reader back to the
// log to reconstruct it by hand.
func CheckParent(t EdgeType, from, to string, kinds map[string]Kind, edges []*Edge) error {
	if t != EdgeDerivesFrom {
		return nil
	}
	if kinds[from] != KindObjective || kinds[to] != KindObjective {
		return nil
	}

	parent, _, _ := parentRelation(edges, kinds)

	if have, taken := parent[from]; taken {
		return fmt.Errorf("an objective derives from at most one objective: %s already derives from %s, "+
			"and this edge would give it a second parent, %s", from, have, to)
	}

	// A cycle closes exactly when `from` is already an ancestor of `to` —
	// including when the two are the same record, which is a cycle of length
	// one. `link` never reaches that case, since the version rule set refuses an
	// edge pointing at its own source first, but the rule is stated whole rather
	// than assuming its only caller.
	//
	// The walk carries a visited set because the log may already hold a cycle
	// that this edge has nothing to do with: nothing sweeps for one, and the
	// derivation above deliberately leaves the links exactly as written.
	path := []string{from}
	seen := map[string]bool{}
	for id := to; id != ""; id = parent[id] {
		path = append(path, id)
		if id == from {
			return fmt.Errorf("the parent relation is a tree and this edge would close a cycle: %s",
				strings.Join(path, " -> "))
		}
		if seen[id] {
			// A cycle already on disk, above the pair being linked. Refusing
			// here would be a refusal about that cycle rather than about this
			// edge, and the read path is where it gets reported.
			break
		}
		seen[id] = true
	}
	return nil
}
