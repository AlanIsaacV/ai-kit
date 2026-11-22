package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type repoAnswer struct {
	Count        int            `json:"count"`
	Returned     int            `json:"returned"`
	Truncated    bool           `json:"truncated"`
	Hint         string         `json:"hint"`
	Logs         []string       `json:"logs"`
	Unattributed []string       `json:"unattributed_logs"`
	Interpreted  map[string]any `json:"interpreted"`
	Records      []struct {
		ID    string `json:"id"`
		Title string `json:"title"`
	} `json:"records"`
}

func elsewhere(t *testing.T, s spread) repo {
	t.Helper()
	return repo{root: t.TempDir(), home: s.home, store: s.store}
}

func readByRepo(t *testing.T, s spread, args ...string) result {
	t.Helper()
	return execMemory(t, elsewhere(t, s), "", args)
}

func answerOf(t *testing.T, res result) repoAnswer {
	t.Helper()
	if res.code != 0 {
		t.Fatalf("exit %d\n%s%s", res.code, res.stdout, res.stderr)
	}
	var out repoAnswer
	if err := json.Unmarshal([]byte(res.stdout), &out); err != nil {
		t.Fatalf("the answer is not JSON: %v\n%s", err, res.stdout)
	}
	return out
}

func remoteFrom(t *testing.T, r repo, line int, remote string) {
	t.Helper()
	db := openStore(t, r)
	defer db.Close()
	if _, err := db.Exec(`UPDATE lines SET repo_id = ? WHERE source_path = ? AND line_number >= ?`,
		remote, r.log, line); err != nil {
		t.Fatal(err)
	}
}

func storeShape(t *testing.T, r repo) (logs, rows int) {
	t.Helper()
	db := openStore(t, r)
	defer db.Close()
	if err := db.QueryRow(`SELECT count(DISTINCT source_path), count(*) FROM lines`).Scan(&logs, &rows); err != nil {
		t.Fatal(err)
	}
	return logs, rows
}

func lastCall(t *testing.T, home string) map[string]any {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(home, ".ai-kit", "calls", "memory.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	all := strings.Split(strings.TrimSpace(string(raw)), "\n")
	var out map[string]any
	if err := json.Unmarshal([]byte(all[len(all)-1]), &out); err != nil {
		t.Fatalf("the call log's last line is not JSON: %v\n%s", err, all[len(all)-1])
	}
	return out
}

func TestSearchByRepoAnswersOnlyFromThatRepositorysLogs(t *testing.T) {
	s := throwawaySpread(t)
	alpha, beta := repoIn(t, s, "alpha"), repoIn(t, s, "beta")
	mine := add(t, alpha, "Rotate the signing key every ninety days")
	add(t, beta, "Rotate the signing key every ninety days")
	remoteFrom(t, alpha, 1, "https://github.com/octo-org/alpha.git")
	remoteFrom(t, beta, 1, "https://github.com/octo-org/beta.git")

	got := answerOf(t, readByRepo(t, s, "search", "--repo", "octo-org/alpha", "--", "signing key"))
	if got.Count != 1 || len(got.Records) != 1 || got.Records[0].ID != mine {
		t.Fatalf("search --repo answers from that repository's log alone, want only %s, got %+v", mine, got)
	}
	if strings.Join(got.Logs, ",") != alpha.log {
		t.Errorf("logs names the one log read, want %s, got %v", alpha.log, got.Logs)
	}
	if got.Interpreted["repo"] != "octo-org/alpha" {
		t.Errorf("interpreted echoes repo, got %v", got.Interpreted)
	}
	if _, ok := got.Interpreted["scope"]; ok {
		t.Errorf("a --repo read chooses no scope, so interpreted carries none: %v", got.Interpreted)
	}
	if _, ok := got.Interpreted["dir"]; ok {
		t.Errorf("a --repo read uses no directory, so interpreted carries none: %v", got.Interpreted)
	}
}

func TestARepoNoLogCarriesIsRefusedWithTheNamesClosestToIt(t *testing.T) {
	s := throwawaySpread(t)
	alpha, beta := repoIn(t, s, "alpha"), repoIn(t, s, "beta")
	add(t, alpha, "Rotate the signing key every ninety days")
	add(t, beta, "Keep the cache warm")
	remoteFrom(t, alpha, 1, "git@github.com:octo-org/payments-service.git")
	remoteFrom(t, beta, 1, "https://github.com/octo-org/web")

	res := readByRepo(t, s, "query", "--repo", "octo-org/payment-service")
	if res.code == 0 {
		t.Fatalf("a name no log carries is refused, not answered as empty:\n%s", res.stdout)
	}
	code, msg := errorCode(t, res)
	if code != "unknown_repo" {
		t.Errorf("want unknown_repo, got %q: %s", code, msg)
	}
	if !strings.Contains(msg, "octo-org/payments-service") {
		t.Errorf("the refusal names the similar name the store carries: %s", msg)
	}
	if strings.Contains(msg, "octo-org/web") {
		t.Errorf("the refusal names only similar names: %s", msg)
	}

	for _, value := range []string{"octo-org", "octo-org/payments/service", "octo-org/pay$ments", "", "octo-org/web ", " octo-org/web"} {
		t.Run(value, func(t *testing.T) {
			res := readByRepo(t, s, "query", "--repo", value)
			if res.code == 0 {
				t.Fatalf("--repo %q is not owner/name and is refused:\n%s", value, res.stdout)
			}
			if code, msg := errorCode(t, res); code != "bad_repo" {
				t.Errorf("--repo %q is refused under its own code, got %q: %s", value, code, msg)
			}
		})
	}
}

func TestRepoTogetherWithDirOrANamedScopeIsRefused(t *testing.T) {
	s := throwawaySpread(t)
	alpha := repoIn(t, s, "alpha")
	add(t, alpha, "Rotate the signing key every ninety days")
	remoteFrom(t, alpha, 1, "https://github.com/octo-org/alpha")

	for name, args := range map[string][]string{
		"--dir":          {"query", "--repo", "octo-org/alpha", "--dir", alpha.root},
		"--scope repo":   {"query", "--repo", "octo-org/alpha", "--scope", "repo"},
		"--scope user":   {"query", "--repo", "octo-org/alpha", "--scope", "user"},
		"--scope both":   {"query", "--repo", "octo-org/alpha", "--scope", "both"},
		"search --dir":   {"search", "--repo", "octo-org/alpha", "--dir", alpha.root, "--", "signing"},
		"search --scope": {"search", "--repo", "octo-org/alpha", "--scope", "repo", "--", "signing"},
	} {
		t.Run(name, func(t *testing.T) {
			res := readByRepo(t, s, args...)
			if res.code == 0 {
				t.Fatalf("%v is refused:\n%s", args, res.stdout)
			}
			code, msg := errorCode(t, res)
			if code != "conflicting_scope" {
				t.Errorf("want conflicting_scope, got %q: %s", code, msg)
			}
			flag := strings.Fields(strings.TrimPrefix(name, "search "))[0]
			if !strings.Contains(msg, flag) || !strings.Contains(msg, "--repo") {
				t.Errorf("the refusal names both flags in conflict: %s", msg)
			}
		})
	}
}

func TestARepoReadAnswersEmptyAndWithheldAsEveryReadDoes(t *testing.T) {
	s := throwawaySpread(t)
	alpha := repoIn(t, s, "alpha")
	for _, title := range []string{"First", "Second", "Third"} {
		add(t, alpha, title)
	}
	remoteFrom(t, alpha, 1, "https://github.com/octo-org/alpha")

	empty := answerOf(t, readByRepo(t, s, "query", "--repo", "octo-org/alpha", "--kind", "preference"))
	if empty.Count != 0 || empty.Interpreted["repo"] != "octo-org/alpha" {
		t.Errorf("nothing matching is count 0 at exit 0 with repo echoed, got %+v", empty)
	}

	withheld := answerOf(t, readByRepo(t, s, "query", "--repo", "octo-org/alpha", "--threshold", "2"))
	if !withheld.Truncated || withheld.Count != 3 || withheld.Returned != 0 || withheld.Hint == "" {
		t.Errorf("above the threshold the records are withheld with a count and a hint, got %+v", withheld)
	}
}

func TestALogIsMatchedByAnyOfItsRowsRemoteNotOnlyItsEarliest(t *testing.T) {
	s := throwawaySpread(t)
	alpha := repoIn(t, s, "alpha")
	add(t, alpha, "Written before the remote existed")
	add(t, alpha, "Written after it was pushed")
	remoteFrom(t, alpha, 1, alpha.root)
	remoteFrom(t, alpha, 2, "https://github.com/octo-org/alpha.git")

	got := answerOf(t, readByRepo(t, s, "query", "--repo", "octo-org/alpha"))
	if strings.Join(got.Logs, ",") != alpha.log || got.Count != 2 {
		t.Errorf("a log whose later rows name the remote is that repository's, whole: got %+v", got)
	}
}

func TestEveryFormAndCaseOfOneRemoteNamesTheSameRepository(t *testing.T) {
	s := throwawaySpread(t)
	var want []string
	for i, remote := range []string{
		"git@github.com:Owner/Name.git",
		"https://github.com/owner/name",
		"https://github.com/OWNER/name.git/",
		"ssh://git@github.com/owner/name",
	} {
		r := repoIn(t, s, "clone"+string(rune('a'+i)))
		add(t, r, "Recorded under "+remote)
		remoteFrom(t, r, 1, remote)
		want = append(want, r.log)
	}
	other := repoIn(t, s, "other")
	add(t, other, "Recorded under another repository")
	remoteFrom(t, other, 1, "https://github.com/owner/name-two")

	got := answerOf(t, readByRepo(t, s, "query", "--repo", "owner/name"))
	if strings.Join(sorted(got.Logs...), ",") != strings.Join(sorted(want...), ",") || got.Count != 4 {
		t.Errorf("ssh, https and case variants all name owner/name, want %v, got %+v", want, got)
	}
}

func TestARecordRepeatedAcrossARepositorysLogsIsReturnedOnce(t *testing.T) {
	s := throwawaySpread(t)
	const remote = "https://github.com/octo-org/alpha.git"
	alpha := repoIn(t, s, "alpha")
	id := add(t, alpha, "Rotate the signing key every ninety days")
	remoteFrom(t, alpha, 1, remote)
	worktree := filepath.Join(s.root, "alpha-worktree", ".ai-kit", "memory.jsonl")
	db := openStore(t, alpha)
	if _, err := db.Exec(`INSERT INTO lines (source_path, line_number, scope, repo_id, raw)
		SELECT ?, 1, 'repo', ?, raw FROM lines WHERE source_path = ? AND line_number = 1`,
		worktree, remote, alpha.log); err != nil {
		t.Fatal(err)
	}
	db.Close()

	got := answerOf(t, readByRepo(t, s, "query", "--repo", "octo-org/alpha"))
	if len(got.Logs) != 2 {
		t.Fatalf("the fixture is one repository with two logs, got %v", got.Logs)
	}
	if got.Count != 1 || len(got.Records) != 1 || got.Records[0].ID != id {
		t.Errorf("a record in two logs of one repository comes back once, got %+v", got)
	}
}

func TestARepoReadCreatesNothing(t *testing.T) {
	s := throwawaySpread(t)
	alpha := repoIn(t, s, "alpha")
	add(t, alpha, "Rotate the signing key every ninety days")
	remoteFrom(t, alpha, 1, "https://github.com/octo-org/alpha")
	logsBefore, rowsBefore := storeShape(t, alpha)

	here := elsewhere(t, s)
	res := execMemory(t, here, "", []string{"query", "--repo", "octo-org/alpha", "--current"})
	if got := answerOf(t, res); got.Count != 1 {
		t.Fatalf("the read answers the repository's record, got %+v", got)
	}
	if entries, err := os.ReadDir(here.root); err != nil || len(entries) != 0 {
		t.Errorf("the directory the read ran in is left as it was, got %v %v", entries, err)
	}
	if logs, rows := storeShape(t, alpha); logs != logsBefore || rows != rowsBefore {
		t.Errorf("the store gains no log and no row: %d logs %d rows before, %d %d after",
			logsBefore, rowsBefore, logs, rows)
	}

	absent := repo{root: t.TempDir(), home: s.home, store: filepath.Join(t.TempDir(), "absent", "memory.db")}
	execMemory(t, absent, "", []string{"query", "--repo", "octo-org/alpha"})
	if _, err := os.Stat(filepath.Dir(absent.store)); !os.IsNotExist(err) {
		t.Errorf("a store path that does not exist is not created by a read: %v", err)
	}
}

func TestTheCallLogCarriesTheRepoAndTheOutcome(t *testing.T) {
	s := throwawaySpread(t)
	alpha := repoIn(t, s, "alpha")
	add(t, alpha, "Rotate the signing key every ninety days")
	remoteFrom(t, alpha, 1, "https://github.com/octo-org/alpha")

	answerOf(t, readByRepo(t, s, "query", "--repo", "octo-org/alpha"))
	call := lastCall(t, s.home)
	if interp, _ := call["interpreted"].(map[string]any); interp["repo"] != "octo-org/alpha" || call["outcome"] != "ok" {
		t.Errorf("a --repo read is logged with its repo and outcome, got %v", call)
	}
	if interp, _ := call["interpreted"].(map[string]any); interp["dir"] != nil || interp["scope"] != nil {
		t.Errorf("a --repo read is logged with no directory and no scope, got %v", call)
	}

	readByRepo(t, s, "query", "--repo", "octo-org/alpah")
	call = lastCall(t, s.home)
	interp, _ := call["interpreted"].(map[string]any)
	if interp["repo"] != "octo-org/alpah" || call["outcome"] != "invalid" || call["outcome_detail"] != "unknown_repo" {
		t.Errorf("an unknown_repo refusal is logged with its repo and outcome, got %v", call)
	}
}

func TestARepoReadIsBoundedByANamedPlanAndNotByTheCallersEnvironment(t *testing.T) {
	s := throwawaySpread(t)
	alpha := repoIn(t, s, "alpha")
	ours := addTo(t, alpha, "ours", "Rotate the signing key every ninety days")
	theirs := addTo(t, alpha, "theirs", "Rotate the signing key on every release")
	remoteFrom(t, alpha, 1, "https://github.com/octo-org/alpha")

	for name, args := range map[string][]string{
		"query":  {"query", "--repo", "octo-org/alpha"},
		"search": {"search", "--repo", "octo-org/alpha", "--", "signing key"},
	} {
		t.Run(name, func(t *testing.T) {
			unbounded := answerOf(t, execMemory(t, elsewhere(t, s), "", args, "AI_KIT_PLAN=ours"))
			if unbounded.Count != 2 || len(unbounded.Records) != 2 {
				t.Fatalf("AI_KIT_PLAN does not bound a --repo read, want both plans' records, got %+v", unbounded)
			}
			if ids := []string{unbounded.Records[0].ID, unbounded.Records[1].ID}; strings.Join(sorted(ids...), ",") != strings.Join(sorted(ours, theirs), ",") {
				t.Errorf("want %s and %s, got %v", ours, theirs, ids)
			}
			if unbounded.Interpreted["plan"] != "" {
				t.Errorf("interpreted echoes no plan when none was applied, got %v", unbounded.Interpreted)
			}

			named := append([]string{args[0], "--plan", "theirs"}, args[1:]...)
			bounded := answerOf(t, execMemory(t, elsewhere(t, s), "", named, "AI_KIT_PLAN=ours"))
			if bounded.Count != 1 || len(bounded.Records) != 1 || bounded.Records[0].ID != theirs {
				t.Errorf("--plan named on the command line bounds a --repo read, want only %s, got %+v", theirs, bounded)
			}
			if bounded.Interpreted["plan"] != "theirs" {
				t.Errorf("interpreted echoes the plan applied, got %v", bounded.Interpreted)
			}
		})
	}
}
