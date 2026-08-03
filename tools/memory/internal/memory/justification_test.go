package memory

import (
	"strings"
	"testing"
)

// The allowlist as decision 01KZ30RR7THXNEWMEST0T7CFHN states it, read back one
// state at a time. Written as a table rather than as five assertions because
// what matters is the whole shape: which kinds may justify a state, and — just
// as much — which may not.
func TestEachStateNamesTheKindsThatMayJustifyIt(t *testing.T) {
	want := map[EdgeType][]Kind{
		EdgeApprovedForExecution: {KindDecision},
		EdgeMet:                  {KindDecision, KindReviewFinding},
		EdgeCancelled:            {KindDecision},
		EdgeSeparated:            {KindDecision},
		EdgeBlocked:              {KindDecision, KindOpenQuestion},
	}
	for typ, kinds := range want {
		got := Justifications(typ)
		if joinKinds(got) != joinKinds(kinds) {
			t.Errorf("%s may be justified by %s, want %s", typ, joinKinds(got), joinKinds(kinds))
		}
		if !NeedsJustification(typ) {
			t.Errorf("%s carries a rule and should say so", typ)
		}
		for _, k := range kinds {
			if err := CheckJustification(typ, k); err != nil {
				t.Errorf("%s pointing at a %s should be accepted: %v", typ, k, err)
			}
		}
	}
	if len(JustifiedEdgeTypes()) != len(want) {
		t.Errorf("expected exactly the five states to carry a rule, got %v", JustifiedEdgeTypes())
	}
}

// The refusal this rule exists for: a state pointing at the thing it is a state
// of, which asserts the transition with nothing behind it and reads exactly like
// one that was earned.
func TestAStateMayNotPointAtAnObjective(t *testing.T) {
	for _, typ := range JustifiedEdgeTypes() {
		err := CheckJustification(typ, KindObjective)
		if err == nil {
			t.Fatalf("%s pointing at an objective must be refused", typ)
		}
		// Both halves, because "not allowed here" without the target's kind
		// sends the reader back to the log to look up the id by hand.
		if !strings.Contains(err.Error(), string(KindObjective)) {
			t.Errorf("%s: the refusal must name the target's actual kind, got %q", typ, err)
		}
		for _, allowed := range Justifications(typ) {
			if !strings.Contains(err.Error(), string(allowed)) {
				t.Errorf("%s: the refusal must name %q as an allowed kind, got %q", typ, allowed, err)
			}
		}
	}
}

// A target with no kind at all reaches this only from a hand-edited log — the
// write gate requires a kind on every entry — so the refusal has to read as a
// sentence rather than as `is of kind ""`, which looks like a bug in the tool
// rather than a fact about the log.
func TestATargetWithNoKindIsRefusedInWords(t *testing.T) {
	err := CheckJustification(EdgeMet, "")
	if err == nil {
		t.Fatal("a target with no kind cannot justify anything")
	}
	if !strings.Contains(err.Error(), "carries no kind") {
		t.Errorf("expected the refusal to say the kind is missing, got %q", err)
	}
	if strings.Contains(err.Error(), `""`) {
		t.Errorf("an empty kind should not be quoted back as a value, got %q", err)
	}
	// Still one rule, stated the same way: the allowed set is named whichever
	// branch produced the message.
	for _, allowed := range Justifications(EdgeMet) {
		if !strings.Contains(err.Error(), string(allowed)) {
			t.Errorf("the refusal must still name %q as allowed, got %q", allowed, err)
		}
	}
}

// `blocked` is refused a target that is another objective along with everything
// else, and that is a decision rather than an oversight: "blocked on a sibling"
// is a real shape, but blocked-by-an-open-child is meant to be derived from the
// graph rather than written down, and allowing the edge would make the two
// indistinguishable. Recorded here so that reopening it is a deliberate act.
func TestBlockedIsNotAllowedToPointAtAnotherObjective(t *testing.T) {
	if err := CheckJustification(EdgeBlocked, KindObjective); err == nil {
		t.Fatal("blocked pointing at an objective is refused; see the unit's open questions")
	}
	if err := CheckJustification(EdgeBlocked, KindOpenQuestion); err != nil {
		t.Fatalf("blocked pointing at an open question is the ordinary case: %v", err)
	}
}

// The six types version 1 shipped carry no rule at all. They describe how two
// records relate rather than what happened to one of them — and `supersedes` in
// particular already has edges on disk that a rule invented afterwards has no
// business condemning.
func TestTheOriginalEdgeTypesCarryNoJustificationRule(t *testing.T) {
	for _, typ := range []EdgeType{EdgeImplements, EdgeDerivesFrom, EdgeSupersedes,
		EdgeAnswers, EdgeCausedBy, EdgeRelatesTo} {
		if NeedsJustification(typ) {
			t.Errorf("%s is a version 1 type and should carry no rule", typ)
		}
		if Justifications(typ) != nil {
			t.Errorf("%s should advertise no allowed set, got %v", typ, Justifications(typ))
		}
		for _, k := range EntryKinds() {
			if err := CheckJustification(typ, k); err != nil {
				t.Errorf("%s pointing at a %s must stay unaffected: %v", typ, k, err)
			}
		}
	}
}

// Every state the allowlist rules is a state the current schema can actually
// write. The two lists are declared apart, and a state ruled but not writable
// would be a rule guarding nothing.
func TestEveryRuledStateIsAWritableEdgeType(t *testing.T) {
	for _, typ := range JustifiedEdgeTypes() {
		if !edgeTypes[typ] {
			t.Errorf("%s carries a justification rule but the current schema will not write it", typ)
		}
	}
}

// Every kind the allowlist names has to be a kind that exists. A typo here
// would not fail anything: it would just quietly make a state unjustifiable.
func TestEveryAllowedKindIsARealKind(t *testing.T) {
	for _, typ := range JustifiedEdgeTypes() {
		for _, k := range Justifications(typ) {
			if !entryKinds[k] {
				t.Errorf("%s allows %q, which is not a kind this schema accepts", typ, k)
			}
		}
	}
}
