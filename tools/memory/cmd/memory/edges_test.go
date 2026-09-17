package main

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestEdgesRefusesAPlanNoEdgeCarries(t *testing.T) {
	r := throwawayRepo(t)
	from := add(t, r, "RS256 over HS256")
	to := add(t, r, "Asymmetric signing")
	mustLink(t, r, "derives_from", from, to)

	res := runBare(t, r, "edges", "--plan", "demo")
	if res.code != 1 {
		t.Fatalf("--plan on edges has to be refused, got exit %d\n%s%s", res.code, res.stdout, res.stderr)
	}
	code, msg := errorCode(t, res)
	if code != "plan_not_on_edges" {
		t.Errorf("expected a plan_not_on_edges refusal, got %q", code)
	}
	if !strings.Contains(msg, "an edge carries no plan") {
		t.Errorf("the refusal has to say an edge carries no plan, got %q", msg)
	}
	var out map[string]json.RawMessage
	if err := json.Unmarshal([]byte(res.stdout), &out); err != nil {
		t.Fatal(err)
	}
	if _, ok := out["edges"]; ok {
		t.Errorf("a refused read hands back no edges at all:\n%s", res.stdout)
	}
}

func TestEdgesAnswersAZeroMatchWithAnEmptyListAndNeverNull(t *testing.T) {
	r := throwawayRepo(t)
	from := add(t, r, "RS256 over HS256")
	to := add(t, r, "Asymmetric signing")

	cases := []struct {
		name     string
		linkedBy string
		args     []string
	}{
		{"a log holding no edges at all", "", []string{"edges"}},
		{"a type nothing in the log carries", "derives_from", []string{"edges", "--type", "caused_by"}},
		{"an endpoint no edge names", "", []string{"edges", "--from", "01JXKQ7M4T8YZPABCDEFGHJKMN"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.linkedBy != "" {
				mustLink(t, r, tc.linkedBy, from, to)
			}
			res := runBare(t, r, tc.args...)
			if res.code != 0 {
				t.Fatalf("a zero match is an answer, not a refusal: exit %d\n%s%s", res.code, res.stdout, res.stderr)
			}
			var out struct {
				Count int             `json:"count"`
				Edges json.RawMessage `json:"edges"`
			}
			if err := json.Unmarshal([]byte(res.stdout), &out); err != nil {
				t.Fatalf("edges printed something that is not JSON: %v\n%s", err, res.stdout)
			}
			if out.Count != 0 {
				t.Fatalf("this filter is meant to match nothing, got %d", out.Count)
			}
			if string(out.Edges) != "[]" {
				t.Errorf("edges must be an empty list, got %s", out.Edges)
			}
		})
	}
}

type edgeAnswer struct {
	Count int `json:"count"`
	Edges []struct {
		Type string `json:"type"`
		From string `json:"from"`
		To   string `json:"to"`
	} `json:"edges"`
}

func listEdges(t *testing.T, r repo, args ...string) edgeAnswer {
	t.Helper()
	res := runBare(t, r, append([]string{"edges"}, args...)...)
	if res.code != 0 {
		t.Fatalf("edges %v: exit %d\n%s%s", args, res.code, res.stdout, res.stderr)
	}
	var out edgeAnswer
	if err := json.Unmarshal([]byte(res.stdout), &out); err != nil {
		t.Fatalf("edges printed something that is not JSON: %v\n%s", err, res.stdout)
	}
	return out
}

func TestEdgesNamingAnEndpointAnswerTheSameFromEitherLog(t *testing.T) {
	_, alpha, beta := twoDepths(t)
	parent := addObjective(t, alpha, "The objective the whole plan hangs from")
	kid := addObjective(t, beta, "The objective in the service next door")
	mustLink(t, beta, "derives_from", kid, parent)

	for _, r := range []repo{alpha, beta} {
		for _, args := range [][]string{{"--from", kid}, {"--to", parent}} {
			out := listEdges(t, r, args...)
			if out.Count != 1 || len(out.Edges) != 1 {
				t.Fatalf("edges %v read in %s: one edge names that endpoint wherever it was written, got %d",
					args, r.root, out.Count)
			}
			if got := out.Edges[0]; got.From != kid || got.To != parent {
				t.Errorf("edges %v read in %s returned %s -> %s", args, r.root, got.From, got.To)
			}
		}
	}

	if out := listEdges(t, alpha); out.Count != 0 {
		t.Errorf("a read that names no endpoint keeps the log it was asked from, got %d edges", out.Count)
	}
	if out := listEdges(t, beta); out.Count != 1 {
		t.Errorf("the log the edge was written into still lists it unasked, got %d edges", out.Count)
	}
}

func addUserScoped(t *testing.T, r repo, title string) string {
	t.Helper()
	res := runBare(t, r, "add", "--scope", "user", "--kind", "decision",
		"--title", title, "--about", "auth")
	if res.code != 0 {
		t.Fatalf("add --scope user %q: exit %d\n%s%s", title, res.code, res.stdout, res.stderr)
	}
	return recordID(t, res)
}

func TestEdgesKeepsScopeLiveWhenNoEndpointIsNamed(t *testing.T) {
	r := throwawayRepo(t)
	held := add(t, r, "RS256 over HS256")
	mustLink(t, r, "derives_from", held, add(t, r, "Asymmetric signing"))

	promoted := addUserScoped(t, r, "Tests name the behaviour")
	if res := runBare(t, r, "link", "--scope", "user", "--type", "derives_from",
		"--from", promoted, "--to", addUserScoped(t, r, "A test says what it pins")); res.code != 0 {
		t.Fatalf("link --scope user: exit %d\n%s%s", res.code, res.stdout, res.stderr)
	}

	type scopedEdges struct {
		Path  string `json:"path"`
		Scope string `json:"scope"`
		Count int    `json:"count"`
		Edges []struct {
			From string `json:"from"`
		} `json:"edges"`
	}
	for _, tc := range []struct {
		scope string
		path  string
		from  string
	}{
		{"user", r.user, promoted},
		{"repo", r.log, held},
	} {
		t.Run(tc.scope, func(t *testing.T) {
			res := runBare(t, r, "edges", "--scope", tc.scope)
			if res.code != 0 {
				t.Fatalf("--scope with no endpoint is not refused: exit %d\n%s%s",
					res.code, res.stdout, res.stderr)
			}
			var out scopedEdges
			if err := json.Unmarshal([]byte(res.stdout), &out); err != nil {
				t.Fatalf("edges printed something that is not JSON: %v\n%s", err, res.stdout)
			}
			if out.Count != 1 || len(out.Edges) != 1 || out.Edges[0].From != tc.from {
				t.Fatalf("--scope %s lists that log's edges and no other log's, got %+v", tc.scope, out)
			}
			if out.Path != tc.path || out.Scope != tc.scope {
				t.Errorf("--scope %s still names the one log it filtered to, got %q %q",
					tc.scope, out.Path, out.Scope)
			}
		})
	}

	res := runBare(t, r, "edges", "--from", promoted, "--scope", "user")
	if res.code != 1 {
		t.Fatalf("naming an endpoint makes the answer the store's, so --scope goes with it: exit %d\n%s%s",
			res.code, res.stdout, res.stderr)
	}
	if code, _ := errorCode(t, res); code != "scope_not_on_store_wide_read" {
		t.Errorf("the refusal code names the flag it refused, got %q", code)
	}
}
