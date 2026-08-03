package memory

import (
	"fmt"
	"sort"
	"strings"
)

// Render produces human-readable markdown for every entry of a kind.
//
// The sequential number (ADR-0003 and friends) is computed here, at read time,
// and never stored. That is deliberate: a stored number assumes a single
// writer and a single scope, so two people would both produce an "ADR-0001"
// and merging two repos would need a renumbering pass that rewrites content.
// Deriving it means aggregation is just concatenation.
func Render(entries []*Entry, edges []*Edge, kind Kind, scope Scope) string {
	var of []*Entry
	for _, e := range entries {
		if e.Kind == kind {
			of = append(of, e)
		}
	}
	// Chronological: a ULID sorts by its timestamp prefix.
	sort.SliceStable(of, func(i, j int) bool { return of[i].ID < of[j].ID })

	byID := map[string]*Entry{}
	for _, e := range entries {
		byID[e.ID] = e
	}
	superseded := SupersededIDs(edges)
	answered := AnsweredIDs(edges)

	label := strings.ToUpper(string(kind))

	var b strings.Builder
	fmt.Fprintf(&b, "# %s — scope: %s\n\n", label, scope)
	if len(of) == 0 {
		fmt.Fprintf(&b, "_No %s records at this scope yet._\n", kind)
		return b.String()
	}

	for i, e := range of {
		fmt.Fprintf(&b, "## %s-%04d — %s\n\n", label, i+1, e.Title)
		fmt.Fprintf(&b, "- id: `%s`\n- slug: `%s`\n- date: %s\n", e.ID, e.Slug, e.At)
		if len(e.About) > 0 {
			fmt.Fprintf(&b, "- about: %s\n", strings.Join(e.About, ", "))
		}
		if e.Origin != nil {
			var parts []string
			if e.Origin.Repo != "" {
				parts = append(parts, "repo "+e.Origin.Repo)
			}
			if e.Origin.Plan != "" {
				parts = append(parts, "plan "+e.Origin.Plan)
			}
			if e.Origin.Author != "" {
				parts = append(parts, "by "+e.Origin.Author)
			}
			if len(parts) > 0 {
				fmt.Fprintf(&b, "- origin: %s\n", strings.Join(parts, ", "))
			}
		}
		switch {
		case superseded[e.ID]:
			b.WriteString("- **status: superseded**\n")
		case answered[e.ID]:
			b.WriteString("- **status: answered**\n")
		}

		var sources []string
		for _, edge := range edges {
			if edge.From == e.ID && edge.Type == EdgeDerivesFrom {
				if src, ok := byID[edge.To]; ok {
					sources = append(sources, fmt.Sprintf("%s (%s)", src.Title, src.Kind))
				} else {
					sources = append(sources, edge.To)
				}
			}
		}
		if len(sources) > 0 {
			fmt.Fprintf(&b, "- derives from: %s\n", strings.Join(sources, "; "))
		}

		if strings.TrimSpace(e.Body) != "" {
			fmt.Fprintf(&b, "\n%s\n", strings.TrimSpace(e.Body))
		}
		b.WriteString("\n")
	}
	return b.String()
}
