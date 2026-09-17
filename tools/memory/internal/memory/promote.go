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

	// Border is what is left half-connected at the destination once this run has
	// been planned: records an edge joins to something that is going to be over
	// there, and that is not. Naming them is the whole difference between a
	// promotion that left reasoning behind and one that says it did.
	//
	// It is deliberately not "what *this run* left behind". The test is
	// atTarget — selected by this run **or already present** — so a record left at
	// the border by an earlier promotion is reported again by every later one that
	// touches an edge into it, whatever that run was about. That is the honest
	// answer to "what is still half-connected over there", which is the question
	// worth asking of an append-only log nobody can tidy; the narrower one goes
	// stale the moment you stop reading the report. The cost is real and is not
	// hidden: the list grows as the user log fills.
	Border []string `json:"border"`
	// EdgesSkipped counts the same edges Border names, one per edge rather than
	// one per record: exactly one endpoint at the target and one not. An edge with
	// neither endpoint there is not counted — that number would be a measure of
	// the rest of the graph.
	EdgesSkipped int `json:"edges_skipped"`
}

// PlanPromotion works out which records move up a scope.
//
// Promotion is a copy with a changed scope and an untouched id. That is the
// entire payoff of using ULIDs: no reference anywhere has to be rewritten, and
// re-promoting the same record is a no-op rather than a duplicate. The
// alternative — ids that encode their level — would mean reading every body and
// patching the references inside it, where one miss breaks a link in silence.
//
// It starts from `ids`, from `kinds`, or from both, which combine as a union:
// naming a class and naming a record are two ways of asking for the same thing,
// and a caller that does both wants everything it named. Which kinds a caller
// may name is the CLI's question, not this function's — see promotableKindsV3.
//
// From there it reaches exactly **one hop**. A record nobody named but that an
// edge joins to one that was named travels too, because an `adr` that arrives
// without the `decision` that fed it arrives as an assertion. One hop and not
// the transitive closure: the closure is the mechanism that would lift a whole
// repository's scaffolding into a log meant to hold only what outlives it. The
// hop is measured from the named set and from nothing else — a record pulled in
// at one hop does not go on to pull in its own neighbours, whatever order the
// edges happen to sit in the log. Two kinds are never reached for at all; see
// notPulledByReach.
//
// Edges come along when both of their endpoints are present at the target,
// otherwise the promoted records would arrive as orphans and the graph the
// user curated would be lost in the move. The expansion does not weaken that
// rule: it adds records so that more edges qualify, never edges whose endpoints
// are absent. What is still half-connected once the expansion has run is
// reported — Border and EdgesSkipped — instead of being dropped in silence.
//
// There is no fallback plan slug any more. There used to be one, filled in from
// the source log's directory when a record's `origin.plan` was empty; since one
// repo has one log there is no directory to read it from, and `add` refuses to
// write a record that cannot name its plan. A promoted copy carries whatever the
// source carried, which for every record on disk is the plan that produced it.
func PlanPromotion(src []*Entry, srcEdges []*Edge, dst []*Entry, dstEdges []*Edge, ids []string, target Scope, kinds ...Kind) PromotionPlan {
	var p PromotionPlan

	byID := map[string]*Entry{}
	for _, e := range src {
		byID[e.ID] = e
	}
	present := map[string]bool{}
	for _, e := range dst {
		present[e.ID] = true
	}

	// The named set: the ids the caller listed, then every record of a kind it
	// asked for. Deduplicated, because the two selectors overlap the moment a
	// caller names a record and its class in one command, and a record reaching
	// the copy loop twice would be two identical lines in an append-only log.
	//
	// `seeded` means a record of the source that the caller named, and an id that
	// resolved to nothing is kept out of it rather than merely reported: an
	// unknown id sitting in that map would let the walk below reach one hop out
	// from a record that does not exist.
	var named []string
	seeded, missing := map[string]bool{}, map[string]bool{}
	for _, id := range ids {
		if seeded[id] || missing[id] {
			continue
		}
		if _, ok := byID[id]; !ok {
			missing[id] = true
			p.Unknown = append(p.Unknown, id)
			continue
		}
		seeded[id] = true
		named = append(named, id)
	}
	if len(kinds) > 0 {
		wanted := map[Kind]bool{}
		for _, k := range kinds {
			wanted[k] = true
		}
		for _, e := range src {
			if !wanted[e.Kind] || seeded[e.ID] {
				continue
			}
			seeded[e.ID] = true
			named = append(named, e.ID)
		}
	}

	// One hop out from the named set. `seeded` is read and never written here, so
	// what the walk reaches cannot depend on the order the edges were logged in:
	// a neighbour added mid-scan would be a second hop for every edge after it.
	//
	// A neighbour already at the target is reached like any other and reported as
	// already present, rather than passed over as needing no work. It is what
	// makes a second run legible: whatever the first run moved, the second one
	// names, and a promotion that says nothing about a record it carried is one
	// nobody can check twice.
	//
	// Two kinds are not reached for at all; see notPulledByReach. What that stops
	// from travelling still shows up as border, because the walk is the only thing
	// that consults the set — everything downstream reads the selection, so an
	// edge into a skipped record has exactly one endpoint at the target and is
	// counted and named like any other.
	var reached []string
	pulled := map[string]bool{}
	for _, e := range srcEdges {
		for _, hop := range [2][2]string{{e.From, e.To}, {e.To, e.From}} {
			from, to := hop[0], hop[1]
			if !seeded[from] || seeded[to] || pulled[to] {
				continue
			}
			n, ok := byID[to]
			if !ok {
				continue // a dangling endpoint: there is nothing to carry
			}
			if notPulledByReach[n.Kind] {
				continue
			}
			pulled[to] = true
			reached = append(reached, to)
		}
	}

	travelling := make([]string, 0, len(named)+len(reached))
	travelling = append(travelling, named...)
	travelling = append(travelling, reached...)

	selected := map[string]bool{}
	for _, id := range travelling {
		s := byID[id]
		if present[id] {
			p.AlreadyPresent = append(p.AlreadyPresent, id)
			selected[id] = true // its edges may still be missing at the target
			continue
		}
		cp := *s
		cp.Scope = target
		// The copy is judged by the rules its content was written against, not
		// by whatever is current at the moment it is promoted — the fields did
		// not change on the way up. Stamping the *resolved* version matters
		// because a source that predates the stamp reads as version 1, and
		// writing a literal 0 to the target log would put a version on disk
		// that never existed.
		cp.Version = s.SchemaVersion()
		// Origin and Anchors are deep-copied rather than shared, so the copy and
		// its source can never be two names for one value: nothing here mutates
		// either today, and a promotion that started to would otherwise reach back
		// into the log it was reading. A struct copy is shallow, so a pointer and
		// a slice both need saying.
		if cp.Origin != nil {
			o := *cp.Origin
			cp.Origin = &o
		}
		if cp.Anchors != nil {
			cp.Anchors = append([]Anchor(nil), cp.Anchors...)
		}
		p.Records = append(p.Records, &cp)
		p.Promoted = append(p.Promoted, id)
		selected[id] = true
	}

	atTarget := func(id string) bool { return selected[id] || present[id] }

	// Everything that ends up half-connected, counted and named before a single
	// edge is copied, so the two answers are read off the same selection. The
	// answer is about the destination and not about this run — see Border.
	onBorder := map[string]bool{}
	for _, e := range srcEdges {
		from, to := atTarget(e.From), atTarget(e.To)
		if from == to {
			continue // both ends travel, or neither end was ever in this promotion
		}
		p.EdgesSkipped++
		left := e.To
		if !from {
			left = e.From
		}
		if !onBorder[left] {
			onBorder[left] = true
			p.Border = append(p.Border, left)
		}
	}

	have := map[string]bool{}
	for _, e := range dstEdges {
		have[edgeKey(e)] = true
	}
	for _, e := range srcEdges {
		if !atTarget(e.From) || !atTarget(e.To) {
			continue
		}
		if have[edgeKey(e)] {
			continue
		}
		cp := *e
		cp.Version = e.SchemaVersion()
		p.Records = append(p.Records, &cp)
		have[edgeKey(e)] = true
		p.EdgesCopied++
	}

	return p
}

// notPulledByReach is what the one-hop expansion will not carry, however
// directly it is connected to something that travelled. Both members are here
// because the reach was otherwise a back door into the user log for exactly what
// promotableKindsV3 keeps out of the front one.
//
//   - `objective` because an `adr` very often derives from the objective that
//     produced it, so reaching indiscriminately would carry a repository's own
//     scaffolding upward. An objective names work that happened in one
//     repository; the ADR it produced is what was meant to outlive it.
//   - `gotcha` because it cannot exist without an Anchor, and an anchor's
//     path:line is resolved against the repository the record's log lives in, or
//     against the checkout `origin.repo_id` names once it is in the user log — a
//     claim about one repository's code, which means nothing in the next one.
//     `--kind` refuses the class for that reason, and following an edge into one
//     would have carried it up regardless.
//
// This bounds the *reach* and not the *selection*: either kind named in `--ids`
// is promoted like anything else, because a caller typing an id has decided.
// Nothing is hidden by the bound — a record the walk skipped is still reported
// as Border and the edge to it still counted, because the walk is the only thing
// that consults this set.
//
// It is a rule about promotion and not about the schema, which is why it lives
// here rather than beside entryKindsV3 — no record is illegal for being of a
// kind in this set, and adding to it changes what one walk reaches and nothing
// else.
var notPulledByReach = map[Kind]bool{KindObjective: true, KindGotcha: true}

func edgeKey(e *Edge) string { return string(e.Type) + "\x00" + e.From + "\x00" + e.To }
