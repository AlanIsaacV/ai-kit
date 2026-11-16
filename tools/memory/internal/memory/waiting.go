package memory

import (
	"fmt"
	"sort"
	"strings"
)

type LogState struct {
	Name   string
	Root   string
	Tree   *Tree
	Report StateReport
}

type Idea struct {
	Root  string `json:"root"`
	Repo  string `json:"repo,omitempty"`
	ID    string `json:"id"`
	Title string `json:"title"`
}

func WaitingIdeas(logs []LogState) []Idea {
	return waitingEntries(logs, (*Tree).OpenIdeas)
}

func WaitingAdjustments(logs []LogState) []Idea {
	return waitingEntries(logs, (*Tree).OpenAdjustments)
}

func waitingEntries(logs []LogState, open func(*Tree) []*Entry) []Idea {
	out := []Idea{}
	seen := map[string]bool{}
	for _, log := range logs {
		if log.Tree == nil {
			continue
		}
		for _, e := range open(log.Tree) {
			if seen[e.ID] {
				continue
			}
			seen[e.ID] = true
			out = append(out, Idea{Root: log.Root, Repo: log.Name, ID: e.ID, Title: e.Title})
		}
	}
	return out
}

const (
	headingAnomaly = "Anomalies in the reading"
	headingLoose   = "Adjustments outside any objective"
	headingRate    = "Closing rate"
	actionAdjust   = "adjustment pending"
)

type waitItem struct {
	repo       string
	title      string
	id         string
	action     string
	adjustment bool
}

type rootBlock struct {
	id    string
	repo  string
	title string
	wait  string
	below int
	items []waitItem
}

func RenderWaiting(logs []LogState, warnings []string) string {
	home := objectiveHomes(logs)
	blocks := map[string]*rootBlock{}
	listed := map[string]bool{}
	seen := map[string]bool{}
	open := 0
	var anomalies []waitItem

	blockOf := func(t *Tree, id string) *rootBlock {
		root := groupRoot(t, id)
		b, ok := blocks[root]
		if ok {
			return b
		}
		b = &rootBlock{id: root, title: root}
		e := t.Entry(root)
		if h, held := home[root]; held {
			b.repo = h
		} else {
			b.repo = entryHome(e)
		}
		if e != nil {
			b.title = e.Title
		}
		blocks[root] = b
		return b
	}

	for _, log := range logs {
		for _, o := range log.Report.Objectives {
			if seen[o.ID] {
				continue
			}
			seen[o.ID] = true
			if !o.State.Open() {
				continue
			}
			open++
			if o.State == StateSeparated || log.Tree == nil {
				continue
			}
			b := blockOf(log.Tree, o.ID)
			parent := len(o.BlockedByOpenChild) > 0
			action := objectiveAction(log.Tree, o)
			if b.id != o.ID {
				if parent {
					continue
				}
				b.items = append(b.items, waitItem{repo: log.Name, title: o.Title, id: o.ID, action: action})
				listed[o.ID] = true
				continue
			}
			listed[o.ID] = true
			if parent {
				b.below = openBelow(log.Tree, o.ID)
				if o.State != StateBlocked && o.State != StateVerified {
					continue
				}
			}
			b.wait = action
		}
		for _, a := range log.Report.Anomalies {
			anomalies = append(anomalies, waitItem{repo: log.Name, title: a.Message, id: a.ID})
		}
	}

	var loose []waitItem
	adjusted := map[string]bool{}
	for _, log := range logs {
		if log.Tree == nil {
			continue
		}
		for _, e := range log.Tree.OpenAdjustments() {
			if adjusted[e.ID] {
				continue
			}
			adjusted[e.ID] = true
			it := waitItem{repo: log.Name, title: e.Title, id: e.ID, action: actionAdjust, adjustment: true}
			if target := derivedObjective(log.Tree, e.ID); target != "" {
				b := blockOf(log.Tree, target)
				b.items = append(b.items, it)
				continue
			}
			loose = append(loose, it)
		}
	}

	ideas := map[string]int{}
	var ideaRepos []string
	for _, idea := range WaitingIdeas(logs) {
		if ideas[idea.Repo] == 0 {
			ideaRepos = append(ideaRepos, idea.Repo)
		}
		ideas[idea.Repo]++
	}
	sort.Strings(ideaRepos)

	var b strings.Builder
	fmt.Fprintf(&b, "What is waiting on you — %s across %s\n",
		count(len(listed), "open objective"), count(len(logs), "log"))
	for _, w := range warnings {
		fmt.Fprintf(&b, "\n! %s\n", w)
	}
	if len(blocks) == 0 && len(loose) == 0 && len(ideaRepos) == 0 && len(anomalies) == 0 {
		if open == 0 {
			b.WriteString("\nNothing is open.\n")
		} else {
			b.WriteString("\nNothing waits on you.\n")
		}
		b.WriteString(renderRate(logs))
		return b.String()
	}

	for _, rb := range sortedBlocks(blocks) {
		if rb.repo == "" {
			fmt.Fprintf(&b, "\n## %s\n", rb.title)
		} else {
			fmt.Fprintf(&b, "\n## %s — %s\n", rb.repo, rb.title)
		}
		if rb.wait != "" {
			fmt.Fprintf(&b, "\n%s `%s`\n", rb.wait, rb.id)
		}
		if rb.below > 0 {
			line := fmt.Sprintf("in progress: %d open below", rb.below)
			if rb.wait == "" {
				line += fmt.Sprintf(" `%s`", rb.id)
			}
			fmt.Fprintf(&b, "\n%s\n", line)
		}
		if len(rb.items) == 0 {
			continue
		}
		b.WriteString("\n")
		sortItems(rb.items)
		for _, it := range rb.items {
			title := it.title
			if it.repo != rb.repo {
				title = fmt.Sprintf("%s (%s)", title, it.repo)
			}
			fmt.Fprintf(&b, "- %s `%s`\n  - %s\n", title, it.id, it.action)
		}
	}

	if len(loose) > 0 {
		fmt.Fprintf(&b, "\n## %s\n\n", headingLoose)
		sortItems(loose)
		for _, it := range loose {
			fmt.Fprintf(&b, "- %s: %s `%s`\n", it.repo, it.title, it.id)
		}
	}

	if len(ideaRepos) > 0 {
		parts := make([]string, 0, len(ideaRepos))
		for _, repo := range ideaRepos {
			parts = append(parts, fmt.Sprintf("%s %d", repo, ideas[repo]))
		}
		fmt.Fprintf(&b, "\nIdeas: %s\n", strings.Join(parts, ", "))
	}

	if len(anomalies) > 0 {
		fmt.Fprintf(&b, "\n## %s\n\n", headingAnomaly)
		sortItems(anomalies)
		for _, it := range anomalies {
			fmt.Fprintf(&b, "- %s: %s `%s`\n", it.repo, it.title, it.id)
		}
	}

	b.WriteString(renderRate(logs))
	return b.String()
}

func objectiveHomes(logs []LogState) map[string]string {
	home := map[string]string{}
	for _, log := range logs {
		if log.Tree == nil {
			continue
		}
		for _, o := range log.Tree.Objectives() {
			if _, taken := home[o.ID]; !taken {
				home[o.ID] = log.Name
			}
		}
	}
	return home
}

func entryHome(e *Entry) string {
	if e == nil {
		return ""
	}
	if e.Scope == ScopeUser {
		return string(ScopeUser)
	}
	if e.Origin == nil {
		return ""
	}
	return strings.TrimSpace(e.Origin.Repo)
}

func groupRoot(t *Tree, id string) string {
	seen := map[string]bool{id: true}
	for {
		if t.State(id) == StateSeparated {
			return id
		}
		p, ok := t.parent[id]
		if !ok || seen[p] {
			return id
		}
		seen[p] = true
		id = p
	}
}

func openBelow(t *Tree, id string) int {
	seen := map[string]bool{id: true}
	n := 0
	stack := t.BlockedByOpenChild(id)
	for len(stack) > 0 {
		cur := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if seen[cur] {
			continue
		}
		seen[cur] = true
		n++
		stack = append(stack, t.BlockedByOpenChild(cur)...)
	}
	return n
}

func derivedObjective(t *Tree, id string) string {
	for _, e := range t.edges {
		if e.Type != EdgeDerivesFrom || e.From != id {
			continue
		}
		if target := t.allEntries[e.To]; target != nil && target.Kind == KindObjective {
			return e.To
		}
	}
	return ""
}

func objectiveAction(t *Tree, o ObjectiveState) string {
	switch o.State {
	case StatePlanned:
		return "approve"
	case StateApprovedForExecution:
		return "start"
	case StateBlocked:
		target, what := waitsOn(t, o.ID)
		if target != nil {
			return fmt.Sprintf("blocked: %s: %s", target.Kind, what)
		}
		return "blocked: " + what
	case StateVerified:
		_, what := waitsOn(t, o.ID)
		return "owe: " + what
	}
	return string(o.State)
}

func waitsOn(t *Tree, id string) (*Entry, string) {
	_, edge := t.StateEdge(id)
	if edge == nil {
		return nil, "no state edge says what it waits on"
	}
	if e := t.Entry(edge.To); e != nil {
		return e, e.Title
	}
	return nil, fmt.Sprintf("%s, a record this log does not hold", edge.To)
}

func sortedBlocks(blocks map[string]*rootBlock) []*rootBlock {
	out := make([]*rootBlock, 0, len(blocks))
	for _, b := range blocks {
		out = append(out, b)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].repo != out[j].repo {
			return out[i].repo < out[j].repo
		}
		if out[i].title != out[j].title {
			return out[i].title < out[j].title
		}
		return out[i].id < out[j].id
	})
	return out
}

func sortItems(items []waitItem) {
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].adjustment != items[j].adjustment {
			return !items[i].adjustment
		}
		if items[i].repo != items[j].repo {
			return items[i].repo < items[j].repo
		}
		return items[i].title < items[j].title
	})
}

type RateRow struct {
	Kind   Kind   `json:"kind,omitempty"`
	Log    string `json:"log,omitempty"`
	Closed int    `json:"closed"`
	Total  int    `json:"total"`
}

type ClosingRate struct {
	ByKind []RateRow `json:"by_kind"`
	ByLog  []RateRow `json:"by_log"`
}

func renderRate(logs []LogState) string {
	var b strings.Builder
	fmt.Fprintf(&b, "\n## %s\n\n", headingRate)
	for _, row := range ClosingRateOf(logs).ByKind {
		if row.Total == 0 {
			fmt.Fprintf(&b, "- %s: none written\n", row.Kind)
			continue
		}
		fmt.Fprintf(&b, "- %s: %d/%d, %.1f%%\n", row.Kind, row.Closed, row.Total, 100*share(row))
	}
	return b.String()
}

func ClosingRateOf(logs []LogState) ClosingRate {
	settled := map[string]bool{}
	for _, log := range logs {
		if log.Tree == nil {
			continue
		}
		for id := range log.Tree.settled {
			settled[id] = true
		}
	}

	kinds := []Kind{KindOpenQuestion, KindAssumption, KindAdjustment}
	perKind := map[Kind]*RateRow{}
	for _, k := range kinds {
		perKind[k] = &RateRow{Kind: k}
	}
	perLog := map[string]*RateRow{}
	var names []string
	counted := map[string]bool{}

	for _, log := range logs {
		if log.Tree == nil {
			continue
		}
		for _, e := range log.Tree.own {
			if perKind[e.Kind] == nil || counted[e.ID] {
				continue
			}
			counted[e.ID] = true
			row, known := perLog[log.Name]
			if !known {
				row = &RateRow{Log: log.Name}
				perLog[log.Name] = row
				names = append(names, log.Name)
			}
			row.Total++
			perKind[e.Kind].Total++
			if settled[e.ID] {
				row.Closed++
				perKind[e.Kind].Closed++
			}
		}
	}

	rate := ClosingRate{ByKind: []RateRow{}, ByLog: []RateRow{}}
	for _, k := range kinds {
		rate.ByKind = append(rate.ByKind, *perKind[k])
	}
	for _, name := range names {
		rate.ByLog = append(rate.ByLog, *perLog[name])
	}
	sort.Slice(rate.ByLog, func(i, j int) bool {
		if a, b := share(rate.ByLog[i]), share(rate.ByLog[j]); a != b {
			return a < b
		}
		return rate.ByLog[i].Log < rate.ByLog[j].Log
	})
	return rate
}

func share(r RateRow) float64 {
	if r.Total == 0 {
		return 0
	}
	return float64(r.Closed) / float64(r.Total)
}

func count(n int, noun string) string {
	if n == 1 {
		return fmt.Sprintf("%d %s", n, noun)
	}
	return fmt.Sprintf("%d %ss", n, noun)
}
