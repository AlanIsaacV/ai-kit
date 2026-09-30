package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
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
		Root               string   `json:"root"`
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
		OpenRecords []struct {
			ID    string `json:"id"`
			Kind  string `json:"kind"`
			Title string `json:"title"`
		} `json:"open_records"`
	} `json:"objectives"`
	Excluded []struct {
		ID     string `json:"id"`
		Reason string `json:"reason"`
	} `json:"excluded"`
	Current struct {
		Root          string `json:"root"`
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

	before := len(lines(t, r, r.log))
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
	if after := len(lines(t, r, r.log)); after != before {
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

	before := len(lines(t, r, r.log))
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
	if after := len(lines(t, r, r.log)); after != before {
		t.Fatalf("a refused link wrote to the log: %d lines became %d", before, after)
	}

	// A self-edge is a cycle of length one, and it never reaches the tree rule:
	// the current version's rule set refuses an edge pointing at its own source
	// first. Both refusals are refusals, and nothing reaches disk either way.
	res = link(t, r, "derives_from", a, a)
	if res.code == 0 {
		t.Fatal("an objective cannot derive from itself")
	}
	if after := len(lines(t, r, r.log)); after != before {
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
	if n := strings.Count(strings.Join(lines(t, r, r.log), "\n"), `"type":"blocked"`); n != 1 {
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
	appendLine(t, r, r.log, stateEdge("met", "2026-08-03T12:00:00Z"))
	appendLine(t, r, r.log, stateEdge("cancelled", "2026-08-03T09:00:00Z"))
	if got := one(t, readState(t, r, "--id", objective)); got.Objectives[0].State != "met" {
		t.Fatalf("a later line with an older `at` must not win, got %q", got.Objectives[0].State)
	}

	// And now one sharing the winner's `at` exactly, which is what two edges
	// appended in the same second look like. Only the file order can separate
	// them, and the later line wins.
	appendLine(t, r, r.log, stateEdge("cancelled", "2026-08-03T12:00:00Z"))
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

	// Marking the only leaf met drops it, and the two parents it leaves with
	// nothing outstanding read met and drop with it.
	mustLink(t, r, "met", leaf, decision)
	got = readQuery(t, r, "query", "--plan", "demo", "--kind", "objective", "--current")
	if got.Count != 0 {
		t.Fatalf("the met leaf and the parents derived met from it should have gone: %v", ids(got))
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
	deviation := addKind(t, r, "deviation", "Something that came out of the first one")
	mustLink(t, r, "relates_to", deviation, touched)

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
	appendLine(t, r, r.log, objectiveLine(o1, "o1"))
	appendLine(t, r, r.log, objectiveLine(o2, "o2"))
	appendLine(t, r, r.log, edgeLine(o2, o1))
	appendLine(t, r, r.log, objectiveLine(o3, "o3"))

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
	deviation := addKind(t, r, "deviation", "Something recorded under the other plan's objective")
	mustLink(t, r, "relates_to", deviation, theirs)

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
	appendLine(t, r, r.log, parentEdge(a, b))
	appendLine(t, r, r.log, parentEdge(b, c))
	appendLine(t, r, r.log, parentEdge(c, a))
	appendLine(t, r, r.log, parentEdge(twoParents, a))
	appendLine(t, r, r.log, parentEdge(twoParents, b))

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

	cmd := exec.CommandContext(ctx, memoryBin,
		afterSubcommand(t, afterSubcommand(t, args, "--dir", r.root), "--store", r.store)...)
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

type spread struct {
	root  string
	home  string
	store string
}

func throwawaySpread(t *testing.T) spread {
	t.Helper()
	return spread{root: t.TempDir(), home: t.TempDir(), store: throwawayStore(t)}
}

func repoIn(t *testing.T, s spread, rel string) repo {
	t.Helper()
	root := filepath.Join(s.root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	return repo{
		root:  root,
		home:  s.home,
		log:   filepath.Join(root, ".ai-kit", "memory.jsonl"),
		user:  filepath.Join(s.home, ".ai-kit", "memory.jsonl"),
		store: s.store,
	}
}

func sorted(paths ...string) []string {
	out := append([]string{}, paths...)
	sort.Strings(out)
	return out
}

func twoDepths(t *testing.T) (spread, repo, repo) {
	t.Helper()
	s := throwawaySpread(t)
	return s, repoIn(t, s, "alpha"), repoIn(t, s, "group/beta")
}

func runSpread(t *testing.T, s spread, r repo, args ...string) result {
	t.Helper()
	return execMemory(t, r, "", args)
}

func stateIn(t *testing.T, s spread, r repo, args ...string) result {
	t.Helper()
	res := runSpread(t, s, r, append([]string{"state"}, args...)...)
	if res.code != 0 {
		t.Fatalf("state %v: exit %d\n%s%s", args, res.code, res.stdout, res.stderr)
	}
	return res
}

func readStateSpread(t *testing.T, s spread, r repo, args ...string) stateOutput {
	t.Helper()
	res := stateIn(t, s, r, args...)
	var out stateOutput
	if err := json.Unmarshal([]byte(res.stdout), &out); err != nil {
		t.Fatalf("state %v printed something that is not JSON: %v\n%s", args, err, res.stdout)
	}
	return out
}

type crossStateOutput struct {
	Logs       []string `json:"logs"`
	Count      int      `json:"count"`
	Objectives []struct {
		Repo     string   `json:"repo"`
		Root     string   `json:"root"`
		ID       string   `json:"id"`
		Title    string   `json:"title"`
		State    string   `json:"state"`
		Parent   string   `json:"parent"`
		Children []string `json:"children"`
	} `json:"objectives"`
	Excluded []struct {
		Repo string `json:"repo"`
		ID   string `json:"id"`
	} `json:"excluded"`
	Current []struct {
		Repo string `json:"repo"`
		Root string `json:"root"`
		ID   string `json:"id"`
		Rule string `json:"rule"`
	} `json:"current"`
	Anomalies []struct {
		Repo    string `json:"repo"`
		ID      string `json:"id"`
		Message string `json:"message"`
	} `json:"anomalies"`
}

func readCrossState(t *testing.T, s spread, r repo, args ...string) crossStateOutput {
	t.Helper()
	res := stateIn(t, s, r, args...)
	var out crossStateOutput
	if err := json.Unmarshal([]byte(res.stdout), &out); err != nil {
		t.Fatalf("state %v printed something that is not JSON: %v\n%s", args, err, res.stdout)
	}
	return out
}

func summaryLine(t *testing.T, summary, title string) string {
	t.Helper()
	for _, line := range strings.Split(summary, "\n") {
		if strings.Contains(line, title) {
			return line
		}
	}
	t.Fatalf("the summary names nothing called %q:\n%s", title, summary)
	return ""
}

func summaryGroup(t *testing.T, summary, heading string) string {
	t.Helper()
	var body []string
	found := false
	for _, line := range strings.Split(summary, "\n") {
		switch {
		case strings.HasPrefix(line, heading):
			found = true
		case !found:
		case line == "" || strings.HasPrefix(line, " "):
			body = append(body, line)
		default:
			return strings.Join(body, "\n")
		}
	}
	if !found {
		t.Fatalf("the summary has no %q group:\n%s", heading, summary)
	}
	return strings.Join(body, "\n")
}

func snapshot(t *testing.T, dir string) map[string]string {
	t.Helper()
	out := map[string]string{}
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		out[rel] = string(raw)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func sameBytes(t *testing.T, what string, before, after map[string]string) {
	t.Helper()
	for rel, was := range before {
		now, survived := after[rel]
		if !survived {
			t.Errorf("%s: a read removed %s", what, rel)
			continue
		}
		if now != was {
			t.Errorf("%s: %s is not byte-identical after a read\nbefore:\n%s\nafter:\n%s", what, rel, was, now)
		}
	}
	for rel := range after {
		if _, existed := before[rel]; !existed {
			t.Errorf("%s: a read created %s", what, rel)
		}
	}
}

func keyAnywhere(v any, want, at string) string {
	switch node := v.(type) {
	case map[string]any:
		for k, sub := range node {
			here := at + "." + k
			if k == want {
				return here
			}
			if found := keyAnywhere(sub, want, here); found != "" {
				return found
			}
		}
	case []any:
		for i, sub := range node {
			if found := keyAnywhere(sub, want, fmt.Sprintf("%s[%d]", at, i)); found != "" {
				return found
			}
		}
	}
	return ""
}

func TestCriterion2ALogAtEitherDepthAppearsInTheSummary(t *testing.T) {
	s := throwawaySpread(t)
	depths := []struct {
		rel   string
		name  string
		title string
	}{
		{"alpha", "alpha", "The objective in the repository directly under the root"},
		{"group/beta", "beta", "The objective in the repository one level further down"},
		{"org/group/gamma", "gamma", "The objective in the repository two levels further down"},
		{"org/group/sub/delta", "delta", "The objective in the repository at the depth bound"},
	}
	var standingIn repo
	for i, d := range depths {
		r := repoIn(t, s, d.rel)
		if i == 0 {
			standingIn = r
		}
		addObjective(t, r, d.title)
	}

	summary := stateIn(t, s, standingIn, "--text").stdout

	for _, d := range depths {
		if line := summaryLine(t, summary, d.title); !strings.Contains(line, d.name) {
			t.Errorf("<root>/%s is a log the store holds and its item must name %s, got %q",
				d.rel, d.name, line)
		}
	}
	if !strings.Contains(summary, "across 4 logs") {
		t.Errorf("every log found has to be counted as read, got:\n%s", summary)
	}
}

func TestCriterion3TheDefaultScopeFollowsTheOutputMode(t *testing.T) {
	s, here, elsewhere := twoDepths(t)
	const (
		mine   = "The objective in the repository I am standing in"
		theirs = "The objective in the other repository"
	)
	mineID := addObjective(t, here, mine)
	theirsID := addObjective(t, elsewhere, theirs)

	bare := readStateSpread(t, s, here)
	if len(bare.Objectives) != 1 || bare.Objectives[0].ID != mineID {
		t.Fatalf("`state` with no flags covers this repository and no other: %+v", bare.Objectives)
	}

	everyRoot := stateIn(t, s, here, "--text").stdout
	if !strings.Contains(everyRoot, "across 2 logs") {
		t.Errorf("`state --text` with no flags covers every root, got:\n%s", everyRoot)
	}
	summaryLine(t, everyRoot, mine)
	summaryLine(t, everyRoot, theirs)

	narrowed := stateIn(t, s, here, "--text", "--here").stdout
	if !strings.Contains(narrowed, "across 1 log") {
		t.Errorf("--here narrows the text mode to one log, got:\n%s", narrowed)
	}
	summaryLine(t, narrowed, mine)
	if strings.Contains(narrowed, theirs) {
		t.Errorf("--here must leave the other repository out, got:\n%s", narrowed)
	}

	widened := readCrossState(t, s, here, "--all")
	if strings.Join(widened.Logs, ",") != strings.Join(sorted(here.log, elsewhere.log), ",") {
		t.Fatalf("--all widens the JSON mode to every log the store holds, and names each of them, got %v",
			widened.Logs)
	}
	got := map[string]bool{}
	for _, o := range widened.Objectives {
		got[o.ID] = true
	}
	if !got[mineID] || !got[theirsID] {
		t.Errorf("--all must carry both repositories' objectives, got %+v", widened.Objectives)
	}
}

func TestCriterion4BareStateEmitsExactlyTheFieldsItEmitsToday(t *testing.T) {
	s, here, elsewhere := twoDepths(t)
	addObjective(t, elsewhere, "The objective in the other repository")

	parent := addObjective(t, here, "The parent")
	second := addObjective(t, here, "Another root, and a second parent")
	kid := child(t, here, parent, "The objective with two parents")
	appendLine(t, here, here.log, `{"version":2,"kind":"edge","type":"derives_from","from":"`+kid+
		`","to":"`+second+`","at":"2026-08-03T10:00:00Z"}`)
	gone := addObjective(t, here, "The first statement of it")
	fresh := addObjective(t, here, "A better statement of it")
	mustLink(t, here, "supersedes", fresh, gone)

	res := stateIn(t, s, here)
	var doc map[string]any
	if err := json.Unmarshal([]byte(res.stdout), &doc); err != nil {
		t.Fatalf("state printed something that is not JSON: %v\n%s", err, res.stdout)
	}

	keys := make([]string, 0, len(doc))
	for k := range doc {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	want := []string{"anomalies", "count", "current", "derived_over", "excluded", "objectives",
		"objectives_from", "path", "scope"}
	if strings.Join(keys, ",") != strings.Join(want, ",") {
		t.Fatalf("the single-repo document is %v, and it has to stay %v", keys, want)
	}

	for _, key := range []string{"objectives", "excluded", "anomalies"} {
		if list, ok := doc[key].([]any); !ok || len(list) == 0 {
			t.Fatalf("this fixture has to put something under %q or the check below proves nothing: %v",
				key, doc[key])
		}
	}
	if current, ok := doc["current"].(map[string]any); !ok || current["id"] == nil {
		t.Fatalf("this fixture has to resolve a current objective: %v", doc["current"])
	}
	if at := keyAnywhere(doc, "repo", "state"); at != "" {
		t.Errorf("the field added for the multi-root case must be absent from single-repo output, found it at %s", at)
	}
}

func TestCriterion5EveryItemNamesItsRepositoryAndNoTwoTreesMerge(t *testing.T) {
	s, alpha, beta := twoDepths(t)
	const (
		alphaRootTitle  = "The root objective of the first repository"
		alphaChildTitle = "The child objective of the first repository"
		betaRootTitle   = "The root objective of the second repository"
	)
	alphaRoot := addObjective(t, alpha, alphaRootTitle)
	alphaChild := child(t, alpha, alphaRoot, alphaChildTitle)
	betaRoot := addObjective(t, beta, betaRootTitle)
	mustLink(t, alpha, "approved_for_execution", alphaChild,
		addKind(t, alpha, "decision", "Work on the first repository's child next"))
	mustLink(t, beta, "approved_for_execution", betaRoot,
		addKind(t, beta, "decision", "Work on the second repository's root next"))

	cross := readCrossState(t, s, alpha, "--all")

	owner := map[string]string{}
	for _, o := range cross.Objectives {
		if o.Repo == "" {
			t.Errorf("every item names the repository it came from; %s (%q) names none", o.ID, o.Title)
		}
		owner[o.ID] = o.Repo
	}
	for id, repoName := range map[string]string{alphaRoot: "alpha", alphaChild: "alpha", betaRoot: "beta"} {
		if owner[id] != repoName {
			t.Errorf("%s belongs to %s, reported under %q", id, repoName, owner[id])
		}
	}
	for _, o := range cross.Objectives {
		if o.Parent != "" && owner[o.Parent] != o.Repo {
			t.Errorf("%s (%s) was given a parent out of %s; the two logs were merged into one tree",
				o.ID, o.Repo, owner[o.Parent])
		}
		for _, kid := range o.Children {
			if owner[kid] != o.Repo {
				t.Errorf("%s (%s) was given a child out of %s; the two logs were merged into one tree",
					o.ID, o.Repo, owner[kid])
			}
		}
	}

	if len(cross.Current) != len(cross.Logs) {
		t.Fatalf("each log keeps its own current objective: %d logs, %d answers", len(cross.Logs), len(cross.Current))
	}
	current := map[string]string{}
	for _, c := range cross.Current {
		if c.Repo == "" {
			t.Errorf("a current objective that names no repository answers nobody's question: %+v", c)
		}
		current[c.Repo] = c.ID
		if c.Rule != "approved_for_execution" {
			t.Errorf("%s resolved its current objective by %q, want the mark", c.Repo, c.Rule)
		}
	}
	if current["alpha"] != alphaChild || current["beta"] != betaRoot {
		t.Errorf("each repository keeps its own current objective, got %v", current)
	}

	summary := stateIn(t, s, alpha, "--text").stdout
	for title, repoName := range map[string]string{
		alphaRootTitle:  "alpha",
		alphaChildTitle: "alpha",
		betaRootTitle:   "beta",
	} {
		if line := summaryLine(t, summary, title); !strings.Contains(line, repoName) {
			t.Errorf("the item for %q has to name %s, got %q", title, repoName, line)
		}
	}
}

func TestCriterion6ABlockedObjectiveNamesWhatItWaitsOn(t *testing.T) {
	s, r, _ := twoDepths(t)
	const (
		waitingOn = "Which store the memory consolidates into"
		untouched = "Whether a depth bound of four stays enough"
		objective = "The objective that is blocked on one of them"
	)
	blocked := addObjective(t, r, objective)
	question := addKind(t, r, "open-question", waitingOn)
	addKind(t, r, "open-question", untouched)
	mustLink(t, r, "blocked", blocked, question)

	summary := stateIn(t, s, r, "--text").stdout
	group := summaryGroup(t, summary, "Blocked on a question or a decision")

	if !strings.Contains(group, "open-question: "+waitingOn) {
		t.Errorf("the blocked line has to name the question its edge points at:\n%s", group)
	}
	if !strings.Contains(group, objective) {
		t.Errorf("and the objective under it:\n%s", group)
	}
	if strings.Contains(summary, untouched) {
		t.Errorf("the other open question is pointed at by nothing and must not be named:\n%s", summary)
	}
}

func TestCriterion7EveryOpenStateAppearsUnderItsOwnGroup(t *testing.T) {
	s, r, _ := twoDepths(t)
	const (
		plannedTitle   = "The one nobody has approved yet"
		blockedTitle   = "The one held up by a question"
		approvedTitle  = "The one cleared to be started"
		separatedTitle = "The one lifted out and left"
	)
	decision := addKind(t, r, "decision", "The decision behind the states")
	question := addKind(t, r, "open-question", "What is holding it up")
	addObjective(t, r, plannedTitle)
	mustLink(t, r, "blocked", addObjective(t, r, blockedTitle), question)
	mustLink(t, r, "approved_for_execution", addObjective(t, r, approvedTitle), decision)
	mustLink(t, r, "separated", addObjective(t, r, separatedTitle), decision)

	summary := stateIn(t, s, r, "--text").stdout

	groups := []struct {
		heading string
		title   string
	}{
		{"Waiting for your approval", plannedTitle},
		{"Blocked on a question or a decision", blockedTitle},
		{"Waiting for you to start it", approvedTitle},
		{"Set aside", separatedTitle},
	}
	for _, g := range groups {
		body := summaryGroup(t, summary, g.heading)
		if !strings.Contains(body, g.title) {
			t.Errorf("%q belongs under %q:\n%s", g.title, g.heading, summary)
		}
		for _, other := range groups {
			if other.title != g.title && strings.Contains(body, other.title) {
				t.Errorf("%q is not in the %q state and must not be listed under it:\n%s",
					other.title, g.heading, body)
			}
		}
	}
}

func TestCriterion8NeitherModeWritesAnything(t *testing.T) {
	s, alpha, beta := twoDepths(t)
	addObjective(t, alpha, "The objective in the first repository")
	addObjective(t, beta, "The objective in the second repository")
	repoIn(t, s, "group/gamma")

	beforeRoot, beforeHome := snapshot(t, s.root), snapshot(t, s.home)

	modes := [][]string{{}, {"--text"}, {"--all"}, {"--text", "--here"}}
	for pass := 0; pass < 2; pass++ {
		for _, mode := range modes {
			stateIn(t, s, alpha, mode...)
		}
	}

	sameBytes(t, "under the root", beforeRoot, snapshot(t, s.root))
	sameBytes(t, "under HOME", beforeHome, snapshot(t, s.home))
}

func TestCriterion9ErrorsStayJSONInBothModes(t *testing.T) {
	s, r, _ := twoDepths(t)
	addObjective(t, r, "The only objective there is")
	const absent = "01KZ34725V37YX845RTVPC7N0E"

	for name, want := range map[string]struct {
		args []string
		code string
	}{
		"text over every root": {[]string{"state", "--text", "--id", absent}, "unknown_objective"},
		"text narrowed here":   {[]string{"state", "--text", "--here", "--id", absent}, "unknown_objective"},
		"json over every root": {[]string{"state", "--all", "--id", absent}, "unknown_objective"},
		"both scopes at once":  {[]string{"state", "--all", "--here"}, "conflicting_scope"},
	} {
		res := runSpread(t, s, r, want.args...)
		if res.code == 0 {
			t.Errorf("%s must be refused, got:\n%s", name, res.stdout)
			continue
		}
		code, msg := errorCode(t, res)
		if code != want.code {
			t.Errorf("%s: expected a %s refusal, got %q", name, want.code, code)
		}
		if msg == "" {
			t.Errorf("%s: the refusal has to say what went wrong", name)
		}
	}
}

func openRecordIDs(out stateOutput) []string {
	got := make([]string, 0, len(out.Objectives[0].OpenRecords))
	for _, r := range out.Objectives[0].OpenRecords {
		got = append(got, r.ID)
	}
	return got
}

func TestTheOpenRecordsOfAnObjectiveAreItsOwn(t *testing.T) {
	r := throwawayRepo(t)
	mine := addObjective(t, r, "The objective whose question this is")
	theirs := addObjective(t, r, "The other objective in the same plan")
	myQuestion := addKind(t, r, "open-question", "Whether the closing rate is read at all")
	theirQuestion := addKind(t, r, "open-question", "Whether the list is short enough to act on")
	mustLink(t, r, "derives_from", myQuestion, mine)
	mustLink(t, r, "derives_from", theirQuestion, theirs)

	got := openRecordIDs(one(t, readState(t, r, "--id", mine)))
	if !contains(got, myQuestion) {
		t.Errorf("the objective's own open question %s is missing from %v", myQuestion, got)
	}
	if contains(got, theirQuestion) {
		t.Errorf("%s belongs to the other objective and must not be listed here: %v", theirQuestion, got)
	}
}

func TestAnAnsweredAssumptionClosesLikeAnOpenQuestion(t *testing.T) {
	r := throwawayRepo(t)
	objective := addObjective(t, r, "The objective this turn settled")
	settled := addKind(t, r, "assumption", "The caller always passes a non-empty list")
	still := addKind(t, r, "assumption", "Amounts are never negative here")
	mustLink(t, r, "derives_from", settled, objective)
	mustLink(t, r, "derives_from", still, objective)
	mustLink(t, r, "answers", addKind(t, r, "decision", "The list may be empty and the caller checks"), settled)

	got := openRecordIDs(one(t, readState(t, r, "--id", objective)))
	if contains(got, settled) {
		t.Errorf("an answered assumption is closed and must not be listed as open: %v", got)
	}
	if !contains(got, still) {
		t.Errorf("the unanswered assumption %s is missing from %v", still, got)
	}

	res := runBare(t, r, "state", "--text", "--here")
	if res.code != 0 {
		t.Fatalf("state --text --here: exit %d\n%s%s", res.code, res.stdout, res.stderr)
	}
	line := summaryLine(t, summaryGroup(t, res.stdout, "Closing rate"), "assumption")
	if !strings.Contains(line, "1/2") {
		t.Errorf("the answered assumption has to be in the rate's numerator, got %q", line)
	}
}

func crossStateUnder(t *testing.T, r repo, home string) crossStateOutput {
	t.Helper()
	res := execMemory(t, r, "", afterSubcommand(t, []string{"state", "--all"}, "--dir", r.root), "HOME="+home)
	if res.code != 0 {
		t.Fatalf("state --all under HOME=%s: exit %d\n%s%s", home, res.code, res.stdout, res.stderr)
	}
	var out crossStateOutput
	if err := json.Unmarshal([]byte(res.stdout), &out); err != nil {
		t.Fatalf("state --all printed something that is not JSON: %v\n%s", err, res.stdout)
	}
	return out
}

func TestEachLogKeepsTheScopeItWasStoredUnderWhoeverIsReading(t *testing.T) {
	r := throwawayRepo(t)
	id := addObjective(t, r, "The objective that was promoted out of its repository")
	if res := runBare(t, r, "promote", "--ids", id, "--to", "user"); res.code != 0 {
		t.Fatalf("promote --ids %s --to user: exit %d\n%s%s", id, res.code, res.stdout, res.stderr)
	}

	mine := crossStateUnder(t, r, r.home)
	stranger := crossStateUnder(t, r, t.TempDir())

	if mine.Count != 2 {
		t.Fatalf("a record and its promoted copy are two records, held apart by scope: %+v", mine.Objectives)
	}
	if stranger.Count != mine.Count {
		t.Errorf("the scope of a log is stored with its rows, so reading from another HOME must not "+
			"collapse the pair: %d records there against %d here", stranger.Count, mine.Count)
	}
	if strings.Join(stranger.Logs, ",") != strings.Join(mine.Logs, ",") {
		t.Errorf("both readers see the same logs: %v against %v", stranger.Logs, mine.Logs)
	}
	if strings.Join(repoLabels(stranger), ",") != strings.Join(repoLabels(mine), ",") {
		t.Errorf("the repository a record came from is stored with its rows, so reading from another HOME "+
			"must not relabel it: %v against %v", repoLabels(stranger), repoLabels(mine))
	}
}

func repoLabels(out crossStateOutput) []string {
	labels := make([]string, 0, len(out.Objectives))
	for _, o := range out.Objectives {
		labels = append(labels, o.Repo)
	}
	return labels
}

func TestLinkJoinsRecordsWrittenIntoTwoRepositories(t *testing.T) {
	_, alpha, beta := twoDepths(t)
	parent := addObjective(t, alpha, "The objective the service next door derives from")
	kid := addObjective(t, beta, "The objective in the service next door")

	if res := link(t, beta, "derives_from", kid, parent); res.code != 0 {
		t.Fatalf("both endpoints are in the store, one log apart: exit %d\n%s%s",
			res.code, res.stdout, res.stderr)
	}

	landed := logText(t, beta, beta.log)
	if !strings.Contains(landed, `"from":"`+kid+`","to":"`+parent+`"`) {
		t.Fatalf("the edge belongs to the log link resolved:\n%s", landed)
	}
	if other := logText(t, alpha, alpha.log); strings.Contains(other, `"kind":"edge"`) {
		t.Errorf("the other repository's log was written to:\n%s", other)
	}
}

func TestLinkStillRefusesAnEndpointNoLogOfTheStoreHolds(t *testing.T) {
	_, alpha, beta := twoDepths(t)
	here := addObjective(t, alpha, "The objective that was actually written")
	addObjective(t, beta, "The objective in the other repository")

	const ghost = "01ARZ3NDEKTSV4RRFFQ69G5FAV"
	res := link(t, alpha, "relates_to", here, ghost)
	if res.code == 0 {
		t.Fatalf("an id no log holds is still a dangling edge: %s", res.stdout)
	}
	code, msg := errorCode(t, res)
	if code != "dangling_edge" {
		t.Fatalf("the refusal keeps the code it has always had, got %q: %s", code, msg)
	}
	if !strings.Contains(msg, ghost) {
		t.Errorf("the refusal has to name the id it could not find: %s", msg)
	}
}

func TestStateReadsATreeSpanningTwoRepositoriesFromEitherEnd(t *testing.T) {
	s, alpha, beta := twoDepths(t)
	parent := addObjective(t, alpha, "The objective the whole plan hangs from")
	kid := addObjective(t, beta, "The objective in the service next door")
	mustLink(t, beta, "derives_from", kid, parent)

	fromParent := one(t, readState(t, alpha, "--id", parent))
	if got := fromParent.Objectives[0].Children; len(got) != 1 || got[0] != kid {
		t.Errorf("read from the parent, the child in the other repository is a child, got %v", got)
	}
	fromChild := one(t, readState(t, beta, "--id", kid))
	if got := fromChild.Objectives[0].Parent; got != parent {
		t.Errorf("read from the child, the parent in the other repository is the parent, got %q", got)
	}
	if got := fromChild.Objectives[0].Depth; got != 1 {
		t.Errorf("the child stands one below a parent in another repository, got depth %d", got)
	}

	cross := readCrossState(t, s, alpha, "--all")
	for _, o := range cross.Objectives {
		switch o.ID {
		case parent:
			if len(o.Children) != 1 || o.Children[0] != kid {
				t.Errorf("the merged report reads the same tree, got children %v for the parent", o.Children)
			}
		case kid:
			if o.Parent != parent {
				t.Errorf("the merged report reads the same tree, got parent %q for the child", o.Parent)
			}
		}
	}
}

func TestStateAcrossLogsDeduplicatesAllFourOfItsLists(t *testing.T) {
	s, alpha, _ := twoDepths(t)
	root := addObjective(t, alpha, "The root of the tree")
	second := addObjective(t, alpha, "Another root, and a second parent")
	kid := child(t, alpha, root, "The objective with two parents")
	appendLine(t, alpha, alpha.log, `{"version":2,"kind":"edge","type":"derives_from","from":"`+kid+
		`","to":"`+second+`","at":"2026-08-03T10:00:00Z"}`)
	gone := addObjective(t, alpha, "The first statement of it")
	fresh := addObjective(t, alpha, "A better statement of it")
	mustLink(t, alpha, "supersedes", fresh, gone)

	twin := repoIn(t, s, "alpha-worktree")
	for _, raw := range lines(t, alpha, alpha.log) {
		appendLine(t, alpha, twin.log, raw)
	}

	cross := readCrossState(t, s, alpha, "--all")
	if len(cross.Logs) != 2 {
		t.Fatalf("the fixture is one repository's rows under two source paths: %v", cross.Logs)
	}
	times := map[string]int{}
	for _, o := range cross.Objectives {
		times[o.ID]++
	}
	for _, e := range cross.Excluded {
		times[e.ID]++
	}
	for _, id := range []string{root, second, kid, gone, fresh} {
		if times[id] != 1 {
			t.Errorf("%s is one objective under two source paths and is reported %d times", id, times[id])
		}
	}
	if len(cross.Anomalies) != 1 || cross.Anomalies[0].ID != kid {
		t.Fatalf("the two-parent anomaly is %s's and is reported once: %+v", kid, cross.Anomalies)
	}
	if len(cross.Current) != 1 {
		t.Fatalf("both logs resolve the same current objective, so the report names it once: %+v", cross.Current)
	}
	if times[cross.Current[0].ID] != 1 {
		t.Errorf("the current objective is one of the objectives the report already lists once: %+v",
			cross.Current[0])
	}
}

func TestEveryAnomalyIsReportedByTheLogHoldingTheRecordItIsAbout(t *testing.T) {
	s, alpha, beta := twoDepths(t)
	addObjective(t, alpha, "The root of the first repository")
	first := addObjective(t, beta, "The parent the file names first")
	second := addObjective(t, beta, "The parent of the second edge")
	third := addObjective(t, beta, "The parent of the third edge")
	kid := child(t, beta, first, "The objective with three parents")
	for _, parent := range []string{second, third} {
		appendLine(t, beta, beta.log, `{"version":2,"kind":"edge","type":"derives_from","from":"`+kid+
			`","to":"`+parent+`","at":"2026-08-03T10:00:00Z"}`)
	}

	if got := readState(t, alpha).Anomalies; len(got) != 0 {
		t.Errorf("alpha holds neither the record nor either edge, so it reports no anomaly: %+v", got)
	}

	inBeta := readState(t, beta).Anomalies
	if len(inBeta) != 2 {
		t.Fatalf("three parents are two ignored edges and two anomalies, got %d: %+v", len(inBeta), inBeta)
	}

	cross := readCrossState(t, s, alpha, "--all")
	if len(cross.Anomalies) != 2 {
		t.Fatalf("two anomalies about one objective are two anomalies under --all, got %d: %+v",
			len(cross.Anomalies), cross.Anomalies)
	}
	ignored := map[string]bool{}
	for _, a := range cross.Anomalies {
		if a.Repo != "beta" {
			t.Errorf("the anomaly belongs to the log holding the record, reported under %q: %+v", a.Repo, a)
		}
		if a.ID != kid {
			t.Errorf("both anomalies are about %s, got %s", kid, a.ID)
		}
		for _, parent := range []string{second, third} {
			if strings.Contains(a.Message, parent) {
				ignored[parent] = true
			}
		}
	}
	if !ignored[second] || !ignored[third] {
		t.Errorf("each ignored edge is named by an anomaly of its own: %+v", cross.Anomalies)
	}
}

func TestStateNamesTheWorkingTreeAnObjectiveIsRecordedInFromAnyRepository(t *testing.T) {
	s, alpha, beta := twoDepths(t)
	there := addObjective(t, alpha, "The objective recorded in the first repository")
	addObjective(t, beta, "The objective recorded in the second repository")
	roots := map[string]string{"alpha": alpha.root, "beta": beta.root}

	cross := readCrossState(t, s, beta, "--all", "--id", there)
	if len(cross.Objectives) != 1 || cross.Objectives[0].ID != there {
		t.Fatalf("state --all --id finds the objective in the other repository: %+v", cross.Objectives)
	}
	if got := cross.Objectives[0].Root; got != alpha.root {
		t.Errorf("read from beta, the objective names alpha's working tree: got %q, want %q", got, alpha.root)
	}
	if len(cross.Current) != 2 {
		t.Fatalf("each log keeps its own current objective: %+v", cross.Current)
	}
	for _, c := range cross.Current {
		if c.Root != roots[c.Repo] {
			t.Errorf("the current objective of %s names %q, want %q", c.Repo, c.Root, roots[c.Repo])
		}
	}

	for name, r := range map[string]repo{"alpha": alpha, "beta": beta} {
		single := one(t, readState(t, r))
		if got := single.Objectives[0].Root; got != r.root {
			t.Errorf("state over %s's log names its working tree on the objective: got %q, want %q", name, got, r.root)
		}
		if got := single.Current.Root; got != r.root {
			t.Errorf("state over %s's log names its working tree on the current objective: got %q, want %q",
				name, got, r.root)
		}
	}
}
