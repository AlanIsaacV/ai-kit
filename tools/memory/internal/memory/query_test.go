package memory

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

// fileOf is the lines a log would hold for these records, in exactly the
// positions given, numbered the way the store numbers them.
//
// Order is spelled out rather than assembled, because two of the rules under
// test here are stated in terms of the file itself — "ties broken by the later
// line" — and a fixture that could not put a record on a chosen line could not
// exercise them.
func fileOf(records ...any) []Line {
	lines := make([]Line, 0, len(records))
	for i, r := range records {
		l := Line{Num: i + 1}
		switch v := r.(type) {
		case *Entry:
			l.Entry = v
		case *Edge:
			l.Edge = v
		default:
			panic(fmt.Sprintf("fileOf: %T is neither an entry nor an edge", r))
		}
		lines = append(lines, l)
	}
	return lines
}

// logOf is fileOf for the fixtures that do not turn on file order: every entry,
// then every edge. That is a log's usual shape anyway — an edge cannot be
// written before both of its endpoints exist.
func logOf(entries []*Entry, edges []*Edge) []Line {
	records := make([]any, 0, len(entries)+len(edges))
	for _, e := range entries {
		records = append(records, e)
	}
	for _, e := range edges {
		records = append(records, e)
	}
	return fileOf(records...)
}

func entry(t *testing.T, kind Kind, title string, about ...string) *Entry {
	t.Helper()
	// Distinct ids in write order, since ULID ordering is what Query sorts on.
	time.Sleep(time.Millisecond)
	return newTestEntry(t, func(e *Entry) {
		e.Kind = kind
		e.Title = title
		e.Slug = deriveTestSlug(title)
		e.About = about
	})
}

func deriveTestSlug(s string) string {
	out := strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			return r
		case r >= 'A' && r <= 'Z':
			return r + 32
		default:
			return '-'
		}
	}, s)
	return strings.Trim(out, "-")
}

func supersedes(t *testing.T, newer, older *Entry) *Edge {
	t.Helper()
	return newTestEdge(t, EdgeSupersedes, newer.ID, older.ID)
}

func TestQueryFiltersByAboutAndKind(t *testing.T) {
	a := entry(t, KindDecision, "RS256 over HS256", "auth")
	b := entry(t, KindDecision, "Cancel is async", "orders")
	c := entry(t, KindWontFix, "Concurrent cancel is impossible", "orders")
	all := []*Entry{a, b, c}

	res := Query(logOf(all, nil), Filter{About: []string{"orders"}})
	if res.Count != 2 {
		t.Fatalf("about=orders: want 2, got %d", res.Count)
	}

	res = Query(logOf(all, nil), Filter{About: []string{"orders"}, Kinds: []Kind{KindWontFix}})
	if res.Count != 1 || res.Records[0].ID != c.ID {
		t.Fatalf("about+kind: want just the wont-fix, got %+v", res.Records)
	}
}

// --plan is what replaced one file per plan. It bounds a query to the records of
// one objective out of a log that holds every objective's, which is the whole
// reason the scopes could collapse without a broad question turning into a
// context dump.
func TestQueryFiltersByOriginPlan(t *testing.T) {
	mine := entry(t, KindDecision, "RS256 over HS256", "auth")
	mine.Origin = &Origin{Plan: "jwt-migration"}
	theirs := entry(t, KindDecision, "Cancel is async", "orders")
	theirs.Origin = &Origin{Plan: "order-cancellation"}
	all := []*Entry{mine, theirs}

	res := Query(logOf(all, nil), Filter{Plan: "jwt-migration"})
	if res.Count != 1 || res.Records[0].ID != mine.ID {
		t.Fatalf("plan=jwt-migration should return only its own record, got %+v", res.Records)
	}

	res = Query(logOf(all, nil), Filter{Plan: "jwt-migration", About: []string{"orders"}})
	if res.Count != 0 {
		t.Fatalf("the plan bound and the tag bound both apply, got %d", res.Count)
	}

	if got := Query(logOf(all, nil), Filter{}).Count; got != 2 {
		t.Fatalf("no plan named means no plan filter, got %d", got)
	}
	if got := Query(logOf(all, nil), Filter{Plan: "order-cancellation"}).Count; got != 1 {
		t.Fatalf("the other plan's record is reachable by its own slug, got %d", got)
	}
}

// A record with no `origin.plan` belongs to the repository, so it is in scope for
// every tree in it. That is the whole of the widening and it is one line in
// Query, which is why the guard has to be on both halves at once: the plan-less
// record comes back from a bound naming any slug, and the plan-carrying one is
// still excluded from a bound naming a different one. A filter that let
// everything through would pass the first half alone.
//
// The plan-less record used to be the shape `add` refused to write; the tag it
// carries is the one the repository is keyed by rather than a tree's.
func TestQueryPlanBoundReturnsTheRepositorysOwnRecordsToo(t *testing.T) {
	tree := entry(t, KindDecision, "Cancel is async", "orders")
	tree.Origin = &Origin{Plan: "order-cancellation"}
	repo := entry(t, KindPreference, "Migrations go through Atlas", "persistence")
	all := []*Entry{tree, repo}

	for _, plan := range []string{"order-cancellation", "some-other-tree", ""} {
		res := Query(logOf(all, nil), Filter{Plan: plan, Kinds: []Kind{KindPreference}})
		if res.Count != 1 || res.Records[0].ID != repo.ID {
			t.Errorf("plan=%q must still see the repository's own record, got %+v", plan, res.Records)
		}
	}

	res := Query(logOf(all, nil), Filter{Plan: "some-other-tree", Kinds: []Kind{KindDecision}})
	if res.Count != 0 {
		t.Fatalf("the widening must not collapse the filter: a record naming a plan stays out of another plan's query, got %+v", res.Records)
	}
	if got := Query(logOf(all, nil), Filter{Plan: "order-cancellation"}).Count; got != 2 {
		t.Errorf("a bounded read returns its tree's records and the repository's, got %d", got)
	}
}

// The count-before-dump guard bounds by plan now, not by file, and it has to
// still bound: a plan whose own matches are over the threshold gets the count
// and the hint, not a partial slice of one objective's history.
func TestQueryWithholdsOverTheThresholdWithinOnePlan(t *testing.T) {
	var all []*Entry
	for i := 0; i < DefaultThreshold+5; i++ {
		e := entry(t, KindDecision, "decision", "auth")
		e.Origin = &Origin{Plan: "busy"}
		all = append(all, e)
	}
	quiet := entry(t, KindDecision, "decision", "auth")
	quiet.Origin = &Origin{Plan: "quiet"}
	all = append(all, quiet)

	res := Query(logOf(all, nil), Filter{Plan: "busy"})
	if res.Count != DefaultThreshold+5 {
		t.Fatalf("the count must cover only the named plan, got %d", res.Count)
	}
	if res.Returned != 0 || !res.Truncated || res.Hint == "" {
		t.Fatalf("over the threshold the records are withheld: returned=%d truncated=%v", res.Returned, res.Truncated)
	}

	res = Query(logOf(all, nil), Filter{Plan: "quiet"})
	if res.Count != 1 || res.Truncated {
		t.Fatalf("the quiet plan is unaffected by the busy one, got count=%d truncated=%v", res.Count, res.Truncated)
	}
}

// A path-shaped tag should catch the files under it, or every query would have
// to know the exact file that was tagged.
func TestQueryAboutMatchesPathPrefix(t *testing.T) {
	e := entry(t, KindDecision, "Token parsing lives in the middleware", "pkg/auth/middleware.go")
	res := Query(logOf([]*Entry{e}, nil), Filter{About: []string{"pkg/auth/"}})
	if res.Count != 1 {
		t.Fatalf("prefix tag should match the file under it, got %d", res.Count)
	}
}

// Validity is derived from the graph, never stored — so --current has to read
// the edges rather than a field.
func TestQueryCurrentDropsSupersededEntries(t *testing.T) {
	old := entry(t, KindDecision, "Use HS256", "auth")
	fresh := entry(t, KindDecision, "Use RS256", "auth")
	edges := []*Edge{supersedes(t, fresh, old)}
	all := []*Entry{old, fresh}

	if got := Query(logOf(all, edges), Filter{}).Count; got != 2 {
		t.Fatalf("without --current both should be visible, got %d", got)
	}
	res := Query(logOf(all, edges), Filter{CurrentOnly: true})
	if res.Count != 1 || res.Records[0].ID != fresh.ID {
		t.Fatalf("--current should leave only the replacement, got %+v", res.Records)
	}
}

// The retrieval contract: a broad question gets a count, not a context dump.
// Handing back a partial slice would read as "here is the answer" when it is a
// fragment of one, so the record list is deliberately empty.
func TestQueryReturnsCountInsteadOfFloodingContext(t *testing.T) {
	var all []*Entry
	for i := 0; i < DefaultThreshold+5; i++ {
		all = append(all, entry(t, KindDecision, "decision", "auth"))
	}

	res := Query(logOf(all, nil), Filter{About: []string{"auth"}})
	if res.Count != DefaultThreshold+5 {
		t.Fatalf("count must always be the true total, got %d", res.Count)
	}
	if res.Returned != 0 || len(res.Records) != 0 {
		t.Fatalf("over threshold must return no records, got %d", res.Returned)
	}
	if !res.Truncated || res.Hint == "" {
		t.Fatal("the caller has to be told why it got nothing, and what to do next")
	}
}

func TestQueryUnderThresholdReturnsEverything(t *testing.T) {
	var all []*Entry
	for i := 0; i < DefaultThreshold; i++ {
		all = append(all, entry(t, KindDecision, "decision", "auth"))
	}
	res := Query(logOf(all, nil), Filter{})
	if res.Truncated || res.Returned != DefaultThreshold {
		t.Fatalf("at the threshold everything should come back, got returned=%d truncated=%v", res.Returned, res.Truncated)
	}
}

func TestQueryExplicitLimitTakesMostRecent(t *testing.T) {
	var all []*Entry
	for i := 0; i < 40; i++ {
		all = append(all, entry(t, KindDecision, "decision", "auth"))
	}
	res := Query(logOf(all, nil), Filter{Limit: 3})
	if res.Count != 40 || res.Returned != 3 || !res.Truncated {
		t.Fatalf("limit: count=%d returned=%d truncated=%v", res.Count, res.Returned, res.Truncated)
	}
	newest := all[len(all)-1]
	if res.Records[0].ID != newest.ID {
		t.Fatal("a limited query must return the most recent matches first")
	}
}

func TestQueryThresholdCanBeDisabled(t *testing.T) {
	var all []*Entry
	for i := 0; i < DefaultThreshold+10; i++ {
		all = append(all, entry(t, KindDecision, "decision", "auth"))
	}
	res := Query(logOf(all, nil), Filter{Threshold: -1})
	if res.Returned != len(all) {
		t.Fatalf("threshold=-1 should hand back everything, got %d", res.Returned)
	}
}

// An answered question is exactly as irrelevant to "what must I still hold in
// mind" as a superseded decision, so --current has to drop it too — otherwise
// every session re-reads questions that were closed turns ago.
func TestQueryCurrentDropsAnsweredQuestions(t *testing.T) {
	question := entry(t, KindOpenQuestion, "Sync or async cancel?", "orders")
	assumption := entry(t, KindAssumption, "Amounts are never negative", "orders")
	answer := entry(t, KindDecision, "Cancel is synchronous", "orders")
	now := time.Now().UTC().Format(time.RFC3339)
	edges := []*Edge{
		{Kind: KindEdge, Type: EdgeAnswers, From: answer.ID, To: question.ID, At: now},
	}
	all := []*Entry{question, assumption, answer}

	res := Query(logOf(all, edges), Filter{Kinds: []Kind{KindOpenQuestion, KindAssumption}, CurrentOnly: true})
	if res.Count != 1 || res.Records[0].ID != assumption.ID {
		t.Fatalf("only the still-open assumption should remain, got %+v", res.Records)
	}

	if got := Query(logOf(all, edges), Filter{Kinds: []Kind{KindOpenQuestion}}).Count; got != 1 {
		t.Fatalf("without --current the answered question is still history, got %d", got)
	}
}

func TestQueryText(t *testing.T) {
	a := entry(t, KindDecision, "RS256 over HS256", "auth")
	a.Body = "Per-service key rotation requires asymmetric signing."
	b := entry(t, KindDecision, "Cancel is async", "orders")

	res := Query(logOf([]*Entry{a, b}, nil), Filter{Text: "KEY ROTATION"})
	if res.Count != 1 || res.Records[0].ID != a.ID {
		t.Fatalf("case-insensitive body search failed: %+v", res.Records)
	}
}

func TestQueryByIDReturnsThoseRecordsAndNamesTheIDsItCouldNotFind(t *testing.T) {
	a := entry(t, KindDecision, "RS256 over HS256", "auth")
	b := entry(t, KindDecision, "Cancel is async", "orders")
	c := entry(t, KindWontFix, "Concurrent cancel is impossible", "orders")
	absent := "01JXKQ7M4T8YZPABCDEFGHJKMN"
	all := []*Entry{a, b, c}

	res := Query(logOf(all, nil), Filter{ID: []string{a.ID, absent, c.ID}})
	if res.Count != 2 || res.Returned != 2 {
		t.Fatalf("two of the three ids are in the log: count=%d returned=%d", res.Count, res.Returned)
	}
	if res.Records[0].ID != c.ID || res.Records[1].ID != a.ID {
		t.Fatalf("--id returns exactly the ids asked for, most recent first, got %+v", res.Records)
	}
	if len(res.NotFound) != 1 || res.NotFound[0] != absent {
		t.Fatalf("the id nothing carries has to come back as not found, got %v", res.NotFound)
	}
	if res.Truncated || res.Hint != "" {
		t.Fatalf("an id that matches nothing is an answer, not a truncation: truncated=%v hint=%q", res.Truncated, res.Hint)
	}

	res = Query(logOf(all, nil), Filter{ID: []string{absent}})
	if res.Count != 0 || len(res.Records) != 0 {
		t.Fatalf("an id nothing carries must not invent a record, got %+v", res.Records)
	}
	if len(res.NotFound) != 1 || res.NotFound[0] != absent {
		t.Fatalf("and it is still reported, got %v", res.NotFound)
	}

	if got := Query(logOf(all, nil), Filter{}).Count; got != 3 {
		t.Fatalf("without --id nothing is bounded by it, got %d", got)
	}
	if res := Query(logOf(all, nil), Filter{}); res.NotFound != nil {
		t.Fatalf("a query that named no id has nothing to report as missing, got %v", res.NotFound)
	}
}

func TestQueryTimeBoundsIncludeTheRecordsSittingOnThem(t *testing.T) {
	since, until := "2026-03-01T12:00:00Z", "2026-03-01T12:01:00Z"
	stamps := []string{
		"2026-03-01T11:59:59Z",
		since,
		"2026-03-01T12:00:01Z",
		"2026-03-01T12:00:59Z",
		until,
		"2026-03-01T12:01:01Z",
	}
	want := []bool{false, true, true, true, true, false}

	var all []*Entry
	for _, at := range stamps {
		e := entry(t, KindDecision, "decision", "auth")
		e.At = at
		all = append(all, e)
	}

	res := Query(logOf(all, nil), Filter{Since: since, Until: until})
	inside := map[string]bool{}
	for _, r := range res.Records {
		inside[r.At] = true
	}
	for i, at := range stamps {
		if inside[at] != want[i] {
			t.Errorf("at %s: inside=%v, want %v, for bounds %s..%s where both bounds are inclusive", at, inside[at], want[i], since, until)
		}
	}
	if res.Count != 4 {
		t.Fatalf("four of the six records are on or inside the bounds, got %d", res.Count)
	}
}

func TestCountReportsWhatTheSameFilterWouldHaveReturned(t *testing.T) {
	old := entry(t, KindDecision, "Use HS256", "auth")
	old.At = "2026-03-01T09:00:00Z"
	fresh := entry(t, KindDecision, "RS256 over HS256", "auth")
	fresh.Body = "Per-service key rotation requires asymmetric signing."
	fresh.At = "2026-03-02T09:00:00Z"
	fresh.Origin = &Origin{Plan: "jwt-migration"}
	orders := entry(t, KindDecision, "Cancel is async", "orders")
	orders.At = "2026-03-03T09:00:00Z"
	wont := entry(t, KindWontFix, "Concurrent cancel is impossible", "orders")
	wont.At = "2026-03-04T09:00:00Z"
	pref := entry(t, KindPreference, "Migrations go through Atlas", "persistence")
	pref.At = "2026-03-05T09:00:00Z"

	lines := logOf([]*Entry{old, fresh, orders, wont, pref}, []*Edge{supersedes(t, fresh, old)})

	cases := []struct {
		name string
		f    Filter
	}{
		{"about", Filter{About: []string{"auth"}}},
		{"kind", Filter{Kinds: []Kind{KindDecision}}},
		{"current", Filter{CurrentOnly: true}},
		{"slug", Filter{Slug: orders.Slug}},
		{"text", Filter{Text: "KEY ROTATION"}},
		{"plan", Filter{Plan: "jwt-migration"}},
		{"id", Filter{ID: []string{fresh.ID, wont.ID, "01JXKQ7M4T8YZPABCDEFGHJKMN"}}},
		{"since and until", Filter{Since: "2026-03-02T09:00:00Z", Until: "2026-03-04T09:00:00Z"}},
		{"every bound at once", Filter{
			ID:          []string{old.ID, fresh.ID, orders.ID},
			About:       []string{"auth"},
			Kinds:       []Kind{KindDecision},
			Text:        "rs256",
			Since:       "2026-03-01T09:00:00Z",
			Until:       "2026-03-05T09:00:00Z",
			CurrentOnly: true,
		}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			records := Query(lines, tc.f)
			counted := tc.f
			counted.CountOnly = true
			got := Query(lines, counted)

			if len(records.Records) == 0 {
				t.Fatalf("the filter has to match something for its count to say anything: %+v", tc.f)
			}
			if got.Count != len(records.Records) {
				t.Fatalf("--count must report the records the same filter hands over: count=%d records=%d", got.Count, len(records.Records))
			}
			if got.Returned != 0 || len(got.Records) != 0 {
				t.Fatalf("--count returns no records, got returned=%d records=%d", got.Returned, len(got.Records))
			}
			if len(got.NotFound) != len(records.NotFound) {
				t.Fatalf("--count reports the ids it could not find exactly as the query does: %v against %v", got.NotFound, records.NotFound)
			}
		})
	}
}

func TestTheNewFiltersDoNotHoleTheCountBeforeDumpGuard(t *testing.T) {
	total := DefaultThreshold + 5
	var all []*Entry
	var ids []string
	for i := 0; i < total; i++ {
		e := entry(t, KindDecision, "decision", "auth")
		e.At = "2026-03-01T12:00:00Z"
		all = append(all, e)
		ids = append(ids, e.ID)
	}
	lines := logOf(all, nil)

	cases := []struct {
		name string
		f    Filter
	}{
		{"by id", Filter{ID: ids}},
		{"inside the bounds", Filter{Since: "2026-03-01T12:00:00Z", Until: "2026-03-01T12:00:00Z"}},
	}

	for _, tc := range cases {
		res := Query(lines, tc.f)
		if res.Count != total {
			t.Fatalf("%s: the count is still the true total, got %d", tc.name, res.Count)
		}
		if res.Returned != 0 || len(res.Records) != 0 || !res.Truncated || res.Hint == "" {
			t.Fatalf("%s: over the threshold the records are still withheld: returned=%d truncated=%v hint=%q",
				tc.name, res.Returned, res.Truncated, res.Hint)
		}

		counted := tc.f
		counted.CountOnly = true
		res = Query(lines, counted)
		if res.Count != total || res.Returned != 0 || len(res.Records) != 0 {
			t.Fatalf("%s with --count: a number and never records, got count=%d returned=%d", tc.name, res.Count, res.Returned)
		}
	}

	for _, f := range []Filter{{CountOnly: true}, {CountOnly: true, Limit: 5}, {CountOnly: true, Threshold: -1}} {
		res := Query(lines, f)
		if res.Count != total || res.Returned != 0 || len(res.Records) != 0 {
			t.Fatalf("--count with %+v is a number and never records, got count=%d returned=%d", f, res.Count, res.Returned)
		}
	}

	under := Query(logOf(all[:3], nil), Filter{CountOnly: true})
	if under.Count != 3 || under.Returned != 0 || len(under.Records) != 0 {
		t.Fatalf("under the threshold --count is still only a number, got count=%d returned=%d", under.Count, under.Returned)
	}
}

// Both directions matter: from an ADR you want its sources, and from a
// decision you want the ADR that consumed it.
func TestGraphWalksBothDirections(t *testing.T) {
	decision := entry(t, KindDecision, "Use RS256", "auth")
	deviation := entry(t, KindDeviation, "HS256 cannot rotate per service", "auth")
	adr := entry(t, KindADR, "Asymmetric signing for service tokens", "auth")
	now := time.Now().UTC().Format(time.RFC3339)
	edges := []*Edge{
		{Kind: KindEdge, Type: EdgeDerivesFrom, From: adr.ID, To: decision.ID, At: now},
		{Kind: KindEdge, Type: EdgeDerivesFrom, From: adr.ID, To: deviation.ID, At: now},
	}
	all := []*Entry{decision, deviation, adr}

	forward := Graph(all, edges, adr.ID, 1)
	if len(forward) != 3 {
		t.Fatalf("from the ADR we should reach both sources, got %d", len(forward))
	}

	backward := Graph(all, edges, decision.ID, 1)
	if len(backward) != 2 {
		t.Fatalf("from a source we should reach the ADR, got %d", len(backward))
	}
	if !backward[1].Backward {
		t.Error("a reverse hop should be flagged as such")
	}

	two := Graph(all, edges, decision.ID, 2)
	if len(two) != 3 {
		t.Fatalf("depth 2 should reach the sibling source, got %d", len(two))
	}
}

func TestGraphUnknownRoot(t *testing.T) {
	if Graph(nil, nil, "01JXKQ7M4T8YZPABCDEFGHJKMN", 2) != nil {
		t.Fatal("an unknown root must be distinguishable from an isolated one")
	}
}

func TestProjectionReturnsExactlyTheFieldsItWasNamed(t *testing.T) {
	store := Open(filepath.Join(t.TempDir(), ".ai-kit", "memory.jsonl"))
	first := entry(t, KindDecision, "RS256 over HS256", "auth")
	first.Body = "Per-service key rotation requires asymmetric signing."
	first.Origin = &Origin{Plan: "jwt-migration"}
	second := entry(t, KindWontFix, "Concurrent cancel is impossible", "orders")
	if err := store.Append(first, second); err != nil {
		t.Fatal(err)
	}
	lines, err := store.Read()
	if err != nil {
		t.Fatal(err)
	}
	res := Query(lines, Filter{})
	if len(res.Records) != 2 {
		t.Fatalf("the log holds two records, the query returned %d", len(res.Records))
	}

	cases := []struct {
		name   string
		fields []string
	}{
		{"one field, and not id", []string{"title"}},
		{"several fields", []string{"id", "kind", "title", "body"}},
		{"every field there is", ProjectableFields()},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p, err := NewProjection(tc.fields, WholeBody)
			if err != nil {
				t.Fatal(err)
			}
			raw, err := json.Marshal(p.Apply(res.Records))
			if err != nil {
				t.Fatal(err)
			}
			var got []map[string]json.RawMessage
			if err := json.Unmarshal(raw, &got); err != nil {
				t.Fatalf("a projected record must still be JSON: %v\n%s", err, raw)
			}
			if len(got) != len(res.Records) {
				t.Fatalf("projecting %d records returned %d", len(res.Records), len(got))
			}
			want := slices.Clone(tc.fields)
			slices.Sort(want)
			for _, rec := range got {
				keys := make([]string, 0, len(rec))
				for k := range rec {
					keys = append(keys, k)
				}
				slices.Sort(keys)
				if !slices.Equal(keys, want) {
					t.Fatalf("--fields %v returned the keys %v", tc.fields, keys)
				}
			}
		})
	}
}
