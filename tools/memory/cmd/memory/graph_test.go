package main

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

func walk(t *testing.T, r repo, into any, args ...string) {
	t.Helper()
	res := run(t, r, args...)
	if res.code != 0 {
		t.Fatalf("%v: exit %d\n%s%s", args, res.code, res.stdout, res.stderr)
	}
	if err := json.Unmarshal([]byte(res.stdout), into); err != nil {
		t.Fatalf("graph printed something that is not JSON: %v\n%s", err, res.stdout)
	}
}

func TestGraphWithholdsAWalkAboveItsThreshold(t *testing.T) {
	r := throwawayRepo(t)
	root := add(t, r, "The record everything hangs off")

	const children = 30
	batch := make([]string, 0, children)
	for i := range children {
		batch = append(batch, entryLine(t, map[string]any{
			"kind": "decision", "title": fmt.Sprintf("child %d", i), "about": []string{"auth"},
			"edges": []map[string]any{{"type": "relates_to", "to": root}},
		}))
	}
	if written := batchResult(t, runStdin(t, r, strings.Join(batch, "\n")+"\n",
		"add", "--plan", "demo", "--batch", "-")); written.Count != children {
		t.Fatalf("the fixture has to hold %d children, got %d", children, written.Count)
	}

	var out struct {
		Count     int             `json:"count"`
		Nodes     json.RawMessage `json:"nodes"`
		Truncated bool            `json:"truncated"`
		Hint      string          `json:"hint"`
	}
	walk(t, r, &out, "graph", "--from", root, "--depth", "1")

	if out.Count != children+1 {
		t.Fatalf("count is the true reach of the walk, want %d got %d", children+1, out.Count)
	}
	if string(out.Nodes) != "[]" {
		t.Errorf("above the threshold a walk returns no nodes at all, got %s", out.Nodes)
	}
	if !out.Truncated {
		t.Error("a withheld walk has to say it was truncated")
	}
	if out.Hint == "" {
		t.Error("a withheld walk has to hand back a hint saying how to get past it")
	}
}

func TestGraphAtDepthZeroIsTheRecordAlone(t *testing.T) {
	r := throwawayRepo(t)
	from := add(t, r, "RS256 over HS256")
	to := add(t, r, "Asymmetric signing")
	mustLink(t, r, "derives_from", from, to)

	type nodes struct {
		Count int `json:"count"`
		Nodes []struct {
			Entry struct {
				ID string `json:"id"`
			} `json:"entry"`
			Depth int `json:"depth"`
		} `json:"nodes"`
	}

	var out nodes
	walk(t, r, &out, "graph", "--from", from, "--depth", "0")
	if out.Count != 1 || len(out.Nodes) != 1 {
		t.Fatalf("--depth 0 walks nowhere: want one node, got count %d and %d nodes", out.Count, len(out.Nodes))
	}
	if out.Nodes[0].Entry.ID != from {
		t.Errorf("the one node is the record at --from, want %s got %s", from, out.Nodes[0].Entry.ID)
	}
	if out.Nodes[0].Depth != 0 {
		t.Errorf("the record at --from sits at depth 0, got %d", out.Nodes[0].Depth)
	}

	var deeper nodes
	walk(t, r, &deeper, "graph", "--from", from, "--depth", "1")
	if deeper.Count != 2 {
		t.Fatalf("one hop out reaches the linked record, so --depth 0 withheld something: got %d", deeper.Count)
	}
}

type graphWalk struct {
	Count int `json:"count"`
	Nodes []struct {
		Entry struct {
			ID    string `json:"id"`
			Title string `json:"title"`
		} `json:"entry"`
		Depth int `json:"depth"`
	} `json:"nodes"`
}

func TestGraphWalksATreeThatSpansTwoLogsFromEitherEnd(t *testing.T) {
	_, alpha, beta := twoDepths(t)
	parent := addObjective(t, alpha, "The objective the whole plan hangs from")
	kid := addObjective(t, beta, "The objective in the service next door")
	mustLink(t, beta, "derives_from", kid, parent)

	titles := map[string]string{
		parent: "The objective the whole plan hangs from",
		kid:    "The objective in the service next door",
	}
	for _, from := range []string{parent, kid} {
		for _, r := range []repo{alpha, beta} {
			var out graphWalk
			walk(t, r, &out, "graph", "--from", from, "--depth", "2")
			if out.Count != 2 || len(out.Nodes) != 2 {
				t.Fatalf("from %s read in %s: the walk crosses the log the edge did not land in, got %d nodes",
					from, r.root, out.Count)
			}
			got := map[string]string{}
			for _, n := range out.Nodes {
				got[n.Entry.ID] = n.Entry.Title
			}
			for id, want := range titles {
				if title, reached := got[id]; !reached || title != want {
					t.Errorf("from %s read in %s: every node carries its own record rather than being "+
						"skipped as dangling; %s came back %q", from, r.root, id, title)
				}
			}
		}
	}
}

func TestGraphRefusesAnIDNoLogOfTheStoreHolds(t *testing.T) {
	_, alpha, beta := twoDepths(t)
	addObjective(t, alpha, "The objective that was actually written")
	addObjective(t, beta, "The objective in the other repository")

	const ghost = "01ARZ3NDEKTSV4RRFFQ69G5FAV"
	res := runBare(t, alpha, "graph", "--from", ghost)
	if res.code != 1 {
		t.Fatalf("an id no log holds is still unknown, got exit %d\n%s%s", res.code, res.stdout, res.stderr)
	}
	code, msg := errorCode(t, res)
	if code != "unknown_id" {
		t.Fatalf("the refusal keeps the code it has always had, got %q: %s", code, msg)
	}
	if !strings.Contains(msg, "in any log of "+alpha.store) {
		t.Errorf("the refusal names the store it searched rather than a file nobody can open: %q", msg)
	}
}
