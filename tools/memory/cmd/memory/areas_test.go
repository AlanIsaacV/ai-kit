package main

import (
	"encoding/json"
	"os/exec"
	"strings"
	"testing"
)

// These tests drive the built binary over throwaway logs, like the rest of this
// package's tests. What `areas` owes a caller is the shape of an answer printed
// by a process — two scopes side by side, each with the path it resolved to —
// and a package-level call cannot see either.

// ------------------------------------------------------------- CLI helpers ---

type areaLiveOutput struct {
	ID    string `json:"id"`
	Kind  string `json:"kind"`
	Slug  string `json:"slug"`
	Title string `json:"title"`
	At    string `json:"at"`
	Key   bool   `json:"key"`
}

type areaRowOutput struct {
	Area    string          `json:"area"`
	Records int             `json:"records"`
	Live    *areaLiveOutput `json:"live"`
}

type areaScopeOutput struct {
	Path  string          `json:"path"`
	Scope string          `json:"scope"`
	Count int             `json:"count"`
	Areas []areaRowOutput `json:"areas"`
}

type areasOutput struct {
	Plan   string            `json:"plan"`
	Kinds  []string          `json:"kinds"`
	Scopes []areaScopeOutput `json:"scopes"`
}

// readAreas runs the command and returns both the parsed answer and the raw
// bytes, because one of the claims under test is about the bytes themselves:
// that two runs over one log print the same thing.
func readAreas(t *testing.T, r repo, args ...string) (areasOutput, string) {
	t.Helper()
	res := runBare(t, r, append([]string{"areas"}, args...)...)
	if res.code != 0 {
		t.Fatalf("areas %v: exit %d\n%s%s", args, res.code, res.stdout, res.stderr)
	}
	var out areasOutput
	if err := json.Unmarshal([]byte(res.stdout), &out); err != nil {
		t.Fatalf("areas printed something that is not JSON: %v\n%s", err, res.stdout)
	}
	return out, res.stdout
}

// scopeOf is the one report for a scope, and a failure when the answer does not
// carry exactly one — a missing section is the failure this command exists to
// stop making, so no test may quietly skip past it.
func scopeOf(t *testing.T, out areasOutput, scope string) areaScopeOutput {
	t.Helper()
	if len(out.Scopes) != 2 {
		t.Fatalf("every run reports both scopes, got %d", len(out.Scopes))
	}
	for _, s := range out.Scopes {
		if s.Scope == scope {
			return s
		}
	}
	t.Fatalf("no %q scope in the answer: %+v", scope, out.Scopes)
	return out.Scopes[0]
}

func namesOf(t *testing.T, out areasOutput, scope string) []string {
	t.Helper()
	s := scopeOf(t, out, scope)
	got := make([]string, 0, len(s.Areas))
	for _, a := range s.Areas {
		got = append(got, a.Area)
	}
	return got
}

func rowOf(t *testing.T, out areasOutput, scope, area string) areaRowOutput {
	t.Helper()
	s := scopeOf(t, out, scope)
	for _, a := range s.Areas {
		if a.Area == area {
			return a
		}
	}
	t.Fatalf("no area %q in the %s scope: %v", area, scope, namesOf(t, out, scope))
	return s.Areas[0]
}

// addPreferenceTagged is `add --kind preference` with the tags spelled out, which
// is what every fixture here varies. Named apart from the local helper in
// main_test.go so neither shadows the other.
func addPreferenceTagged(t *testing.T, r repo, title, about string) string {
	t.Helper()
	res := runBare(t, r, "add", "--kind", "preference", "--title", title, "--about", about)
	if res.code != 0 {
		t.Fatalf("add %q: exit %d\n%s%s", title, res.code, res.stdout, res.stderr)
	}
	return recordID(t, res)
}

// ----------------------------------------------------------------- tests ---

// The whole answer in one run: both scopes, each with the path it resolved to,
// every distinct tag in the repo log with how many records carry it, in an order
// that does not move between runs.
func TestAreasEnumeratesBothScopesWithCountsInAStableOrder(t *testing.T) {
	r := throwawayRepo(t)

	addPreferenceTagged(t, r, "Migrations go through Atlas", "persistence,migrations")
	addPreferenceTagged(t, r, "Errors wrap rather than replace", "errors")
	addTo(t, r, "tree-a", "Cancel is async") // tagged `auth` by the helper

	out, first := readAreas(t, r)
	if got := namesOf(t, out, "repo"); strings.Join(got, ",") != "auth,errors,migrations,persistence" {
		t.Fatalf("every tag, sorted by name ascending: got %v", got)
	}
	if s := scopeOf(t, out, "repo"); s.Count != 4 || s.Path != r.log {
		t.Errorf("the repo scope names its own log and counts its areas: count=%d path=%q", s.Count, s.Path)
	}
	if s := scopeOf(t, out, "user"); s.Path != r.user {
		t.Errorf("the user scope names the user log, got %q", s.Path)
	}
	for _, area := range []string{"persistence", "migrations", "errors", "auth"} {
		if got := rowOf(t, out, "repo", area).Records; got != 1 {
			t.Errorf("one record carries %q, got %d", area, got)
		}
	}

	// Byte-identical, not merely equivalent. A caller comparing two runs is the
	// use this ordering rule exists for.
	if _, second := readAreas(t, r); second != first {
		t.Errorf("two runs over one log must print the same thing:\n%s\n---\n%s", first, second)
	}
}

// The row names the record that currently answers for the area, and says whether
// the area is what that record claims to be about. Both halves matter to a
// reader deciding whether an area already has an answer they would be
// contradicting.
func TestAreasNamesTheLiveAnswerAndWhetherTheTagIsItsKey(t *testing.T) {
	r := throwawayRepo(t)

	older := addPreferenceTagged(t, r, "Persistence is GORM", "persistence,orm")
	newer := addPreferenceTagged(t, r, "Persistence is sqlc", "persistence,orm")
	if res := runBare(t, r, "link", "--type", "supersedes", "--from", newer, "--to", older); res.code != 0 {
		t.Fatalf("link: exit %d\n%s%s", res.code, res.stdout, res.stderr)
	}

	out, _ := readAreas(t, r)
	row := rowOf(t, out, "repo", "persistence")
	if row.Records != 2 {
		t.Errorf("the superseded record still counts under the tag, got %d", row.Records)
	}
	if row.Live == nil || row.Live.ID != newer {
		t.Fatalf("the live answer is the record nothing supersedes, got %+v", row.Live)
	}
	if row.Live.Title != "Persistence is sqlc" || row.Live.Kind != "preference" {
		t.Errorf("the row carries enough to recognise the record it names: %+v", row.Live)
	}
	if !row.Live.Key {
		t.Errorf("`persistence` is that record's first --about, so it is its key: %+v", row.Live)
	}
	if orm := rowOf(t, out, "repo", "orm"); orm.Live == nil || orm.Live.Key {
		t.Errorf("`orm` is the same record's second tag, so it is not its key: %+v", orm.Live)
	}

	// A third record keyed elsewhere but carrying the tag takes over the row,
	// and the row then reports the tag as not that record's key.
	third := addPreferenceTagged(t, r, "Persistence is Ent", "storage,persistence")
	out, _ = readAreas(t, r)
	row = rowOf(t, out, "repo", "persistence")
	if row.Live == nil || row.Live.ID != third {
		t.Fatalf("the newest surviving record carrying the tag answers for it, got %+v", row.Live)
	}
	if row.Live.Key {
		t.Errorf("the tag sits second in that record, so the row must say it is not the key: %+v", row.Live)
	}
	if row.Records != 3 {
		t.Errorf("three records carry `persistence` now, got %d", row.Records)
	}
}

// A tag that only ever appears in second position is still an area. This is what
// keeps a preference tagged `migrations,db` findable by a reader who came looking
// for `db`.
func TestATagInSecondPositionIsEnumeratedAndMarkedAsNotTheKey(t *testing.T) {
	r := throwawayRepo(t)
	id := addPreferenceTagged(t, r, "Migrations go through Atlas", "migrations,db")

	out, _ := readAreas(t, r)
	if got := namesOf(t, out, "repo"); strings.Join(got, ",") != "db,migrations" {
		t.Fatalf("both tags are areas: got %v", got)
	}
	if key := rowOf(t, out, "repo", "migrations"); key.Live == nil || key.Live.ID != id || !key.Live.Key {
		t.Errorf("`migrations` is the key: %+v", key.Live)
	}
	if second := rowOf(t, out, "repo", "db"); second.Live == nil || second.Live.ID != id || second.Live.Key {
		t.Errorf("`db` names the same record and is not its key: %+v", second.Live)
	}
}

// --kind narrows what is enumerated, and the counts follow it rather than
// reporting the whole log's.
func TestAreasNarrowsByKindAndTheCountsFollow(t *testing.T) {
	r := throwawayRepo(t)

	pref := addPreferenceTagged(t, r, "Migrations go through Atlas", "migrations")
	res := runBare(t, r, "add", "--kind", "gotcha", "--title", "The lock is taken before the write",
		"--about", "migrations", "--anchor", anchorSpec(t, r))
	if res.code != 0 {
		t.Fatalf("add gotcha: exit %d\n%s%s", res.code, res.stdout, res.stderr)
	}

	out, _ := readAreas(t, r)
	if got := rowOf(t, out, "repo", "migrations").Records; got != 2 {
		t.Errorf("unfiltered, both records carry the tag, got %d", got)
	}

	out, _ = readAreas(t, r, "--kind", "preference")
	if strings.Join(out.Kinds, ",") != "preference" {
		t.Errorf("the answer says what it was narrowed to, got %v", out.Kinds)
	}
	row := rowOf(t, out, "repo", "migrations")
	if row.Records != 1 {
		t.Errorf("--kind preference counts only the preference, got %d", row.Records)
	}
	if row.Live == nil || row.Live.ID != pref {
		t.Errorf("and names it, got %+v", row.Live)
	}
	if got := namesOf(t, out, "repo"); strings.Join(got, ",") != "migrations" {
		t.Errorf("nothing else is left in the enumeration: %v", got)
	}
}

// The case this command exists to make answerable on a machine that has never
// promoted anything: the user scope is reported, empty, and the run succeeds.
// An omitted section would leave "I have never promoted anything"
// indistinguishable from "I did not look there".
func TestAreasAnswersForAUserLogThatDoesNotExist(t *testing.T) {
	r := throwawayRepo(t)
	addPreferenceTagged(t, r, "Migrations go through Atlas", "migrations")

	res := runBare(t, r, "areas")
	if res.code != 0 {
		t.Fatalf("a missing user log is an empty answer, not a refusal: exit %d\n%s%s",
			res.code, res.stdout, res.stderr)
	}
	var out areasOutput
	if err := json.Unmarshal([]byte(res.stdout), &out); err != nil {
		t.Fatal(err)
	}
	user := scopeOf(t, out, "user")
	if user.Count != 0 || len(user.Areas) != 0 {
		t.Errorf("nothing was ever promoted, so the user scope is empty: %+v", user)
	}
	if user.Path != r.user {
		t.Errorf("and it still says which log it looked in, got %q", user.Path)
	}
	if !strings.Contains(res.stdout, `"areas": []`) {
		t.Errorf("an empty scope prints an empty list rather than null:\n%s", res.stdout)
	}
	if got := namesOf(t, out, "repo"); len(got) != 1 {
		t.Errorf("the repo scope answered as usual beside it: %v", got)
	}
}

// What is promoted becomes an area of the user log, and the two lists stay
// apart. This is the read the objective is for: which preferences travel with
// their owner, and which belong to the repository in front of them.
func TestTheTwoScopesAreEnumeratedApart(t *testing.T) {
	r := throwawayRepo(t)
	addPreferenceTagged(t, r, "Migrations go through Atlas", "migrations")
	addPreferenceTagged(t, r, "Errors wrap rather than replace", "errors")
	promote(t, r, "--ids", addPreferenceTagged(t, r, "Tests name the behaviour", "testing"), "--to", "user")

	out, _ := readAreas(t, r)
	if got := namesOf(t, out, "repo"); strings.Join(got, ",") != "errors,migrations,testing" {
		t.Errorf("the repo log keeps everything it wrote: %v", got)
	}
	if got := namesOf(t, out, "user"); strings.Join(got, ",") != "testing" {
		t.Errorf("the user log holds only what was promoted into it: %v", got)
	}
	if got := rowOf(t, out, "user", "testing").Live; got == nil || got.Title != "Tests name the behaviour" {
		t.Errorf("and it names its own live answer: %+v", got)
	}
}

// The plan bound is `query`'s, which means a record the repository owns is
// enumerated under every plan and a record that names one stays out of another's.
func TestAreasBoundsByPlanTheWayQueryDoes(t *testing.T) {
	r := throwawayRepo(t)

	addPreferenceTagged(t, r, "Migrations go through Atlas", "migrations")
	addTo(t, r, "tree-a", "Cancel is async") // tagged `auth`

	out, _ := readAreas(t, r, "--plan", "tree-b")
	if got := namesOf(t, out, "repo"); strings.Join(got, ",") != "migrations" {
		t.Fatalf("a plan-less record is in scope for every plan, another tree's is not: %v", got)
	}
	if out.Plan != "tree-b" {
		t.Errorf("the answer says which bound it was read under, got %q", out.Plan)
	}

	out, _ = readAreas(t, r, "--plan", "tree-a")
	if got := namesOf(t, out, "repo"); strings.Join(got, ",") != "auth,migrations" {
		t.Errorf("a tree's own records are enumerated beside the repository's: %v", got)
	}
}

// A command nobody can find is a command that does not exist. The usage is where
// a caller meets the CLI's whole surface.
func TestAreasIsListedInTheUsage(t *testing.T) {
	cmd := exec.Command(memoryBin)
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatal("running with no subcommand should exit non-zero")
	}
	usage := string(out)
	if !strings.Contains(usage, "memory areas     [--kind KINDS] [--plan SLUG]") {
		t.Errorf("`areas` and its flags are missing from the usage:\n%s", usage)
	}
}

// --scope is not a flag here: the command answers for both logs, and a flag that
// looked like it chose one would be a flag that silently did nothing.
func TestAreasRefusesAScopeItCannotHonour(t *testing.T) {
	r := throwawayRepo(t)
	res := runBare(t, r, "areas", "--scope", "user")
	if res.code == 0 {
		t.Fatalf("--scope must be refused rather than accepted and ignored:\n%s", res.stdout)
	}
	if !strings.Contains(res.stderr, "scope") {
		t.Errorf("the refusal should name the flag it did not understand:\n%s", res.stderr)
	}
}

func TestAreasDropsARecordSettledByAnEdgeInAnotherLog(t *testing.T) {
	_, alpha, beta := twoDepths(t)
	settled := addPreferenceTagged(t, alpha, "Persistence is GORM", "persistence")
	mustLink(t, beta, "supersedes",
		addPreferenceTagged(t, beta, "The service next door replaced it", "persistence"), settled)

	out, _ := readAreas(t, alpha)
	row := rowOf(t, out, "repo", "persistence")
	if row.Records != 1 {
		t.Fatalf("the count is over the settled records too, got %d", row.Records)
	}
	if row.Live != nil {
		t.Errorf("a supersedes edge written next door settles the record held here, got %+v", row.Live)
	}
}
