package memory

// PromotionPlan is what a promotion would write, computed before anything is
// appended so the caller can report it and so the operation stays testable
// without touching disk.
type PromotionPlan struct {
	Records        []any    `json:"-"`
	Promoted       []string `json:"promoted"`
	AlreadyPresent []string `json:"already_present"`
	Unknown        []string `json:"unknown"`
	EdgesCopied    int      `json:"edges_copied"`
}

// PlanPromotion works out which records move up a scope.
//
// Promotion is a copy with a changed scope and an untouched id. That is the
// entire payoff of using ULIDs: no reference anywhere has to be rewritten, and
// re-promoting the same record is a no-op rather than a duplicate. The
// alternative — ids that encode their level — would mean reading every body and
// patching the references inside it, where one miss breaks a link in silence.
//
// Edges come along when both of their endpoints are present at the target,
// otherwise the promoted records would arrive as orphans and the graph the
// user curated would be lost in the move.
func PlanPromotion(src []*Entry, srcEdges []*Edge, dst []*Entry, dstEdges []*Edge, ids []string, target Scope, fallbackPlan string) PromotionPlan {
	var p PromotionPlan

	byID := map[string]*Entry{}
	for _, e := range src {
		byID[e.ID] = e
	}
	present := map[string]bool{}
	for _, e := range dst {
		present[e.ID] = true
	}

	selected := map[string]bool{}
	for _, id := range ids {
		s, ok := byID[id]
		if !ok {
			p.Unknown = append(p.Unknown, id)
			continue
		}
		if present[id] {
			p.AlreadyPresent = append(p.AlreadyPresent, id)
			selected[id] = true // its edges may still be missing at the target
			continue
		}
		cp := *s
		cp.Scope = target
		if cp.Origin == nil {
			cp.Origin = &Origin{}
		} else {
			o := *cp.Origin
			cp.Origin = &o
		}
		if cp.Origin.Plan == "" {
			cp.Origin.Plan = fallbackPlan
		}
		p.Records = append(p.Records, &cp)
		p.Promoted = append(p.Promoted, id)
		selected[id] = true
	}

	have := map[string]bool{}
	for _, e := range dstEdges {
		have[edgeKey(e)] = true
	}
	for _, e := range srcEdges {
		if !(selected[e.From] || present[e.From]) || !(selected[e.To] || present[e.To]) {
			continue
		}
		if have[edgeKey(e)] {
			continue
		}
		cp := *e
		p.Records = append(p.Records, &cp)
		have[edgeKey(e)] = true
		p.EdgesCopied++
	}

	return p
}

func edgeKey(e *Edge) string { return string(e.Type) + "\x00" + e.From + "\x00" + e.To }
