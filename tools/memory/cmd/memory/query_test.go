package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func addWithBody(t *testing.T, r repo, title, body string) string {
	t.Helper()
	res := runBare(t, r, "add", "--plan", "demo", "--kind", "decision",
		"--title", title, "--about", "auth", "--body", body)
	if res.code != 0 {
		t.Fatalf("add %q: exit %d\n%s%s", title, res.code, res.stdout, res.stderr)
	}
	return recordID(t, res)
}

func queryRecords(t *testing.T, res result) []map[string]json.RawMessage {
	t.Helper()
	var out struct {
		Records []map[string]json.RawMessage `json:"records"`
	}
	if err := json.Unmarshal([]byte(res.stdout), &out); err != nil {
		t.Fatalf("query printed something that is not JSON: %v\n%s%s", err, res.stdout, res.stderr)
	}
	return out.Records
}

func bodiesByTitle(t *testing.T, res result) map[string]*string {
	t.Helper()
	out := map[string]*string{}
	for _, rec := range queryRecords(t, res) {
		var title string
		if err := json.Unmarshal(rec["title"], &title); err != nil {
			t.Fatalf("a record came back without a readable title: %v\n%s", err, res.stdout)
		}
		raw, ok := rec["body"]
		if !ok {
			out[title] = nil
			continue
		}
		var body string
		if err := json.Unmarshal(raw, &body); err != nil {
			t.Fatalf("a record came back without a readable body: %v\n%s", err, res.stdout)
		}
		out[title] = &body
	}
	return out
}

func TestQueryRefusesAFieldNoRecordHas(t *testing.T) {
	r := throwawayRepo(t)
	add(t, r, "RS256 over HS256")

	res := run(t, r, "query", "--fields", "title,shape")
	if res.code == 0 {
		t.Fatalf("--fields shape must be refused:\n%s", res.stdout)
	}
	code, msg := errorCode(t, res)
	if code != "unknown_field" {
		t.Errorf("expected an unknown_field refusal, got %q", code)
	}
	if !strings.Contains(msg, "shape") {
		t.Errorf("the refusal must name the field it was handed, got %q", msg)
	}
	var out map[string]json.RawMessage
	if err := json.Unmarshal([]byte(res.stdout), &out); err != nil {
		t.Fatal(err)
	}
	if _, ok := out["records"]; ok {
		t.Errorf("a refused query hands back no records:\n%s", res.stdout)
	}
}

func TestBodyLinesTakesTheHeadOfEachBody(t *testing.T) {
	r := throwawayRepo(t)
	const long = "one\ntwo\nthree\nfour\nfive"
	const median = "one\ntwo"
	addWithBody(t, r, "The long body", long)
	addWithBody(t, r, "The median body", median)

	whole := bodiesByTitle(t, run(t, r, "query"))
	if len(whole) != 2 {
		t.Fatalf("both records have to come back for their bodies to say anything: %v", whole)
	}
	if whole["The long body"] == nil || *whole["The long body"] != long {
		t.Errorf("with no --body-lines the whole body comes back, got %v", whole["The long body"])
	}

	head := bodiesByTitle(t, run(t, r, "query", "--body-lines", "2"))
	if head["The long body"] == nil || *head["The long body"] != "one\ntwo" {
		t.Errorf("--body-lines 2 returns the first two lines, got %v", head["The long body"])
	}
	if head["The median body"] == nil || *head["The median body"] != median {
		t.Errorf("a body of exactly two lines comes back whole, got %v", head["The median body"])
	}

	none := bodiesByTitle(t, run(t, r, "query", "--body-lines", "0"))
	if len(none) != 2 {
		t.Fatalf("--body-lines 0 drops the body, not the record: %v", none)
	}
	for title, body := range none {
		if body != nil {
			t.Errorf("--body-lines 0 leaves no body key at all on %q, got %q", title, *body)
		}
	}
}

type scopedRecord struct {
	ID    string `json:"id"`
	Scope string `json:"scope"`
}

func scopedAnswer(t *testing.T, res result) (records []scopedRecord, notFound []string) {
	t.Helper()
	if res.code != 0 {
		t.Fatalf("query --scope both: exit %d\n%s%s", res.code, res.stdout, res.stderr)
	}
	var out struct {
		Records  []scopedRecord `json:"records"`
		NotFound []string       `json:"not_found"`
	}
	if err := json.Unmarshal([]byte(res.stdout), &out); err != nil {
		t.Fatalf("query printed something that is not JSON: %v\n%s", err, res.stdout)
	}
	return out.Records, out.NotFound
}

func TestQueryScopeBothFindsEachIDInTheLogThatHoldsIt(t *testing.T) {
	r := throwawayRepo(t)
	inRepo := add(t, r, "RS256 over HS256")
	res := runBare(t, r, "add", "--scope", "user", "--kind", "decision",
		"--title", "Tests name the behaviour", "--about", "testing")
	if res.code != 0 {
		t.Fatalf("add --scope user: exit %d\n%s%s", res.code, res.stdout, res.stderr)
	}
	inUser := recordID(t, res)
	const inNeither = "01JXKQ7M4T8YZPABCDEFGHJKMN"

	records, notFound := scopedAnswer(t, runBare(t, r, "query", "--scope", "both",
		"--id", strings.Join([]string{inRepo, inUser, inNeither}, ",")))

	got := map[string]string{}
	for _, rec := range records {
		got[rec.ID] = rec.Scope
	}
	want := map[string]string{inRepo: "repo", inUser: "user"}
	if len(got) != len(want) || len(records) != len(want) {
		t.Fatalf("one list holding the repo record and the user record and nothing else, got %+v", records)
	}
	for id, scope := range want {
		if got[id] != scope {
			t.Errorf("record %s carries scope %q, want %q", id, got[id], scope)
		}
	}
	if strings.Join(notFound, ",") != inNeither {
		t.Errorf("not_found names only the id neither log holds, got %v", notFound)
	}
}

func TestQueryScopeBothAnswersFromTheRepoLogWhenThereIsNoUserLog(t *testing.T) {
	r := throwawayRepo(t)
	first := add(t, r, "RS256 over HS256")
	second := add(t, r, "Asymmetric signing")
	if _, err := os.Stat(filepath.Dir(r.user)); !os.IsNotExist(err) {
		t.Fatalf("the throwaway HOME has to start with no .ai-kit/ at all, stat said %v", err)
	}

	records, _ := scopedAnswer(t, runBare(t, r, "query", "--scope", "both"))

	if len(records) != 2 {
		t.Fatalf("both repo records come back, got %+v", records)
	}
	for _, rec := range records {
		if rec.ID != first && rec.ID != second {
			t.Errorf("a record the repo log never held came back: %+v", rec)
		}
		if rec.Scope != "repo" {
			t.Errorf("record %s carries scope %q, want repo", rec.ID, rec.Scope)
		}
	}
}

func TestScopeBothIsRefusedByEveryReadButQuery(t *testing.T) {
	r := throwawayRepo(t)
	add(t, r, "RS256 over HS256")

	for _, args := range [][]string{
		{"search", "--scope", "both", "RS256"},
		{"edges", "--scope", "both"},
	} {
		t.Run(args[0], func(t *testing.T) {
			res := runBare(t, r, args...)
			if res.code != 1 {
				t.Fatalf("--scope both on %s has to be refused, got exit %d\n%s%s", args[0], res.code, res.stdout, res.stderr)
			}
			code, msg := errorCode(t, res)
			if code != "bad_scope" {
				t.Errorf("expected a bad_scope refusal, got %q", code)
			}
			if !strings.Contains(msg, `"both"`) {
				t.Errorf("the refusal has to name the scope it was handed, got %q", msg)
			}
		})
	}
}

func TestQueryAnswersAZeroMatchWithAnEmptyListAndNeverNull(t *testing.T) {
	r := throwawayRepo(t)
	add(t, r, "RS256 over HS256")
	const missing = "01JXKQ7M4T8YZPABCDEFGHJKMN"

	cases := []struct {
		name     string
		args     []string
		notFound []string
	}{
		{"nothing matched", []string{"query", "--about", "nothing-carries-this"}, nil},
		{"an id the log does not carry", []string{"query", "--id", missing}, []string{missing}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res := run(t, r, tc.args...)
			if res.code != 0 {
				t.Fatalf("a zero match is an answer, not a refusal: exit %d\n%s%s", res.code, res.stdout, res.stderr)
			}
			var out struct {
				Count    int             `json:"count"`
				Records  json.RawMessage `json:"records"`
				NotFound []string        `json:"not_found"`
			}
			if err := json.Unmarshal([]byte(res.stdout), &out); err != nil {
				t.Fatalf("query printed something that is not JSON: %v\n%s", err, res.stdout)
			}
			if out.Count != 0 {
				t.Fatalf("this query is meant to match nothing, got %d", out.Count)
			}
			if string(out.Records) != "[]" {
				t.Errorf("records must be an empty list, got %s", out.Records)
			}
			if strings.Join(out.NotFound, ",") != strings.Join(tc.notFound, ",") {
				t.Errorf("not_found must stay what it was, want %v got %v", tc.notFound, out.NotFound)
			}
		})
	}
}

func returnedIDs(t *testing.T, res result) []string {
	t.Helper()
	if res.code != 0 {
		t.Fatalf("exit %d\n%s%s", res.code, res.stdout, res.stderr)
	}
	ids := make([]string, 0, 4)
	for _, rec := range queryRecords(t, res) {
		var id string
		if err := json.Unmarshal(rec["id"], &id); err != nil {
			t.Fatalf("a record came back without a readable id: %v\n%s", err, res.stdout)
		}
		ids = append(ids, id)
	}
	return ids
}

func TestARecordSettledByAnEdgeInAnotherLogIsDroppedFromCurrent(t *testing.T) {
	cases := []struct{ name, edge, kind string }{
		{"a supersedes edge", "supersedes", "decision"},
		{"an answers edge", "answers", "open-question"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, alpha, beta := twoDepths(t)
			settled := addKind(t, alpha, tc.kind, "RS256 over HS256")
			standing := addKind(t, alpha, tc.kind, "Asymmetric signing")
			by := addKind(t, beta, "decision", "The service next door settled it")
			mustLink(t, beta, tc.edge, by, settled)

			live := returnedIDs(t, runBare(t, alpha, "query", "--kind", tc.kind, "--current"))
			if slices.Contains(live, settled) {
				t.Errorf("%s written next door settles a record held here, got %v", tc.name, live)
			}
			if !slices.Contains(live, standing) {
				t.Errorf("nothing settles the other record, so it stays, got %v", live)
			}

			history := returnedIDs(t, runBare(t, alpha, "query", "--kind", tc.kind))
			if !slices.Contains(history, settled) {
				t.Errorf("without --current the history is intact, got %v", history)
			}

			found := returnedIDs(t, runBare(t, alpha, "search", "--current", "RS256"))
			if len(found) != 0 {
				t.Errorf("search --current drops what query --current drops, got %v", found)
			}
			if kept := returnedIDs(t, runBare(t, alpha, "search", "RS256")); len(kept) != 1 {
				t.Errorf("without --current search still finds it, got %v", kept)
			}
		})
	}
}
