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
