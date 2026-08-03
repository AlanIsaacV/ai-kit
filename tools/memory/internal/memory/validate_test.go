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
	e := &Entry{
		ID:    id,
		Kind:  KindDecision,
		Slug:  "rs256-key-rotation",
		Scope: ScopePlan,
		At:    time.Now().UTC().Format(time.RFC3339),
		About: []string{"auth"},
		Title: "RS256 over HS256",
	}
	if mutate != nil {
		mutate(e)
	}
	return e
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

func TestValidateEdge(t *testing.T) {
	a, _ := NewULID(time.Now())
	b, _ := NewULID(time.Now())
	now := time.Now().UTC().Format(time.RFC3339)

	good := &Edge{Kind: KindEdge, Type: EdgeDerivesFrom, From: a, To: b, At: now}
	if ps := ValidateEdge(good); len(ps) != 0 {
		t.Fatalf("expected no problems, got %v", ps)
	}

	bad := &Edge{Kind: KindEdge, Type: "inspired_by", From: a, To: b, At: now}
	if ps := ValidateEdge(bad); len(ps) == 0 || ps[0].Field != "type" {
		t.Errorf("unknown edge type should be rejected, got %v", ps)
	}

	self := &Edge{Kind: KindEdge, Type: EdgeRelatesTo, From: a, To: a, At: now}
	if ps := ValidateEdge(self); len(ps) == 0 {
		t.Errorf("self-edge should be rejected")
	}
}

// A dangling edge is silent: the graph walk just returns less than it should
// and nothing says the answer was partial. That is why validate hunts for it.
func TestValidateLogFindsDanglingEdge(t *testing.T) {
	e := newTestEntry(t, nil)
	ghost, _ := NewULID(time.Now())
	now := time.Now().UTC().Format(time.RFC3339)

	lines := []Line{
		{Num: 1, Entry: e},
		{Num: 2, Edge: &Edge{Kind: KindEdge, Type: EdgeDerivesFrom, From: e.ID, To: ghost, At: now}},
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

// A typo in a key must surface, not vanish into an ignored field.
func TestParseLineRejectsUnknownField(t *testing.T) {
	l := ParseLine(1, `{"id":"01JXKQ7M4T8YZPABCDEFGHJKMN","kind":"decision","titel":"typo"}`)
	if l.Err == nil {
		t.Fatal("expected an unknown field to be rejected")
	}
}
