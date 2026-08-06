package memory

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
)

// SchemaVersion1 is the record shape this tool has written since it existed:
// the fields on Entry and Edge below, with `kind` drawn from entryKindsV1,
// `type` from edgeTypesV1 and `scope` from scopesV1.
//
// A version is a whole rule set, not a diff. Once records carrying a version
// are on disk, that version's rules are frozen *against narrowing* — see
// validateEntryV1 — because the only thing that makes a stamp worth writing is
// that a record it accepted yesterday is a record it accepts today.
//
// Frozen in that direction only. A closed set may still gain a value in place,
// since widening makes no record already written illegal and so takes nothing
// back from anyone; the edge types did exactly that after eight version 2 edges
// were on disk. What it costs is a reader built before the widening, which sees
// the new value as a problem rather than as an honest unjudged. See
// SchemaVersion2 and edgeTypesV2, and the README's compatibility rule for when
// to take the bump anyway.
const SchemaVersion1 = 1

// SchemaVersion2 narrows the scope set to {repo, user}. `plan` is gone, because
// there is no longer a per-plan log for a record to live in: one repo has one
// log, and the plan a record belongs to is `origin.plan` rather than a
// directory. That was forced by a property of the log itself — an edge needs
// both of its endpoints in the file it is written to, so one file per plan made
// "this objective derives from that one" inexpressible across plans.
//
// The kinds and every field are version 1's, which is why validateEntryV2 names
// entryKindsV1 rather than declaring a set of its own.
//
// The edge types are not. Version 2 later gained five more of them — the state
// edges — and gaining them did not need a version 3, because widening a closed
// set makes no record already written illegal. Only narrowing does, and
// narrowing is what a bump is for. See edgeTypesV2.
//
// The records already on disk declare version 1, most of them carrying
// `"scope": "plan"`, and they go on being judged by version 1's rules, where
// that value is valid. Not one byte of them was rewritten to get here.
const SchemaVersion2 = 2

// SchemaVersion3 narrows the entry kinds to the ten that are left: `unit` is
// gone. The model it belonged to is gone with it — an objective may now derive
// from an objective, so a smaller piece of work under one is simply another
// objective, and a separate kind for it named a distinction the tool no longer
// draws.
//
// It narrows nothing else, and no field changed. The scopes are version 2's and
// the edge types are version 2's, which is why validateEntryV3 names scopesV2
// and validateEdgeV3 names edgeTypesV2 rather than declaring sets of their own.
// `implements` in particular stays valid: six edges on disk use it.
//
// The kinds have since gained `hypothesis`, in place and without a version 4,
// the way version 2 gained the state edges: widening a closed set makes no
// record already written illegal, so there is nothing for a bump to protect.
// What it costs is a binary built before the widening, which has rules for
// version 3 and so reports the new kind as a problem rather than as an honest
// unjudged. See entryKindsV3.
//
// The bump is what makes this safe rather than tidy. Six entries on disk carry
// kind `unit` — two reading as version 1, four declaring version 2 — and both of
// those versions' validators name entryKindsV1. Narrowing that map in place
// would condemn all six for a value that was legal when they were written, so
// version 3 declares entryKindsV3 as its own literal and leaves version 1's set
// alone. Nothing was migrated and no line was rewritten to get here.
const SchemaVersion3 = 3

// CurrentSchemaVersion is what every record written from now on is stamped
// with. There is no flag for it: a version the caller can choose is a version
// the caller can get wrong, and the tool always knows which schema it writes.
const CurrentSchemaVersion = SchemaVersion3

// resolveVersion turns a declared version into the version a record is judged
// at. Zero means the `version` key was absent, which reads as version 1: the
// stamp was introduced after those records were written and nothing on disk is
// ever rewritten to add it. An unstamped record is not a broken record.
func resolveVersion(v int) int {
	if v == 0 {
		return SchemaVersion1
	}
	return v
}

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
	KindDecision  Kind = "decision"
	KindDeviation Kind = "deviation"

	// Learning — what stops verification theater and repeated work.
	KindAssumption   Kind = "assumption"
	KindOpenQuestion Kind = "open-question"
	KindWontFix      Kind = "wont-fix"
	KindDeadEnd      Kind = "dead-end"
	// KindHypothesis is a claim under test during an investigation, written with
	// the evidence that tested it and a verdict. A confirmed root cause is not a
	// kind of its own: it is the hypothesis whose verdict is `confirmed`, which
	// is what keeps one claim from being stored twice under two names. A rejected
	// one is the record of what has already been eliminated — the thing a later
	// session most often lacks. It is not `dead-end`, which means an approach
	// abandoned during implementation.
	KindHypothesis Kind = "hypothesis"

	// Knowledge — what is meant to outlive the plan and be promoted upward.
	KindADR           Kind = "adr"
	KindPostmortem    Kind = "postmortem"
	KindReviewFinding Kind = "review-finding"

	// Structural.
	KindEdge Kind = "edge"
)

// entryKindsV1 is the closed set of entry kinds as schema version 1 defines
// it. It is frozen: a later version that adds or removes a kind declares its
// own set and leaves this one alone, because the records already on disk still
// have to be judged by the set they were written against.
//
// `unit` is spelled here as a bare Kind("unit") and has no constant anywhere,
// and that asymmetry is the point: the constants above are the kinds you may
// write, and nothing may write this one — version 3 removed it. The value still
// has to sit in this set, because six entries on disk carry it and this is the
// set version 1's and version 2's validators both judge them by. So it appears
// as a literal, once, in the one place that has to hold it, next to the sentence
// saying why. A constant would have advertised it in the same breath.
var entryKindsV1 = map[Kind]bool{
	KindObjective: true, Kind("unit"): true, KindDecision: true, KindDeviation: true,
	KindAssumption: true, KindOpenQuestion: true, KindWontFix: true, KindDeadEnd: true,
	KindADR: true, KindPostmortem: true, KindReviewFinding: true,
}

// entryKindsV3 is the closed set of entry kinds as schema version 3 defines it:
// version 1's eleven without `unit`, plus `hypothesis`. Dropping `unit` is the
// whole of what the bump changed; `hypothesis` arrived afterwards, appended to
// this set in place, because widening takes nothing back from any record on disk
// and so needs no version of its own. The two edits are listed apart here on
// purpose — a reader asking what version 3 *bought* is asking about the
// narrowing, and a widening folded into that sentence would look like part of
// the price.
//
// Every member is written out rather than copied from entryKindsV1 or aliased to
// it, and that is not ceremony — it is the trap this version had to walk around.
// A Go map is a reference, so `entryKindsV3 = entryKindsV1` would be one map
// under two names, either of which looks safe to narrow, and narrowing it
// through either name would take `unit` out of version 1's frozen set as well.
// Six entries on disk are judged by that set and every one of them carries the
// kind this version drops.
var entryKindsV3 = map[Kind]bool{
	KindObjective: true, KindDecision: true, KindDeviation: true,
	KindAssumption: true, KindOpenQuestion: true, KindWontFix: true, KindDeadEnd: true,
	KindADR: true, KindPostmortem: true, KindReviewFinding: true,
	KindHypothesis: true,
}

// entryKinds is the current schema's set: what `memory kinds` prints and what
// the write gate enforces. Whoever adds a version repoints this at the new
// version's set — that is one of the three obligations the README names.
//
// Version 3's, the first version to touch the kinds. Version 2 did not, so this
// line read `entryKinds = entryKindsV1` until now — which is precisely why
// version 3 could not narrow the current set without declaring its own: a map is
// a reference, and under that alias "narrow what the tool accepts today" and
// "narrow version 1's frozen set" were one edit. See entryKindsV3.
var entryKinds = entryKindsV3

// EntryKinds returns every entry kind the current schema accepts, sorted. Used
// by the CLI's help and error messages so the closed set is stated in exactly
// one place.
func EntryKinds() []Kind { return sortedKinds(entryKinds) }

func sortedKinds(set map[Kind]bool) []Kind {
	out := make([]Kind, 0, len(set))
	for k := range set {
		out = append(out, k)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

// EdgeType is the closed set of relations the graph can express.
type EdgeType string

const (
	// EdgeImplements points a piece of work at the objective it serves. It was
	// written for the `unit` kind version 3 removed, and it stays valid because
	// six edges on disk use it: an edge type is narrowed by a bump like any other
	// closed set, and version 3 narrowed none. What implements an objective now
	// is another objective, and that relation is `derives_from`.
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

	// The state edges, added by schema version 2.
	//
	// What state a record is in is a relation, not a field. A field would have
	// to be rewritten to change, and nothing in this log is ever rewritten — so
	// the state is an edge, the absence of every state edge is a state too, and
	// leaving that default state costs one appended line and takes nothing
	// back.
	//
	// Each of them points at the record that justifies the transition, and the
	// write path refuses one that does not; see CheckJustification. Which is
	// also why there is no reflexive form: an edge from a record to itself would
	// be a state claimed with nothing behind it.

	// EdgeApprovedForExecution points a record at the decision that approved it.
	EdgeApprovedForExecution EdgeType = "approved_for_execution"
	// EdgeMet points a record at what establishes that it was satisfied.
	EdgeMet EdgeType = "met"
	// EdgeCancelled points a record at the decision that dropped it.
	EdgeCancelled EdgeType = "cancelled"
	// EdgeSeparated points a record at the decision that lifted it out from
	// under whatever it sat below. The work goes on; it goes on elsewhere.
	EdgeSeparated EdgeType = "separated"
	// EdgeBlocked points a record at what is holding it up.
	EdgeBlocked EdgeType = "blocked"
)

// edgeTypesV1 is the closed set of edge types as schema version 1 defines it,
// frozen for the same reason entryKindsV1 is.
var edgeTypesV1 = map[EdgeType]bool{
	EdgeImplements: true, EdgeDerivesFrom: true, EdgeSupersedes: true,
	EdgeAnswers: true, EdgeCausedBy: true, EdgeRelatesTo: true,
}

// edgeTypesV2 is the closed set of edge types as schema version 2 defines it:
// version 1's six, plus the five that say what state a record is in.
//
// The widening happened inside version 2 rather than in a version 3, because
// adding a value to a closed set is additive — it makes no record on disk
// illegal and forces no frozen rule set to change. That was chosen with its
// cost known and it is stated here rather than left to be discovered: a binary
// built before the widening has rules for version 2, so meeting a `met` edge
// stamped 2 it reports `type: unknown` — a false problem — where for a version
// it did not know at all it would have reported an honest `unjudged`. Narrowing
// is the direction that still costs a bump; see the README's compatibility rule.
//
// Every member is spelled out rather than copied from edgeTypesV1, and that is
// the trap entryKinds names above: a Go map is a reference, so
// `edgeTypesV2 = edgeTypesV1` would be two names for one map, either of which
// looks safe to narrow — and narrowing either would silently narrow version 1's
// frozen set, which is the one thing the versioning exists to prevent.
var edgeTypesV2 = map[EdgeType]bool{
	EdgeImplements: true, EdgeDerivesFrom: true, EdgeSupersedes: true,
	EdgeAnswers: true, EdgeCausedBy: true, EdgeRelatesTo: true,
	EdgeApprovedForExecution: true, EdgeMet: true, EdgeCancelled: true,
	EdgeSeparated: true, EdgeBlocked: true,
}

// edgeTypes is the current schema's set: version 2's, which is version 1's six
// widened by the five state types. See entryKinds for what repointing this is
// for.
//
// Version 3 left it here. It narrowed the entry kinds and nothing else, so there
// is no edgeTypesV3 to point at — the same arrangement validateEntryV2 had with
// entryKindsV1, and for the same reason: a version owes the reader its rules
// stated in full, not a private copy of every set they close over.
var edgeTypes = edgeTypesV2

// EdgeTypes returns every edge type the current schema accepts, sorted.
func EdgeTypes() []EdgeType { return sortedEdgeTypes(edgeTypes) }

func sortedEdgeTypes(set map[EdgeType]bool) []EdgeType {
	out := make([]EdgeType, 0, len(set))
	for t := range set {
		out = append(out, t)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

// Scope is the level a record lives at. Promotion between levels changes this
// field and nothing else — no id and no reference is ever rewritten.
type Scope string

const (
	// ScopePlan is version 1 vocabulary. Version 2 removed it — there is no
	// per-plan log any more — but the constant stays, because scopesV1 names it,
	// version 1's rules are frozen, and the records carrying it are still read
	// every day. What went away is `plan` as a place Resolve can find and as a
	// value the CLI accepts, not the word itself.
	ScopePlan Scope = "plan"
	ScopeRepo Scope = "repo"
	ScopeUser Scope = "user"
)

// scopesV1 is the closed set of scopes as schema version 1 defines it. If a
// later version drops one of these, it does so by declaring its own set — not
// by editing this line, and not by rewriting the records that used the value.
var scopesV1 = map[Scope]bool{ScopePlan: true, ScopeRepo: true, ScopeUser: true}

// scopesV2 is the closed set of scopes as schema version 2 defines it: `plan`
// is absent. This is the whole of what version 2 changed, and it is exactly the
// case the versioning machinery was built for — a value that was legal
// yesterday is illegal today, and no record written yesterday becomes invalid.
var scopesV2 = map[Scope]bool{ScopeRepo: true, ScopeUser: true}

// scopes is the current schema's set. Still version 2's: version 3 took a kind
// away and left the scopes where they were.
var scopes = scopesV2

// Scopes returns every scope the current schema accepts, sorted. Like
// EntryKinds and EdgeTypes, it exists so the closed set is stated in exactly
// one place: when a version narrows it, `memory kinds` narrows with it instead
// of going on advertising a value the write gate now refuses.
func Scopes() []Scope { return sortedScopes(scopes) }

func sortedScopes(set map[Scope]bool) []Scope {
	out := make([]Scope, 0, len(set))
	for s := range set {
		out = append(out, s)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

// Origin records where a record came from. It survives promotion, which is
// what lets a user-level store say which repo and which plan produced an entry.
//
// Plan carries more weight than the rest of it. Since version 2 there is one
// log per repo, so `origin.plan` is the only thing a bounded query has left to
// bound by — a record written without one is not untidy, it is unreachable by
// every query that names a plan. That is why the write path refuses to guess it;
// see ErrNoPlan.
type Origin struct {
	Repo   string `json:"repo,omitempty"`
	Plan   string `json:"plan,omitempty"`
	Author string `json:"author,omitempty"`

	// Unit is dead vocabulary and stays anyway. Version 3 removed the `unit`
	// kind; no record in either log populates this key, and `omitempty` means
	// deleting the field would change not one byte on disk.
	//
	// Which is the whole argument for deleting it, and the reason not to. The
	// compatibility rule is additive only — never rename a field, never repurpose
	// one, never remove one — and a rule that holds except where removal happens
	// to be invisible is a rule a reader cannot lean on. What it costs to keep is
	// one unused string; what it buys is that no later reader has to work out
	// whether some key they remember was dropped or was never there.
	Unit string `json:"unit,omitempty"`
}

// Entry is one recorded thing. Written once, never modified: a correction is a
// new entry with a supersedes edge pointing at this one.
//
// Version comes first on the line, and without omitempty, on purpose. It is
// the field that says how to read every field after it, so a reader — human or
// program — should not have to scan past the body to find it, and a version
// nobody can see is a version nobody checks. It is per-entry rather than
// per-file because a record survives promotion into another log, where it
// keeps the rules it was written under.
type Entry struct {
	Version int      `json:"version"`
	ID      string   `json:"id"`
	Kind    Kind     `json:"kind"`
	Slug    string   `json:"slug"`
	Scope   Scope    `json:"scope"`
	At      string   `json:"at"`
	About   []string `json:"about"`
	Title   string   `json:"title"`
	Body    string   `json:"body,omitempty"`
	Origin  *Origin  `json:"origin,omitempty"`
}

// SchemaVersion is the version of the record schema this entry is judged
// against. An absent `version` key reads as 1; see resolveVersion.
func (e *Entry) SchemaVersion() int { return resolveVersion(e.Version) }

// entryJSON is Entry stripped of Entry's marshaller, so MarshalJSON can encode
// the struct without calling itself. Same fields in the same order — the
// version stays the first key on the line.
type entryJSON Entry

// MarshalJSON writes the version the record is judged at rather than the raw
// field, so a record written before the stamp existed renders as version 1 and
// not as version 0 — a number that is on no line and names no schema that ever
// existed. A reader who saw it would go looking for version 0's rules.
//
// Only the rendering is resolved, never the parse: ParseLine leaves Version at
// zero for a line with no `version` key, because zero is what the line says,
// and SchemaVersion is where the two are reconciled.
//
// This changes no bytes that get written. Every writer stamps, so the resolved
// value is the field it replaces; and a writer that forgot cannot reach disk
// once the current version is anything but 1, because the write gate judges an
// unstamped record as version 1 and refuses it.
func (e Entry) MarshalJSON() ([]byte, error) {
	out := entryJSON(e)
	out.Version = resolveVersion(e.Version)
	return marshalCompact(out)
}

// Edge is one relation. Also append-only, and stamped with its own version:
// an edge is a record like any other, and the rules for what a relation may
// say can change independently of what an entry may say.
type Edge struct {
	Version int      `json:"version"`
	Kind    Kind     `json:"kind"` // always KindEdge
	Type    EdgeType `json:"type"`
	From    string   `json:"from"`
	To      string   `json:"to"`
	At      string   `json:"at"`
}

// SchemaVersion is the version of the record schema this edge is judged
// against. An absent `version` key reads as 1.
func (e *Edge) SchemaVersion() int { return resolveVersion(e.Version) }

type edgeJSON Edge

// MarshalJSON resolves the rendered version for the same reason Entry's does.
func (e Edge) MarshalJSON() ([]byte, error) {
	out := edgeJSON(e)
	out.Version = resolveVersion(e.Version)
	return marshalCompact(out)
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
func MarshalLine(v any) ([]byte, error) { return marshalCompact(v) }

// marshalCompact encodes one value as a single line with HTML escaping off.
//
// Off, because Go escapes <, > and & by default, which turns a body mentioning
// a tag or a URL into unicode escapes and makes the log unpleasant to read by
// hand. It is a separate helper because the MarshalJSON methods above need the
// same setting: json.Marshal inside a marshaller would re-escape, and the outer
// encoder cannot undo that.
func marshalCompact(v any) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	// Encode appends a newline; the store adds its own, so trim here.
	return bytes.TrimRight(buf.Bytes(), "\n"), nil
}

// ParseLine turns one line of the log into a Line.
//
// It accepts keys it does not know. That is the read path's half of the
// compatibility bargain: a field some later version adds is exactly what an
// older binary will meet on disk, and calling that line malformed would turn
// every forward-compatible record into a validation failure. The unknown key
// is ignored and the fields this binary does know are parsed as normal.
//
// Typo protection did not disappear, it moved to where typos originate: the
// CLI parses `--titel` as an unknown flag and refuses the write, so a mistyped
// key never reaches the log to be read back.
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
