package memory

import (
	"fmt"
	"sort"
	"strings"
)

type LogState struct {
	Name   string
	Tree   *Tree
	Report StateReport
}

const (
	headingPlanned  = "Waiting for your approval"
	headingBlocked  = "Blocked on a question or a decision"
	headingApproved = "Waiting for you to start it"
	headingAside    = "Set aside"
	headingAnomaly  = "Anomalies in the reading"
	quietWidth      = 76
)

type waitItem struct {
	repo string
	text string
	id   string
}

type waitGroup struct {
	key     string
	heading string
	items   []waitItem
}

type waitSection struct {
	heading string
	groups  []waitGroup
}

func RenderWaiting(logs []LogState, warnings []string) string {
	items := map[State][]waitItem{}
	blocked := map[string]*waitGroup{}
	named := map[string]bool{}
	var anomalies []waitItem
	total := 0

	for _, log := range logs {
		for _, o := range log.Report.Objectives {
			if !o.State.Open() {
				continue
			}
			total++
			named[log.Name] = true
			it := waitItem{repo: log.Name, text: o.Title, id: o.ID}
			if o.State != StateBlocked {
				items[o.State] = append(items[o.State], it)
				continue
			}
			key, heading := blockedOn(log.Tree, o.ID)
			g, ok := blocked[key]
			if !ok {
				g = &waitGroup{key: key, heading: heading}
				blocked[key] = g
			}
			g.items = append(g.items, it)
		}
		for _, a := range log.Report.Anomalies {
			named[log.Name] = true
			anomalies = append(anomalies, waitItem{repo: log.Name, text: a.Message, id: a.ID})
		}
	}

	var quiet []string
	for _, log := range logs {
		if !named[log.Name] {
			quiet = append(quiet, log.Name)
		}
	}
	sort.Strings(quiet)

	sections := []waitSection{
		{heading: headingPlanned, groups: oneGroup(items[StatePlanned])},
		{heading: headingBlocked, groups: blockedGroups(blocked)},
		{heading: headingApproved, groups: oneGroup(items[StateApprovedForExecution])},
		{heading: headingAside, groups: oneGroup(items[StateSeparated])},
		{heading: headingAnomaly, groups: oneGroup(anomalies)},
	}

	width := 0
	for _, s := range sections {
		for _, g := range s.groups {
			for _, it := range g.items {
				if len(it.repo) > width {
					width = len(it.repo)
				}
			}
		}
	}

	var b strings.Builder
	fmt.Fprintf(&b, "What is waiting on you — %s across %s\n",
		count(total, "open objective"), count(len(logs), "log"))
	for _, w := range warnings {
		fmt.Fprintf(&b, "! %s\n", w)
	}
	if total == 0 && len(anomalies) == 0 {
		b.WriteString("\nNothing is open.\n")
		return b.String()
	}

	for _, s := range sections {
		if len(s.groups) == 0 {
			continue
		}
		n := 0
		for _, g := range s.groups {
			n += len(g.items)
		}
		fmt.Fprintf(&b, "\n%s (%d)\n", s.heading, n)
		for _, g := range s.groups {
			indent := "  "
			if g.heading != "" {
				fmt.Fprintf(&b, "  %s\n", g.heading)
				indent = "    "
			}
			for _, it := range g.items {
				fmt.Fprintf(&b, "%s%-*s  %s  (%s)\n", indent, width, it.repo, it.text, it.id)
			}
		}
	}

	if len(quiet) > 0 {
		fmt.Fprintf(&b, "\nNothing waiting (%s)\n", count(len(quiet), "log"))
		for _, line := range wrapList(quiet, quietWidth) {
			fmt.Fprintf(&b, "  %s\n", line)
		}
	}
	return b.String()
}

func wrapList(names []string, width int) []string {
	var lines []string
	cur := ""
	for i, n := range names {
		piece := n
		if i < len(names)-1 {
			piece += ","
		}
		switch {
		case cur == "":
			cur = piece
		case len(cur)+1+len(piece) <= width:
			cur += " " + piece
		default:
			lines = append(lines, cur)
			cur = piece
		}
	}
	if cur != "" {
		lines = append(lines, cur)
	}
	return lines
}

func blockedOn(t *Tree, id string) (string, string) {
	_, edge := t.StateEdge(id)
	if edge == nil {
		return "", "no state edge says what it waits on"
	}
	if e := t.Entry(edge.To); e != nil {
		return edge.To, fmt.Sprintf("%s: %s", e.Kind, e.Title)
	}
	return edge.To, fmt.Sprintf("%s, a record this log does not hold", edge.To)
}

func oneGroup(items []waitItem) []waitGroup {
	if len(items) == 0 {
		return nil
	}
	sortItems(items)
	return []waitGroup{{items: items}}
}

func blockedGroups(byRecord map[string]*waitGroup) []waitGroup {
	out := make([]waitGroup, 0, len(byRecord))
	for _, g := range byRecord {
		sortItems(g.items)
		out = append(out, *g)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].heading != out[j].heading {
			return out[i].heading < out[j].heading
		}
		return out[i].key < out[j].key
	})
	return out
}

func sortItems(items []waitItem) {
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].repo != items[j].repo {
			return items[i].repo < items[j].repo
		}
		return items[i].text < items[j].text
	})
}

func count(n int, noun string) string {
	if n == 1 {
		return fmt.Sprintf("%d %s", n, noun)
	}
	return fmt.Sprintf("%d %ss", n, noun)
}
