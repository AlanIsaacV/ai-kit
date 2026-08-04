package main

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

// These tests drive the built binary over throwaway logs, for the same reason
// the ones in main_test.go do: what is being checked is the shape of the answer
// a caller actually gets, and a package-level call cannot see the JSON the
// process prints.
//
// The fixtures are built through the CLI wherever the CLI will build them. The
// two shapes it refuses — a cycle and a second parent — are hand-written,
// because refusing them is the point of the rule being tested.

// ------------------------------------------------------------- CLI helpers ---

func link(t *testing.T, r repo, typ, from, to string) result {
	t.Helper()
	return runBare(t, r, "link", "--type", typ, "--from", from, "--to", to)
}

func mustLink(t *testing.T, r repo, typ, from, to string) {
	t.Helper()
	if res := link(t, r, typ, from, to); res.code != 0 {
		t.Fatalf("link --type %s %s → %s: exit %d\n%s%s", typ, from, to, res.code, res.stdout, res.stderr)
	}
}

// addObjective is `add --kind objective`, which is most of what these fixtures
// are made of.
func addObjective(t *testing.T, r repo, title string) string {
	t.Helper()
	return addKind(t, r, "objective", title)
}

// child adds an objective and points it at its parent in one step, so a fixture
// tree reads as a tree.
func child(t *testing.T, r repo, parent, title string) string {
	t.Helper()
	id := addObjective(t, r, title)
	mustLink(t, r, "derives_from", id, parent)
	return id
}

type stateOutput struct {
	Count      int `json:"count"`
	Objectives []struct {
		ID                 string   `json:"id"`
		Title              string   `json:"title"`
		Plan               string   `json:"plan"`
		State              string   `json:"state"`
		Open               bool     `json:"open"`
		Depth              int      `json:"depth"`
		Parent             string   `json:"parent"`
		Children           []string `json:"children"`
		BlockedByOpenChild []string `json:"blocked_by_open_child"`
		Completion         struct {
			CountedDone []string `json:"counted_done"`
			NotCounted  []string `json:"not_counted"`
			Outstanding []string `json:"outstanding"`
		} `json:"completion"`
	} `json:"objectives"`
	Excluded []struct {
		ID     string `json:"id"`
		Reason string `json:"reason"`
	} `json:"excluded"`
	Current struct {
		ID            string `json:"id"`
		Rule          string `json:"rule"`
		Why           string `json:"why"`
		OutsideFilter bool   `json:"outside_filter"`
	} `json:"current"`
	Anomalies []struct {
		ID      string `json:"id"`
		Message string `json:"message"`
	} `json:"anomalies"`
}

func readState(t *testing.T, r repo, args ...string) stateOutput {
	t.Helper()
	res := runBare(t, r, append([]string{"state"}, args...)...)
	if res.code != 0 {
		t.Fatalf("state %v: exit %d\n%s%s", args, res.code, res.stdout, res.stderr)
	}
	var out stateOutput
	if err := json.Unmarshal([]byte(res.stdout), &out); err != nil {
		t.Fatalf("state printed something that is not JSON: %v\n%s", err, res.stdout)
	}
	return out
}

// one is the objective a `state --id` run reported, and a failure when it
// reported anything else.
func one(t *testing.T, out stateOutput) stateOutput {
	t.Helper()
	if len(out.Objectives) != 1 {
		t.Fatalf("expected exactly one objective reported, got %d", len(out.Objectives))
	}
	return out
}

type queryOutput struct {
	Count            int    `json:"count"`
	Returned         int    `json:"returned"`
	Hint             string `json:"hint"`
	CurrentObjective *struct {
		ID            string `json:"id"`
		Title         string `json:"title"`
		Rule          string `json:"rule"`
		Why           string `json:"why"`
		OutsideFilter bool   `json:"outside_filter"`
	} `json:"current_objective"`
	Records []struct {
		ID   string `json:"id"`
		Kind string `json:"kind"`
	} `json:"records"`
}

func readQuery(t *testing.T, r repo, args ...string) queryOutput {
	t.Helper()
	res := runBare(t, r, args...)
	if res.code != 0 {
		t.Fatalf("%v: exit %d\n%s%s", args, res.code, res.stdout, res.stderr)
	}
	var out queryOutput
	if err := json.Unmarshal([]byte(res.stdout), &out); err != nil {
		t.Fatalf("%v printed something that is not JSON: %v\n%s", args, err, res.stdout)
	}
	return out
}

func ids(out queryOutput) []string {
	got := make([]string, 0, len(out.Records))
	for _, r := range out.Records {
		got = append(got, r.ID)
	}
	return got
}

func contains(haystack []string, want string) bool {
	for _, h := range haystack {
		if h == want {
			return true
		}
	}
	return false
}

// ------------------------------------------------------------ the tree rule ---

// One parent per objective, refused where it can be refused: on the way in, by
// `link`, since the log is append-only and there is no taking a second parent
// back afterwards.
func TestLinkRefusesASecondObjectiveParent(t *testing.T) {
	r := throwawayRepo(t)
	first := addObjective(t, r, "The parent written first")
	second := addObjective(t, r, "Another root")
	kid := child(t, r, first, "The objective below")

	before := len(lines(t, r.log))
	res := link(t, r, "derives_from", kid, second)
	if res.code == 0 {
		t.Fatal("an objective has one parent; the second must be refused")
	}
	code, msg := errorCode(t, res)
	if code != "not_a_tree" {
		t.Errorf("expected a not_a_tree refusal, got %q", code)
	}
	// The parent it already has, by id: without it the reader goes back to the
	// log to look it up by hand, which is the discipline the justification
	// refusals already follow.
	if !strings.Contains(msg, first) {
		t.Errorf("the refusal must name the parent it already has, got %q", msg)
	}
	if after := len(lines(t, r.log)); after != before {
		t.Fatalf("a refused link wrote to the log: %d lines became %d", before, after)
	}

	// Several roots stay legal. Nothing requires a parent, and the tree this
	// repo actually holds has more than one.
	if got := readState(t, r); len(got.Objectives) != 3 {
		t.Fatalf("expected three objectives, two of them roots, got %d", len(got.Objectives))
	}
}

// A cycle is refused and the whole path is named, so the refusal can be acted on
// without reconstructing the chain by hand.
func TestLinkRefusesACycleAndNamesThePath(t *testing.T) {
	r := throwawayRepo(t)
	a := addObjective(t, r, "A")
	b := child(t, r, a, "B")
	c := child(t, r, b, "C")

	before := len(lines(t, r.log))
	res := link(t, r, "derives_from", a, c)
	if res.code == 0 {
		t.Fatal("a → c closes a cycle and must be refused")
	}
	code, msg := errorCode(t, res)
	if code != "not_a_tree" {
		t.Errorf("expected a not_a_tree refusal, got %q", code)
	}
	for _, id := range []string{a, b, c} {
		if !strings.Contains(msg, id) {
			t.Errorf("the refusal must name the whole path; %s is missing from %q", id, msg)
		}
	}
	if after := len(lines(t, r.log)); after != before {
		t.Fatalf("a refused link wrote to the log: %d lines became %d", before, after)
	}

	// A self-edge is a cycle of length one, and it never reaches the tree rule:
	// the current version's rule set refuses an edge pointing at its own source
	// first. Both refusals are refusals, and nothing reaches disk either way.
	res = link(t, r, "derives_from", a, a)
	if res.code == 0 {
		t.Fatal("an objective cannot derive from itself")
	}
	if after := len(lines(t, r.log)); after != before {
		t.Fatalf("a refused self-edge wrote to the log: %d lines became %d", before, after)
	}
}

// The rule applies to objective → objective and to nothing else. `derives_from`
// is overloaded on purpose — it also points a decision at the objective it was
// taken under, and an ADR at the decisions it came from — and both of those go on
// meaning what they meant.
func TestTheTreeRuleLeavesEveryOtherDerivesFromAlone(t *testing.T) {
	r := throwawayRepo(t)
	objective := addObjective(t, r, "The objective")
	kid := child(t, r, objective, "Its child")
	first := addKind(t, r, "decision", "The first decision")
	second := addKind(t, r, "decision", "The second decision")
	adr := addKind(t, r, "adr", "What the two decisions add up to")

	// A decision deriving from an objective, and an objective deriving from a
	// decision: neither end of either is parentage.
	mustLink(t, r, "derives_from", first, objective)
	mustLink(t, r, "derives_from", kid, second)
	// And an ADR may derive from as many decisions as produced it, which is the
	// case a one-parent rule applied to the type rather than the kinds would
	// have broken.
	mustLink(t, r, "derives_from", adr, first)
	mustLink(t, r, "derives_from", adr, second)

	got := readState(t, r, "--id", kid)
	one(t, got)
	if got.Objectives[0].Parent != objective {
		t.Errorf("the objective parent is unchanged by any of it, got %q", got.Objectives[0].Parent)
	}
	if len(got.Anomalies) != 0 {
		t.Fatalf("none of those edges is parentage, so none is an anomaly: %+v", got.Anomalies)
	}
}

// ------------------------------------------------------------ state, resolved ---

// The README's rule, end to end and through the binary: four transitions, no
// retraction, and the answer is the last one.
func TestStateResolvesTheWholeSequenceThroughTheCLI(t *testing.T) {
	r := throwawayRepo(t)
	objective := addObjective(t, r, "The objective whose state this is")
	decision := addKind(t, r, "decision", "The decision behind it")
	question := addKind(t, r, "open-question", "What was holding it up")

	if got := readState(t, r, "--id", objective); one(t, got).Objectives[0].State != "planned" {
		t.Fatalf("no state edge at all is planned, got %q", got.Objectives[0].State)
	}

	mustLink(t, r, "approved_for_execution", objective, decision)
	mustLink(t, r, "blocked", objective, question)
	mustLink(t, r, "approved_for_execution", objective, decision)
	mustLink(t, r, "met", objective, decision)

	got := one(t, readState(t, r, "--id", objective))
	if got.Objectives[0].State != "met" {
		t.Fatalf("approved → blocked → approved → met resolves to met, got %q", got.Objectives[0].State)
	}
	if got.Objectives[0].Open {
		t.Error("met is terminal")
	}
	// The stale edges are still on disk. That trace is the history a mutable
	// status field would have destroyed the moment the block cleared.
	if n := strings.Count(strings.Join(lines(t, r.log), "\n"), `"type":"blocked"`); n != 1 {
		t.Errorf("the blocked edge must still be on disk, found %d", n)
	}
}

// The other half of the same rule, which only a hand-written fixture can show:
// two state edges sharing an `at` are separated by their order in the file, and
// a later line with an older `at` still loses.
func TestStateTiesAndOutOfOrderEdgesResolveByTheStatedRule(t *testing.T) {
	r := throwawayRepo(t)
	objective := addObjective(t, r, "The objective whose state this is")
	decision := addKind(t, r, "decision", "The decision behind it")

	stateEdge := func(typ, at string) string {
		return `{"version":2,"kind":"edge","type":"` + typ + `","from":"` + objective +
			`","to":"` + decision + `","at":"` + at + `"}`
	}
	// A `met` at noon, then a `cancelled` written afterwards but dated the
	// morning: recency decides, so it loses.
	appendLine(t, r.log, stateEdge("met", "2026-08-03T12:00:00Z"))
	appendLine(t, r.log, stateEdge("cancelled", "2026-08-03T09:00:00Z"))
	if got := one(t, readState(t, r, "--id", objective)); got.Objectives[0].State != "met" {
		t.Fatalf("a later line with an older `at` must not win, got %q", got.Objectives[0].State)
	}

	// And now one sharing the winner's `at` exactly, which is what two edges
	// appended in the same second look like. Only the file order can separate
	// them, and the later line wins.
	appendLine(t, r.log, stateEdge("cancelled", "2026-08-03T12:00:00Z"))
	if got := one(t, readState(t, r, "--id", objective)); got.Objectives[0].State != "cancelled" {
		t.Fatalf("with a shared `at` the later line wins, got %q", got.Objectives[0].State)
	}
}

// ----------------------------------------------------------------- --current ---

// --current over objectives means what it means for every other kind: drop what
// is settled, return the rest. Neither depth nor an open child removes anything.
func TestCurrentOverObjectivesReturnsEveryOpenOne(t *testing.T) {
	r := throwawayRepo(t)
	root := addObjective(t, r, "Root")
	middle := child(t, r, root, "Middle")
	leaf := child(t, r, middle, "Leaf")
	decision := addKind(t, r, "decision", "The decision behind the states")

	got := readQuery(t, r, "query", "--plan", "demo", "--kind", "objective", "--current")
	if got.Count != 3 {
		t.Fatalf("a chain of three open objectives returns three: %d, %v", got.Count, ids(got))
	}
	for _, want := range []string{root, middle, leaf} {
		if !contains(ids(got), want) {
			t.Errorf("%s is open and was not returned: %v", want, ids(got))
		}
	}

	// Marking the leaf met drops the leaf and only the leaf.
	mustLink(t, r, "met", leaf, decision)
	got = readQuery(t, r, "query", "--plan", "demo", "--kind", "objective", "--current")
	if got.Count != 2 || contains(ids(got), leaf) {
		t.Fatalf("only the met objective should have gone: %v", ids(got))
	}

	// `separated` is open. The work continues; it continues elsewhere.
	mustLink(t, r, "separated", middle, decision)
	got = readQuery(t, r, "query", "--plan", "demo", "--kind", "objective", "--current")
	if !contains(ids(got), middle) {
		t.Fatalf("a separated objective is still open and still current: %v", ids(got))
	}

	// And the two things --current already dropped for every kind it still
	// drops: a superseded record.
	replacement := addObjective(t, r, "A better statement of the root")
	mustLink(t, r, "supersedes", replacement, root)
	got = readQuery(t, r, "query", "--plan", "demo", "--kind", "objective", "--current")
	if contains(ids(got), root) {
		t.Fatalf("a superseded objective is not current: %v", ids(got))
	}
}

// Every kind other than objective returns exactly what it returned before, and
// gets no answer about objectives it did not ask for.
func TestCurrentOverOtherKindsIsUntouched(t *testing.T) {
	r := throwawayRepo(t)
	objective := addObjective(t, r, "An objective")
	question := addKind(t, r, "open-question", "Still open")
	answered := addKind(t, r, "open-question", "Since answered")
	decision := addKind(t, r, "decision", "The answer")
	mustLink(t, r, "answers", decision, answered)
	mustLink(t, r, "met", objective, decision)

	got := readQuery(t, r, "query", "--plan", "demo", "--kind", "open-question,assumption", "--current")
	if got.Count != 1 || got.Records[0].ID != question {
		t.Fatalf("the answered question is dropped and the open one is not: %v", ids(got))
	}
	if got.CurrentObjective != nil {
		t.Fatalf("a query that cannot return an objective is not asked about one: %+v", got.CurrentObjective)
	}
}

// ------------------------------------------------- which one is being worked on ---

// One answer, in its own field, with the rule that produced it — demonstrated
// both ways round, because a field that only ever says one thing proves nothing.
func TestTheResolvedCurrentObjectiveIsReportedWithItsRule(t *testing.T) {
	r := throwawayRepo(t)
	touched := addObjective(t, r, "The objective touched most recently")
	marked := addObjective(t, r, "The objective that will be approved")
	unit := addKind(t, r, "unit", "A unit of work under the first one")
	mustLink(t, r, "implements", unit, touched)

	got := readQuery(t, r, "query", "--plan", "demo", "--kind", "objective", "--current")
	if got.CurrentObjective == nil {
		t.Fatal("a query over objectives has to say which one is current")
	}
	if got.CurrentObjective.ID != touched || got.CurrentObjective.Rule != "activity" {
		t.Fatalf("with nothing approved the most recently touched wins: %+v", got.CurrentObjective)
	}
	if got.CurrentObjective.Why == "" {
		t.Error("the rule has to be readable, not only named")
	}
	// Hint means truncation and nothing else. Nothing about the current
	// objective may leak into it.
	if got.Hint != "" {
		t.Errorf("hint must go on meaning only truncation, got %q", got.Hint)
	}

	// Approve the other one, then touch the first one again so activity would
	// still name it. The mark has to win anyway, or it is not a mark.
	decision := addKind(t, r, "decision", "Work on the second one next")
	mustLink(t, r, "approved_for_execution", marked, decision)
	note := addKind(t, r, "wont-fix", "Something recorded against the first one")
	mustLink(t, r, "relates_to", note, touched)

	got = readQuery(t, r, "query", "--plan", "demo", "--kind", "objective", "--current")
	if got.CurrentObjective.ID != marked || got.CurrentObjective.Rule != "approved_for_execution" {
		t.Fatalf("the mark beats activity: %+v", got.CurrentObjective)
	}

	// And when there is nothing open to name, the field says so rather than
	// naming one anyway.
	for _, id := range []string{touched, marked} {
		mustLink(t, r, "met", id, decision)
	}
	got = readQuery(t, r, "query", "--plan", "demo", "--kind", "objective", "--current")
	if got.CurrentObjective.ID != "" || got.CurrentObjective.Rule != "none" {
		t.Fatalf("with no open objective the field must say so: %+v", got.CurrentObjective)
	}
}

// The tie-break, through the binary, on the shape that actually occurs.
//
// `at` is written to the second and this loop appends a batch of adds and links
// at the end of a turn, so most lines of a real log share their second with
// another record and some of those seconds mix an entry with an edge. When every
// candidate ties, the file is all that is left, and the rule is the later line.
//
// Hand-written, whole, because that is the only way to pin four records to one
// second and to four chosen lines. The ids are picked so the answer cannot come
// out right by accident: O3, the one the rule names, has the *lowest* id of the
// three, so nothing that fell back to "most recent ULID" would produce it.
func TestTheCurrentObjectiveTieBreaksByTheLaterLineInTheFile(t *testing.T) {
	const (
		at = "2026-08-04T01:26:09Z"
		o1 = "01KZ3020N7RDAETA6ZCFA58BGE"
		o2 = "01KZ30Q5RZ6JKXP5QDN2N1WY03"
		o3 = "01KZ2WFZ2ASGEZFYG12NQ6QHW6"
	)
	objectiveLine := func(id, slug string) string {
		return `{"version":2,"id":"` + id + `","kind":"objective","slug":"` + slug +
			`","scope":"repo","at":"` + at + `","about":["auth"],"title":"` + strings.ToUpper(slug) +
			`","origin":{"plan":"demo"}}`
	}
	edgeLine := func(from, to string) string {
		return `{"version":2,"kind":"edge","type":"relates_to","from":"` + from +
			`","to":"` + to + `","at":"` + at + `"}`
	}

	r := throwawayRepo(t)
	if res := runBare(t, r, "init"); res.code != 0 {
		t.Fatalf("init: exit %d\n%s%s", res.code, res.stdout, res.stderr)
	}
	// Line 1: O1. Line 2: O2. Line 3: an edge touching both. Line 4: O3.
	// O1 and O2 are last touched on line 3; O3's own entry is line 4.
	appendLine(t, r.log, objectiveLine(o1, "o1"))
	appendLine(t, r.log, objectiveLine(o2, "o2"))
	appendLine(t, r.log, edgeLine(o2, o1))
	appendLine(t, r.log, objectiveLine(o3, "o3"))

	got := readQuery(t, r, "query", "--plan", "demo", "--kind", "objective", "--current")
	if got.Count != 3 {
		t.Fatalf("all three are open and all three come back: %v", ids(got))
	}
	if got.CurrentObjective == nil || got.CurrentObjective.Rule != "activity" {
		t.Fatalf("nothing is approved here, so activity is what answers: %+v", got.CurrentObjective)
	}
	if got.CurrentObjective.ID != o3 {
		t.Fatalf("the later line wins the tie: want O3 %s, got %s (%q)",
			o3, got.CurrentObjective.ID, got.CurrentObjective.Title)
	}
	// `state` resolves it the same way, since both read the same derivation.
	if st := readState(t, r); st.Current.ID != o3 {
		t.Fatalf("state must agree with query: want %s, got %s", o3, st.Current.ID)
	}
}

// The derivation reads the whole log and the filter applies afterwards. Naming
// the runner-up instead would answer a question nobody asked, so it names the
// real one and says it is out of view.
func TestTheCurrentObjectiveIsDerivedBeforeThePlanFilter(t *testing.T) {
	r := throwawayRepo(t)
	mine := addObjectiveIn(t, r, "mine", "The objective in the plan being queried")
	theirs := addObjectiveIn(t, r, "elsewhere", "The objective actually being worked on")
	unit := addKind(t, r, "unit", "A unit under the other plan's objective")
	mustLink(t, r, "implements", unit, theirs)

	got := readQuery(t, r, "query", "--plan", "mine", "--kind", "objective", "--current")
	if got.Count != 1 || got.Records[0].ID != mine {
		t.Fatalf("the filter still bounds what comes back: %v", ids(got))
	}
	if got.CurrentObjective == nil || got.CurrentObjective.ID != theirs {
		t.Fatalf("the current objective is derived from the whole log: %+v", got.CurrentObjective)
	}
	if !got.CurrentObjective.OutsideFilter {
		t.Error("it falls outside the filter and the answer has to say so")
	}
	if !strings.Contains(got.CurrentObjective.Why, "filter") {
		t.Errorf("and say why in words, got %q", got.CurrentObjective.Why)
	}
}

func addObjectiveIn(t *testing.T, r repo, plan, title string) string {
	t.Helper()
	res := runBare(t, r, "add", "--plan", plan, "--kind", "objective", "--title", title, "--about", "auth")
	if res.code != 0 {
		t.Fatalf("add --plan %s: exit %d\n%s%s", plan, res.code, res.stdout, res.stderr)
	}
	return recordID(t, res)
}

// ------------------------------------------------- blocked, and how far along ---

// blocked-by-an-open-child is derived from the tree, reported, and acted on by
// nothing. The edge that would say it is still refused, which is what keeps
// "blocked on a sibling" and "blocked by my own child" distinguishable.
func TestBlockedByOpenChildIsDerivedAndChangesNothing(t *testing.T) {
	r := throwawayRepo(t)
	parent := addObjective(t, r, "The parent")
	open := child(t, r, parent, "Still open")
	gone := child(t, r, parent, "Lifted out from under it")
	decision := addKind(t, r, "decision", "The decision behind the states")
	mustLink(t, r, "separated", gone, decision)

	got := one(t, readState(t, r, "--id", parent))
	if len(got.Objectives[0].BlockedByOpenChild) != 1 || got.Objectives[0].BlockedByOpenChild[0] != open {
		t.Fatalf("only the open, unseparated child blocks: %v", got.Objectives[0].BlockedByOpenChild)
	}
	if got.Objectives[0].State != "planned" {
		t.Errorf("blocked-by-a-child is not the `blocked` state and writes no edge, got %q", got.Objectives[0].State)
	}

	// Informational, and nothing acts on it: the parent is still returned by
	// --current and is still eligible to be the resolved current objective.
	q := readQuery(t, r, "query", "--plan", "demo", "--kind", "objective", "--current")
	if !contains(ids(q), parent) {
		t.Fatalf("a parent with an open child is still current: %v", ids(q))
	}

	// And the edge that would say it is still refused, unchanged.
	if res := link(t, r, "blocked", parent, open); res.code == 0 {
		t.Fatal("blocked pointing at another objective must still be refused")
	}
}

// The tally, one child of every shape. Superseded counting toward the parent is
// decision 01KZ30RRADAT4FM5DF3QJRCT55: absorbed work exists, someone else did it.
func TestCompletionIsComputedOverTheChildren(t *testing.T) {
	r := throwawayRepo(t)
	parent := addObjective(t, r, "The parent")
	met := child(t, r, parent, "Met")
	absorbed := child(t, r, parent, "Absorbed by another objective")
	cancelled := child(t, r, parent, "Cancelled")
	separated := child(t, r, parent, "Separated")
	open := child(t, r, parent, "Still open")
	absorber := addObjective(t, r, "The objective that absorbed it")
	decision := addKind(t, r, "decision", "The decision behind the states")

	mustLink(t, r, "met", met, decision)
	mustLink(t, r, "cancelled", cancelled, decision)
	mustLink(t, r, "separated", separated, decision)
	mustLink(t, r, "supersedes", absorber, absorbed)

	got := one(t, readState(t, r, "--id", parent)).Objectives[0]
	for label, pair := range map[string][2][]string{
		"counted done": {got.Completion.CountedDone, {met, absorbed}},
		"not counted":  {got.Completion.NotCounted, {cancelled, separated}},
		"outstanding":  {got.Completion.Outstanding, {open}},
	} {
		have, want := pair[0], pair[1]
		if len(have) != len(want) {
			t.Errorf("%s: want %d children, got %v", label, len(want), have)
			continue
		}
		for _, id := range want {
			if !contains(have, id) {
				t.Errorf("%s: %s is missing from %v", label, id, have)
			}
		}
	}
	if len(got.Children) != 5 {
		t.Errorf("all five are still children of it, got %v", got.Children)
	}
}

// --id has to find a record it deliberately keeps out of the live list, or the
// answer to "what happened to that objective" is silence.
func TestStateNamesASupersededObjectiveAsExcluded(t *testing.T) {
	r := throwawayRepo(t)
	old := addObjective(t, r, "The first statement of it")
	fresh := addObjective(t, r, "A better statement of it")
	mustLink(t, r, "supersedes", fresh, old)

	got := readState(t, r)
	if len(got.Objectives) != 1 || got.Objectives[0].ID != fresh {
		t.Fatalf("only the live objective is reported: %+v", got.Objectives)
	}
	if len(got.Excluded) != 1 || got.Excluded[0].ID != old || got.Excluded[0].Reason != "superseded" {
		t.Fatalf("the one left out has to be named with its reason: %+v", got.Excluded)
	}
	if got := readState(t, r, "--id", old); len(got.Excluded) != 1 {
		t.Fatalf("asking for it by id has to find it: %+v", got)
	}

	res := runBare(t, r, "state", "--id", "01KZ34725V37YX845RTVPC7N0E")
	if res.code == 0 {
		t.Fatal("an id that is in no log must be refused rather than answered emptily")
	}
	if code, _ := errorCode(t, res); code != "unknown_objective" {
		t.Errorf("expected an unknown_objective refusal, got %q", code)
	}
}

// ------------------------------------------------------------ a malformed tree ---

// The write path is where the tree is enforced; the read path stays tolerant,
// which is the discipline that keeps a record on disk from ever being condemned
// by a rule that arrived after it. So a log holding both shapes `link` refuses
// has to be read, reported and survived — not hung on, not panicked over, and
// not turned into a validation failure.
func TestAMalformedTreeIsReportedAndNeitherHangsNorFails(t *testing.T) {
	r := throwawayRepo(t)
	a := addObjective(t, r, "A")
	b := addObjective(t, r, "B")
	c := addObjective(t, r, "C")
	twoParents := addObjective(t, r, "The objective with two parents")

	// Hand-written, because `link` is exactly what will not write them.
	parentEdge := func(from, to string) string {
		return `{"version":2,"kind":"edge","type":"derives_from","from":"` + from +
			`","to":"` + to + `","at":"2026-08-03T10:00:00Z"}`
	}
	appendLine(t, r.log, parentEdge(a, b))
	appendLine(t, r.log, parentEdge(b, c))
	appendLine(t, r.log, parentEdge(c, a))
	appendLine(t, r.log, parentEdge(twoParents, a))
	appendLine(t, r.log, parentEdge(twoParents, b))

	res := runWithin(t, r, 30*time.Second, "state")
	if res.code != 0 {
		t.Fatalf("state over a malformed tree still answers: exit %d\n%s%s", res.code, res.stdout, res.stderr)
	}
	var out stateOutput
	if err := json.Unmarshal([]byte(res.stdout), &out); err != nil {
		t.Fatalf("state printed something that is not JSON: %v\n%s", err, res.stdout)
	}
	if len(out.Objectives) != 4 {
		t.Fatalf("every objective is still reported, got %d", len(out.Objectives))
	}

	var sawCycle, sawTwoParents bool
	for _, an := range out.Anomalies {
		if strings.Contains(an.Message, "cycle") {
			sawCycle = true
		}
		if strings.Contains(an.Message, "two") {
			sawTwoParents = true
		}
	}
	if !sawCycle || !sawTwoParents {
		t.Fatalf("both anomalies have to be reported rather than swallowed: %+v", out.Anomalies)
	}
	// Deterministic, and stated: the first parent edge in the file wins.
	for _, o := range out.Objectives {
		if o.ID == twoParents && o.Parent != a {
			t.Errorf("the first parent edge in the file is the one used, got %q", o.Parent)
		}
	}

	// And the sweep is unmoved. Criterion 10 asks the read path to survive a
	// malformed tree, not to start condemning one — teaching `validate` about it
	// is a separate decision nobody has taken.
	out2, code := validateOne(t, r)
	if !out2.OK || code != 0 {
		t.Fatalf("validate must still produce its usual report: ok=%v exit=%d %+v",
			out2.OK, code, out2.Reports[0].Problems)
	}
}

// runWithin is runBare with a deadline, for the one test whose failure mode is
// not a wrong answer but no answer at all.
func runWithin(t *testing.T, r repo, d time.Duration, args ...string) result {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), d)
	defer cancel()

	cmd := exec.CommandContext(ctx, memoryBin, afterSubcommand(t, args, "--dir", r.root)...)
	cmd.Env = append(os.Environ(), "HOME="+r.home, "AI_KIT_AUTHOR=test-suite", "AI_KIT_PLAN=")
	cmd.Dir = r.root
	var stdout, stderr strings.Builder
	cmd.Stdout, cmd.Stderr = &stdout, &stderr

	err := cmd.Run()
	if ctx.Err() != nil {
		t.Fatalf("%v did not terminate within %s; the derivation walked a cycle forever", args, d)
	}
	res := result{stdout: stdout.String(), stderr: stderr.String()}
	if ee, ok := err.(*exec.ExitError); ok {
		res.code = ee.ExitCode()
	} else if err != nil {
		t.Fatalf("running %v: %v", args, err)
	}
	return res
}
