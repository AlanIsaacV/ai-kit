package main

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

const citedULID = "01HZY3S9Q8T5V2W4X6Y8Z0A1B2"

const cited = "sentrii:" + citedULID

func withoutSentriiEnvironment(t *testing.T) {
	t.Helper()
	for _, kv := range os.Environ() {
		name, _, _ := strings.Cut(kv, "=")
		if strings.Contains(strings.ToUpper(name), "SENTRII") {
			t.Setenv(name, "")
			os.Unsetenv(name)
		}
	}
}

func sentriiLine(t *testing.T, title, typ, to string) string {
	t.Helper()
	return entryLine(t, map[string]any{
		"kind": "decision", "title": title, "about": []string{"auth"},
		"edges": []map[string]any{{"type": typ, "to": to}},
	}) + "\n"
}

type versionedEdges struct {
	Count int `json:"count"`
	Edges []struct {
		Version int    `json:"version"`
		Type    string `json:"type"`
		From    string `json:"from"`
		To      string `json:"to"`
	} `json:"edges"`
}

func TestASentriiCitationLandsFromLinkAndFromABatchWithNoConnection(t *testing.T) {
	withoutSentriiEnvironment(t)
	r := throwawayRepo(t)
	local := add(t, r, "Rotate the signing key")

	if res := link(t, r, "relates_to", local, cited); res.code != 0 {
		t.Fatalf("link relates_to %s: exit %d\n%s%s", cited, res.code, res.stdout, res.stderr)
	}
	out := batchResult(t, runStdin(t, r, sentriiLine(t, "Caused by the sentrii incident", "caused_by", cited),
		"add", "--plan", "demo", "--batch", "-"))
	if out.Edges != 1 {
		t.Fatalf("the batch line's caused_by edge is written with it, got %d edges", out.Edges)
	}
	batched := out.Records[0].ID

	res := runBare(t, r, "edges", "--to", cited)
	if res.code != 0 {
		t.Fatalf("edges --to %s: exit %d\n%s%s", cited, res.code, res.stdout, res.stderr)
	}
	var got versionedEdges
	if err := json.Unmarshal([]byte(res.stdout), &got); err != nil {
		t.Fatalf("edges printed something that is not JSON: %v\n%s", err, res.stdout)
	}
	if got.Count != 2 || len(got.Edges) != 2 {
		t.Fatalf("both edges to %s are listed, got count %d: %s", cited, got.Count, res.stdout)
	}
	want := map[string]string{"relates_to": local, "caused_by": batched}
	for _, e := range got.Edges {
		if from, ok := want[e.Type]; !ok || e.From != from || e.To != cited {
			t.Errorf("unexpected edge %s %s -> %s", e.Type, e.From, e.To)
		}
		delete(want, e.Type)
		if e.Version != 4 {
			t.Errorf("the %s edge is stamped version %d, not 4", e.Type, e.Version)
		}
	}
	if len(want) > 0 {
		t.Errorf("edges missing from the answer: %v", want)
	}
}

func refusedByLinkAndBatch(t *testing.T, r repo, from, typ, to string) {
	t.Helper()
	before := len(lines(t, r, r.log))

	res := link(t, r, typ, from, to)
	if res.code != 1 {
		t.Fatalf("link --type %s --to %s: exit 1 expected, got %d\n%s%s", typ, to, res.code, res.stdout, res.stderr)
	}
	if code, msg := errorCode(t, res); code != "invalid_edge" {
		t.Errorf("link --type %s --to %s: code invalid_edge expected, got %q (%s)", typ, to, code, msg)
	}
	if after := len(lines(t, r, r.log)); after != before {
		t.Fatalf("link --type %s --to %s: the store held %d lines and now holds %d", typ, to, before, after)
	}

	res = runStdin(t, r, sentriiLine(t, "Cites "+to, typ, to), "add", "--plan", "demo", "--batch", "-")
	if refusal := batchRefused(t, res); refusal.Code != "invalid_batch" {
		t.Errorf("add --batch with a %s edge to %s: code invalid_batch expected, got %q\n%s", typ, to, refusal.Code, res.stdout)
	}
	if after := len(lines(t, r, r.log)); after != before {
		t.Fatalf("add --batch with a %s edge to %s: the store held %d lines and now holds %d", typ, to, before, after)
	}
}

func TestAMisspelledSentriiPrefixIsRefused(t *testing.T) {
	r := throwawayRepo(t)
	local := add(t, r, "Rotate the signing key")
	for _, to := range []string{"Sentrii:" + citedULID, "sentri:" + citedULID, "sentrii" + citedULID} {
		refusedByLinkAndBatch(t, r, local, "relates_to", to)
	}
}

func TestASentriiCitationThatIsNotAULIDIsRefused(t *testing.T) {
	r := throwawayRepo(t)
	local := add(t, r, "Rotate the signing key")
	short := "sentrii:" + citedULID[:25]
	withU := "sentrii:" + citedULID[:10] + "U" + citedULID[11:]
	for _, to := range []string{short, withU} {
		refusedByLinkAndBatch(t, r, local, "relates_to", to)
	}
}

func TestOnlyRelatesToAndCausedByMayPointAtSentrii(t *testing.T) {
	r := throwawayRepo(t)
	local := add(t, r, "Rotate the signing key")
	refusedByLinkAndBatch(t, r, local, "supersedes", cited)
}

func TestALogCarryingSentriiEdgesIsReadLikeAnyOther(t *testing.T) {
	withoutSentriiEnvironment(t)
	r := throwawayRepo(t)
	objective := addObjective(t, r, "Rotate keys after the sentrii incident")
	decision := addKind(t, r, "decision", "Rotate weekly")
	mustLink(t, r, "caused_by", objective, cited)
	mustLink(t, r, "relates_to", decision, cited)

	report, code := validateOne(t, r)
	if code != 0 || !report.OK {
		t.Fatalf("validate: exit %d, ok %v", code, report.OK)
	}
	if ps := report.Reports[0].Problems; len(ps) > 0 {
		t.Fatalf("validate reports problems over a log carrying sentrii edges: %+v", ps)
	}

	state := readState(t, r)
	listed := false
	for _, o := range state.Objectives {
		if o.ID == objective {
			listed = true
		}
	}
	if !listed {
		t.Fatalf("state does not list the objective %s", objective)
	}
	if len(state.Anomalies) > 0 {
		t.Fatalf("state reports anomalies over a log carrying sentrii edges: %+v", state.Anomalies)
	}

	if res := run(t, r, "anchors"); res.code != 0 {
		t.Fatalf("anchors: exit %d\n%s%s", res.code, res.stdout, res.stderr)
	}
}
