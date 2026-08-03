package memory

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
)

// Kind is the discriminator of a line in the log. Every line is either an
// entry (a thing that happened or was decided) or an edge (a relation between
// two entries).
//
// Relations are their own lines rather than fields on an entry because the log
// is append-only: an ADR written today can be fed tomorrow by a deviation
// nobody had seen yet, and recording that must not require rewriting the ADR's
// line.
type Kind string

const (
	// Core — what makes the work loop run.
	KindObjective Kind = "objective"
	KindUnit      Kind = "unit"
	KindDecision  Kind = "decision"
	KindDeviation Kind = "deviation"

	// Learning — what stops verification theater and repeated work.
	KindAssumption   Kind = "assumption"
	KindOpenQuestion Kind = "open-question"
	KindWontFix      Kind = "wont-fix"
	KindDeadEnd      Kind = "dead-end"

	// Knowledge — what is meant to outlive the plan and be promoted upward.
	KindADR           Kind = "adr"
	KindPostmortem    Kind = "postmortem"
	KindReviewFinding Kind = "review-finding"

	// Structural.
	KindEdge Kind = "edge"
)

var entryKinds = map[Kind]bool{
	KindObjective: true, KindUnit: true, KindDecision: true, KindDeviation: true,
	KindAssumption: true, KindOpenQuestion: true, KindWontFix: true, KindDeadEnd: true,
	KindADR: true, KindPostmortem: true, KindReviewFinding: true,
}

// EntryKinds returns every valid entry kind, sorted. Used by the CLI's help
// and error messages so the closed set is stated in exactly one place.
func EntryKinds() []Kind {
	out := make([]Kind, 0, len(entryKinds))
	for k := range entryKinds {
		out = append(out, k)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

// EdgeType is the closed set of relations the graph can express.
type EdgeType string

const (
	// EdgeImplements points a unit at the objective it serves. This is the
	// edge that keeps the objective reachable from any unit of work.
	EdgeImplements EdgeType = "implements"
	// EdgeDerivesFrom points a synthesis at its sources: an ADR at the
	// decisions and deviations that produced it.
	EdgeDerivesFrom EdgeType = "derives_from"
	// EdgeSupersedes points a newer record at the one it replaces. There is
	// no mutable "still valid" field; supersession is derived from this edge,
	// because a boolean would have to be rewritten and the log never rewrites.
	EdgeSupersedes EdgeType = "supersedes"
	// EdgeAnswers points a record at the open question or assumption it
	// closes.
	EdgeAnswers EdgeType = "answers"
	// EdgeCausedBy points an incident at what led to it.
	EdgeCausedBy EdgeType = "caused_by"
	// EdgeRelatesTo is the untyped fallback. Prefer a specific type.
	EdgeRelatesTo EdgeType = "relates_to"
)

var edgeTypes = map[EdgeType]bool{
	EdgeImplements: true, EdgeDerivesFrom: true, EdgeSupersedes: true,
	EdgeAnswers: true, EdgeCausedBy: true, EdgeRelatesTo: true,
}

// EdgeTypes returns every valid edge type, sorted.
func EdgeTypes() []EdgeType {
	out := make([]EdgeType, 0, len(edgeTypes))
	for t := range edgeTypes {
		out = append(out, t)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

// Scope is the level a record lives at. Promotion between levels changes this
// field and nothing else — no id and no reference is ever rewritten.
type Scope string

const (
	ScopePlan Scope = "plan"
	ScopeRepo Scope = "repo"
	ScopeUser Scope = "user"
)

var scopes = map[Scope]bool{ScopePlan: true, ScopeRepo: true, ScopeUser: true}

// Origin records where a record came from. It survives promotion, which is
// what lets a user-level store say which repo and which plan produced an entry.
type Origin struct {
	Repo   string `json:"repo,omitempty"`
	Plan   string `json:"plan,omitempty"`
	Author string `json:"author,omitempty"`
	Unit   string `json:"unit,omitempty"`
}

// Entry is one recorded thing. Written once, never modified: a correction is a
// new entry with a supersedes edge pointing at this one.
type Entry struct {
	ID     string   `json:"id"`
	Kind   Kind     `json:"kind"`
	Slug   string   `json:"slug"`
	Scope  Scope    `json:"scope"`
	At     string   `json:"at"`
	About  []string `json:"about"`
	Title  string   `json:"title"`
	Body   string   `json:"body,omitempty"`
	Origin *Origin  `json:"origin,omitempty"`
}

// Edge is one relation. Also append-only.
type Edge struct {
	Kind Kind     `json:"kind"` // always KindEdge
	Type EdgeType `json:"type"`
	From string   `json:"from"`
	To   string   `json:"to"`
	At   string   `json:"at"`
}

// Line is one parsed line of the log: exactly one of Entry or Edge is set,
// unless the line failed to parse, in which case Err is set and Raw holds the
// original text. A malformed line never makes the lines around it unreadable.
type Line struct {
	Num   int
	Raw   string
	Entry *Entry
	Edge  *Edge
	Err   error
}

// MarshalLine renders a record as the single JSON line that gets appended.
// Keys are emitted in a stable order so a hand-read log stays scannable.
func MarshalLine(v any) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	// Encode appends a newline; the store adds its own, so trim here.
	return bytes.TrimRight(buf.Bytes(), "\n"), nil
}

// ParseLine turns one line of the log into a Line. It rejects unknown fields
// so a typo in a key surfaces as a validation error instead of silently
// vanishing into an ignored field.
func ParseLine(num int, raw string) Line {
	l := Line{Num: num, Raw: raw}

	var probe struct {
		Kind Kind `json:"kind"`
	}
	if err := json.Unmarshal([]byte(raw), &probe); err != nil {
		l.Err = fmt.Errorf("not valid JSON: %w", err)
		return l
	}

	decode := func(dst any) error {
		dec := json.NewDecoder(bytes.NewReader([]byte(raw)))
		dec.DisallowUnknownFields()
		return dec.Decode(dst)
	}

	if probe.Kind == KindEdge {
		var e Edge
		if err := decode(&e); err != nil {
			l.Err = fmt.Errorf("not a valid edge: %w", err)
			return l
		}
		l.Edge = &e
		return l
	}

	var e Entry
	if err := decode(&e); err != nil {
		l.Err = fmt.Errorf("not a valid entry: %w", err)
		return l
	}
	l.Entry = &e
	return l
}
