package memory

import (
	"strings"
	"testing"
	"time"
)

// objective is `entry` with the kind spelled out, since every fixture here is a
// tree of them.
func objective(t *testing.T, title string) *Entry {
	t.Helper()
	return entry(t, KindObjective, title, "auth")
}

// edgeAt is an edge with its `at` chosen rather than taken from the clock. Every
// rule in this file is about recency, and `at` is written to the second — two
// edges written by the tool in the same second share one, which is exactly the
// tie the rule has to break rather than a case to arrange around.
func edgeAt(typ EdgeType, from, to, at string) *Edge {
	return &Edge{Version: CurrentSchemaVersion, Kind: KindEdge, Type: typ, From: from, To: to, At: at}
}

func derivesFrom(child, parent *Entry) *Edge {
	return edgeAt(EdgeDerivesFrom, child.ID, parent.ID, "2026-08-03T10:00:00Z")
}

// ------------------------------------------------------------------ state ---

// The README states the rule normatively and nothing computed it. This is the
// sequence decision 01KZ4ZWK2MK4YN3T485N7FDF58 settled on, read back: four state
// edges, no retraction anywhere, and the answer is the last transition rather
// than a contradiction to repair.
func TestStateIsTheMostRecentEdgeByAt(t *testing.T) {
	o := objective(t, "The objective whose state this is")
	d := entry(t, KindDecision, "The decision behind it", "auth")
	q := entry(t, KindOpenQuestion, "What is holding it up", "auth")

	edges := []*Edge{
		edgeAt(EdgeApprovedForExecution, o.ID, d.ID, "2026-08-01T10:00:00Z"),
		edgeAt(EdgeBlocked, o.ID, q.ID, "2026-08-02T10:00:00Z"),
		edgeAt(EdgeApprovedForExecution, o.ID, d.ID, "2026-08-03T10:00:00Z"),
		edgeAt(EdgeMet, o.ID, d.ID, "2026-08-04T10:00:00Z"),
	}
	if got := ResolveState(edges, o.ID); got != StateMet {
		t.Errorf("approved → blocked → approved → met resolves to met, got %q", got)
	}

	// Recency, not line order: the stale `blocked` is written after the `met`
	// and still loses, because it is older. A trace of a state machine arrives in
	// whatever order it arrives in.
	outOfOrder := []*Edge{
		edgeAt(EdgeMet, o.ID, d.ID, "2026-08-04T10:00:00Z"),
		edgeAt(EdgeBlocked, o.ID, q.ID, "2026-08-02T10:00:00Z"),
	}
	if got := ResolveState(outOfOrder, o.ID); got != StateMet {
		t.Errorf("a later line with an older `at` must not win, got %q", got)
	}

	if got := ResolveState(nil, o.ID); got != StatePlanned {
		t.Errorf("no state edge at all is `planned`, got %q", got)
	}
}

// The tie-break, which is the half of the rule that only bites on a log the tool
// wrote itself: `at` is second-precision, so two edges appended in one second
// carry the same one and only their order in the file can separate them.
func TestStateTiesAreBrokenByTheLaterLine(t *testing.T) {
	o := objective(t, "The objective whose state this is")
	d := entry(t, KindDecision, "The decision behind it", "auth")
	const same = "2026-08-03T10:00:00Z"

	if got := ResolveState([]*Edge{
		edgeAt(EdgeCancelled, o.ID, d.ID, same),
		edgeAt(EdgeMet, o.ID, d.ID, same),
	}, o.ID); got != StateMet {
		t.Errorf("with a shared `at` the later line wins, got %q", got)
	}
	// The same two lines the other way round resolve the other way, which is
	// what makes this a rule about the file rather than about the two types.
	if got := ResolveState([]*Edge{
		edgeAt(EdgeMet, o.ID, d.ID, same),
		edgeAt(EdgeCancelled, o.ID, d.ID, same),
	}, o.ID); got != StateCancelled {
		t.Errorf("swapping the lines must swap the answer, got %q", got)
	}
}

// A state edge points *out* of the record whose state it is. An edge arriving at
// an objective says nothing about that objective's state, and reading it as
// though it did would let any record set anyone else's state.
func TestOnlyEdgesLeavingARecordDecideItsState(t *testing.T) {
	o := objective(t, "The objective whose state this is")
	other := objective(t, "Some other objective")
	if got := ResolveState([]*Edge{
		edgeAt(EdgeMet, other.ID, o.ID, "2026-08-03T10:00:00Z"),
	}, o.ID); got != StatePlanned {
		t.Errorf("an incoming edge must leave the state alone, got %q", got)
	}
}

// Open is met and cancelled inverted, and `separated` sitting on the open side
// is the decision worth pinning down: the work continues, so the objective is
// still a candidate for --current.
func TestSeparatedAndBlockedAreOpenAndOnlyTwoStatesAreTerminal(t *testing.T) {
	for _, s := range []State{StatePlanned, StateApprovedForExecution, StateBlocked, StateSeparated} {
		if !s.Open() {
			t.Errorf("%q is an open state", s)
		}
	}
	for _, s := range []State{StateMet, StateCancelled} {
		if s.Open() {
			t.Errorf("%q is terminal", s)
		}
	}
}

// The two tables are declared apart on purpose — one says what may justify a
// transition, the other says where the transition lands — so something has to
// hold them in step. A state edge type with no state would move a record
// nowhere, silently.
func TestEveryStateEdgeTypeResolvesToAState(t *testing.T) {
	for _, typ := range JustifiedEdgeTypes() {
		if _, ok := stateEdges[typ]; !ok {
			t.Errorf("%s carries a justification rule but resolves to no state", typ)
		}
	}
	for typ := range stateEdges {
		if !NeedsJustification(typ) {
			t.Errorf("%s moves a record to a state but needs no justification", typ)
		}
		if !edgeTypes[typ] {
			t.Errorf("%s resolves to a state but the current schema will not write it", typ)
		}
	}
	// And `planned` is not among them, because it cannot be: it is the absence
	// of every state edge, and an edge cannot assert an absence.
	for typ, s := range stateEdges {
		if s == StatePlanned {
			t.Errorf("%s claims to put a record back into `planned`, which no edge can say", typ)
		}
	}
}

// ------------------------------------------------------------------- tree ---

// The positional rule, both halves. objective → objective is parentage;
// everything else `derives_from` says keeps meaning what it said, which is what
// makes reusing the type affordable at all.
func TestParentIsDerivesFromWithAnObjectiveAtBothEnds(t *testing.T) {
	root := objective(t, "The objective above")
	child := objective(t, "The objective below")
	decision := entry(t, KindDecision, "A decision taken under the root", "auth")
	adr := entry(t, KindADR, "An ADR that came out of the decision", "auth")

	edges := []*Edge{
		derivesFrom(child, root),
		edgeAt(EdgeDerivesFrom, decision.ID, root.ID, "2026-08-03T10:00:00Z"),
		edgeAt(EdgeDerivesFrom, adr.ID, decision.ID, "2026-08-03T10:00:00Z"),
	}
	tree := NewTree(logOf([]*Entry{root, child, decision, adr}, edges))

	if got := tree.Parent(child.ID); got != root.ID {
		t.Errorf("the child's parent should be the root, got %q", got)
	}
	if got := tree.Parent(root.ID); got != "" {
		t.Errorf("nothing requires a parent; the root should have none, got %q", got)
	}
	if got := tree.Children(root.ID); len(got) != 1 || got[0] != child.ID {
		t.Errorf("the root should have exactly the one child, got %v", got)
	}
	// The decision derives from the root and is not its child: reading it as one
	// would put a decision into the completion tally of the objective it was
	// taken under.
	if got := tree.Depth(child.ID); got != 1 {
		t.Errorf("the child sits one below the root, got %d", got)
	}
	if len(tree.Anomalies()) != 0 {
		t.Errorf("nothing here is malformed: %+v", tree.Anomalies())
	}
}

// Children is handed out as a copy. The slice inside the tree is the
// derivation's own, and a caller that appended to it would be editing the tree —
// the same trap record.go names about maps, one container along.
func TestChildrenIsACopy(t *testing.T) {
	root := objective(t, "The objective above")
	child := objective(t, "The objective below")
	tree := NewTree(logOf([]*Entry{root, child}, []*Edge{derivesFrom(child, root)}))

	_ = append(tree.Children(root.ID), "01KZ34725V37YX845RTVPC7N0E")
	if got := tree.Children(root.ID); len(got) != 1 {
		t.Fatalf("appending to the returned slice reached the tree: %v", got)
	}
}

// A cycle cannot be written through `link`, and can still be on disk: `promote`
// copies edges through no gate, and a log can be hand-edited. So the derivation
// has to terminate on one and say what it found — the read path stays tolerant,
// which is the whole reason the rule lives on the write path.
func TestACycleIsReportedAndDoesNotHang(t *testing.T) {
	a := objective(t, "A")
	b := objective(t, "B")
	c := objective(t, "C")
	tree := NewTree(logOf([]*Entry{a, b, c}, []*Edge{
		derivesFrom(a, b), derivesFrom(b, c), derivesFrom(c, a),
	}))

	if len(tree.Anomalies()) != 1 {
		t.Fatalf("expected one cycle reported once, got %+v", tree.Anomalies())
	}
	msg := tree.Anomalies()[0].Message
	if !strings.Contains(msg, "cycle") {
		t.Errorf("the anomaly should name what it is, got %q", msg)
	}
	for _, id := range []string{a.ID, b.ID, c.ID} {
		if !strings.Contains(msg, id) {
			t.Errorf("the anomaly should name the whole path; %s is missing from %q", id, msg)
		}
	}
	// Every walk that follows the parent chain has to survive it too, since the
	// links are left exactly as written rather than cut.
	for _, id := range []string{a.ID, b.ID, c.ID} {
		if d := tree.Depth(id); d > 3 {
			t.Errorf("Depth(%s) = %d; the walk must stop at the cycle", id, d)
		}
	}
	// And the whole report, which walks the chain once per objective.
	if rep := tree.Report(); len(rep.Objectives) != 3 {
		t.Fatalf("every objective is still reported over a cycle, got %d", len(rep.Objectives))
	}
}

// Two parents is the other shape the write path refuses and the read path has to
// survive. The first edge in the file wins, so the reading is deterministic, and
// the one that lost is named rather than swallowed.
func TestTwoParentsTakesTheFirstAndReportsTheSecond(t *testing.T) {
	child := objective(t, "The objective below")
	first := objective(t, "The parent written first")
	second := objective(t, "The parent written second")

	tree := NewTree(logOf([]*Entry{child, first, second}, []*Edge{
		derivesFrom(child, first), derivesFrom(child, second),
	}))

	if got := tree.Parent(child.ID); got != first.ID {
		t.Errorf("the first parent edge in the file wins, got %q", got)
	}
	if len(tree.Anomalies()) != 1 {
		t.Fatalf("expected the ignored edge reported, got %+v", tree.Anomalies())
	}
	msg := tree.Anomalies()[0].Message
	if !strings.Contains(msg, first.ID) || !strings.Contains(msg, second.ID) {
		t.Errorf("the anomaly must name the parent kept and the one ignored, got %q", msg)
	}
	if got := tree.Children(second.ID); len(got) != 0 {
		t.Errorf("the losing parent gains no child, got %v", got)
	}
}

// ------------------------------------------------- blocking and completion ---

// blocked-by-an-open-child is derived from the tree and never written, so that
// "blocked on a sibling" and "blocked by my own child" cannot become
// indistinguishable on disk. It is informational: nothing here removes the
// parent from anything.
func TestBlockedByOpenChildCountsOpenChildrenAndNotSeparatedOnes(t *testing.T) {
	parent := objective(t, "The parent")
	open := objective(t, "Still open")
	done := objective(t, "Finished")
	gone := objective(t, "Lifted out from under it")
	decision := entry(t, KindDecision, "The decision behind the states", "auth")

	tree := NewTree(logOf([]*Entry{parent, open, done, gone, decision}, []*Edge{
		derivesFrom(open, parent), derivesFrom(done, parent), derivesFrom(gone, parent),
		edgeAt(EdgeMet, done.ID, decision.ID, "2026-08-03T11:00:00Z"),
		edgeAt(EdgeSeparated, gone.ID, decision.ID, "2026-08-03T11:00:00Z"),
	}))

	got := tree.BlockedByOpenChild(parent.ID)
	if len(got) != 1 || got[0] != open.ID {
		t.Fatalf("only the open, unseparated child blocks; got %v", got)
	}
	if len(tree.BlockedByOpenChild(open.ID)) != 0 {
		t.Error("a leaf is blocked by nothing")
	}
	// And the separated child is open in its own right, whatever it does to its
	// old parent.
	if !tree.State(gone.ID).Open() {
		t.Error("separated is an open state; the work continues elsewhere")
	}
}

// The tally, one child of each shape. Superseded counting toward the parent is
// decision 01KZ30RRADAT4FM5DF3QJRCT55: absorbed work exists, someone else did it.
func TestCompletionCountsMetAndSupersededAndNotCancelledOrSeparated(t *testing.T) {
	parent := objective(t, "The parent")
	met := objective(t, "Met")
	absorbed := objective(t, "Absorbed by another objective")
	absorber := objective(t, "The objective that absorbed it")
	cancelled := objective(t, "Cancelled")
	separated := objective(t, "Separated")
	open := objective(t, "Still open")
	decision := entry(t, KindDecision, "The decision behind the states", "auth")

	tree := NewTree(logOf([]*Entry{parent, met, absorbed, absorber, cancelled, separated, open, decision}, []*Edge{
		derivesFrom(met, parent), derivesFrom(absorbed, parent), derivesFrom(cancelled, parent),
		derivesFrom(separated, parent), derivesFrom(open, parent),
		edgeAt(EdgeMet, met.ID, decision.ID, "2026-08-03T11:00:00Z"),
		edgeAt(EdgeCancelled, cancelled.ID, decision.ID, "2026-08-03T11:00:00Z"),
		edgeAt(EdgeSeparated, separated.ID, decision.ID, "2026-08-03T11:00:00Z"),
		edgeAt(EdgeSupersedes, absorber.ID, absorbed.ID, "2026-08-03T11:00:00Z"),
	}))

	c := tree.Completion(parent.ID)
	want := map[string][]string{
		"counted done": {met.ID, absorbed.ID},
		"not counted":  {cancelled.ID, separated.ID},
		"outstanding":  {open.ID},
	}
	got := map[string][]string{
		"counted done": c.CountedDone,
		"not counted":  c.NotCounted,
		"outstanding":  c.Outstanding,
	}
	for bucket, ids := range want {
		if strings.Join(sortedCopy(got[bucket]), ",") != strings.Join(sortedCopy(ids), ",") {
			t.Errorf("%s: want %v, got %v", bucket, ids, got[bucket])
		}
	}
}

func sortedCopy(ids []string) []string {
	out := append([]string{}, ids...)
	for i := range out {
		for j := i + 1; j < len(out); j++ {
			if out[j] < out[i] {
				out[i], out[j] = out[j], out[i]
			}
		}
	}
	return out
}

// --------------------------------------------------- the current objective ---

// With nothing approved, the most recently touched open objective is current —
// and activity counts an edge with the objective at *either* end, since most of
// what happens to an objective is written pointing at it.
func TestCurrentFallsBackToActivityAndThenPrefersTheMark(t *testing.T) {
	older := objective(t, "Touched first")
	newer := objective(t, "Touched last")
	decision := entry(t, KindDecision, "The decision that pins the focus", "auth")
	deviation := entry(t, KindDeviation, "Something that came out of the newer objective", "auth")

	edges := []*Edge{
		edgeAt(EdgeRelatesTo, deviation.ID, newer.ID, "2026-08-03T12:00:00Z"),
	}
	all := []*Entry{older, newer, decision, deviation}

	cur := NewTree(logOf(all, edges)).Current()
	if cur.ID != newer.ID || cur.Rule != RuleActivity {
		t.Fatalf("with nothing approved the most recently touched wins: %+v", cur)
	}
	if cur.Why == "" {
		t.Error("the rule that resolved it has to be readable, not just named")
	}

	// Approving the older, less recently touched one moves the answer and
	// changes the rule with it. That is the whole point of the hybrid: pinning
	// the focus stays available without costing anything when it is not used.
	edges = append(edges, edgeAt(EdgeApprovedForExecution, older.ID, decision.ID, "2026-08-01T09:00:00Z"))
	cur = NewTree(logOf(all, edges)).Current()
	if cur.ID != older.ID || cur.Rule != RuleApproved {
		t.Fatalf("the mark beats activity, however old it is: %+v", cur)
	}
	if cur.Title != older.Title {
		t.Errorf("the answer should carry enough to read without a second lookup, got %+v", cur)
	}
}

// The tie-break, and the reason NewTree takes lines rather than the two slices
// split out of them.
//
// `at` is written to the second and this loop appends a batch of records at the
// end of a turn, so a shared `at` is the ordinary case: on this repo's own log
// most lines share their second with another record, and some of those seconds
// mix an entry with an edge. When every candidate ties, the file is the only
// thing left, and the rule says the later line wins.
//
// The shape below is the one that separates the rule from an approximation of
// it. O1, O2, an edge touching both, then O3 — four lines, one second. O1 and O2
// are last touched on line 3; O3's own entry is line 4, so O3 is current.
// Ranking by "every entry, then every edge" instead puts the edge after O3's
// entry and answers O2, which is a different objective and a wrong one.
func TestActivityTiesAreBrokenByTheLaterLineInTheFile(t *testing.T) {
	const at = "2026-08-04T01:26:09Z"
	stamp := func(e *Entry) *Entry { e.At = at; return e }

	o1 := stamp(objective(t, "O1"))
	o2 := stamp(objective(t, "O2"))
	o3 := stamp(objective(t, "O3"))
	touchesBoth := edgeAt(EdgeRelatesTo, o2.ID, o1.ID, at)

	cur := NewTree(fileOf(o1, o2, touchesBoth, o3)).Current()
	if cur.Rule != RuleActivity {
		t.Fatalf("nothing is approved here, so activity is what should answer: %+v", cur)
	}
	if cur.ID != o3.ID {
		t.Fatalf("the last line of the file wins the tie: want O3 %s, got %s (%q)",
			o3.ID, cur.ID, cur.Title)
	}

	// Moving that same edge to the last line moves the answer with it, which is
	// what makes this a rule about the file rather than about the four records.
	//
	// It also lands on the residual tie the stated rule cannot reach. The edge
	// bumps *both* of its endpoints, so O1 and O2 are tied on `at` and on the
	// line as well, and the later line cannot separate two records that share a
	// line. What separates them is the order of their own entries: O2's is the
	// later of the two, so O2 takes it — not because it is the edge's source,
	// which is a coincidence of how this fixture happens to point. See
	// recordActivity.
	cur = NewTree(fileOf(o1, o2, o3, touchesBoth)).Current()
	if cur.ID != o2.ID {
		t.Fatalf("tied on the line, the later entry wins: want O2 %s, got %s", o2.ID, cur.ID)
	}
}

// The residual tie, pinned. Two objectives touched by one edge share a line, so
// the answer comes down to which of their entries is later in the file — and the
// proof that it is entry order rather than the edge's direction is that swapping
// the two entries swaps the answer while the edge goes on pointing the same way.
//
// Documented rather than fixed: any rule that separated them would have to invent
// a reason to prefer the parent or the child, and the edge is one fact about both
// of them, written once. This test exists so that behaviour cannot drift without
// somebody deciding to let it — in particular so the tie clause in `beats` is not
// tidied away as a stray `>=`.
func TestObjectivesTiedOnOneLineFallBackToEntryOrder(t *testing.T) {
	const at = "2026-08-04T01:26:09Z"
	stamp := func(e *Entry) *Entry { e.At = at; return e }

	o1 := stamp(objective(t, "O1"))
	o2 := stamp(objective(t, "O2"))
	o3 := stamp(objective(t, "O3"))
	// A parent edge, which is the shape this actually arises from: an objective
	// deriving from another puts both of them on one line.
	parentEdge := edgeAt(EdgeDerivesFrom, o2.ID, o1.ID, at)

	if cur := NewTree(fileOf(o1, o2, o3, parentEdge)).Current(); cur.ID != o2.ID {
		t.Fatalf("O2's entry is the later of the tied pair: want %s, got %s", o2.ID, cur.ID)
	}
	if cur := NewTree(fileOf(o2, o1, o3, parentEdge)).Current(); cur.ID != o1.ID {
		t.Fatalf("swapping the entries swaps the answer: want %s, got %s", o1.ID, cur.ID)
	}
}

// Depth removes nothing and neither does having an open child. A chain of three
// open objectives has three candidates, and the deepest is not privileged.
func TestCurrentIgnoresDepthAndPrefersNothingForIt(t *testing.T) {
	root := objective(t, "Root")
	middle := objective(t, "Middle")
	leaf := objective(t, "Leaf")
	decision := entry(t, KindDecision, "The decision that pins the focus", "auth")

	all := []*Entry{root, middle, leaf, decision}
	edges := []*Edge{derivesFrom(middle, root), derivesFrom(leaf, middle)}

	// The root is the one approved, and it is the shallowest and has an open
	// child. Neither fact takes it out of the running.
	edges = append(edges, edgeAt(EdgeApprovedForExecution, root.ID, decision.ID, "2026-08-03T12:00:00Z"))
	cur := NewTree(logOf(all, edges)).Current()
	if cur.ID != root.ID {
		t.Fatalf("depth and an open child must not remove the root: %+v", cur)
	}
}

// A terminal objective is not a candidate, and neither is one something
// superseded — the same two things --current drops for every kind.
func TestCurrentSkipsTerminalAndSupersededObjectives(t *testing.T) {
	done := objective(t, "Met")
	replaced := objective(t, "Replaced by a better statement of itself")
	replacement := objective(t, "The better statement")
	decision := entry(t, KindDecision, "The decision behind it", "auth")

	all := []*Entry{done, replaced, replacement, decision}
	edges := []*Edge{
		edgeAt(EdgeMet, done.ID, decision.ID, "2026-08-04T10:00:00Z"),
		edgeAt(EdgeSupersedes, replacement.ID, replaced.ID, "2026-08-04T10:00:00Z"),
	}

	cur := NewTree(logOf(all, edges)).Current()
	if cur.ID != replacement.ID {
		t.Fatalf("only the live objective is a candidate: %+v", cur)
	}

	// Nothing open at all says so rather than naming one anyway.
	cur = NewTree(logOf([]*Entry{done, decision}, edges[:1])).Current()
	if cur.ID != "" || cur.Rule != RuleNone {
		t.Fatalf("with no open objective the field must say so: %+v", cur)
	}
}

// The derivation reads the whole log; the filter applies afterwards. A --plan
// that excludes the current objective still gets told which one it is, because
// naming the runner-up would answer a question nobody asked.
func TestQueryNamesTheCurrentObjectiveEvenWhenItIsOutsideTheFilter(t *testing.T) {
	mine := objective(t, "The objective in the plan being queried")
	mine.Origin = &Origin{Plan: "this-plan"}
	theirs := objective(t, "The objective actually being worked on")
	theirs.Origin = &Origin{Plan: "another-plan"}

	edges := []*Edge{edgeAt(EdgeRelatesTo, mine.ID, theirs.ID, "2026-08-04T10:00:00Z")}
	res := Query(logOf([]*Entry{mine, theirs}, edges), Filter{Plan: "this-plan", Kinds: []Kind{KindObjective}})

	if res.Count != 1 || res.Records[0].ID != mine.ID {
		t.Fatalf("the filter still bounds what comes back: %+v", res.Records)
	}
	if res.CurrentObjective == nil {
		t.Fatal("a query over objectives has to report which one is current")
	}
	if res.CurrentObjective.ID != theirs.ID {
		t.Errorf("the current objective is derived from the whole log, got %+v", res.CurrentObjective)
	}
	if !res.CurrentObjective.OutsideFilter {
		t.Error("it falls outside the filter and the answer has to say so")
	}
	if !strings.Contains(res.CurrentObjective.Why, "filter") {
		t.Errorf("the reason should be readable, got %q", res.CurrentObjective.Why)
	}
	// Hint means truncation and nothing else; nothing about the current
	// objective may leak into it.
	if res.Hint != "" {
		t.Errorf("hint must still mean only truncation, got %q", res.Hint)
	}
}

// A query that cannot return an objective is not asked about one, so it gets no
// answer about one. This is what keeps every other kind's output where it was.
func TestQueryOverOtherKindsReportsNoCurrentObjective(t *testing.T) {
	o := objective(t, "An objective")
	q := entry(t, KindOpenQuestion, "An open question", "auth")

	res := Query(logOf([]*Entry{o, q}, nil), Filter{Kinds: []Kind{KindOpenQuestion, KindAssumption}, CurrentOnly: true})
	if res.CurrentObjective != nil {
		t.Fatalf("a question-shaped query asked nothing about objectives: %+v", res.CurrentObjective)
	}
	if res.Count != 1 || res.Records[0].ID != q.ID {
		t.Fatalf("and it returns exactly what it returned before: %+v", res.Records)
	}
}

// --current over objectives drops the two terminal states and nothing else.
func TestQueryCurrentDropsOnlyTerminalObjectives(t *testing.T) {
	root := objective(t, "Root")
	middle := objective(t, "Middle")
	leaf := objective(t, "Leaf")
	decision := entry(t, KindDecision, "The decision behind the states", "auth")
	all := []*Entry{root, middle, leaf, decision}
	edges := []*Edge{derivesFrom(middle, root), derivesFrom(leaf, middle)}

	res := Query(logOf(all, edges), Filter{Kinds: []Kind{KindObjective}, CurrentOnly: true})
	if res.Count != 3 {
		t.Fatalf("all three open objectives come back; depth removes nothing: %d", res.Count)
	}

	edges = append(edges, edgeAt(EdgeMet, leaf.ID, decision.ID, "2026-08-04T10:00:00Z"))
	res = Query(logOf(all, edges), Filter{Kinds: []Kind{KindObjective}, CurrentOnly: true})
	if res.Count != 2 {
		t.Fatalf("marking the leaf met drops the leaf and only the leaf: %d", res.Count)
	}
	for _, r := range res.Records {
		if r.ID == leaf.ID {
			t.Error("the met objective is still being returned")
		}
	}
}

// -------------------------------------------------------------- write path ---

// The rule applies to objective → objective and to nothing else, which is what
// makes reusing `derives_from` affordable: every other thing it says keeps
// saying it.
func TestCheckParentLeavesEveryOtherDerivesFromAlone(t *testing.T) {
	const (
		objA     = "01KZ2WFZ2ASGEZFYG12NQ6QHW6"
		objB     = "01KZ3020N7RDAETA6ZCFA58BGE"
		decision = "01KZ30RR7THXNEWMEST0T7CFHN"
		adr      = "01KZ34725V37YX845RTVPC7N0E"
	)
	kinds := map[string]Kind{
		objA: KindObjective, objB: KindObjective,
		decision: KindDecision, adr: KindADR,
	}
	existing := []*Edge{edgeAt(EdgeDerivesFrom, objB, objA, "2026-08-03T10:00:00Z")}

	// The objective already has a parent, and none of these are parent edges.
	for _, c := range []struct{ typ, from, to string }{
		{"derives_from", decision, objA},
		{"derives_from", objB, decision},
		{"derives_from", adr, decision},
		{"relates_to", objB, objA},
		{"supersedes", objB, objA},
	} {
		if err := CheckParent(EdgeType(c.typ), c.from, c.to, kinds, existing); err != nil {
			t.Errorf("%s %s → %s is not parentage and must be untouched: %v", c.typ, c.from, c.to, err)
		}
	}
}

// One parent, and the refusal names the parent it already has — otherwise the
// reader goes back to the log to look the id up by hand, which is the discipline
// CheckJustification's errors follow.
func TestCheckParentRefusesASecondParent(t *testing.T) {
	const (
		child = "01KZ30Q5RZ6JKXP5QDN2N1WY03"
		first = "01KZ3020N7RDAETA6ZCFA58BGE"
		other = "01KZ34725V37YX845RTVPC7N0E"
	)
	kinds := map[string]Kind{child: KindObjective, first: KindObjective, other: KindObjective}
	existing := []*Edge{edgeAt(EdgeDerivesFrom, child, first, "2026-08-03T10:00:00Z")}

	err := CheckParent(EdgeDerivesFrom, child, other, kinds, existing)
	if err == nil {
		t.Fatal("an objective has one parent")
	}
	if !strings.Contains(err.Error(), first) {
		t.Errorf("the refusal must name the parent it already has, got %q", err)
	}
	if !strings.Contains(err.Error(), other) {
		t.Errorf("and the one it was asked for, got %q", err)
	}
	// A second root is not a second parent. Nothing requires a parent at all.
	if err := CheckParent(EdgeDerivesFrom, other, first, kinds, existing); err != nil {
		t.Errorf("an objective with no parent yet may take one: %v", err)
	}
}

// A cycle is refused and the path is named, self-edge included — the rule is
// stated whole even though `link` refuses an edge pointing at its own source
// before it ever gets here.
func TestCheckParentRefusesACycleAndNamesThePath(t *testing.T) {
	const (
		a = "01KZ2WFZ2ASGEZFYG12NQ6QHW6"
		b = "01KZ3020N7RDAETA6ZCFA58BGE"
		c = "01KZ30Q5RZ6JKXP5QDN2N1WY03"
	)
	kinds := map[string]Kind{a: KindObjective, b: KindObjective, c: KindObjective}
	existing := []*Edge{
		edgeAt(EdgeDerivesFrom, b, a, "2026-08-03T10:00:00Z"),
		edgeAt(EdgeDerivesFrom, c, b, "2026-08-03T10:00:00Z"),
	}

	err := CheckParent(EdgeDerivesFrom, a, c, kinds, existing)
	if err == nil {
		t.Fatal("a → c closes a → b → c → a and must be refused")
	}
	for _, id := range []string{a, b, c} {
		if !strings.Contains(err.Error(), id) {
			t.Errorf("the refusal must name the whole path; %s missing from %q", id, err)
		}
	}

	if err := CheckParent(EdgeDerivesFrom, a, a, kinds, nil); err == nil {
		t.Error("a self-edge is a cycle of length one")
	}
}

// The write path cannot assume the ground it stands on is a tree: nothing sweeps
// the log for a cycle, and `promote` copies edges through no gate. So the
// ancestor walk has to terminate on one it had no part in making.
func TestCheckParentSurvivesACycleAlreadyOnDisk(t *testing.T) {
	const (
		a       = "01KZ2WFZ2ASGEZFYG12NQ6QHW6"
		b       = "01KZ3020N7RDAETA6ZCFA58BGE"
		orphan  = "01KZ30Q5RZ6JKXP5QDN2N1WY03"
		newRoot = "01KZ34725V37YX845RTVPC7N0E"
	)
	kinds := map[string]Kind{
		a: KindObjective, b: KindObjective, orphan: KindObjective, newRoot: KindObjective,
	}
	existing := []*Edge{
		edgeAt(EdgeDerivesFrom, a, b, "2026-08-03T10:00:00Z"),
		edgeAt(EdgeDerivesFrom, b, a, "2026-08-03T10:00:00Z"),
	}

	done := make(chan error, 1)
	go func() { done <- CheckParent(EdgeDerivesFrom, orphan, a, kinds, existing) }()
	select {
	case err := <-done:
		if err != nil {
			t.Errorf("an unrelated cycle above the pair is not this edge's problem: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the ancestor walk did not terminate on a cycle already on disk")
	}
	if err := CheckParent(EdgeDerivesFrom, orphan, newRoot, kinds, existing); err != nil {
		t.Errorf("and a link nowhere near it is untouched: %v", err)
	}
}
