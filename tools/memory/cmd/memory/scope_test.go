package main

import (
	"encoding/json"
	"strings"
	"testing"
)

type storeWideRead struct {
	name string
	args []string
}

func storeWideReads(from string) []storeWideRead {
	return []storeWideRead{
		{"graph", []string{"graph", "--from", from}},
		{"edges --from", []string{"edges", "--from", from}},
		{"anchors --all", []string{"anchors", "--all"}},
		{"state --all", []string{"state", "--all"}},
		{"state --text", []string{"state", "--text"}},
		{"validate --all", []string{"validate", "--all"}},
	}
}

func withScope(args []string, scope string) []string {
	out := append([]string{}, args...)
	return append(out, "--scope", scope)
}

func aTreeAcrossTwoLogs(t *testing.T) (repo, repo, string, string) {
	t.Helper()
	_, alpha, beta := twoDepths(t)
	parent := addObjective(t, alpha, "The objective the whole plan hangs from")
	kid := addObjective(t, beta, "The objective in the service next door")
	mustLink(t, beta, "derives_from", kid, parent)
	return alpha, beta, parent, kid
}

func TestAStoreWideAnswerRefusesAScopeTheCallerNamed(t *testing.T) {
	alpha, _, parent, _ := aTreeAcrossTwoLogs(t)

	for _, tc := range storeWideReads(parent) {
		t.Run(tc.name, func(t *testing.T) {
			res := runBare(t, alpha, withScope(tc.args, "user")...)
			if res.code != 1 {
				t.Fatalf("%v answers from every log, so a --scope naming one has to be refused: exit %d\n%s%s",
					tc.args, res.code, res.stdout, res.stderr)
			}
			code, msg := errorCode(t, res)
			if code != "scope_not_on_store_wide_read" {
				t.Errorf("the refusal code names the flag it refused, got %q", code)
			}
			if !strings.Contains(msg, "--scope") {
				t.Errorf("the message names the flag it refused, got %q", msg)
			}
			if !strings.Contains(msg, "every log in "+alpha.store) {
				t.Errorf("the message says the answer spans every log in the store, got %q", msg)
			}
		})
	}
}

func TestAScopeNobodyNamedChangesNothing(t *testing.T) {
	alpha, _, parent, _ := aTreeAcrossTwoLogs(t)

	for _, tc := range storeWideReads(parent) {
		t.Run(tc.name, func(t *testing.T) {
			res := runBare(t, alpha, tc.args...)
			if res.code != 0 {
				t.Fatalf("%v answers as it always did when nobody named --scope: exit %d\n%s%s",
					tc.args, res.code, res.stdout, res.stderr)
			}
			if strings.Contains(res.stdout, "scope_not_on_store_wide_read") {
				t.Fatalf("a --scope nobody named is refused by nothing:\n%s", res.stdout)
			}
			named := runBare(t, alpha, withScope(tc.args, "repo")...)
			if named.code != 1 {
				t.Fatalf("the refusal keys on the flag being passed rather than on its value, so the "+
					"default spelled out loud is still refused: exit %d\n%s%s",
					named.code, named.stdout, named.stderr)
			}
		})
	}
}

func TestAStoreWideAnswerNamesEveryLogItRead(t *testing.T) {
	alpha, beta, parent, kid := aTreeAcrossTwoLogs(t)

	for _, args := range [][]string{
		{"graph", "--from", parent},
		{"edges", "--from", kid},
	} {
		t.Run(args[0], func(t *testing.T) {
			res := runBare(t, alpha, args...)
			if res.code != 0 {
				t.Fatalf("%v: exit %d\n%s%s", args, res.code, res.stdout, res.stderr)
			}
			var out map[string]json.RawMessage
			if err := json.Unmarshal([]byte(res.stdout), &out); err != nil {
				t.Fatalf("%v printed something that is not JSON: %v\n%s", args, err, res.stdout)
			}
			if _, named := out["path"]; named {
				t.Errorf("an answer read out of every log names no single one:\n%s", res.stdout)
			}
			if _, named := out["scope"]; named {
				t.Errorf("an answer read out of every log carries no scope:\n%s", res.stdout)
			}
			var logs []string
			if err := json.Unmarshal(out["logs"], &logs); err != nil {
				t.Fatalf("%v has to name the logs it read under `logs`: %v\n%s", args, err, res.stdout)
			}
			got, want := sorted(logs...), sorted(alpha.log, beta.log)
			if strings.Join(got, ",") != strings.Join(want, ",") {
				t.Errorf("%v names every log it read, got %v want %v", args, got, want)
			}
		})
	}
}
