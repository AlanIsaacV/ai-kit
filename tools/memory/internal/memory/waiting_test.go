package memory

import (
	"strings"
	"testing"
)

type heldLog struct {
	name  string
	root  string
	lines []Line
}

func waitingOver(logs ...heldLog) []LogState {
	var store []Line
	for _, l := range logs {
		store = append(store, l.lines...)
	}
	out := make([]LogState, 0, len(logs))
	for _, l := range logs {
		tree := NewTreeIn(l.lines, store)
		out = append(out, LogState{Name: l.name, Root: l.root, Tree: tree, Report: tree.Report()})
	}
	return out
}

func section(out, heading string) string {
	var body []string
	in := false
	for _, line := range strings.Split(out, "\n") {
		switch {
		case line == "## "+heading:
			in = true
		case in && strings.HasPrefix(line, "## "):
			return strings.Join(body, "\n")
		case in:
			body = append(body, line)
		}
	}
	return strings.Join(body, "\n")
}

func TestAnAnsweredIdeaLeavesTheIdeasCount(t *testing.T) {
	kept := entry(t, KindIdea, "A digest of pending items every morning")
	dropped := entry(t, KindIdea, "Colour the blocked group red")
	drop := entry(t, KindDecision, "Colour adds nothing a heading does not")

	out := RenderWaiting(waitingOver(
		heldLog{name: "user", lines: logOf([]*Entry{kept, dropped}, nil)},
		heldLog{name: "api", root: "/src/api", lines: logOf([]*Entry{drop}, []*Edge{newTestEdge(t, EdgeAnswers, drop.ID, dropped.ID)})},
	), nil)

	if !strings.Contains(out, "\nIdeas: user 1\n") {
		t.Errorf("the unanswered idea has to be counted under its log, and the answered one not at all:\n%s", out)
	}
	if strings.Contains(out, kept.ID) || strings.Contains(out, dropped.ID) {
		t.Errorf("ideas are counted, not listed:\n%s", out)
	}
}

func TestCriterion3AParentWithOpenChildrenIsNeverApproveAndTheRootCountsWhatIsOpenBelow(t *testing.T) {
	root := objective(t, "The root of the whole tree")
	middle := objective(t, "The intermediate parent")
	leaf := objective(t, "The leaf cleared to start")
	decision := entry(t, KindDecision, "Start the leaf")

	out := RenderWaiting(waitingOver(heldLog{name: "api", lines: logOf(
		[]*Entry{root, middle, leaf, decision},
		[]*Edge{
			derivesFrom(middle, root),
			derivesFrom(leaf, middle),
			newTestEdge(t, EdgeApprovedForExecution, leaf.ID, decision.ID),
		})}), nil)

	if strings.Contains(out, "approve") {
		t.Errorf("no objective here is waiting for approval; both planned ones are parents with open children:\n%s", out)
	}
	if strings.Contains(out, middle.ID) || strings.Contains(out, middle.Title) {
		t.Errorf("an intermediate parent with open children has no line of its own:\n%s", out)
	}
	body := section(out, "api — "+root.Title)
	if !strings.Contains(body, "in progress: 2 open below `"+root.ID+"`") {
		t.Errorf("the root carries the count of its open descendants:\n%s", out)
	}
	if !strings.Contains(body, "- "+leaf.Title+" `"+leaf.ID+"`\n  - start") {
		t.Errorf("the leaf is listed under its root with its action:\n%s", out)
	}
}

func TestCriterion4TheHeaderCountsAnObjectiveTwoLogsReportOnce(t *testing.T) {
	parent := objective(t, "The parent in the first repository")
	kid := objective(t, "The child in the second repository")

	first := logOf([]*Entry{parent}, nil)
	second := logOf([]*Entry{parent, kid}, []*Edge{derivesFrom(kid, parent)})
	logs := waitingOver(
		heldLog{name: "alpha", root: "/src/alpha", lines: first},
		heldLog{name: "beta", root: "/src/beta", lines: second},
	)
	reported := 0
	for _, l := range logs {
		for _, o := range l.Report.Objectives {
			if o.ID == parent.ID {
				reported++
			}
		}
	}
	if reported != 2 {
		t.Fatalf("the fixture has to make both logs report the parent, got %d", reported)
	}

	out := RenderWaiting(logs, nil)

	if !strings.HasPrefix(out, "What is waiting on you — 2 open objectives across 2 logs\n") {
		t.Errorf("two distinct open objectives are listed, so the header counts two:\n%s", out)
	}
	if n := strings.Count(out, parent.ID); n != 1 {
		t.Errorf("the parent is listed once, got %d times:\n%s", n, out)
	}
}

func TestCriterion5EachQuestionCountsInTheLogThatHoldsIt(t *testing.T) {
	open := entry(t, KindOpenQuestion, "Which store the memory consolidates into")
	answered := entry(t, KindOpenQuestion, "Whether a depth bound of four is enough")
	answer := entry(t, KindDecision, "Four is enough")
	assumed := entry(t, KindAssumption, "Amounts are never negative here")

	rate := ClosingRateOf(waitingOver(
		heldLog{name: "alpha", lines: logOf([]*Entry{open}, nil)},
		heldLog{name: "beta", lines: logOf([]*Entry{answered, answer, assumed},
			[]*Edge{newTestEdge(t, EdgeAnswers, answer.ID, answered.ID)})},
	))

	byLog := map[string]RateRow{}
	for _, row := range rate.ByLog {
		byLog[row.Log] = row
	}
	if got := byLog["alpha"]; got.Closed != 0 || got.Total != 1 {
		t.Errorf("alpha holds one open question, got %+v", got)
	}
	if got := byLog["beta"]; got.Closed != 1 || got.Total != 2 {
		t.Errorf("beta holds an answered question and an open assumption, got %+v", got)
	}
	byKind := map[Kind]RateRow{}
	for _, row := range rate.ByKind {
		byKind[row.Kind] = row
	}
	if got := byKind[KindOpenQuestion]; got.Closed != 1 || got.Total != 2 {
		t.Errorf("two open-questions across the logs, one answered, got %+v", got)
	}
}

func TestCriterion6AnAdjustmentAppearsOnceUnderTheRootOfWhatItDerivesFrom(t *testing.T) {
	root := objective(t, "The root of the tree")
	leaf := objective(t, "The leaf the adjustment came out of")
	attached := entry(t, KindAdjustment, "Name the repository on every line")
	loose := entry(t, KindAdjustment, "Shorten the header")

	out := RenderWaiting(waitingOver(heldLog{name: "api", lines: logOf(
		[]*Entry{root, leaf, attached, loose},
		[]*Edge{
			derivesFrom(leaf, root),
			newTestEdge(t, EdgeDerivesFrom, attached.ID, leaf.ID),
		})}), nil)

	for _, a := range []*Entry{attached, loose} {
		if n := strings.Count(out, a.ID); n != 1 {
			t.Errorf("%q has to appear exactly once, got %d:\n%s", a.Title, n, out)
		}
	}
	if !strings.Contains(section(out, "api — "+root.Title), attached.ID) {
		t.Errorf("the adjustment deriving from the leaf belongs under the leaf's root:\n%s", out)
	}
	if !strings.Contains(section(out, headingLoose), "- api: "+loose.Title+" `"+loose.ID+"`") {
		t.Errorf("the adjustment deriving from nothing belongs in the trailing section, by repo:\n%s", out)
	}
}

func TestCriterion7ARootInAnotherRepositoryGroupsTheSliceHeldHere(t *testing.T) {
	root := objective(t, "The root held in the platform repository")
	slice := objective(t, "The slice held in the api repository")

	logs := waitingOver(
		heldLog{name: "platform", lines: logOf([]*Entry{root}, nil)},
		heldLog{name: "api", lines: logOf([]*Entry{slice}, []*Edge{derivesFrom(slice, root)})},
	)
	out := RenderWaiting(logs, nil)

	body := section(out, "platform — "+root.Title)
	if !strings.Contains(body, "- "+slice.Title+" (api) `"+slice.ID+"`\n  - approve") {
		t.Errorf("the slice held in api is listed under the root held in platform, naming its repository:\n%s", out)
	}
	if strings.Contains(out, "## api — ") {
		t.Errorf("the root decides the block, not the repository holding the slice:\n%s", out)
	}

	alone := RenderWaiting(logs[1:2], nil)
	if !strings.Contains(alone, slice.ID) {
		t.Errorf("reading the api log alone still lists its slice:\n%s", alone)
	}
}

func TestReadingOneLogNamesTheRepositoryThatHoldsTheRoot(t *testing.T) {
	root := objective(t, "The root held in the platform repository")
	root.Origin = &Origin{Repo: "platform"}
	slice := objective(t, "The slice held in the api repository")
	unstamped := objective(t, "A root written before records named their repository")
	orphan := objective(t, "The slice under the unstamped root")

	logs := waitingOver(
		heldLog{name: "platform", root: "/src/platform", lines: logOf([]*Entry{root, unstamped}, nil)},
		heldLog{name: "api", root: "/src/api", lines: logOf([]*Entry{slice, orphan},
			[]*Edge{derivesFrom(slice, root), derivesFrom(orphan, unstamped)})},
	)
	out := RenderWaiting(logs[1:2], nil)

	if !strings.Contains(section(out, "platform — "+root.Title), slice.ID) {
		t.Errorf("reading api alone, the root held in platform is headed by platform:\n%s", out)
	}
	if !strings.Contains(section(out, unstamped.Title), orphan.ID) {
		t.Errorf("a root whose repository nothing names is headed by its title alone:\n%s", out)
	}
	if strings.Contains(out, "## api — ") {
		t.Errorf("no root here is held in api, so no heading names api:\n%s", out)
	}
}

func TestCriterion7AnObjectiveWithNoParentIsItsOwnBlock(t *testing.T) {
	alone := objective(t, "The standalone objective")

	out := RenderWaiting(waitingOver(heldLog{name: "api", lines: logOf([]*Entry{alone}, nil)}), nil)

	if !strings.Contains(section(out, "api — "+alone.Title), "approve `"+alone.ID+"`") {
		t.Errorf("an objective with no parent is its own block, with its own line:\n%s", out)
	}
}

func TestCriterion7NothingOpenSaysSoAndKeepsTheClosingRate(t *testing.T) {
	done := objective(t, "The objective already met")
	decision := entry(t, KindDecision, "It is met")

	out := RenderWaiting(waitingOver(heldLog{name: "api", lines: logOf(
		[]*Entry{done, decision},
		[]*Edge{newTestEdge(t, EdgeMet, done.ID, decision.ID)})}), nil)

	want := "What is waiting on you — 0 open objectives across 1 log\n\nNothing is open.\n\n## " + headingRate + "\n\n"
	if !strings.HasPrefix(out, want) {
		t.Errorf("an empty listing is the header, \"Nothing is open.\" and the closing rate:\n%s", out)
	}
	var rows []string
	for _, line := range strings.Split(section(out, headingRate), "\n") {
		if line != "" {
			rows = append(rows, line)
		}
	}
	if len(rows) != 3 {
		t.Errorf("the closing rate is three lines, one per kind, got %q", rows)
	}
}
