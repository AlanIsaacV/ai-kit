package memory

import (
	"testing"
	"time"
)

func derives(t *testing.T, from, to *Entry) *Edge {
	t.Helper()
	return newTestEdge(t, EdgeDerivesFrom, from.ID, to.ID)
}

// The id survives promotion untouched. That is the property that makes moving
// a record between levels free: nothing that points at it has to change.
//
// The source here is a record as the log actually holds one: stamped version 1,
// scoped `plan` — a value version 2 removed and version 1 still accepts. It
// lands at the user level with `scope: user`, still declaring version 1, and
// version 1's rules are what judge it there.
func TestPromotionKeepsIDsAndRewritesScope(t *testing.T) {
	adr := entry(t, KindADR, "Asymmetric signing", "auth")
	adr.Version = SchemaVersion1
	adr.Scope = ScopePlan
	adr.Origin = &Origin{Repo: "ai-kit", Plan: "demo"}

	p := PlanPromotion([]*Entry{adr}, nil, nil, nil, []string{adr.ID}, ScopeUser)

	if len(p.Records) != 1 {
		t.Fatalf("expected one record to move, got %d", len(p.Records))
	}
	moved := p.Records[0].(*Entry)
	if moved.ID != adr.ID {
		t.Fatalf("promotion changed the id: %s -> %s", adr.ID, moved.ID)
	}
	if moved.Scope != ScopeUser {
		t.Fatalf("scope should be the target, got %s", moved.Scope)
	}
	if moved.Version != SchemaVersion1 {
		t.Fatalf("promotion re-stamped the record: got version %d", moved.Version)
	}
	if ps, judged := CurrentSchema().JudgeEntry(moved); !judged || len(ps) != 0 {
		t.Fatalf("a promoted v1 record must still be valid v1 at the target: judged=%v %v", judged, ps)
	}
	if moved.Origin == nil || moved.Origin.Plan != "demo" {
		t.Fatal("the promoted copy should remember which plan produced it")
	}
	if moved.Origin == adr.Origin {
		t.Error("the copy shares its origin struct with the source: a later mutation would reach back into the log")
	}
	if adr.Scope != ScopePlan {
		t.Fatal("the source record must not be mutated")
	}
}

// Re-running a promotion is something a user will do by accident. It has to be
// a no-op, not a duplicate.
func TestPromotionIsIdempotent(t *testing.T) {
	adr := entry(t, KindADR, "Asymmetric signing", "auth")
	already := *adr
	already.Scope = ScopeUser

	p := PlanPromotion([]*Entry{adr}, nil, []*Entry{&already}, nil, []string{adr.ID}, ScopeUser)

	if len(p.Records) != 0 {
		t.Fatalf("nothing should be written twice, got %d records", len(p.Records))
	}
	if len(p.AlreadyPresent) != 1 || len(p.Promoted) != 0 {
		t.Fatalf("the report should say it was already there: %+v", p)
	}
}

// Promoted records must not arrive as orphans — the graph the user curated is
// most of the value.
func TestPromotionCarriesEdgesBetweenPromotedRecords(t *testing.T) {
	decision := entry(t, KindDecision, "Use RS256", "auth")
	adr := entry(t, KindADR, "Asymmetric signing", "auth")
	edges := []*Edge{derives(t, adr, decision)}

	p := PlanPromotion([]*Entry{decision, adr}, edges, nil, nil,
		[]string{adr.ID, decision.ID}, ScopeUser)

	if p.EdgesCopied != 1 {
		t.Fatalf("the edge between two promoted records should come along, got %d", p.EdgesCopied)
	}
	if len(p.Records) != 3 {
		t.Fatalf("expected two entries plus one edge, got %d", len(p.Records))
	}
}

// Promoting one end later should reconnect the pair, not leave the graph
// permanently broken because of the order the user happened to pick.
func TestPromotionReconnectsOnSecondPass(t *testing.T) {
	decision := entry(t, KindDecision, "Use RS256", "auth")
	adr := entry(t, KindADR, "Asymmetric signing", "auth")
	edges := []*Edge{derives(t, adr, decision)}

	atUser := *adr
	atUser.Scope = ScopeUser

	p := PlanPromotion([]*Entry{decision, adr}, edges, []*Entry{&atUser}, nil,
		[]string{decision.ID}, ScopeUser)

	if p.EdgesCopied != 1 {
		t.Fatalf("promoting the missing end should bring the edge, got %d", p.EdgesCopied)
	}
}

// Promotion copies a record into another log; it does not re-author it. The
// copy keeps the version its content was written against, because the fields
// did not change on the way up — promoting a record must not quietly claim it
// satisfies rules nobody checked it against.
func TestPromotionCarriesTheSourceVersion(t *testing.T) {
	old := entry(t, KindADR, "Asymmetric signing", "auth")
	old.Version = 0 // written before the stamp existed
	future := entry(t, KindADR, "Something later", "auth")
	future.Version = 99

	p := PlanPromotion([]*Entry{old, future}, nil, nil, nil,
		[]string{old.ID, future.ID}, ScopeUser)

	got := map[string]int{}
	for _, r := range p.Records {
		e := r.(*Entry)
		got[e.ID] = e.Version
	}
	// Resolved, not copied verbatim: a literal 0 on the target line would be a
	// version that never existed, and the source is not touched either way.
	if got[old.ID] != SchemaVersion1 {
		t.Errorf("an unstamped source should land stamped as v1, got %d", got[old.ID])
	}
	if got[future.ID] != 99 {
		t.Errorf("a version this binary cannot judge must survive promotion unchanged, got %d", got[future.ID])
	}
	if old.Version != 0 {
		t.Error("the source record must not be mutated")
	}
}

// Idempotency was proven for records and asserted for edges: the test above
// passes nil for both edge slices, so the dedup at `have[edgeKey(e)]` had never
// once been run by a test. Here both slices are real, and the destination
// already holds the pair *and* the edge between them — which is the state a
// first promotion leaves behind, and the state a second one meets.
func TestPromotionIsIdempotentForEdgesToo(t *testing.T) {
	decision := entry(t, KindDecision, "Use RS256", "auth")
	adr := entry(t, KindADR, "Asymmetric signing", "auth")
	edge := derives(t, adr, decision)

	atUser := []*Entry{}
	for _, e := range []*Entry{decision, adr} {
		cp := *e
		cp.Scope = ScopeUser
		atUser = append(atUser, &cp)
	}
	landed := *edge

	p := PlanPromotion([]*Entry{decision, adr}, []*Edge{edge}, atUser, []*Edge{&landed},
		[]string{adr.ID, decision.ID}, ScopeUser)

	if len(p.Records) != 0 {
		t.Fatalf("a second pass must append nothing at all, got %d records", len(p.Records))
	}
	if p.EdgesCopied != 0 {
		t.Errorf("the edge is already at the target: %d copied", p.EdgesCopied)
	}
	if len(p.AlreadyPresent) != 2 || len(p.Promoted) != 0 {
		t.Errorf("both records should read as already present: %+v", p)
	}
	if p.EdgesSkipped != 0 || len(p.Border) != 0 {
		t.Errorf("nothing was left behind, so nothing sits at the border: %+v", p)
	}
}

// An `adr` that arrives without the `decision` that fed it arrives as an
// assertion. One hop is what stops that, and it is one hop from the named set
// and no further.
func TestPromotionReachesOneHopAndStopsThere(t *testing.T) {
	adr := entry(t, KindADR, "Asymmetric signing", "auth")
	decision := entry(t, KindDecision, "Use RS256", "auth")
	deviation := entry(t, KindDeviation, "Rotated the key by hand", "auth")
	edges := []*Edge{derives(t, adr, decision), derives(t, decision, deviation)}

	p := PlanPromotion([]*Entry{adr, decision, deviation}, edges, nil, nil,
		[]string{adr.ID}, ScopeUser)

	if len(p.Promoted) != 2 {
		t.Fatalf("the adr and the decision it derives from should travel: %v", p.Promoted)
	}
	for _, id := range p.Promoted {
		if id == deviation.ID {
			t.Fatal("two hops is the transitive closure this refused to be")
		}
	}
	if p.EdgesCopied != 1 {
		t.Errorf("only the edge with both ends at the target may be copied, got %d", p.EdgesCopied)
	}
}

// The reach cannot depend on the order the edges happen to sit in the log. A
// walk that added neighbours as it scanned would take the chain in one direction
// and stop at one hop in the other.
func TestOneHopIsMeasuredFromTheNamedSetWhateverTheEdgeOrder(t *testing.T) {
	a := entry(t, KindADR, "Asymmetric signing", "auth")
	b := entry(t, KindDecision, "Use RS256", "auth")
	c := entry(t, KindDeviation, "Rotated the key by hand", "auth")
	forward := []*Edge{derives(t, a, b), derives(t, b, c)}
	backward := []*Edge{derives(t, b, c), derives(t, a, b)}

	for name, edges := range map[string][]*Edge{"forward": forward, "backward": backward} {
		p := PlanPromotion([]*Entry{a, b, c}, edges, nil, nil, []string{a.ID}, ScopeUser)
		if len(p.Promoted) != 2 {
			t.Errorf("%s: expected exactly the named record and its neighbour, got %v", name, p.Promoted)
		}
	}
}

// The border is the whole point of criterion 5: a promotion that left reasoning
// behind must not print success in silence.
func TestPromotionNamesTheBorderAndCountsSkippedEdges(t *testing.T) {
	adr := entry(t, KindADR, "Asymmetric signing", "auth")
	decision := entry(t, KindDecision, "Use RS256", "auth")
	deviation := entry(t, KindDeviation, "Rotated the key by hand", "auth")
	edges := []*Edge{derives(t, adr, decision), derives(t, decision, deviation)}

	p := PlanPromotion([]*Entry{adr, decision, deviation}, edges, nil, nil,
		[]string{adr.ID}, ScopeUser)

	if len(p.Border) != 1 || p.Border[0] != deviation.ID {
		t.Fatalf("the record two hops out is what sat at the border: %v", p.Border)
	}
	if p.EdgesSkipped != 1 {
		t.Errorf("the edge into it could not be copied and has to be counted, got %d", p.EdgesSkipped)
	}
	// The rule the expansion must not weaken: more records qualify, never an edge
	// whose endpoint is absent. An edge whose other end stayed behind would be
	// dangling at the target, and a dangling edge fails silently — a graph walk
	// there would simply return less than it should, with nothing saying so.
	if p.EdgesCopied != 1 {
		t.Errorf("only the edge between two records at the target travels, got %d", p.EdgesCopied)
	}
}

// The reach carries the reasoning and stops at the scaffolding. An `adr` very
// often derives from the objective that produced it, and following that edge
// would lift a repository's own work planning into a log meant to outlive it.
//
// What it must not do is disappear: an objective left behind is border like
// anything else, and the edge to it is counted.
func TestOneHopDoesNotReachForAnObjectiveButStillNamesIt(t *testing.T) {
	objective := entry(t, KindObjective, "Ship the token service", "auth")
	decision := entry(t, KindDecision, "Use RS256", "auth")
	adr := entry(t, KindADR, "Asymmetric signing", "auth")
	edges := []*Edge{derives(t, adr, decision), derives(t, adr, objective)}

	p := PlanPromotion([]*Entry{objective, decision, adr}, edges, nil, nil,
		[]string{adr.ID}, ScopeUser)

	if len(p.Promoted) != 2 {
		t.Fatalf("the adr and its decision travel, and nothing else: %v", p.Promoted)
	}
	for _, id := range p.Promoted {
		if id == objective.ID {
			t.Fatal("the objective was reached for: a repository's scaffolding is not knowledge")
		}
	}
	if len(p.Border) != 1 || p.Border[0] != objective.ID {
		t.Fatalf("an objective that was not reached for is still named: %v", p.Border)
	}
	if p.EdgesSkipped != 1 {
		t.Errorf("the edge into it stayed behind and has to be counted, got %d", p.EdgesSkipped)
	}
	if p.EdgesCopied != 1 {
		t.Errorf("only the edge to the decision travels, got %d", p.EdgesCopied)
	}
}

// The other half of the same hole. `--kind` refuses `gotcha` because its anchor
// resolves against the repository it was written in, and the reach would have
// carried one upward anyway by following an edge into it.
func TestOneHopDoesNotReachForAGotchaButStillNamesIt(t *testing.T) {
	got := entry(t, KindGotcha, "The lock is taken before the write", "auth")
	decision := entry(t, KindDecision, "Use RS256", "auth")
	adr := entry(t, KindADR, "Asymmetric signing", "auth")
	edges := []*Edge{derives(t, adr, decision), derives(t, adr, got)}

	p := PlanPromotion([]*Entry{got, decision, adr}, edges, nil, nil,
		[]string{adr.ID}, ScopeUser)

	for _, id := range p.Promoted {
		if id == got.ID {
			t.Fatal("the gotcha was reached for: its anchor means nothing at the target")
		}
	}
	if len(p.Promoted) != 2 {
		t.Fatalf("the adr and its decision travel, and nothing else: %v", p.Promoted)
	}
	if len(p.Border) != 1 || p.Border[0] != got.ID {
		t.Fatalf("a gotcha that was not reached for is still named: %v", p.Border)
	}
	if p.EdgesSkipped != 1 || p.EdgesCopied != 1 {
		t.Errorf("one edge travels and one is counted as left behind: %+v", p)
	}
}

// The rule bounds the reach and not the selection: a caller who types an id has
// decided, and nothing here overrides that. Both excluded kinds, since the two
// are excluded for different reasons and could drift apart.
func TestAKindExcludedFromTheReachIsStillPromotedByID(t *testing.T) {
	for name, kind := range map[string]Kind{"objective": KindObjective, "gotcha": KindGotcha} {
		neighbour := entry(t, kind, "Whatever this is", "auth")
		adr := entry(t, KindADR, "Asymmetric signing", "auth")
		edges := []*Edge{derives(t, adr, neighbour)}

		p := PlanPromotion([]*Entry{neighbour, adr}, edges, nil, nil,
			[]string{adr.ID, neighbour.ID}, ScopeUser)

		if len(p.Promoted) != 2 || p.EdgesCopied != 1 {
			t.Errorf("%s: both named records and the edge between them travel: %+v", name, p)
		}
		if len(p.Border) != 0 || p.EdgesSkipped != 0 {
			t.Errorf("%s: nothing was left behind here: %+v", name, p)
		}
	}
}

// An edge with neither endpoint in the promotion is not a skipped edge — it is
// the rest of the graph, and counting it would make the number say nothing.
func TestPromotionDoesNotCountEdgesItNeverConsidered(t *testing.T) {
	adr := entry(t, KindADR, "Asymmetric signing", "auth")
	elsewhere := entry(t, KindDecision, "Use Postgres", "storage")
	alsoElsewhere := entry(t, KindDeviation, "Skipped the migration", "storage")
	edges := []*Edge{derives(t, elsewhere, alsoElsewhere)}

	p := PlanPromotion([]*Entry{adr, elsewhere, alsoElsewhere}, edges, nil, nil,
		[]string{adr.ID}, ScopeUser)

	if p.EdgesSkipped != 0 || len(p.Border) != 0 {
		t.Fatalf("an edge nowhere near this promotion is not something it left behind: %+v", p)
	}
}

// Selecting by class is the other half of not having to know every id, and the
// two selectors are a union: a record named twice is still promoted once.
func TestPromotionSelectsByKindAndUnionsWithNamedIDs(t *testing.T) {
	pref := entry(t, KindPreference, "Migrations go through Atlas", "persistence")
	other := entry(t, KindPreference, "Errors wrap with %w", "errors")
	decision := entry(t, KindDecision, "Use RS256", "auth")

	p := PlanPromotion([]*Entry{pref, other, decision}, nil, nil, nil,
		[]string{pref.ID, decision.ID}, ScopeUser, KindPreference)

	if len(p.Promoted) != 3 {
		t.Fatalf("both preferences and the named decision should travel: %v", p.Promoted)
	}
	seen := map[string]int{}
	for _, id := range p.Promoted {
		seen[id]++
	}
	if seen[pref.ID] != 1 {
		t.Errorf("a record named by id and matched by kind is one record, got %d copies", seen[pref.ID])
	}
	if len(p.Records) != 3 {
		t.Errorf("expected exactly three lines to append, got %d", len(p.Records))
	}
}

func TestPromotionReportsUnknownIDs(t *testing.T) {
	ghost, _ := NewULID(time.Now())
	p := PlanPromotion(nil, nil, nil, nil, []string{ghost}, ScopeUser)
	if len(p.Unknown) != 1 || len(p.Records) != 0 {
		t.Fatalf("an id that is not in the source should be reported, not invented: %+v", p)
	}
}
