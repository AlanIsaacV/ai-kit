package memory

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"
)

var slugRE = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)

// Problem is one thing wrong with the log, always anchored to a line number so
// it can be found and fixed by hand.
type Problem struct {
	// Line is 0 when the problem comes from validating a record before it is
	// written, where there is no line to point at yet.
	Line    int    `json:"line,omitempty"`
	Field   string `json:"field,omitempty"`
	Message string `json:"message"`
	Raw     string `json:"raw,omitempty"`
}

func (p Problem) String() string {
	if p.Field != "" {
		return fmt.Sprintf("line %d: %s: %s", p.Line, p.Field, p.Message)
	}
	return fmt.Sprintf("line %d: %s", p.Line, p.Message)
}

// ValidateEntry checks one entry in isolation. This is the gate every write
// goes through, so garbage never reaches the log in the first place — the
// alternative is discovering three months later that half the records have an
// invented kind and an empty about, at which point the memory is worthless.
func ValidateEntry(e *Entry) []Problem {
	var ps []Problem
	add := func(field, msg string) { ps = append(ps, Problem{Field: field, Message: msg}) }

	if e.ID == "" {
		add("id", "required")
	} else if !ValidULID(e.ID) {
		add("id", fmt.Sprintf("%q is not a ULID (26 Crockford base32 chars)", e.ID))
	}

	if e.Kind == "" {
		add("kind", "required")
	} else if !entryKinds[e.Kind] {
		add("kind", fmt.Sprintf("%q is not a known kind; valid: %s", e.Kind, joinKinds(EntryKinds())))
	}

	if e.Slug == "" {
		add("slug", "required")
	} else if !slugRE.MatchString(e.Slug) {
		add("slug", fmt.Sprintf("%q must be kebab-case: lowercase letters, digits and hyphens", e.Slug))
	}

	if e.Scope == "" {
		add("scope", "required")
	} else if !scopes[e.Scope] {
		add("scope", fmt.Sprintf("%q is not a known scope; valid: plan, repo, user", e.Scope))
	}

	if e.At == "" {
		add("at", "required")
	} else if _, err := time.Parse(time.RFC3339, e.At); err != nil {
		add("at", fmt.Sprintf("%q is not RFC3339", e.At))
	}

	// `about` is what makes retrieval by area possible at all. An entry with
	// no `about` can only ever be found by a full scan, which is the failure
	// mode this whole store exists to avoid — so it is required, not optional.
	if len(e.About) == 0 {
		add("about", "at least one tag is required; without it the entry is unreachable by any filtered query")
	}
	for i, a := range e.About {
		if strings.TrimSpace(a) == "" {
			add(fmt.Sprintf("about[%d]", i), "must not be blank")
		}
	}

	if strings.TrimSpace(e.Title) == "" {
		add("title", "required")
	}

	return ps
}

// ValidateEdge checks one edge in isolation. Whether its endpoints exist is a
// separate, log-wide question — see ValidateLog.
func ValidateEdge(e *Edge) []Problem {
	var ps []Problem
	add := func(field, msg string) { ps = append(ps, Problem{Field: field, Message: msg}) }

	if e.Kind != KindEdge {
		add("kind", fmt.Sprintf("must be %q", KindEdge))
	}
	if e.Type == "" {
		add("type", "required")
	} else if !edgeTypes[e.Type] {
		add("type", fmt.Sprintf("%q is not a known edge type; valid: %s", e.Type, joinEdgeTypes(EdgeTypes())))
	}
	if e.From == "" {
		add("from", "required")
	} else if !ValidULID(e.From) {
		add("from", fmt.Sprintf("%q is not a ULID", e.From))
	}
	if e.To == "" {
		add("to", "required")
	} else if !ValidULID(e.To) {
		add("to", fmt.Sprintf("%q is not a ULID", e.To))
	}
	if e.From != "" && e.From == e.To {
		add("to", "an edge must not point at its own source")
	}
	if e.At == "" {
		add("at", "required")
	} else if _, err := time.Parse(time.RFC3339, e.At); err != nil {
		add("at", fmt.Sprintf("%q is not RFC3339", e.At))
	}

	return ps
}

// Report is the outcome of validating a whole log.
type Report struct {
	Path     string    `json:"path"`
	Entries  int       `json:"entries"`
	Edges    int       `json:"edges"`
	Problems []Problem `json:"problems"`
	OK       bool      `json:"ok"`
}

// ValidateLog checks every line plus the graph-wide invariants: unique ids and
// no dangling edges. A dangling edge is worth catching because it is silent —
// a graph walk simply returns less than it should, and nothing announces that
// the answer was incomplete.
func ValidateLog(path string, lines []Line) Report {
	rep := Report{Path: path, Problems: []Problem{}}

	ids := map[string]int{} // id -> first line it appeared on

	for _, l := range lines {
		switch {
		case l.Err != nil:
			rep.Problems = append(rep.Problems, Problem{Line: l.Num, Message: l.Err.Error(), Raw: truncate(l.Raw, 120)})
		case l.Entry != nil:
			rep.Entries++
			for _, p := range ValidateEntry(l.Entry) {
				p.Line = l.Num
				rep.Problems = append(rep.Problems, p)
			}
			if l.Entry.ID != "" {
				if prev, dup := ids[l.Entry.ID]; dup {
					rep.Problems = append(rep.Problems, Problem{
						Line: l.Num, Field: "id",
						Message: fmt.Sprintf("duplicate id %s, first seen on line %d", l.Entry.ID, prev),
					})
				} else {
					ids[l.Entry.ID] = l.Num
				}
			}
		case l.Edge != nil:
			rep.Edges++
			for _, p := range ValidateEdge(l.Edge) {
				p.Line = l.Num
				rep.Problems = append(rep.Problems, p)
			}
		}
	}

	// Endpoints are resolved after the whole file is read, so an edge may
	// legitimately be written before... nothing, actually: both endpoints must
	// already exist when the edge is written. But checking here rather than
	// per-line keeps the rule in one place and tolerates any write order.
	for _, l := range lines {
		if l.Edge == nil {
			continue
		}
		for field, id := range map[string]string{"from": l.Edge.From, "to": l.Edge.To} {
			if id == "" {
				continue
			}
			if _, ok := ids[id]; !ok {
				rep.Problems = append(rep.Problems, Problem{
					Line: l.Num, Field: field,
					Message: fmt.Sprintf("dangling edge: no entry with id %s in this log", id),
				})
			}
		}
	}

	sort.SliceStable(rep.Problems, func(i, j int) bool { return rep.Problems[i].Line < rep.Problems[j].Line })
	rep.OK = len(rep.Problems) == 0
	return rep
}

func joinKinds(ks []Kind) string {
	s := make([]string, len(ks))
	for i, k := range ks {
		s[i] = string(k)
	}
	return strings.Join(s, ", ")
}

func joinEdgeTypes(ts []EdgeType) string {
	s := make([]string, len(ts))
	for i, t := range ts {
		s[i] = string(t)
	}
	return strings.Join(s, ", ")
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
