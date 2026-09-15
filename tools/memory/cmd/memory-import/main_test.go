package main

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
)

var importBin string

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "memory-import-bin")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	importBin = filepath.Join(dir, "memory-import")
	if out, err := exec.Command("go", "build", "-o", importBin, ".").CombinedOutput(); err != nil {
		fmt.Fprintf(os.Stderr, "building the binary under test: %v\n%s", err, out)
		os.RemoveAll(dir)
		os.Exit(1)
	}
	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}

// host is a throwaway MEMORY_ROOTS, HOME and store, so no run can reach the real
// logs or the shared store.
type host struct {
	root  string
	home  string
	store string
}

func newHost(t *testing.T) host {
	t.Helper()
	return host{
		root:  t.TempDir(),
		home:  t.TempDir(),
		store: filepath.Join(t.TempDir(), "shared", "memory.db"),
	}
}

// repoLog writes a log for a repository with no remote, whose identity is then
// its absolute path.
func (h host) repoLog(t *testing.T, name, content string) (repo, log string) {
	t.Helper()
	repo = filepath.Join(h.root, name)
	for _, dir := range []string{".git", ".ai-kit"} {
		if err := os.MkdirAll(filepath.Join(repo, dir), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	log = filepath.Join(repo, ".ai-kit", "memory.jsonl")
	writeFile(t, log, content)
	return repo, log
}

type outcome struct {
	code   int
	rep    report
	stdout string
}

func (h host) run(t *testing.T) outcome {
	t.Helper()
	return h.runCommand(t, exec.Command(importBin, "--store", h.store))
}

// runLeaving is run, failing the test if the run changed anything in repo's
// .ai-kit/.
func (h host) runLeaving(t *testing.T, repo string) outcome {
	t.Helper()
	before := snapshot(t, repo)
	o := h.run(t)
	if after := snapshot(t, repo); after != before {
		t.Fatalf("the run changed %s/.ai-kit/:\nbefore:\n%s\nafter:\n%s", repo, before, after)
	}
	return o
}

func (h host) runCommand(t *testing.T, cmd *exec.Cmd) outcome {
	t.Helper()
	// GIT_CEILING_DIRECTORIES keeps git from finding a repository above the
	// throwaway root, whose remote would then be the identity.
	cmd.Env = append(os.Environ(), "MEMORY_ROOTS="+h.root, "HOME="+h.home,
		"GIT_CEILING_DIRECTORIES="+filepath.Dir(h.root))
	var stdout, stderr strings.Builder
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	o := outcome{}
	if err := cmd.Run(); err != nil {
		var ee *exec.ExitError
		if !errors.As(err, &ee) {
			t.Fatalf("running memory-import: %v\n%s", err, stderr.String())
		}
		o.code = ee.ExitCode()
	}
	o.stdout = stdout.String()
	if err := json.Unmarshal([]byte(o.stdout), &o.rep); err != nil {
		t.Fatalf("stdout is not one JSON object: %v\nstdout: %s\nstderr: %s", err, o.stdout, stderr.String())
	}
	accounted(t, o)
	return o
}

// accounted fails the test unless every line each log read is counted once, as
// inserted, skipped or not committed, nothing is inserted by a run that
// committed nothing, and the totals are the sums of the logs.
func accounted(t *testing.T, o outcome) {
	t.Helper()
	var sum totals
	sumSkipped, skippedKnown := 0, true
	for _, l := range o.rep.Logs {
		skipped := 0
		if l.RowsSkipped == nil {
			skippedKnown = false
		} else {
			skipped = *l.RowsSkipped
		}
		sumSkipped += skipped
		if l.RowsInserted+skipped+l.LinesNotCommitted != l.LinesRead {
			t.Errorf("%s: inserted %d + skipped %d + not committed %d != read %d\n%s",
				l.Path, l.RowsInserted, skipped, l.LinesNotCommitted, l.LinesRead, o.stdout)
		}
		if !o.rep.Committed && l.RowsInserted != 0 {
			t.Errorf("%s: %d rows reported inserted by a run that committed nothing\n%s", l.Path, l.RowsInserted, o.stdout)
		}
		if (l.Identity == nil) != (l.OriginRepoIDDiffers == nil) {
			t.Errorf("%s: origin_repo_id_differs is reported for a log with no identity, or missing for one with it\n%s", l.Path, o.stdout)
		}
		sum.LinesRead += l.LinesRead
		sum.RowsInserted += l.RowsInserted
		sum.LinesNotCommitted += l.LinesNotCommitted
		if l.OriginRepoIDDiffers != nil {
			sum.OriginRepoIDDiffers += *l.OriginRepoIDDiffers
		}
	}
	got := o.rep.Totals
	if got.LinesRead != sum.LinesRead || got.RowsInserted != sum.RowsInserted ||
		got.LinesNotCommitted != sum.LinesNotCommitted || got.OriginRepoIDDiffers != sum.OriginRepoIDDiffers ||
		got.Logs != len(o.rep.Logs) {
		t.Errorf("totals are not the sums of the logs\n%s", o.stdout)
	}
	switch {
	case !skippedKnown && got.RowsSkipped != nil:
		t.Errorf("totals report rows_skipped %d while a log's is unknown\n%s", *got.RowsSkipped, o.stdout)
	case skippedKnown && (got.RowsSkipped == nil || *got.RowsSkipped != sumSkipped):
		t.Errorf("totals rows_skipped is not the sum of the logs\n%s", o.stdout)
	}
}

type row struct {
	path   string
	num    int
	scope  string
	repoID sql.NullString
	raw    string
}

func storeRows(t *testing.T, store string) []row {
	t.Helper()
	db, err := sql.Open("sqlite", store)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	rows, err := db.Query(`SELECT source_path, line_number, scope, repo_id, raw FROM lines ORDER BY source_path, line_number`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var out []row
	for rows.Next() {
		var r row
		if err := rows.Scan(&r.path, &r.num, &r.scope, &r.repoID, &r.raw); err != nil {
			t.Fatal(err)
		}
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return out
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func appendFile(t *testing.T, path, content string) {
	t.Helper()
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_APPEND, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if _, err := f.WriteString(content); err != nil {
		t.Fatal(err)
	}
}

// snapshot is everything a run could have changed in a repository's .ai-kit/:
// each entry's name, size and modification time, and the log's bytes.
func snapshot(t *testing.T, repo string) string {
	t.Helper()
	dir := filepath.Join(repo, ".ai-kit")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var b strings.Builder
	for _, e := range entries {
		info, err := e.Info()
		if err != nil {
			t.Fatal(err)
		}
		fmt.Fprintf(&b, "%s %d %d\n", e.Name(), info.Size(), info.ModTime().UnixNano())
	}
	log, err := os.ReadFile(filepath.Join(dir, "memory.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	b.Write(log)
	return b.String()
}

func TestEveryLineIsStoredOnceAndOnlyAppendedLinesAreAdded(t *testing.T) {
	h := newHost(t)
	decision := `{"kind":"decision","title":"one","origin":{"repo_id":"git@example.com:elsewhere.git"}}`
	edge := `{"kind":"edge","type":"supersedes","from":"A","to":"B"}`
	repo, log := h.repoLog(t, "svc", decision+"\n   \nnot json\n  "+edge+"  \n")
	if err := os.MkdirAll(filepath.Join(h.home, ".ai-kit"), 0o755); err != nil {
		t.Fatal(err)
	}
	preference := `{"kind":"preference","title":"user","origin":{"repo_id":"git@example.com:elsewhere.git"}}`
	writeFile(t, filepath.Join(h.home, ".ai-kit", "memory.jsonl"), preference+"\n")

	first := h.runLeaving(t, repo)
	if first.code != 0 || !first.rep.Committed {
		t.Fatalf("first run: exit %d\n%s", first.code, first.stdout)
	}
	want := []row{
		{log, 1, "repo", sql.NullString{String: repo, Valid: true}, decision},
		{log, 3, "repo", sql.NullString{String: repo, Valid: true}, "not json"},
		{log, 4, "repo", sql.NullString{String: repo, Valid: true}, edge},
		{filepath.Join(h.home, ".ai-kit", "memory.jsonl"), 1, "user", sql.NullString{}, preference},
	}
	slices.SortStableFunc(want, func(a, b row) int { return strings.Compare(a.path, b.path) })
	if got := storeRows(t, h.store); !slices.Equal(got, want) {
		t.Fatalf("store after the first run:\n got %+v\nwant %+v", got, want)
	}
	repoReport := first.rep.Logs[0]
	if repoReport.Identity == nil || *repoReport.Identity != repo || repoReport.LinesRead != 3 ||
		repoReport.RowsInserted != 3 || repoReport.LinesNotCommitted != 0 ||
		repoReport.OriginRepoIDDiffers == nil || *repoReport.OriginRepoIDDiffers != 1 {
		t.Errorf("repo log report: %+v", repoReport)
	}
	if userReport := first.rep.Logs[1]; userReport.Identity != nil || userReport.OriginRepoIDDiffers != nil {
		t.Errorf("user log reported an identity or lines differing from one: %s", first.stdout)
	}
	if first.rep.Totals.OriginRepoIDDiffers != 1 {
		t.Errorf("totals origin_repo_id_differs %d, want the repo log's 1 alone", first.rep.Totals.OriginRepoIDDiffers)
	}

	second := h.runLeaving(t, repo)
	if second.code != 0 || second.rep.Totals.RowsInserted != 0 ||
		second.rep.Totals.RowsSkipped == nil || *second.rep.Totals.RowsSkipped != 4 ||
		second.rep.RowsInStore == nil || *second.rep.RowsInStore != 4 {
		t.Fatalf("a run over unchanged logs: exit %d\n%s", second.code, second.stdout)
	}

	appended := `{"kind":"gotcha","title":"appended"}`
	appendFile(t, log, appended+"\n")
	third := h.runLeaving(t, repo)
	if third.code != 0 || third.rep.Totals.RowsInserted != 1 {
		t.Fatalf("a run after one append: exit %d\n%s", third.code, third.stdout)
	}
	afterAppend := storeRows(t, h.store)
	if len(afterAppend) != 5 || !slices.Contains(afterAppend, row{log, 5, "repo", sql.NullString{String: repo, Valid: true}, appended}) {
		t.Fatalf("store after one append: %+v", afterAppend)
	}

	content, err := os.ReadFile(log)
	if err != nil {
		t.Fatal(err)
	}
	rewritten := strings.Replace(string(content), `"title":"one"`, `"title":"uno"`, 1) + `{"kind":"gotcha","title":"never stored"}` + "\n"
	writeFile(t, log, rewritten)
	fourth := h.runLeaving(t, repo)
	if fourth.code == 0 || fourth.rep.Committed || fourth.rep.Code != "log_rewritten" {
		t.Fatalf("a run over a rewritten line: exit %d\n%s", fourth.code, fourth.stdout)
	}
	if c := fourth.rep.Logs[0].Conflicts; len(c) != 1 || c[0].Line != 1 {
		t.Errorf("conflicts named: %+v", c)
	}
	// Lines 3-5 are stored as read; line 1 differs from its row and line 6 has none.
	if l := fourth.rep.Logs[0]; l.LinesRead != 5 || l.RowsSkipped == nil || *l.RowsSkipped != 3 ||
		l.RowsInserted != 0 || l.LinesNotCommitted != 2 {
		t.Errorf("a refused log's lines are not all accounted for: %s", fourth.stdout)
	}
	if got := storeRows(t, h.store); !slices.Equal(got, afterAppend) {
		t.Fatalf("a refused run changed the store:\n got %+v\nwant %+v", got, afterAppend)
	}
}

func TestAnUnreadableLogFailsTheRunAndCommitsNothing(t *testing.T) {
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("needs a file permission the running user cannot bypass")
	}
	h := newHost(t)
	_, readable := h.repoLog(t, "a", `{"kind":"decision","title":"a"}`+"\n")
	_, unreadable := h.repoLog(t, "b", `{"kind":"decision","title":"b"}`+"\n")
	if first := h.run(t); first.code != 0 {
		t.Fatalf("first run: exit %d\n%s", first.code, first.stdout)
	}

	appendFile(t, readable, `{"kind":"decision","title":"a2"}`+"\n")
	if err := os.Chmod(unreadable, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(unreadable, 0o644) })

	o := h.run(t)
	if o.code == 0 || o.rep.Committed || o.rep.Code != "unreadable_log" || !strings.Contains(o.rep.Error, unreadable) {
		t.Fatalf("exit %d\n%s", o.code, o.stdout)
	}
	if rows := storeRows(t, h.store); len(rows) != 2 {
		t.Fatalf("the failed run left %d rows, want the 2 the first run committed", len(rows))
	}
	if l := o.rep.Logs[0]; l.Path != readable || l.LinesRead != 2 || l.RowsSkipped != nil || l.LinesNotCommitted != 2 {
		t.Errorf("a log read before the store was reached must report skipped as unknown and every line not committed: %s", o.stdout)
	}
	if o.rep.Totals.RowsSkipped != nil {
		t.Errorf("totals report rows_skipped %d for a run that never reached the store", *o.rep.Totals.RowsSkipped)
	}
}

func TestTheStoreIsGroupWritableUnderARestrictiveUmask(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("no umask or setgid")
	}
	h := newHost(t)
	h.repoLog(t, "a", `{"kind":"decision","title":"a"}`+"\n")

	cmd := exec.Command("/bin/sh", "-c", `umask 077 && exec "$0" --store "$1"`, importBin, h.store)
	if o := h.runCommand(t, cmd); o.code != 0 {
		t.Fatalf("exit %d\n%s", o.code, o.stdout)
	}

	dir, err := os.Stat(filepath.Dir(h.store))
	if err != nil {
		t.Fatal(err)
	}
	if dir.Mode()&os.ModeSetgid == 0 || dir.Mode().Perm()&0o070 != 0o070 {
		t.Errorf("store directory mode %v, want setgid and group rwx", dir.Mode())
	}
	entries, err := os.ReadDir(filepath.Dir(h.store))
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		info, err := e.Info()
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().Perm()&0o060 != 0o060 {
			t.Errorf("%s mode %v, want group read and write", e.Name(), info.Mode())
		}
	}
}
