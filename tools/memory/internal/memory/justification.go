package memory

import "fmt"

// justifications is the closed set of entry kinds that may stand behind each
// state edge.
//
// A state edge says a record moved — approved, met, cancelled, separated,
// blocked — and the second endpoint is the record that says why. Without a
// rule, the endpoint drifts into being decorative: a `met` pointing at the
// thing that was met, which asserts a transition with nothing behind it and
// reads exactly like one that was earned. So each state names the kinds that
// can carry its weight, and anything else is refused on the way in.
//
// A type absent from this map carries no rule. That is deliberate for the six
// types version 1 shipped: they describe how two records relate rather than
// what happened to one of them, and four `supersedes` edges are already on
// disk that no rule invented afterwards should be able to condemn.
var justifications = map[EdgeType]map[Kind]bool{
	EdgeApprovedForExecution: {KindDecision: true},
	// Two kinds, not one. A review that finds the thing genuinely done is
	// evidence of the same weight as a decision that declares it done.
	EdgeMet:       {KindDecision: true, KindReviewFinding: true},
	EdgeCancelled: {KindDecision: true},
	EdgeSeparated: {KindDecision: true},
	// An open question is the honest shape of most blocks: the thing being
	// waited on is usually not yet decided, and pointing at a decision that
	// does not exist yet is how a block becomes prose again.
	EdgeBlocked: {KindOpenQuestion: true, KindDecision: true},
}

// NeedsJustification reports whether an edge type has to point at a record of a
// particular kind.
func NeedsJustification(t EdgeType) bool {
	_, ruled := justifications[t]
	return ruled
}

// Justifications returns the entry kinds that may justify an edge of type t,
// sorted, and nil for a type that carries no rule.
//
// Like EntryKinds and EdgeTypes, it exists so the closed set is stated in
// exactly one place, and so the rule can be read rather than discovered by
// tripping over a refusal: `memory kinds` publishes it under `justifications`.
func Justifications(t EdgeType) []Kind {
	allowed, ruled := justifications[t]
	if !ruled {
		return nil
	}
	return sortedKinds(allowed)
}

// JustifiedEdgeTypes returns every edge type that carries a justification rule,
// sorted.
func JustifiedEdgeTypes() []EdgeType {
	out := make(map[EdgeType]bool, len(justifications))
	for t := range justifications {
		out[t] = true
	}
	return sortedEdgeTypes(out)
}

// CheckJustification reports whether an edge of type t may point at a target
// entry of kind target. A nil error means it may.
//
// This is a write-path rule, and only a write-path rule. It is not part of any
// version's rule set: a rule set judges one record on its own and cannot see
// what the target is, and the read sweep stays tolerant on purpose, so an edge
// already on disk is never condemned by a requirement that arrived after it was
// written. The caller is `link`, which has already loaded every entry to refuse
// a dangling edge and therefore knows the target's kind for free.
//
// The error names both halves — what was allowed and what was actually there —
// because "not allowed here" without the target's kind sends the reader back to
// the log to look up the id by hand.
func CheckJustification(t EdgeType, target Kind) error {
	allowed, ruled := justifications[t]
	if !ruled {
		return nil
	}
	if allowed[target] {
		return nil
	}
	// Only the trailing clause varies. Stating the sentence once means the two
	// refusals cannot drift into reading like two different rules.
	found := fmt.Sprintf("the target is of kind %q", target)
	if target == "" {
		// Reachable only from a hand-edited log: the write gate requires a kind
		// on every entry, so nothing this tool wrote can land here.
		found = "the target carries no kind"
	}
	return fmt.Errorf("a %q edge must point at the record that justifies it, one of: %s; %s",
		t, joinKinds(sortedKinds(allowed)), found)
}
