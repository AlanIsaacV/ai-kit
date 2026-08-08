package memory

import (
	"bytes"
	"testing"
	"time"
)

func areaNamed(t *testing.T, areas []Area, name string) Area {
	t.Helper()
	for _, a := range areas {
		if a.Name == name {
			return a
		}
	}
	t.Fatalf("no area %q in %v", name, areaNames(areas))
	return Area{}
}

func areaNames(areas []Area) []string {
	out := make([]string, 0, len(areas))
	for _, a := range areas {
		out = append(out, a.Name)
	}
	return out
}

// Every tag is an area, wherever it sits in the list. The one in second position
// is the case that matters: a preference tagged `migrations,db` has to stay
// findable under `db`, and the row for `db` has to say it is not what the record
// claims to be about.
func TestAreasEnumeratesEveryTagAndNamesWhichOneIsTheKey(t *testing.T) {
	e := entry(t, KindPreference, "Migrations go through Atlas", "migrations", "db")

	areas := Areas(fileOf(e), Filter{})
	if got := areaNames(areas); len(got) != 2 || got[0] != "db" || got[1] != "migrations" {
		t.Fatalf("both tags, sorted by name: got %v", got)
	}

	if key := areaNamed(t, areas, "migrations"); key.Live == nil || !key.Live.Key {
		t.Errorf("the first tag is the record's key: %+v", key.Live)
	}
	if second := areaNamed(t, areas, "db"); second.Live == nil || second.Live.Key {
		t.Errorf("a tag in second position is enumerated but is not the key: %+v", second.Live)
	}
	if got := areaNamed(t, areas, "db"); got.Live.ID != e.ID || got.Live.Title != e.Title {
		t.Errorf("the row names the record that answers for it, got %+v", got.Live)
	}
}

// The count is over every record carrying the tag; liveness is a separate
// question asked of the same set. A superseded answer still counts, because the
// number a reader wants is how many times this area has been answered.
func TestAreasCountsTheSettledAndNamesOnlyTheLive(t *testing.T) {
	older := entry(t, KindPreference, "Persistence is GORM", "persistence")
	newer := entry(t, KindPreference, "Persistence is sqlc", "persistence")

	areas := Areas(logOf([]*Entry{older, newer}, []*Edge{supersedes(t, newer, older)}), Filter{})
	a := areaNamed(t, areas, "persistence")
	if a.Records != 2 {
		t.Errorf("both records carry the tag: got %d", a.Records)
	}
	if a.Live == nil || a.Live.ID != newer.ID {
		t.Errorf("the live answer is the one nothing supersedes, got %+v", a.Live)
	}
}

// An area every record of which is settled is still an area. It reports no live
// answer rather than disappearing, since "answered and retired" is not the same
// claim as "never written about".
func TestAnAreaWhoseEveryRecordIsSettledKeepsItsRowAndLosesItsAnswer(t *testing.T) {
	retired := entry(t, KindPreference, "Persistence is GORM", "persistence")
	replacement := entry(t, KindPreference, "Persistence is sqlc", "orm")

	areas := Areas(logOf([]*Entry{retired, replacement}, []*Edge{supersedes(t, replacement, retired)}), Filter{})
	a := areaNamed(t, areas, "persistence")
	if a.Records != 1 {
		t.Errorf("the settled record is still counted: got %d", a.Records)
	}
	if a.Live != nil {
		t.Errorf("nothing survives --current under this tag, so there is no live answer: %+v", a.Live)
	}
}

// The plan bound is Query's, not a second reading of it: a record the repository
// owns is enumerated under every plan, and one that names a plan stays out of
// another plan's enumeration.
func TestAreasInheritsTheRuleThatAPlanLessRecordBelongsToEveryPlan(t *testing.T) {
	owned := entry(t, KindPreference, "Migrations go through Atlas", "migrations")
	theirs := entry(t, KindDecision, "Cancel is async", "orders")
	theirs.Origin = &Origin{Plan: "tree-a"}

	areas := Areas(logOf([]*Entry{owned, theirs}, nil), Filter{Plan: "tree-b"})
	if got := areaNames(areas); len(got) != 1 || got[0] != "migrations" {
		t.Fatalf("only the repository's own record is in scope for another tree: got %v", got)
	}
	if got := areaNames(Areas(logOf([]*Entry{owned, theirs}, nil), Filter{Plan: "tree-a"})); len(got) != 2 {
		t.Fatalf("a tree's own records are enumerated beside the repository's: got %v", got)
	}
}

// --kind narrows what is counted, and the counts follow it.
func TestAreasNarrowsByKind(t *testing.T) {
	pref := entry(t, KindPreference, "Migrations go through Atlas", "migrations")
	other := entry(t, KindDecision, "Migrate on boot", "migrations")
	log := logOf([]*Entry{pref, other}, nil)

	if got := areaNamed(t, Areas(log, Filter{}), "migrations").Records; got != 2 {
		t.Errorf("unfiltered, both records carry the tag: got %d", got)
	}
	a := areaNamed(t, Areas(log, Filter{Kinds: []Kind{KindPreference}}), "migrations")
	if a.Records != 1 || a.Live == nil || a.Live.ID != pref.ID {
		t.Errorf("--kind preference counts and names only the preference: %d, %+v", a.Records, a.Live)
	}
}

// Case is not a distinction here, because it is not one to `--about`: a query for
// `persistence` returns both of these, so an enumeration that split them into two
// rows of one record each would contradict the count that query returns.
func TestAreasFoldsCaseTheWayMatchingFoldsIt(t *testing.T) {
	shouty := entry(t, KindPreference, "Persistence is GORM", "Persistence")
	quiet := entry(t, KindPreference, "Persistence is sqlc", "persistence")

	log := logOf([]*Entry{shouty, quiet}, nil)
	areas := Areas(log, Filter{})
	if got := areaNames(areas); len(got) != 1 || got[0] != "persistence" {
		t.Fatalf("one area under the folded spelling: got %v", got)
	}
	if got := areas[0].Records; got != 2 {
		t.Errorf("both records carry it: got %d", got)
	}
	// The claim the fold rests on, checked rather than assumed: matching really
	// does return both, so the one row is the same population the count is over.
	if got := Query(log, Filter{About: []string{"persistence"}}).Count; got != areas[0].Records {
		t.Errorf("the row's count must be the count a query for that name gives back: %d vs %d",
			areas[0].Records, got)
	}
}

// Padding is *not* stripped, and that asymmetry with the case fold is what this
// pins: matchesAbout folds case and does not trim, so two spellings that differ
// only in padding match separately and are therefore two areas. For a Go caller
// holding a reported name and passing it straight to Query, the round trip below
// holds — that is the whole of what this test crosses.
//
// **It does not hold through the CLI, and a reader who assumes it does will be
// wrong.** `memory add`/`query` route --about through splitList, which trims the
// *wanted* side while the stored side keeps whatever is in the log. So against a
// log carrying "  padded-tag  ", `query --about '  padded-tag  '` and
// `query --about padded-tag` both return count 0: the padded spelling never
// reaches matchesAbout, and the trimmed one is not what is stored. `areas` still
// reports the row, under a name beginning with a space, that nothing typed at a
// shell will answer.
//
// Only a hand-edited log can get into that state, since splitList trims on the
// way in too — which is why this is a wart and not a defect. Removing the trim
// from normalizeTag moved the mismatch rather than removing it; what it bought is
// that the row now says exactly what is on disk instead of a tidied spelling that
// is equally unqueryable. Which side should eventually trim is open question
// `01KZHJ6EG06DVHXX2ZCMW3EWX8`.
func TestAPaddedTagIsReportedWithItsPaddingBecauseMatchingKeepsIt(t *testing.T) {
	padded := entry(t, KindPreference, "Persistence is GORM", " persistence ")
	plain := entry(t, KindPreference, "Persistence is sqlc", "persistence")

	log := logOf([]*Entry{padded, plain}, nil)
	areas := Areas(log, Filter{})
	if got := areaNames(areas); len(got) != 2 {
		t.Fatalf("two spellings matching separately are two areas: got %q", got)
	}
	for _, name := range []string{" persistence ", "persistence"} {
		row := areaNamed(t, areas, name)
		if row.Records != 1 {
			t.Errorf("%q holds one record: got %d", name, row.Records)
		}
		// The reported name, handed back to Query unchanged, finds exactly the
		// records the row counted. In-package only — see the header: the same name
		// through `query --about` is trimmed before it gets here and finds nothing.
		if got := Query(log, Filter{About: []string{name}}).Count; got != row.Records {
			t.Errorf("querying for the reported name %q gives %d, the row says %d", name, got, row.Records)
		}
	}
}

// A record tagged with the same thing twice is one record under it.
func TestARepeatedTagCountsItsRecordOnce(t *testing.T) {
	e := entry(t, KindPreference, "Migrations go through Atlas", "db", "DB")

	areas := Areas(fileOf(e), Filter{})
	if len(areas) != 1 || areas[0].Records != 1 {
		t.Fatalf("one area, one record: got %v", areas)
	}
}

// The threshold is a guard against spending a caller's context on record bodies,
// and this returns none — so it must not withhold, at any size.
func TestAreasIsNeverWithheldByTheThreshold(t *testing.T) {
	entries := make([]*Entry, 0, DefaultThreshold+5)
	for i := 0; i <= DefaultThreshold+4; i++ {
		entries = append(entries, entry(t, KindPreference, "Preference "+itoa(i), "area-"+itoa(i)))
	}
	if got := len(Areas(logOf(entries, nil), Filter{})); got != len(entries) {
		t.Fatalf("every area is enumerated past the threshold: want %d, got %d", len(entries), got)
	}
}

// ------------------------------------------- which record wins a tie ---

// These two are characterization tests: they state what `Areas` does today when
// two records under one tag carry the same `at`, they do not ask for it. Nothing
// designed this — the tie-break falls out of `Query` sorting on the id — and it
// is worth pinning because `at` is written to the second while records arrive
// far faster than that. A fixture of three preferences stamped
// `2026-08-08T20:19:24Z` is what prompted this.

// tieBreakID is a ULID for an exact millisecond with the entropy spelled out, so
// a fixture can put two ids in a chosen order instead of hoping the clock does.
func tieBreakID(t *testing.T, at time.Time, entropy byte) string {
	t.Helper()
	id, err := newULID(at, bytes.NewReader(bytes.Repeat([]byte{entropy}, 10)))
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func tieBreakEntry(t *testing.T, id, at, title string) *Entry {
	t.Helper()
	return newTestEntry(t, func(e *Entry) {
		e.ID = id
		e.At = at
		e.Kind = KindPreference
		e.Slug = deriveTestSlug(title)
		e.Title = title
		e.About = []string{"persistence"}
	})
}

// Same `at` to the second, a millisecond apart underneath: the later write is
// live. `at` is stamped RFC3339 and loses the milliseconds, but the id keeps
// them, and the id is what the sort reads — so records that look simultaneous in
// the log still resolve in the order they were written.
func TestWithinOneSecondTheLaterWriteIsTheLiveAnswer(t *testing.T) {
	base := time.Date(2026, 8, 8, 20, 19, 24, 0, time.UTC)
	stamp := base.Format(time.RFC3339)

	earlier := tieBreakEntry(t, tieBreakID(t, base, 0x01), stamp, "Persistence is GORM")
	later := tieBreakEntry(t, tieBreakID(t, base.Add(time.Millisecond), 0x01), stamp, "Persistence is sqlc")
	if earlier.At != later.At {
		t.Fatalf("the fixture must give both records one stamp: %q vs %q", earlier.At, later.At)
	}

	a := areaNamed(t, Areas(logOf([]*Entry{earlier, later}, nil), Filter{}), "persistence")
	if a.Records != 2 {
		t.Fatalf("both records carry the tag: got %d", a.Records)
	}
	if a.Live == nil || a.Live.ID != later.ID {
		t.Errorf("the later of two writes in one second answers for the area, got %+v", a.Live)
	}
}

// Same millisecond, and this is the honest half: the tie is broken by the id's
// random suffix, which is not write order and not anything a caller can predict.
// It is stable — the ids are fixed once written, so the same log always gives the
// same answer, and file order does not enter into it. But two preferences landing
// in one millisecond have their live answer decided by entropy, and this test
// exists so that whoever changes that knows they are changing it rather than
// discovering it.
func TestWithinOneMillisecondTheIdBreaksTheTieAndFileOrderDoesNot(t *testing.T) {
	base := time.Date(2026, 8, 8, 20, 19, 24, 0, time.UTC)
	stamp := base.Format(time.RFC3339)

	lower := tieBreakEntry(t, tieBreakID(t, base, 0x01), stamp, "Persistence is GORM")
	higher := tieBreakEntry(t, tieBreakID(t, base, 0x02), stamp, "Persistence is sqlc")
	if lower.ID >= higher.ID {
		t.Fatalf("the fixture must order the two ids: %q, %q", lower.ID, higher.ID)
	}

	for _, order := range [][]*Entry{{lower, higher}, {higher, lower}} {
		a := areaNamed(t, Areas(logOf(order, nil), Filter{}), "persistence")
		if a.Live == nil || a.Live.ID != higher.ID {
			t.Errorf("the greater id answers whatever order the file holds, got %+v", a.Live)
		}
	}
}

// An empty log answers rather than failing, and answers with a list rather than
// with nothing — which is what the CLI needs to print `[]` for a scope nobody
// has ever written to.
func TestAreasOverAnEmptyLogIsAnEmptyListAndNotNil(t *testing.T) {
	areas := Areas(nil, Filter{})
	if areas == nil || len(areas) != 0 {
		t.Fatalf("want an empty non-nil slice, got %#v", areas)
	}
}
