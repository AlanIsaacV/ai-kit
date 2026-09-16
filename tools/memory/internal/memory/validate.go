package memory

import (
	"fmt"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

var slugRE = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)

// hashRE is the shape of an anchor's fingerprint, not the algorithm. Pinning
// sha256 here would turn a digest a later binary wrote into a problem, which is
// the one thing this schema's versioning is careful never to do.
var hashRE = regexp.MustCompile(`^[a-z0-9]+:[0-9a-f]{32,}$`)

// Problem is one thing wrong with a record, anchored to a line number wherever
// there is one so it can be found and fixed by hand.
type Problem struct {
	// Line is the line of the file the record came out of: the log when the sweep
	// reports it, the batch input when `add --batch` does. It is 0 when the problem
	// comes from validating a single record before it is written, where there is no
	// line to point at yet.
	Line    int    `json:"line,omitempty"`
	Field   string `json:"field,omitempty"`
	Message string `json:"message"`
	Raw     string `json:"raw,omitempty"`
}

func (p Problem) String() string {
	if p.Field != "" {
		return fmt.Sprintf("line %d: %s: %s", p.Line, p.Field, p.Message)
	}
	return fmt.Sprintf("line %d: %s", p.Line, p.Message)
}

// ---------------------------------------------------------------- checks ---

// checks accumulates problems while one version's rule set runs.
//
// Every rule body lives here and is written exactly once. A version's
// validator is the *list* of rules it names, not a copy of their bodies — so
// when a later version narrows the scope set, version 1 goes on calling this
// same scope rule with the set it was written against, and "is this a ULID"
// never has two implementations that can drift apart.
type checks struct{ ps []Problem }

func (c *checks) add(field, format string, args ...any) {
	c.ps = append(c.ps, Problem{Field: field, Message: fmt.Sprintf(format, args...)})
}

// version requires the record to declare the version whose rules are running,
// or nothing at all. It is what makes a rule set complete rather than a set of
// field checks that happen to be reached from the right place.
func (c *checks) version(field string, got, want int) {
	if resolveVersion(got) != want {
		c.add(field, "record declares schema version %d but is being judged as version %d", got, want)
	}
}

func (c *checks) ulid(field, v string) {
	if v == "" {
		c.add(field, "required")
		return
	}
	if !ValidULID(v) {
		c.add(field, "%q is not a ULID (26 Crockford base32 chars)", v)
	}
}

func (c *checks) entryKind(field string, k Kind, valid map[Kind]bool) {
	if k == "" {
		c.add(field, "required")
		return
	}
	if !valid[k] {
		c.add(field, "%q is not a known kind; valid: %s", k, joinKinds(sortedKinds(valid)))
	}
}

// literalKind pins a field to one value, which is how an edge announces that
// it is an edge.
func (c *checks) literalKind(field string, got, want Kind) {
	if got != want {
		c.add(field, "must be %q", want)
	}
}

func (c *checks) edgeType(field string, t EdgeType, valid map[EdgeType]bool) {
	if t == "" {
		c.add(field, "required")
		return
	}
	if !valid[t] {
		c.add(field, "%q is not a known edge type; valid: %s", t, joinEdgeTypes(sortedEdgeTypes(valid)))
	}
}

func (c *checks) scope(field string, s Scope, valid map[Scope]bool) {
	if s == "" {
		c.add(field, "required")
		return
	}
	if !valid[s] {
		c.add(field, "%q is not a known scope; valid: %s", s, joinScopes(sortedScopes(valid)))
	}
}

func (c *checks) slug(field, v string) {
	if v == "" {
		c.add(field, "required")
		return
	}
	if !slugRE.MatchString(v) {
		c.add(field, "%q must be kebab-case: lowercase letters, digits and hyphens", v)
	}
}

func (c *checks) timestamp(field, v string) {
	if v == "" {
		c.add(field, "required")
		return
	}
	if _, err := time.Parse(time.RFC3339, v); err != nil {
		c.add(field, "%q is not RFC3339", v)
	}
}

// text requires a field to carry something other than whitespace.
func (c *checks) text(field, v string) {
	if strings.TrimSpace(v) == "" {
		c.add(field, "required")
	}
}

// tags requires at least one non-blank tag.
//
// `about` is what makes retrieval by area possible at all. An entry with no
// `about` can only ever be found by a full scan, which is the failure mode
// this whole store exists to avoid — so it is required, not optional.
func (c *checks) tags(field string, v []string) {
	if len(v) == 0 {
		c.add(field, "at least one tag is required; without it the entry is unreachable by any filtered query")
	}
	for i, tag := range v {
		if strings.TrimSpace(tag) == "" {
			c.add(fmt.Sprintf("%s[%d]", field, i), "must not be blank")
		}
	}
}

// anchors judges the code locations a record cites, and whether it had to cite
// any at all.
//
// The two halves are one rule because they answer one question — an anchor that
// cannot be resolved is not an anchor, so requiring one that is malformed would
// require nothing. `required` is the caller's to decide, since which kinds owe an
// anchor is a property of the version's rule set rather than of an anchor.
//
// What it does not check is whether the hash is one this binary can compute. A
// digest from a later binary is a record this one cannot judge, not a record that
// is wrong, and `memory anchors` reports exactly that as `could_not_check`.
func (c *checks) anchors(field string, as []Anchor, required bool) {
	if required && len(as) == 0 {
		c.add(field, "at least one anchor is required: pass --anchor path:line or --anchor path:first-last")
	}
	for i, a := range as {
		at := fmt.Sprintf("%s[%d]", field, i)
		switch {
		case strings.TrimSpace(a.Path) == "":
			c.add(at+".path", "required")
		case filepath.IsAbs(a.Path):
			c.add(at+".path", "%q must be repo-relative; an absolute path names a machine rather than a repository", a.Path)
		case a.Path == ".." || strings.HasPrefix(filepath.ToSlash(filepath.Clean(a.Path)), "../"):
			c.add(at+".path", "%q climbs out of the repository", a.Path)
		}
		if a.Line < 1 {
			c.add(at+".line", "must be 1 or more; line numbers are 1-based")
		}
		if a.Lines < 1 {
			c.add(at+".lines", "must be 1 or more; a citation covers at least the line it starts on")
		}
		if !hashRE.MatchString(a.Hash) {
			c.add(at+".hash", "%q must be <algorithm>:<hex digest>", a.Hash)
		}
	}
}

// distinct refuses a relation that points at its own source.
func (c *checks) distinct(field, from, to string) {
	if from != "" && from == to {
		c.add(field, "an edge must not point at its own source")
	}
}

// ------------------------------------------------------- version 1 rules ---

// validateEntryV1 is the complete rule set for an entry written under schema
// version 1. Not a diff against some other version and not a table of
// exceptions: the whole list, stated here, so that reading this function tells
// you everything a v1 entry had to satisfy without holding another version in
// your head.
//
// This function is frozen. When the schema moves on, a validateEntryV2 appears
// beside it and this one keeps judging the records that were written while it
// was current. Deleting it would condemn every v1 record on disk.
func validateEntryV1(e *Entry) []Problem {
	var c checks
	c.version("version", e.Version, SchemaVersion1)
	c.ulid("id", e.ID)
	c.entryKind("kind", e.Kind, entryKindsV1)
	c.slug("slug", e.Slug)
	c.scope("scope", e.Scope, scopesV1)
	c.timestamp("at", e.At)
	c.tags("about", e.About)
	c.text("title", e.Title)
	return c.ps
}

// validateEdgeV1 is the complete rule set for an edge written under schema
// version 1. Whether its endpoints exist is a separate, log-wide question —
// see ValidateLog.
func validateEdgeV1(e *Edge) []Problem {
	var c checks
	c.version("version", e.Version, SchemaVersion1)
	c.literalKind("kind", e.Kind, KindEdge)
	c.edgeType("type", e.Type, edgeTypesV1)
	c.ulid("from", e.From)
	c.ulid("to", e.To)
	c.distinct("to", e.From, e.To)
	c.timestamp("at", e.At)
	return c.ps
}

// ------------------------------------------------------- version 2 rules ---

// validateEntryV2 is the complete rule set for an entry written under schema
// version 2. Like validateEntryV1, the whole list and not a diff: version 2
// differs from version 1 in exactly one rule, and you should not have to know
// which one to read this.
//
// The difference is scopesV2, which no longer holds `plan`. Every other rule is
// the same rule — the same helper, with the same closed set — because version 2
// narrowed where a log lives, not what a record says. Naming entryKindsV1 here
// is deliberate: what a version owes the reader is its rules stated in full, not
// a private copy of every value they close over.
func validateEntryV2(e *Entry) []Problem {
	var c checks
	c.version("version", e.Version, SchemaVersion2)
	c.ulid("id", e.ID)
	c.entryKind("kind", e.Kind, entryKindsV1)
	c.slug("slug", e.Slug)
	c.scope("scope", e.Scope, scopesV2)
	c.timestamp("at", e.At)
	c.tags("about", e.About)
	c.text("title", e.Title)
	return c.ps
}

// validateEdgeV2 is the complete rule set for an edge written under schema
// version 2. It differs from validateEdgeV1 in one rule, the edge-type set:
// version 2 widened it from six types to eleven by adding the state edges, and
// edgeTypesV1 still holds the six it was written with.
//
// It was written out in full rather than delegated to validateEdgeV1 back when
// the two were rule for rule identical, and that was not ceremony. The first
// rule any version states is which version the record declares, and v1's says
// 1: an edge stamped 2 judged by v1's rules would be refused for the stamp the
// tool itself put on it, and since the write gate is the current version's
// rules, `link` would refuse every edge it tried to write. A rule set is a whole
// rule set, and the version rule is one of them. The widening is what that
// separation bought — the set moved here and version 1's did not move at all.
//
// What this does not check is whether a state edge points at a record that
// could justify it. That question is log-wide — a rule set judges one record in
// isolation and cannot see the target's kind — and it belongs on the write
// path, where `link` has every entry in hand. See CheckJustification. Keeping
// it out of here is also what stops a rule that arrived today from condemning an
// edge written yesterday.
func validateEdgeV2(e *Edge) []Problem {
	var c checks
	c.version("version", e.Version, SchemaVersion2)
	c.literalKind("kind", e.Kind, KindEdge)
	c.edgeType("type", e.Type, edgeTypesV2)
	c.ulid("from", e.From)
	c.ulid("to", e.To)
	c.distinct("to", e.From, e.To)
	c.timestamp("at", e.At)
	return c.ps
}

// ------------------------------------------------------- version 3 rules ---

// validateEntryV3 is the complete rule set for an entry written under schema
// version 3 — the whole list, like the two before it, so that reading this one
// function tells you everything a v3 entry has to satisfy.
//
// It differs from version 2 in two rules. entryKindsV3 no longer holds `unit`,
// which is what the bump bought. And the anchors rule arrived later, inside this
// version: every entry may carry code anchors, they have to be resolvable if it
// does, and anchoredKindsV3 says which kinds are refused without one. That rule
// exists nowhere in version 1 or version 2, so a record declaring either of those
// is never asked for an anchor — which is the point of dispatching per version
// and the reason those two validators were not touched to add it.
//
// Everything else is the same rule with the same set: scopesV2, because version 3
// did not touch where a log lives, and naming version 2's set here is deliberate
// for the reason validateEntryV2 gives for naming entryKindsV1 — a version states
// its rules in full, not a private copy of every value they close over.
func validateEntryV3(e *Entry) []Problem {
	var c checks
	c.version("version", e.Version, SchemaVersion3)
	c.ulid("id", e.ID)
	c.entryKind("kind", e.Kind, entryKindsV3)
	c.slug("slug", e.Slug)
	c.scope("scope", e.Scope, scopesV2)
	c.timestamp("at", e.At)
	c.tags("about", e.About)
	c.text("title", e.Title)
	c.anchors("anchors", e.Anchors, anchoredKindsV3[e.Kind])
	return c.ps
}

// validateEdgeV3 is the complete rule set for an edge written under schema
// version 3. Version 3 changed nothing about how an edge is judged: the type set
// is edgeTypesV2's eleven, and every other rule is the rule version 2 stated.
//
// So it looks like a copy, and the copy is not optional — this is validateEdgeV2's
// own lesson applied a second time. The first rule any version states is which
// version the record declares, and v2's says 2. Every edge written from here on
// is stamped 3, and the write gate is the current version's rules, so a ruleSetV3
// that reused validateEdgeV2 whole would make `link` refuse every edge it tried
// to write — for the stamp the tool itself put on it. Measured rather than
// reasoned about, in the pass that added this version: with the reuse in place,
// nine tests in this package failed on an edge with "record declares schema
// version 3 but is being judged as version 2", among them the write gate
// accepting a state edge and the sweep judging what this binary just wrote.
//
// A rule set is a whole rule set, and the version rule is one of them. The three
// edge validators now differ only in the stamp they require and the set they
// name, and whether that duplication stays affordable is open question
// 01KZ3472NSEBRE0C9XE6AP6TCC — which asked to be revisited at version 3, and is
// noted here rather than answered.
func validateEdgeV3(e *Edge) []Problem {
	var c checks
	c.version("version", e.Version, SchemaVersion3)
	c.literalKind("kind", e.Kind, KindEdge)
	c.edgeType("type", e.Type, edgeTypesV2)
	c.ulid("from", e.From)
	c.ulid("to", e.To)
	c.distinct("to", e.From, e.To)
	c.timestamp("at", e.At)
	return c.ps
}

// ---------------------------------------------------------------- schema ---

// RuleSet is the complete validation rule set for one schema version: how an
// entry written under it is judged, and how an edge written under it is
// judged.
type RuleSet struct {
	Entry func(*Entry) []Problem
	Edge  func(*Edge) []Problem
}

var ruleSetV1 = RuleSet{Entry: validateEntryV1, Edge: validateEdgeV1}
var ruleSetV2 = RuleSet{Entry: validateEntryV2, Edge: validateEdgeV2}
var ruleSetV3 = RuleSet{Entry: validateEntryV3, Edge: validateEdgeV3}

// currentRules is the rule set for CurrentSchemaVersion, and the write gate
// uses it directly instead of dispatching.
//
// That asymmetry is the point of the split. Whatever the read sweep tolerates
// on disk, `add` and `link` refuse anything the *current* schema disallows —
// a record cannot buy itself lenience by declaring an older version on the way
// in.
//
// It is read out of the same table the read sweep dispatches through rather
// than named a second time, so "the version this binary writes" and "the rules
// this binary writes it against" cannot drift apart. Stated twice, a bump that
// repointed one and forgot the other would produce a log in which every new
// record is reported unjudged and `ok` stays true — a validate that passes
// because it checked nothing, which is the exact silence this design exists to
// break.
var currentRules = mustRules(CurrentSchemaVersion)

// mustRules fetches a version's rules and refuses to start without them.
//
// Loud at init, in the manner of regexp.MustCompile, because the only way to
// reach it is a programming error: a version was stamped onto records without
// being registered, or registered with half a rule set. Neither can be
// diagnosed from the log it would produce.
func mustRules(v int) RuleSet {
	rs, ok := CurrentSchema()[v]
	if !ok {
		panic(fmt.Sprintf("memory: schema version %d is what this binary writes, but CurrentSchema has no rules for it", v))
	}
	if rs.Entry == nil || rs.Edge == nil {
		panic(fmt.Sprintf("memory: schema version %d is registered with an incomplete rule set", v))
	}
	return rs
}

// Schema maps a declared schema version to the complete rule set for it. It is
// a plain map so that the mapping is a value rather than a switch statement:
// a caller can hold one, copy it, and extend it.
type Schema map[int]RuleSet

// CurrentSchema returns the version→rules table this binary knows. This is the
// one place a new version is registered, and the write gate reads its own rules
// back out of it, so registering is not a step that can be skipped.
//
// A fresh map every call, deliberately. The only caller that needs its own
// copy is a test proving the dispatch really dispatches — which it does by
// registering a version with different rules via With, and which would be
// impossible if the mapping were a switch. No test should be able to change
// what another test, or the CLI, validates against.
func CurrentSchema() Schema {
	return Schema{
		SchemaVersion1: ruleSetV1,
		SchemaVersion2: ruleSetV2,
		SchemaVersion3: ruleSetV3,
	}
}

// With returns a copy of s that also knows version v.
//
// Nothing in the tool calls this. It exists so the dispatch can be exercised
// against a version whose rules genuinely differ, without a version this tool
// does not actually write having to ship inside it.
func (s Schema) With(v int, rs RuleSet) Schema {
	out := make(Schema, len(s)+1)
	for k, r := range s {
		out[k] = r
	}
	out[v] = rs
	return out
}

// JudgeEntry validates an entry against the rules of the version it declares,
// rather than against the rules in force today. judged is false when this
// binary has no rule set for that version — the caller reports that as
// unjudged, not as a failure.
func (s Schema) JudgeEntry(e *Entry) (ps []Problem, judged bool) {
	rs, ok := s[e.SchemaVersion()]
	if !ok || rs.Entry == nil {
		return nil, false
	}
	return rs.Entry(e), true
}

// JudgeEdge is JudgeEntry for edges. Edges travel the same dispatch: they are
// records, they carry a version, and the rules for what a relation may say can
// change without the rules for entries changing.
func (s Schema) JudgeEdge(e *Edge) (ps []Problem, judged bool) {
	rs, ok := s[e.SchemaVersion()]
	if !ok || rs.Edge == nil {
		return nil, false
	}
	return rs.Edge(e), true
}

// ------------------------------------------------------------ write gate ---

// ValidateEntry checks one entry against the *current* schema. This is the
// gate every write goes through, so garbage never reaches the log in the first
// place — the alternative is discovering three months later that half the
// records have an invented kind and an empty about, at which point the memory
// is worthless.
//
// It does not dispatch on the entry's declared version, and that is not an
// oversight: the read sweep is lenient about the past, the write path is not.
// See Schema.JudgeEntry for the reading side.
func ValidateEntry(e *Entry) []Problem { return currentRules.Entry(e) }

// ValidateEdge checks one edge against the current schema, at write time.
// Whether its endpoints exist is a separate, log-wide question — see
// ValidateLog.
func ValidateEdge(e *Edge) []Problem { return currentRules.Edge(e) }

// ---------------------------------------------------------------- report ---

// Unjudged is a record this binary declined to check because it declares a
// schema version this binary has no rules for.
//
// It is reported apart from Problems on purpose: "not checked" must never read
// as "checked and fine". It is not a failure either — an older binary meeting
// a record written by a newer one is a normal state of an evolving schema, not
// a corrupt log — so it leaves Report.OK alone.
type Unjudged struct {
	Line    int    `json:"line"`
	Version int    `json:"version"`
	ID      string `json:"id,omitempty"`
	Message string `json:"message"`
}

func (u Unjudged) String() string {
	return fmt.Sprintf("line %d: %s", u.Line, u.Message)
}

func unjudged(num, version int, id string) Unjudged {
	return Unjudged{
		Line: num, Version: version, ID: id,
		Message: fmt.Sprintf("schema version %d is not known to this binary; the record's own fields were not checked", version),
	}
}

// Report is the outcome of validating a whole log. Every record is counted in
// Entries or Edges whether or not it could be judged; Unjudged then says which
// of them went unchecked and why.
type Report struct {
	Path     string     `json:"path"`
	Entries  int        `json:"entries"`
	Edges    int        `json:"edges"`
	Problems []Problem  `json:"problems"`
	Unjudged []Unjudged `json:"unjudged"`
	OK       bool       `json:"ok"`
}

// ValidateLog checks a log against the versions this binary knows. See
// Schema.ValidateLog.
func ValidateLog(path string, lines []Line) Report {
	return CurrentSchema().ValidateLog(path, lines)
}

func ValidateLogIn(path string, lines []Line, store map[string]bool) Report {
	return CurrentSchema().ValidateLogIn(path, lines, store)
}

// ValidateLog checks every line against the rules of the version that line
// declares, plus the graph-wide invariants: unique ids and no dangling edges.
// A dangling edge is worth catching because it is silent — a graph walk simply
// returns less than it should, and nothing announces that the answer was
// incomplete.
//
// The graph-wide invariants are applied to every line, including one whose
// version is unknown. They are properties of the log rather than of a record's
// shape, and they only lean on `id`, `from` and `to` — three keys the
// compatibility rule forbids any version from renaming or repurposing. The
// alternative would be worse: one unrecognised record would make the edges
// pointing at it look dangling, so an unknown version elsewhere in the file
// would manufacture problems on lines that are perfectly fine.
func (s Schema) ValidateLog(path string, lines []Line) Report {
	return s.ValidateLogIn(path, lines, nil)
}

func (s Schema) ValidateLogIn(path string, lines []Line, store map[string]bool) Report {
	rep := Report{Path: path, Problems: []Problem{}, Unjudged: []Unjudged{}}

	ids := map[string]int{} // id -> first line it appeared on

	for _, l := range lines {
		switch {
		case l.Err != nil:
			rep.Problems = append(rep.Problems, Problem{Line: l.Num, Message: l.Err.Error(), Raw: truncate(l.Raw, 120)})
		case l.Entry != nil:
			rep.Entries++
			if ps, judged := s.JudgeEntry(l.Entry); judged {
				for _, p := range ps {
					p.Line = l.Num
					rep.Problems = append(rep.Problems, p)
				}
			} else {
				rep.Unjudged = append(rep.Unjudged, unjudged(l.Num, l.Entry.SchemaVersion(), l.Entry.ID))
			}
			if l.Entry.ID != "" {
				if prev, dup := ids[l.Entry.ID]; dup {
					rep.Problems = append(rep.Problems, Problem{
						Line: l.Num, Field: "id",
						Message: fmt.Sprintf("duplicate id %s, first seen on line %d", l.Entry.ID, prev),
					})
				} else {
					ids[l.Entry.ID] = l.Num
				}
			}
		case l.Edge != nil:
			rep.Edges++
			if ps, judged := s.JudgeEdge(l.Edge); judged {
				for _, p := range ps {
					p.Line = l.Num
					rep.Problems = append(rep.Problems, p)
				}
			} else {
				rep.Unjudged = append(rep.Unjudged, unjudged(l.Num, l.Edge.SchemaVersion(), ""))
			}
		}
	}

	// Endpoints are resolved after the whole file is read, so an edge may
	// legitimately be written before... nothing, actually: both endpoints must
	// already exist when the edge is written. But checking here rather than
	// per-line keeps the rule in one place and tolerates any write order.
	where := "this log"
	if store != nil {
		where = "any log of the store"
	}
	for _, l := range lines {
		if l.Edge == nil {
			continue
		}
		for field, id := range map[string]string{"from": l.Edge.From, "to": l.Edge.To} {
			if id == "" {
				continue
			}
			if _, ok := ids[id]; ok || store[id] {
				continue
			}
			rep.Problems = append(rep.Problems, Problem{
				Line: l.Num, Field: field,
				Message: fmt.Sprintf("dangling edge: no entry with id %s in %s", id, where),
			})
		}
	}

	sort.SliceStable(rep.Problems, func(i, j int) bool { return rep.Problems[i].Line < rep.Problems[j].Line })
	sort.SliceStable(rep.Unjudged, func(i, j int) bool { return rep.Unjudged[i].Line < rep.Unjudged[j].Line })
	rep.OK = len(rep.Problems) == 0
	return rep
}

func joinKinds(ks []Kind) string {
	s := make([]string, len(ks))
	for i, k := range ks {
		s[i] = string(k)
	}
	return strings.Join(s, ", ")
}

func joinEdgeTypes(ts []EdgeType) string {
	s := make([]string, len(ts))
	for i, t := range ts {
		s[i] = string(t)
	}
	return strings.Join(s, ", ")
}

func joinScopes(ss []Scope) string {
	s := make([]string, len(ss))
	for i, sc := range ss {
		s[i] = string(sc)
	}
	return strings.Join(s, ", ")
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
