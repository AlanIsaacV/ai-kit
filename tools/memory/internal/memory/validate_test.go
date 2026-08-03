package memory

import (
	"strings"
	"testing"
	"time"
)

func newTestEntry(t *testing.T, mutate func(*Entry)) *Entry {
	t.Helper()
	id, err := NewULID(time.Now())
	if err != nil {
		t.Fatal(err)
	}
	// Stamped and scoped the way the tool stamps and scopes: fixtures that skip
	// the version would only ever exercise the absent-key path, and the
	// absent-key path is not what `memory add` writes. The unstamped case and the
	// version 1 case get their own fixtures below.
	e := &Entry{
		Version: CurrentSchemaVersion,
		ID:      id,
		Kind:    KindDecision,
		Slug:    "rs256-key-rotation",
		Scope:   ScopeRepo,
		At:      time.Now().UTC().Format(time.RFC3339),
		About:   []string{"auth"},
		Title:   "RS256 over HS256",
	}
	if mutate != nil {
		mutate(e)
	}
	return e
}

// newV1Entry is a record in the shape the log already holds: stamped version 1
// and scoped `plan`, a value version 2 removed and version 1 still accepts.
// Every entry written before this bump looks like this, and none of them was
// touched to get here.
func newV1Entry(t *testing.T, mutate func(*Entry)) *Entry {
	t.Helper()
	return newTestEntry(t, func(e *Entry) {
		e.Version = SchemaVersion1
		e.Scope = ScopePlan
		if mutate != nil {
			mutate(e)
		}
	})
}

func problemFields(ps []Problem) string {
	var s []string
	for _, p := range ps {
		s = append(s, p.Field)
	}
	return strings.Join(s, ",")
}

func TestValidateEntryAcceptsWellFormed(t *testing.T) {
	if ps := ValidateEntry(newTestEntry(t, nil)); len(ps) != 0 {
		t.Fatalf("expected no problems, got %v", ps)
	}
}

// An invented kind is the failure mode that quietly rots the store: nothing
// crashes, the record just becomes unfindable by every query that names kinds.
func TestValidateEntryRejectsUnknownKind(t *testing.T) {
	ps := ValidateEntry(newTestEntry(t, func(e *Entry) { e.Kind = "brilliant-idea" }))
	if problemFields(ps) != "kind" {
		t.Fatalf("expected a kind problem, got %v", ps)
	}
	if !strings.Contains(ps[0].Message, "decision") {
		t.Errorf("the error should name the valid set, got %q", ps[0].Message)
	}
}

// Without `about`, an entry can only be found by a full scan — which is the
// context blowup the whole store exists to prevent.
func TestValidateEntryRejectsEmptyAbout(t *testing.T) {
	for name, mutate := range map[string]func(*Entry){
		"nil":   func(e *Entry) { e.About = nil },
		"empty": func(e *Entry) { e.About = []string{} },
		"blank": func(e *Entry) { e.About = []string{"   "} },
	} {
		ps := ValidateEntry(newTestEntry(t, mutate))
		if len(ps) == 0 {
			t.Errorf("%s about: expected rejection", name)
		}
	}
}

func TestValidateEntryRejectsBadFields(t *testing.T) {
	cases := map[string]struct {
		mutate func(*Entry)
		field  string
	}{
		"non-ulid id":  {func(e *Entry) { e.ID = "adr01" }, "id"},
		"empty title":  {func(e *Entry) { e.Title = "  " }, "title"},
		"shouty slug":  {func(e *Entry) { e.Slug = "RS256_Rotation" }, "slug"},
		"bad scope":    {func(e *Entry) { e.Scope = "team" }, "scope"},
		"bad datetime": {func(e *Entry) { e.At = "2026-08-01" }, "at"},
	}
	for name, tc := range cases {
		ps := ValidateEntry(newTestEntry(t, tc.mutate))
		if len(ps) == 0 || ps[0].Field != tc.field {
			t.Errorf("%s: expected a %q problem, got %v", name, tc.field, ps)
		}
	}
}

func newTestEdge(t *testing.T, typ EdgeType, from, to string) *Edge {
	t.Helper()
	return &Edge{
		Version: CurrentSchemaVersion,
		Kind:    KindEdge, Type: typ, From: from, To: to,
		At: time.Now().UTC().Format(time.RFC3339),
	}
}

func mustULID(t *testing.T) string {
	t.Helper()
	id, err := NewULID(time.Now())
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func TestValidateEdge(t *testing.T) {
	a, _ := NewULID(time.Now())
	b, _ := NewULID(time.Now())

	if ps := ValidateEdge(newTestEdge(t, EdgeDerivesFrom, a, b)); len(ps) != 0 {
		t.Fatalf("expected no problems, got %v", ps)
	}

	bad := newTestEdge(t, "inspired_by", a, b)
	if ps := ValidateEdge(bad); len(ps) == 0 || ps[0].Field != "type" {
		t.Errorf("unknown edge type should be rejected, got %v", ps)
	}

	self := newTestEdge(t, EdgeRelatesTo, a, a)
	if ps := ValidateEdge(self); len(ps) == 0 {
		t.Errorf("self-edge should be rejected")
	}
}

// A dangling edge is silent: the graph walk just returns less than it should
// and nothing says the answer was partial. That is why validate hunts for it.
func TestValidateLogFindsDanglingEdge(t *testing.T) {
	e := newTestEntry(t, nil)
	ghost, _ := NewULID(time.Now())

	lines := []Line{
		{Num: 1, Entry: e},
		{Num: 2, Edge: newTestEdge(t, EdgeDerivesFrom, e.ID, ghost)},
	}
	rep := ValidateLog("test.jsonl", lines)
	if rep.OK {
		t.Fatal("expected the dangling edge to be reported")
	}
	found := false
	for _, p := range rep.Problems {
		if strings.Contains(p.Message, "dangling edge") && p.Line == 2 {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected a dangling-edge problem on line 2, got %v", rep.Problems)
	}
}

func TestValidateLogFindsDuplicateID(t *testing.T) {
	a := newTestEntry(t, nil)
	b := newTestEntry(t, nil)
	b.ID = a.ID

	rep := ValidateLog("test.jsonl", []Line{{Num: 1, Entry: a}, {Num: 2, Entry: b}})
	if rep.OK {
		t.Fatal("expected duplicate id to be reported")
	}
}

// One unparseable line must not poison the lines around it.
func TestValidateLogIsolatesMalformedLine(t *testing.T) {
	good := newTestEntry(t, nil)
	lines := []Line{
		{Num: 1, Entry: good},
		ParseLine(2, `{"kind":"decision",`),
		{Num: 3, Entry: newTestEntry(t, nil)},
	}
	rep := ValidateLog("test.jsonl", lines)
	if rep.Entries != 2 {
		t.Errorf("expected the two good entries to survive, counted %d", rep.Entries)
	}
	if len(rep.Problems) != 1 || rep.Problems[0].Line != 2 {
		t.Fatalf("expected exactly one problem on line 2, got %v", rep.Problems)
	}
}

// A key this binary does not know is exactly what a record written by a newer
// binary looks like from here. Refusing it would turn every forward-compatible
// record into a malformed line, so the read path ignores it — and must not let
// it disturb the fields it does know. Typos are caught on the write path,
// where the CLI refuses an unknown flag; see cmd/memory's tests.
func TestParseLineAcceptsAnUnknownKey(t *testing.T) {
	id, _ := NewULID(time.Now())
	raw := `{"version":1,"id":"` + id + `","kind":"decision","slug":"rs256-key-rotation",` +
		`"scope":"plan","at":"2026-08-02T10:00:00Z","about":["auth"],"title":"RS256 over HS256",` +
		`"confidence":"high"}`

	l := ParseLine(1, raw)
	if l.Err != nil {
		t.Fatalf("an unknown key must not make the line unreadable: %v", l.Err)
	}
	if l.Entry == nil {
		t.Fatal("expected an entry")
	}
	if l.Entry.Title != "RS256 over HS256" || l.Entry.ID != id || l.Entry.Scope != ScopePlan {
		t.Fatalf("the unknown key corrupted the record: %+v", l.Entry)
	}
	// Judged through the dispatch, at the version the line declares. The write
	// gate is not the right question here: this line is not being written, it is
	// being read back off disk, where `scope: plan` is a version 1 record saying
	// what version 1 allowed.
	if ps, judged := CurrentSchema().JudgeEntry(l.Entry); !judged || len(ps) != 0 {
		t.Fatalf("the record is valid apart from a key nobody asked about: judged=%v %v", judged, ps)
	}
}

func TestParseLineAcceptsAnUnknownKeyOnAnEdge(t *testing.T) {
	a, _ := NewULID(time.Now())
	b, _ := NewULID(time.Now())
	raw := `{"version":1,"kind":"edge","type":"supersedes","from":"` + a + `","to":"` + b + `",` +
		`"at":"2026-08-02T10:00:00Z","weight":3}`

	l := ParseLine(1, raw)
	if l.Err != nil {
		t.Fatalf("an unknown key must not make the edge unreadable: %v", l.Err)
	}
	if l.Edge == nil || l.Edge.Type != EdgeSupersedes || l.Edge.From != a || l.Edge.To != b {
		t.Fatalf("the unknown key corrupted the edge: %+v", l.Edge)
	}
}

// ------------------------------------------------ version stamp and dispatch ---

// testSchemaVersion is a schema version that exists only in this file.
//
// It is deliberately not a version the tool ships. Two is now real — it is what
// dropping the plan scope cost — and the tests for it are the ones just above,
// against records the binary actually writes. What this one is still for is the
// dispatch's other half: a version registered from outside, proving that
// CurrentSchema is a table a caller can extend and that the write gate does not
// follow it.
const testSchemaVersion = 900

// testRuleSet is the complete rule set for testSchemaVersion, written the way
// a real version is written: the whole list of rules named in one place,
// composed from the same check helpers version 1 uses, differing only where
// the schema differs.
//
// It differs from v1 in exactly the two ways a narrowing bump differs — a
// value dropped from a closed set. `plan` is gone from the scope set, which is
// what version 2 really did, and `relates_to` is gone from the edge types,
// which nothing has done, so an edge can prove the same dispatch an entry
// proves.
func testRuleSet() RuleSet {
	scopesTest := map[Scope]bool{ScopeRepo: true, ScopeUser: true}
	edgeTypesTest := map[EdgeType]bool{
		EdgeImplements: true, EdgeDerivesFrom: true, EdgeSupersedes: true,
		EdgeAnswers: true, EdgeCausedBy: true,
	}
	return RuleSet{
		Entry: func(e *Entry) []Problem {
			var c checks
			c.version("version", e.Version, testSchemaVersion)
			c.ulid("id", e.ID)
			c.entryKind("kind", e.Kind, entryKindsV1)
			c.slug("slug", e.Slug)
			c.scope("scope", e.Scope, scopesTest)
			c.timestamp("at", e.At)
			c.tags("about", e.About)
			c.text("title", e.Title)
			return c.ps
		},
		Edge: func(e *Edge) []Problem {
			var c checks
			c.version("version", e.Version, testSchemaVersion)
			c.literalKind("kind", e.Kind, KindEdge)
			c.edgeType("type", e.Type, edgeTypesTest)
			c.ulid("from", e.From)
			c.ulid("to", e.To)
			c.distinct("to", e.From, e.To)
			c.timestamp("at", e.At)
			return c.ps
		},
	}
}

// ------------------------------------------------------------- version 2 ---
//
// The bump that dropped the plan scope, which is the first time the versioning
// machinery was asked to do the thing it was built for.

// The whole point of the stamp, now with two versions the tool really writes:
// two records with identical fields, judged differently because they declare
// different versions. The one written when `plan` was still a scope stays valid
// forever; the one written after it was dropped does not.
//
// This is why the 39 records already on disk needed no migration and no legacy
// value kept alive in the current validator.
func TestThePlanScopeStaysValidAtV1AndIsRefusedAtV2(t *testing.T) {
	onDisk := newV1Entry(t, nil) // version 1, scope: plan
	sameFieldsAtV2 := newV1Entry(t, func(e *Entry) { e.Version = SchemaVersion2 })

	rep := ValidateLog("test.jsonl", []Line{{Num: 1, Entry: onDisk}, {Num: 2, Entry: sameFieldsAtV2}})

	if len(rep.Unjudged) != 0 {
		t.Fatalf("both versions ship in this binary, nothing should go unjudged: %v", rep.Unjudged)
	}
	if len(rep.Problems) != 1 {
		t.Fatalf("expected exactly one problem, the v2 scope, got %v", rep.Problems)
	}
	p := rep.Problems[0]
	if p.Line != 2 || p.Field != "scope" {
		t.Fatalf("expected a scope problem on line 2, got %v", p)
	}
	if !strings.Contains(p.Message, "plan") {
		t.Errorf("the problem should name the offending value, got %q", p.Message)
	}
	if strings.Contains(p.Message, "valid: plan") {
		t.Errorf("version 2 must not go on advertising `plan` as a scope, got %q", p.Message)
	}
}

// Version 1's rules are frozen, and "frozen" has to mean the value it accepted
// is still the value it accepts — read out of v1's own set and out of v1's own
// validator, not out of whatever is current.
func TestVersion1sRulesDidNotMoveWhenVersion2Arrived(t *testing.T) {
	if !scopesV1[ScopePlan] || !scopesV1[ScopeRepo] || !scopesV1[ScopeUser] {
		t.Fatalf("version 1 knew three scopes and still has to: %v", sortedScopes(scopesV1))
	}
	if scopesV2[ScopePlan] {
		t.Fatal("version 2 exists to drop the plan scope")
	}
	if ps := validateEntryV1(newV1Entry(t, nil)); len(ps) != 0 {
		t.Fatalf("v1's validator must still accept a v1 record: %v", ps)
	}
	if ps := validateEntryV2(newTestEntry(t, func(e *Entry) { e.Scope = ScopePlan })); problemFields(ps) != "scope" {
		t.Fatalf("v2's validator must refuse the plan scope and nothing else: %v", ps)
	}
}

// Version 2 narrowed the scopes and took nothing else away. Stated as a test
// because the cheapest way to break a version bump is to smuggle a second
// change into it, where nobody looking at the scope set would think to check.
//
// It has since widened the edge types, which is a different act: adding a value
// to a closed set makes no record on disk illegal, so it needs no bump. What
// this guards is the direction that does — nothing version 1 accepted may have
// gone missing.
func TestVersion2TookNothingAwayButTheScope(t *testing.T) {
	if len(entryKinds) != len(entryKindsV1) {
		t.Errorf("the current kind set is not version 1's: %v", EntryKinds())
	}
	for k := range entryKindsV1 {
		if !entryKinds[k] {
			t.Errorf("kind %q went missing from the current set", k)
		}
	}
	for t2 := range edgeTypesV1 {
		if !edgeTypes[t2] {
			t.Errorf("edge type %q went missing from the current set", t2)
		}
	}
	// `unit` in particular: removing it is a later objective's business, and a
	// bump that quietly took it out would condemn records nobody meant to touch.
	if ps := ValidateEntry(newTestEntry(t, func(e *Entry) { e.Kind = KindUnit })); len(ps) != 0 {
		t.Errorf("version 2 keeps version 1's kinds, `unit` included: %v", ps)
	}
	if ps := ValidateEdge(newTestEdge(t, EdgeImplements, mustULID(t), mustULID(t))); len(ps) != 0 {
		t.Errorf("version 2 keeps version 1's edge types: %v", ps)
	}
}

// ------------------------------------------------------ the state edges ---
//
// Five edge types added to version 2 in place, because widening a closed set
// makes nothing already written illegal.

// The widening did not leak backwards. edgeTypesV1 is its own map with its own
// six members, and if the two had been aliased — `edgeTypesV2 = edgeTypesV1`,
// which is two names for one map, since a map is a reference — this is the test
// that would notice, because v1's set would have grown to eleven behind its own
// back.
func TestTheWideningDidNotLeakIntoVersion1(t *testing.T) {
	states := []EdgeType{EdgeApprovedForExecution, EdgeMet, EdgeCancelled, EdgeSeparated, EdgeBlocked}

	if len(edgeTypesV1) != 6 {
		t.Fatalf("version 1 knew six edge types and still has to: %v", sortedEdgeTypes(edgeTypesV1))
	}
	if len(edgeTypesV2) != 11 {
		t.Fatalf("version 2 knows version 1's six plus five states: %v", sortedEdgeTypes(edgeTypesV2))
	}
	for _, s := range states {
		if edgeTypesV1[s] {
			t.Errorf("%q reached version 1's frozen set", s)
		}
		if !edgeTypesV2[s] {
			t.Errorf("%q is missing from version 2's set", s)
		}
	}
	for typ := range edgeTypesV1 {
		if !edgeTypesV2[typ] {
			t.Errorf("version 1's %q went missing from version 2's set", typ)
		}
	}
	if CurrentSchemaVersion != SchemaVersion2 {
		t.Errorf("the widening was additive and must not have moved the current version, got %d",
			CurrentSchemaVersion)
	}
	for _, s := range states {
		if !edgeTypes[s] {
			t.Errorf("the current set must be version 2's, and %q is not in it", s)
		}
	}
}

// The pair that says a closed set really was widened *inside* a version rather
// than everywhere at once: the same edge, byte for byte apart from the stamp, is
// refused when it declares version 1 and accepted when it declares version 2.
//
// This is the cost the additive choice was taken with, and it is worth seeing:
// the refusal a v1 reader gives is `type`, a problem, rather than the honest
// `unjudged` it would give for a version it had never heard of.
func TestAStateEdgeIsVersion2sAndVersion1RefusesIt(t *testing.T) {
	a := newTestEntry(t, nil)
	b := newTestEntry(t, func(e *Entry) { e.Kind = KindDecision })

	atV1 := newTestEdge(t, EdgeMet, a.ID, b.ID)
	atV1.Version = SchemaVersion1
	atV2 := newTestEdge(t, EdgeMet, a.ID, b.ID) // stamped CurrentSchemaVersion

	rep := ValidateLog("test.jsonl", []Line{
		{Num: 1, Entry: a}, {Num: 2, Entry: b},
		{Num: 3, Edge: atV1}, {Num: 4, Edge: atV2},
	})

	if len(rep.Unjudged) != 0 {
		t.Fatalf("both versions ship in this binary, nothing should go unjudged: %v", rep.Unjudged)
	}
	if len(rep.Problems) != 1 {
		t.Fatalf("expected exactly one problem, the v1 edge, got %v", rep.Problems)
	}
	p := rep.Problems[0]
	if p.Line != 3 || p.Field != "type" {
		t.Fatalf("expected a type problem on line 3, got %v", p)
	}
	if !strings.Contains(p.Message, string(EdgeMet)) {
		t.Errorf("the problem should name the offending value, got %q", p.Message)
	}
	if ps := validateEdgeV2(atV2); len(ps) != 0 {
		t.Errorf("v2's edge rules must accept a state edge: %v", ps)
	}
	if ps := validateEdgeV1(atV1); problemFields(ps) != "type" {
		t.Errorf("v1's edge rules must refuse it, and only for the type: %v", ps)
	}
}

// The write gate writes every state. Guarded because the gate reads its rules
// out of the version table, so a widening that touched edgeTypesV2 but left
// validateEdgeV2 naming edgeTypesV1 would compile, pass everything above, and
// still refuse every state edge `link` tried to write.
func TestTheWriteGateAcceptsEveryStateEdge(t *testing.T) {
	for _, typ := range JustifiedEdgeTypes() {
		if ps := ValidateEdge(newTestEdge(t, typ, mustULID(t), mustULID(t))); len(ps) != 0 {
			t.Errorf("the write gate must accept a %q edge: %v", typ, ps)
		}
	}
}

// The read sweep does not know the justification rule, and must not learn it. A
// state edge pointing at something the allowlist would refuse is a record on
// disk, and a rule that arrived after it was written does not get to condemn it
// retroactively — the refusal happens once, in `link`, on the way in.
func TestTheReadSweepDoesNotApplyTheJustificationRule(t *testing.T) {
	objective := newTestEntry(t, func(e *Entry) { e.Kind = KindObjective })
	other := newTestEntry(t, func(e *Entry) { e.Kind = KindObjective })

	if err := CheckJustification(EdgeMet, KindObjective); err == nil {
		t.Fatal("the premise of this test is that the write path would refuse this edge")
	}

	rep := ValidateLog("test.jsonl", []Line{
		{Num: 1, Entry: objective}, {Num: 2, Entry: other},
		{Num: 3, Edge: newTestEdge(t, EdgeMet, objective.ID, other.ID)},
	})
	if !rep.OK || len(rep.Problems) != 0 {
		t.Fatalf("the sweep stays tolerant of a state edge already on disk: %v", rep.Problems)
	}
}

// The write gate cannot be talked into leniency. The plan scope is refused going
// in whatever a caller says, and there is no flag or variable that reopens it —
// the gate is the current version's rules, and the current version does not have
// the value.
func TestTheWriteGateRefusesThePlanScope(t *testing.T) {
	if ps := ValidateEntry(newTestEntry(t, func(e *Entry) { e.Scope = ScopePlan })); problemFields(ps) != "scope" {
		t.Fatalf("expected the write gate to refuse scope plan, got %v", ps)
	}
	// Declaring the older version on the way in does not buy the value back:
	// the gate does not dispatch, so a v1 record hits v2's rules and is refused
	// twice over — for the scope and for the stamp.
	ps := ValidateEntry(newV1Entry(t, nil))
	if len(ps) != 2 || problemFields(ps) != "version,scope" {
		t.Fatalf("expected the gate to refuse both the version and the scope, got %v", ps)
	}
}

// Edges ride the same dispatch. They are records too: they carry a version,
// and what a relation may say can narrow without anything about entries
// changing.
func TestEdgesAreJudgedByTheVersionTheyDeclare(t *testing.T) {
	s := CurrentSchema().With(testSchemaVersion, testRuleSet())

	a := newTestEntry(t, nil)
	b := newTestEntry(t, nil)
	atCurrent := newTestEdge(t, EdgeRelatesTo, a.ID, b.ID)
	atTest := newTestEdge(t, EdgeRelatesTo, a.ID, b.ID)
	atTest.Version = testSchemaVersion

	rep := s.ValidateLog("test.jsonl", []Line{
		{Num: 1, Entry: a}, {Num: 2, Entry: b},
		{Num: 3, Edge: atCurrent}, {Num: 4, Edge: atTest},
	})

	if len(rep.Unjudged) != 0 {
		t.Fatalf("both versions are known here: %v", rep.Unjudged)
	}
	if len(rep.Problems) != 1 || rep.Problems[0].Line != 4 || rep.Problems[0].Field != "type" {
		t.Fatalf("expected one type problem on line 4, got %v", rep.Problems)
	}
}

// An edge stamped with one version judged by another version's rules is refused
// for the stamp, which is why version 2 states its own edge rules rather than
// pointing at version 1's — even though the two are rule for rule identical.
// Delegating would have made `link` refuse every edge it wrote.
func TestEachVersionsEdgeRulesRequireThatVersionsStamp(t *testing.T) {
	a, b := mustULID(t), mustULID(t)

	atV2 := newTestEdge(t, EdgeDerivesFrom, a, b) // stamped CurrentSchemaVersion
	if ps := validateEdgeV2(atV2); len(ps) != 0 {
		t.Fatalf("v2's edge rules must accept the edge the tool writes: %v", ps)
	}
	if ps := validateEdgeV1(atV2); problemFields(ps) != "version" {
		t.Fatalf("v1's edge rules must refuse a v2 stamp, and only for the stamp: %v", ps)
	}

	atV1 := newTestEdge(t, EdgeDerivesFrom, a, b)
	atV1.Version = SchemaVersion1
	if ps := validateEdgeV1(atV1); len(ps) != 0 {
		t.Fatalf("v1's edge rules must still accept a v1 edge: %v", ps)
	}
	if ps := validateEdgeV2(atV1); problemFields(ps) != "version" {
		t.Fatalf("v2's edge rules must refuse a v1 stamp: %v", ps)
	}
}

// A test must not be able to change what the binary validates against.
//
// Deliberately not asserting how many versions the table holds: that would
// fail the day a real version is added correctly, which is the opposite of
// what wants guarding. What must stay true is that the version invented here
// is not in it.
func TestTheShippedSchemaDoesNotKnowTheTestVersion(t *testing.T) {
	_ = CurrentSchema().With(testSchemaVersion, testRuleSet())

	atTest := newTestEntry(t, func(e *Entry) { e.Version = testSchemaVersion })
	if _, judged := CurrentSchema().JudgeEntry(atTest); judged {
		t.Fatal("the test version leaked into the schema the binary ships with")
	}
	if _, known := CurrentSchema()[testSchemaVersion]; known {
		t.Fatal("a version that exists only in a test is registered in the shipped table")
	}
	if _, known := CurrentSchema()[CurrentSchemaVersion]; !known {
		t.Fatalf("the shipped table must know the version it writes (%d)", CurrentSchemaVersion)
	}
}

// The failure this catches is silent by construction: stamp records with a
// version the table has no rules for, and every one of them is reported
// unjudged while `ok` stays true. A sweep that passes because it checked
// nothing looks exactly like a sweep that passes.
//
// The next schema change is a version bump, so this is the test that has to
// hold when someone is halfway through making one.
func TestWhatThisBinaryWritesIsWhatItCanJudge(t *testing.T) {
	a := newTestEntry(t, nil)
	b := newTestEntry(t, nil)
	if a.Version != CurrentSchemaVersion {
		t.Fatalf("fixture is stamped %d, expected the current version %d", a.Version, CurrentSchemaVersion)
	}

	rep := ValidateLog("test.jsonl", []Line{
		{Num: 1, Entry: a}, {Num: 2, Entry: b},
		{Num: 3, Edge: newTestEdge(t, EdgeDerivesFrom, a.ID, b.ID)},
	})
	if len(rep.Unjudged) != 0 {
		t.Fatalf("records this binary just wrote must be records it can judge, got %+v", rep.Unjudged)
	}
	if !rep.OK {
		t.Fatalf("and they must pass: %v", rep.Problems)
	}
}

// A RuleSet is a *complete* rule set or it is nothing. Registered with a nil
// half, it sends every record of that version down the unjudged path — which
// reads as tolerance for an old record and is in fact a missing validator.
func TestEveryRegisteredVersionJudgesBothEntriesAndEdges(t *testing.T) {
	s := CurrentSchema()
	if len(s) == 0 {
		t.Fatal("the shipped schema knows no versions at all")
	}
	for v, rs := range s {
		if rs.Entry == nil {
			t.Errorf("version %d is registered with no entry rules", v)
		}
		if rs.Edge == nil {
			t.Errorf("version %d is registered with no edge rules", v)
		}
	}
}

// The write gate reads its rules out of the same table the sweep dispatches
// through, so there is no second place to forget.
func TestTheWriteGateUsesTheRegisteredRulesForTheVersionItStamps(t *testing.T) {
	registered, ok := CurrentSchema()[CurrentSchemaVersion]
	if !ok {
		t.Fatalf("version %d is not registered", CurrentSchemaVersion)
	}
	bad := newTestEntry(t, func(e *Entry) { e.Scope = "team" })

	fromTable := registered.Entry(bad)
	fromGate := ValidateEntry(bad)
	if len(fromTable) != len(fromGate) || len(fromGate) == 0 {
		t.Fatalf("the gate and the table disagree: gate=%v table=%v", fromGate, fromTable)
	}
	for i := range fromGate {
		if fromGate[i] != fromTable[i] {
			t.Errorf("problem %d differs: gate=%v table=%v", i, fromGate[i], fromTable[i])
		}
	}
}

// A record from a binary newer than this one is a normal state of an evolving
// schema, not a corrupt log. It is counted and named so nobody mistakes it for
// checked, and it does not fail the sweep.
func TestUnknownVersionIsUnjudgedRatherThanRejected(t *testing.T) {
	good := newTestEntry(t, nil)
	// Invalid under v1 on purpose: if the sweep ever fell back to today's
	// rules, this scope would produce a problem and the test would say so.
	future := newTestEntry(t, func(e *Entry) { e.Version = 99; e.Scope = "team" })
	futureEdge := newTestEdge(t, "endorses", good.ID, future.ID)
	futureEdge.Version = 99

	rep := ValidateLog("test.jsonl", []Line{
		{Num: 1, Entry: good}, {Num: 2, Entry: future}, {Num: 3, Edge: futureEdge},
	})

	if !rep.OK {
		t.Fatalf("an unrecognised version must not fail the log: %v", rep.Problems)
	}
	if len(rep.Problems) != 0 {
		t.Fatalf("nothing was wrong with what could be read, got %v", rep.Problems)
	}
	if rep.Entries != 2 || rep.Edges != 1 {
		t.Errorf("every record is counted whether or not it could be judged, got %d entries and %d edges", rep.Entries, rep.Edges)
	}
	if len(rep.Unjudged) != 2 {
		t.Fatalf("expected the two v99 records to be listed as unjudged, got %v", rep.Unjudged)
	}
	if rep.Unjudged[0].Line != 2 || rep.Unjudged[0].Version != 99 || rep.Unjudged[0].ID != future.ID {
		t.Errorf("an unjudged record must be findable by hand: %+v", rep.Unjudged[0])
	}
	if rep.Unjudged[1].Line != 3 {
		t.Errorf("expected the edge on line 3 to be unjudged, got %+v", rep.Unjudged[1])
	}
	if !strings.Contains(rep.Unjudged[0].Message, "not checked") &&
		!strings.Contains(rep.Unjudged[0].Message, "were not checked") {
		t.Errorf("the message must say the fields went unchecked, got %q", rep.Unjudged[0].Message)
	}
}

// The graph-wide invariants are properties of the log, not of a record's
// shape, so they still apply to a record nobody could judge. If they did not,
// one record from a newer binary would make every edge pointing at it look
// dangling — manufacturing problems on lines that are perfectly fine.
func TestUnjudgedEntriesStillAnchorEdgesAndStillCollide(t *testing.T) {
	future := newTestEntry(t, func(e *Entry) { e.Version = 99 })

	rep := ValidateLog("test.jsonl", []Line{
		{Num: 1, Entry: future},
		{Num: 2, Entry: newTestEntry(t, func(e *Entry) { e.ID = future.ID })},
	})
	if rep.OK {
		t.Error("a duplicate id is a duplicate id whatever version declared it")
	}

	src := newTestEntry(t, nil)
	rep = ValidateLog("test.jsonl", []Line{
		{Num: 1, Entry: src},
		{Num: 2, Entry: future},
		{Num: 3, Edge: newTestEdge(t, EdgeDerivesFrom, src.ID, future.ID)},
	})
	if !rep.OK {
		t.Fatalf("an edge into an unjudged entry is not dangling: %v", rep.Problems)
	}
}

// Records written before the stamp existed carry no `version` key at all.
// They are version 1 — the stamp was introduced after they were written, and
// nothing on disk is ever rewritten to add it.
func TestAnAbsentVersionKeyReadsAsVersionOne(t *testing.T) {
	id, _ := NewULID(time.Now())
	raw := `{"id":"` + id + `","kind":"decision","slug":"rs256-key-rotation","scope":"plan",` +
		`"at":"2026-08-02T10:00:00Z","about":["auth"],"title":"RS256 over HS256"}`

	l := ParseLine(1, raw)
	if l.Err != nil {
		t.Fatalf("an unstamped line is a normal line: %v", l.Err)
	}
	if l.Entry.Version != 0 {
		t.Fatalf("reading must not invent a stamp the line does not carry, got %d", l.Entry.Version)
	}
	if l.Entry.SchemaVersion() != SchemaVersion1 {
		t.Fatalf("an absent version reads as 1, got %d", l.Entry.SchemaVersion())
	}
	if l.Raw != raw {
		t.Fatal("parsing rewrote the line")
	}

	rep := ValidateLog("test.jsonl", []Line{l})
	if !rep.OK || len(rep.Unjudged) != 0 {
		t.Fatalf("an unstamped record is judged as v1, not left unjudged: %+v", rep)
	}
}

func TestAnAbsentVersionKeyReadsAsVersionOneOnAnEdge(t *testing.T) {
	a, _ := NewULID(time.Now())
	b, _ := NewULID(time.Now())
	e := &Edge{Kind: KindEdge, Type: EdgeSupersedes, From: a, To: b, At: time.Now().UTC().Format(time.RFC3339)}

	if e.SchemaVersion() != SchemaVersion1 {
		t.Fatalf("an absent version reads as 1, got %d", e.SchemaVersion())
	}
	if ps, judged := CurrentSchema().JudgeEdge(e); !judged || len(ps) != 0 {
		t.Fatalf("an unstamped edge is a v1 edge and v1's rules accept it: judged=%v %v", judged, ps)
	}
	// And it cannot be written, now that the current version is not 1. This is
	// the promise record.go makes about the stamp: a writer that forgot cannot
	// reach disk, because the gate judges an unstamped record as version 1 and
	// the gate is version 2's rules.
	if ps := ValidateEdge(e); problemFields(ps) != "version" {
		t.Fatalf("the write gate must refuse an unstamped edge, for the stamp: %v", ps)
	}
}

// The write path never gets more permissive than the current schema. A record
// cannot buy itself lenience on the way in by declaring some other version:
// whatever the read sweep tolerates on disk, `add` and `link` do not.
func TestTheWriteGateRefusesAForeignVersion(t *testing.T) {
	ps := ValidateEntry(newTestEntry(t, func(e *Entry) { e.Version = 99 }))
	if problemFields(ps) != "version" {
		t.Fatalf("expected the write gate to refuse a v99 record, got %v", ps)
	}

	a, _ := NewULID(time.Now())
	b, _ := NewULID(time.Now())
	edge := newTestEdge(t, EdgeRelatesTo, a, b)
	edge.Version = 99
	if ps := ValidateEdge(edge); problemFields(ps) != "version" {
		t.Fatalf("expected the write gate to refuse a v99 edge, got %v", ps)
	}
}

// The write gate does not dispatch, and that asymmetry is the whole split: it
// judges against the current schema even when a caller hands it a rule set
// that would say otherwise.
func TestTheWriteGateDoesNotDispatch(t *testing.T) {
	s := CurrentSchema().With(testSchemaVersion, testRuleSet())
	atTest := newTestEntry(t, func(e *Entry) { e.Version = testSchemaVersion })

	if _, judged := s.JudgeEntry(atTest); !judged {
		t.Fatal("the read sweep should judge a version it was given rules for")
	}
	if ps := ValidateEntry(atTest); len(ps) == 0 {
		t.Fatal("the write gate must still refuse it: the current schema is v1")
	}
}
