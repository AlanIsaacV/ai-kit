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

// An edge whose other end stays behind must not follow: it would be dangling
// at the target, and a dangling edge fails silently.
func TestPromotionLeavesHalfEdgesBehind(t *testing.T) {
	decision := entry(t, KindDecision, "Use RS256", "auth")
	adr := entry(t, KindADR, "Asymmetric signing", "auth")
	edges := []*Edge{derives(t, adr, decision)}

	p := PlanPromotion([]*Entry{decision, adr}, edges, nil, nil,
		[]string{adr.ID}, ScopeUser)

	if p.EdgesCopied != 0 {
		t.Fatalf("an edge into a record that stayed behind must not be copied, got %d", p.EdgesCopied)
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

func TestPromotionReportsUnknownIDs(t *testing.T) {
	ghost, _ := NewULID(time.Now())
	p := PlanPromotion(nil, nil, nil, nil, []string{ghost}, ScopeUser)
	if len(p.Unknown) != 1 || len(p.Records) != 0 {
		t.Fatalf("an id that is not in the source should be reported, not invented: %+v", p)
	}
}
