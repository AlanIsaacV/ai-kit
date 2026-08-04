package memory

import (
	"fmt"
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
	// No origin at all: what a record written before `add` insisted on a plan
	// looks like. It is unreachable by any bounded query, which is exactly why
	// writing one is refused.
	orphan := entry(t, KindDecision, "Nobody knows where this came from", "auth")
	all := []*Entry{mine, theirs, orphan}

	res := Query(logOf(all, nil), Filter{Plan: "jwt-migration"})
	if res.Count != 1 || res.Records[0].ID != mine.ID {
		t.Fatalf("plan=jwt-migration should return only its own record, got %+v", res.Records)
	}

	res = Query(logOf(all, nil), Filter{Plan: "jwt-migration", About: []string{"orders"}})
	if res.Count != 0 {
		t.Fatalf("the plan bound and the tag bound both apply, got %d", res.Count)
	}

	if got := Query(logOf(all, nil), Filter{}).Count; got != 3 {
		t.Fatalf("no plan named means no plan filter, got %d", got)
	}
	if got := Query(logOf(all, nil), Filter{Plan: "order-cancellation"}).Count; got != 1 {
		t.Fatalf("the other plan's record is reachable by its own slug, got %d", got)
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

// The sequential number is computed at read time. Nothing stores it, which is
// what makes aggregating two repos a concatenation instead of a renumbering
// pass that rewrites content.
func TestRenderNumbersChronologically(t *testing.T) {
	first := entry(t, KindADR, "First decision record", "auth")
	second := entry(t, KindADR, "Second decision record", "orders")
	other := entry(t, KindDecision, "Not an ADR", "auth")

	out := Render([]*Entry{second, other, first}, nil, KindADR, ScopeRepo)

	iFirst := strings.Index(out, "ADR-0001 — First decision record")
	iSecond := strings.Index(out, "ADR-0002 — Second decision record")
	if iFirst < 0 || iSecond < 0 {
		t.Fatalf("expected both ADRs numbered in write order:\n%s", out)
	}
	if iFirst > iSecond {
		t.Error("numbering ran backwards")
	}
	if strings.Contains(out, "Not an ADR") {
		t.Error("render leaked a record of another kind")
	}
}

func TestRenderMarksSuperseded(t *testing.T) {
	old := entry(t, KindADR, "Old record", "auth")
	fresh := entry(t, KindADR, "New record", "auth")
	out := Render([]*Entry{old, fresh}, []*Edge{supersedes(t, fresh, old)}, KindADR, ScopeRepo)
	if !strings.Contains(out, "status: superseded") {
		t.Fatalf("a superseded record must say so:\n%s", out)
	}
}
