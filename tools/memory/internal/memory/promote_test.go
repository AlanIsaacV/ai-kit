package memory

import (
	"testing"
	"time"
)

func derives(from, to *Entry) *Edge {
	return &Edge{Kind: KindEdge, Type: EdgeDerivesFrom, From: from.ID, To: to.ID,
		At: time.Now().UTC().Format(time.RFC3339)}
}

// The id survives promotion untouched. That is the property that makes moving
// a record between levels free: nothing that points at it has to change.
func TestPromotionKeepsIDsAndRewritesScope(t *testing.T) {
	adr := entry(t, KindADR, "Asymmetric signing", "auth")
	adr.Scope = ScopePlan

	p := PlanPromotion([]*Entry{adr}, nil, nil, nil, []string{adr.ID}, ScopeRepo, "demo")

	if len(p.Records) != 1 {
		t.Fatalf("expected one record to move, got %d", len(p.Records))
	}
	moved := p.Records[0].(*Entry)
	if moved.ID != adr.ID {
		t.Fatalf("promotion changed the id: %s -> %s", adr.ID, moved.ID)
	}
	if moved.Scope != ScopeRepo {
		t.Fatalf("scope should be the target, got %s", moved.Scope)
	}
	if moved.Origin == nil || moved.Origin.Plan != "demo" {
		t.Fatal("the promoted copy should remember which plan produced it")
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
	already.Scope = ScopeRepo

	p := PlanPromotion([]*Entry{adr}, nil, []*Entry{&already}, nil, []string{adr.ID}, ScopeRepo, "demo")

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
	edges := []*Edge{derives(adr, decision)}

	p := PlanPromotion([]*Entry{decision, adr}, edges, nil, nil,
		[]string{adr.ID, decision.ID}, ScopeRepo, "demo")

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
	edges := []*Edge{derives(adr, decision)}

	p := PlanPromotion([]*Entry{decision, adr}, edges, nil, nil,
		[]string{adr.ID}, ScopeRepo, "demo")

	if p.EdgesCopied != 0 {
		t.Fatalf("an edge into a record that stayed behind must not be copied, got %d", p.EdgesCopied)
	}
}

// Promoting one end later should reconnect the pair, not leave the graph
// permanently broken because of the order the user happened to pick.
func TestPromotionReconnectsOnSecondPass(t *testing.T) {
	decision := entry(t, KindDecision, "Use RS256", "auth")
	adr := entry(t, KindADR, "Asymmetric signing", "auth")
	edges := []*Edge{derives(adr, decision)}

	atRepo := *adr
	atRepo.Scope = ScopeRepo

	p := PlanPromotion([]*Entry{decision, adr}, edges, []*Entry{&atRepo}, nil,
		[]string{decision.ID}, ScopeRepo, "demo")

	if p.EdgesCopied != 1 {
		t.Fatalf("promoting the missing end should bring the edge, got %d", p.EdgesCopied)
	}
}

func TestPromotionReportsUnknownIDs(t *testing.T) {
	ghost, _ := NewULID(time.Now())
	p := PlanPromotion(nil, nil, nil, nil, []string{ghost}, ScopeRepo, "demo")
	if len(p.Unknown) != 1 || len(p.Records) != 0 {
		t.Fatalf("an id that is not in the source should be reported, not invented: %+v", p)
	}
}
