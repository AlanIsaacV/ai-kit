package main

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/alanv/ai-kit/tools/memory/internal/memory"
	_ "modernc.org/sqlite"
)

// These tests drive the built binary rather than calling into the package.
//
// What is being checked here is the bytes that land on disk and what the
// process prints when it meets them again — and neither is observable from
// inside the package. A stamp that only appears when a test constructs an
// Entry itself would prove nothing about what `memory add` writes.

var memoryBin string

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "memory-bin")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	memoryBin = filepath.Join(dir, "memory")
	if out, err := exec.Command("go", "build", "-o", memoryBin, ".").CombinedOutput(); err != nil {
		fmt.Fprintf(os.Stderr, "building the binary under test: %v\n%s", err, out)
		os.RemoveAll(dir)
		os.Exit(1)
	}
	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}

// repo is a throwaway repo plus the private HOME the tool's user scope resolves
// into.
type repo struct {
	root  string // the repo root, whose .ai-kit/memory.jsonl names its log
	home  string // HOME, so nothing here can reach the real user-scope log
	log   string // <root>/.ai-kit/memory.jsonl
	user  string // <home>/.ai-kit/memory.jsonl
	store string
}

// throwawayRepo is a repo root the tool will accept. RepoRoot only looks for a
// .git entry, so an empty directory of that name is enough and no real git
// repository — or git binary — is involved.
//
// HOME is a *sibling* of the repo, not a directory inside it. With HOME set to
// the repo root the repo log and the user log would resolve to the same file,
// and `promote` — the one operation left that crosses logs — would have nothing
// to cross.
func throwawayRepo(t *testing.T) repo {
	t.Helper()
	r := repo{root: t.TempDir(), home: t.TempDir(), store: throwawayStore(t)}
	if err := os.MkdirAll(filepath.Join(r.root, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	r.log = filepath.Join(r.root, ".ai-kit", "memory.jsonl")
	r.user = filepath.Join(r.home, ".ai-kit", "memory.jsonl")
	return r
}

func throwawayStore(t *testing.T) string {
	t.Helper()
	return filepath.Join(t.TempDir(), "memory.db")
}

type result struct {
	stdout string
	stderr string
	code   int
}

// run is the common case: the throwaway repo, and the `demo` plan named
// explicitly.
func run(t *testing.T, r repo, args ...string) result {
	t.Helper()
	return runBare(t, r, afterSubcommand(t, args, "--plan", "demo")...)
}

// runBare leaves the plan to the caller, which is what a test about the plan
// itself needs — whether it is absent, invalid, or one of several.
func runBare(t *testing.T, r repo, args ...string) result {
	t.Helper()
	return runStdin(t, r, "", args...)
}

// runStdin is runBare with something on the process's standard input, for the one
// flag that reads it.
func runStdin(t *testing.T, r repo, stdin string, args ...string) result {
	t.Helper()
	return execMemory(t, r, stdin, afterSubcommand(t, args, "--dir", r.root))
}

// execMemory runs the binary with the arguments exactly as given — no --dir, no
// --plan — which is what a test about `kinds` needs, since that is the one
// subcommand binding neither. The store and the environment are set here and
// only here, so no way of running the binary can reach the real store.
//
// --store is what bounds a write now, and it is the only thing that does: --dir
// picks the source path a row is filed under, not the file the row lands in, so
// an invocation that named the repo and forgot the store would append to
// /srv/projects/.ai-kit/memory.db — permanently, since the store is insert-only.
//
// extraEnv is appended last, and os/exec keeps the last occurrence of a name, so a
// caller may override one of the three set here — PATH included, which is how a
// test counts what the tool shells out to.
func execMemory(t *testing.T, r repo, stdin string, args []string, extraEnv ...string) result {
	t.Helper()
	cmd := exec.Command(memoryBin, afterSubcommand(t, args, "--store", r.store)...)
	if stdin != "" {
		cmd.Stdin = strings.NewReader(stdin)
	}
	cmd.Env = append(os.Environ(), "HOME="+r.home, "AI_KIT_AUTHOR=test-suite", "AI_KIT_PLAN=")
	cmd.Env = append(cmd.Env, extraEnv...)
	cmd.Dir = r.root
	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	res := result{}
	err := cmd.Run()
	var ee *exec.ExitError
	switch {
	case err == nil:
	case errors.As(err, &ee):
		res.code = ee.ExitCode()
	default:
		t.Fatalf("running %v: %v", args, err)
	}
	res.stdout, res.stderr = stdout.String(), stderr.String()
	return res
}

// afterSubcommand puts flags immediately behind the subcommand rather than at
// the end of the line.
//
// Go's flag package stops parsing at the first non-flag argument, so a --dir
// appended after `search`'s text would be swallowed as more text and never take
// effect — pointing the command at whatever repo the test process happens to be
// running inside, which is this one.
func afterSubcommand(t *testing.T, args []string, flags ...string) []string {
	t.Helper()
	if len(args) == 0 {
		t.Fatal("no subcommand to run")
	}
	out := make([]string, 0, len(args)+len(flags))
	out = append(out, args[0])
	out = append(out, flags...)
	return append(out, args[1:]...)
}

func add(t *testing.T, r repo, title string) string {
	t.Helper()
	return addTo(t, r, "demo", title)
}

// addTo names the plan, so a test can build a log holding records from more than
// one — which is the situation the collapse created and the case `--plan` has to
// bound.
func addTo(t *testing.T, r repo, plan, title string) string {
	t.Helper()
	res := runBare(t, r, "add", "--plan", plan, "--kind", "decision", "--title", title, "--about", "auth")
	if res.code != 0 {
		t.Fatalf("add %q: exit %d\n%s%s", title, res.code, res.stdout, res.stderr)
	}
	return recordID(t, res)
}

func recordID(t *testing.T, res result) string {
	t.Helper()
	var out struct {
		Record struct {
			ID string `json:"id"`
		} `json:"record"`
	}
	if err := json.Unmarshal([]byte(res.stdout), &out); err != nil {
		t.Fatalf("add printed something that is not JSON: %v\n%s", err, res.stdout)
	}
	if out.Record.ID == "" {
		t.Fatalf("add printed no record id: %s", res.stdout)
	}
	return out.Record.ID
}

func errorCode(t *testing.T, res result) (code, message string) {
	t.Helper()
	var out struct {
		Code  string `json:"code"`
		Error string `json:"error"`
	}
	if err := json.Unmarshal([]byte(res.stdout), &out); err != nil {
		t.Fatalf("a refusal must still print JSON: %v\n%s%s", err, res.stdout, res.stderr)
	}
	return out.Code, out.Error
}

func lines(t *testing.T, r repo, path string) []string {
	t.Helper()
	if _, err := os.Stat(r.store); os.IsNotExist(err) {
		return nil
	}
	db := openStore(t, r)
	defer db.Close()

	rows, err := db.Query(`SELECT raw FROM lines WHERE source_path = ? ORDER BY line_number ASC`, path)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var raw string
		if err := rows.Scan(&raw); err != nil {
			t.Fatal(err)
		}
		out = append(out, raw)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return out
}

func logText(t *testing.T, r repo, path string) string {
	t.Helper()
	return strings.Join(lines(t, r, path), "\n")
}

func openStore(t *testing.T, r repo) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", r.store)
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	return db
}

func appendLine(t *testing.T, r repo, path, line string) {
	t.Helper()
	db := openStore(t, r)
	defer db.Close()

	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS lines (
		source_path TEXT    NOT NULL,
		line_number INTEGER NOT NULL,
		scope       TEXT    NOT NULL,
		repo_id     TEXT,
		raw         TEXT    NOT NULL,
		PRIMARY KEY (source_path, line_number)
	)`); err != nil {
		t.Fatal(err)
	}
	scope := "repo"
	if path == r.user {
		scope = "user"
	}
	if _, err := db.Exec(
		`INSERT INTO lines (source_path, line_number, scope, repo_id, raw)
		 SELECT ?, coalesce(max(line_number), 0) + 1, ?, NULL, ? FROM lines WHERE source_path = ?`,
		path, scope, line, path); err != nil {
		t.Fatal(err)
	}
}

func appendToFile(t *testing.T, path, line string) {
	t.Helper()
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if _, err := f.WriteString(line + "\n"); err != nil {
		t.Fatal(err)
	}
}

// ------------------------------------------------------------- the stamp ---

// Every record the tool writes says which schema it was written against, in
// the bytes, as a key of its own. Read back as JSON rather than matched as a
// substring, so a `version` mentioned inside a body could not pass for one.
func TestAddAndLinkStampTheCurrentSchemaVersion(t *testing.T) {
	r := throwawayRepo(t)

	from := add(t, r, "RS256 over HS256")
	to := add(t, r, "Asymmetric signing")
	if res := run(t, r, "link", "--type", "derives_from", "--from", from, "--to", to); res.code != 0 {
		t.Fatalf("link: exit %d\n%s%s", res.code, res.stdout, res.stderr)
	}

	got := lines(t, r, r.log)
	if len(got) != 3 {
		t.Fatalf("expected two entries and one edge, got %d lines", len(got))
	}
	for i, line := range got {
		var rec map[string]any
		if err := json.Unmarshal([]byte(line), &rec); err != nil {
			t.Fatalf("line %d is not JSON: %v", i+1, err)
		}
		v, ok := rec["version"]
		if !ok {
			t.Fatalf("line %d carries no version key: %s", i+1, line)
		}
		if v != float64(memory.CurrentSchemaVersion) {
			t.Errorf("line %d declares version %v, want %d", i+1, v, memory.CurrentSchemaVersion)
		}
		// First on the line: the field that says how to read every field
		// after it should not be something you have to scroll past a body to
		// find.
		if !strings.HasPrefix(line, fmt.Sprintf(`{"version":%d,`, memory.CurrentSchemaVersion)) {
			t.Errorf("line %d should open with its version: %s", i+1, line)
		}
	}
}

// The default scope is the only scope a record can be written at without saying
// so, and it has to be the repo log — the plan scope it used to default to is
// gone, and defaulting to `user` would scatter a plan's working noise into a log
// meant to outlive every repo.
func TestAddDefaultsToTheRepoScope(t *testing.T) {
	r := throwawayRepo(t)
	add(t, r, "RS256 over HS256")

	var rec struct {
		Scope  string `json:"scope"`
		Origin struct {
			Plan string `json:"plan"`
		} `json:"origin"`
	}
	if err := json.Unmarshal([]byte(lines(t, r, r.log)[0]), &rec); err != nil {
		t.Fatal(err)
	}
	if rec.Scope != "repo" {
		t.Errorf("expected scope repo on disk, got %q", rec.Scope)
	}
	if rec.Origin.Plan != "demo" {
		t.Errorf("expected origin.plan to carry the plan, got %q", rec.Origin.Plan)
	}
	if got := lines(t, r, r.user); len(got) != 0 {
		t.Errorf("nothing should have touched the user log, it holds %d lines", len(got))
	}
}

// The version is stamped, not chosen. A caller that could pick one would
// eventually pick wrong, and a record that lies about its version is worse
// than one with no version at all.
func TestThereIsNoFlagForTheVersion(t *testing.T) {
	r := throwawayRepo(t)
	add(t, r, "RS256 over HS256")
	before := lines(t, r, r.log)

	res := run(t, r, "add", "--kind", "decision", "--title", "Forced", "--about", "auth", "--version", "3")
	if res.code == 0 {
		t.Fatal("--version should not be a flag this tool accepts")
	}
	if after := lines(t, r, r.log); len(after) != len(before) {
		t.Fatalf("a refused add must write nothing, log went from %d to %d lines", len(before), len(after))
	}
}

// The write gate cannot be talked into accepting the scope version 2 removed —
// not by a flag, and not by an environment variable, because there is no code
// path that reads either. The refusal is the current version's rule set, and
// that is the only lever there is.
func TestNoFlagOrVariableReopensThePlanScope(t *testing.T) {
	r := throwawayRepo(t)
	add(t, r, "RS256 over HS256")
	before := lines(t, r, r.log)

	for _, args := range [][]string{
		{"add", "--scope", "plan", "--kind", "decision", "--title", "Sneaky", "--about", "auth"},
		{"add", "--scope", "plan", "--kind", "decision", "--title", "Sneaky", "--about", "auth", "--force"},
		{"add", "--scope", "plan", "--kind", "decision", "--title", "Sneaky", "--about", "auth", "--legacy"},
	} {
		res := run(t, r, args...)
		if res.code == 0 {
			t.Fatalf("%v was accepted; the plan scope must not be reachable", args)
		}
	}
	if after := lines(t, r, r.log); len(after) != len(before) {
		t.Fatalf("nothing refused may reach disk, log went from %d to %d lines", len(before), len(after))
	}

	// Via the environment, which is the other place leniency tends to hide.
	cmd := exec.Command(memoryBin, "add", "--store", r.store, "--kind", "decision", "--title", "Sneaky",
		"--about", "auth", "--plan", "demo", "--dir", r.root)
	cmd.Env = append(os.Environ(), "HOME="+r.home, "AI_KIT_AUTHOR=test-suite",
		"AI_KIT_SCOPE=plan", "MEMORY_SCOPE=plan", "AI_KIT_ALLOW_PLAN_SCOPE=1")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("the add itself should still succeed at the repo scope: %v\n%s", err, out)
	}
	for _, line := range lines(t, r, r.log) {
		if strings.Contains(line, `"scope":"plan"`) {
			t.Fatalf("an environment variable talked the gate into the plan scope: %s", line)
		}
	}
}

// The kind version 3 removed, from both sides at once: nothing may write it, and
// everything already written with it still reads.
//
// The write half is the gate, which is the current version's rules and has no
// lever on it — `--kind unit` is refused for the kind, and a refused add appends
// nothing, so the log is the same length afterwards.
//
// The read half is the dispatch, and it is the reason the removal cost a version
// bump rather than an edit to version 1's kind set. The two hand-written lines
// below are the shape this repo's own log holds six of: one reading as version 1
// with `scope: plan`, one declaring version 2, both carrying kind `unit`. They
// sweep clean, they are counted, and nothing about them is unjudged — this binary
// has rules for both versions and those rules still say `unit` is a kind.
func TestNothingWritesTheUnitKindAndTheRecordsThatDoStillRead(t *testing.T) {
	r := throwawayRepo(t)
	add(t, r, "RS256 over HS256")
	appendLine(t, r, r.log, `{"id":"01KZ34GQZ7T0WZPT4BBBMN93EK","kind":"unit","slug":"an-unstamped-unit",`+
		`"scope":"plan","at":"2026-08-01T10:00:00Z","about":["auth"],`+
		`"title":"A unit written before the stamp existed","origin":{"plan":"demo"}}`)
	appendLine(t, r, r.log, `{"version":2,"id":"01KZ4N1PM38APVV93MG3J38BD2","kind":"unit","slug":"a-v2-unit",`+
		`"scope":"repo","at":"2026-08-02T10:00:00Z","about":["auth"],`+
		`"title":"A unit written while the kind was still current","origin":{"plan":"demo"}}`)
	before := lines(t, r, r.log)

	res := run(t, r, "add", "--kind", "unit", "--title", "One more unit", "--about", "auth")
	if res.code == 0 {
		t.Fatal("`unit` is not a kind any more and the gate must refuse it")
	}
	code, _ := errorCode(t, res)
	if code != "invalid_record" {
		t.Errorf("expected an invalid_record refusal, got %q", code)
	}
	// The refusal carries the field and the value, so a caller who still has the
	// old vocabulary in hand learns which word the tool stopped speaking — and the
	// set it advertises instead no longer holds it.
	var refusal struct {
		Problems []struct {
			Field   string `json:"field"`
			Message string `json:"message"`
		} `json:"problems"`
	}
	if err := json.Unmarshal([]byte(res.stdout), &refusal); err != nil {
		t.Fatalf("a refusal must still print JSON: %v\n%s", err, res.stdout)
	}
	if len(refusal.Problems) != 1 || refusal.Problems[0].Field != "kind" {
		t.Fatalf("expected one problem, on the kind: %+v", refusal.Problems)
	}
	if !strings.Contains(refusal.Problems[0].Message, "unit") {
		t.Errorf("the refusal should name the kind that was passed, got %q", refusal.Problems[0].Message)
	}
	if !strings.Contains(refusal.Problems[0].Message, "valid:") ||
		strings.Contains(refusal.Problems[0].Message, ", unit") {
		t.Errorf("and the set it may use instead, with `unit` no longer in it, got %q",
			refusal.Problems[0].Message)
	}
	if after := lines(t, r, r.log); len(after) != len(before) {
		t.Fatalf("a refused add must write nothing, log went from %d to %d lines", len(before), len(after))
	}

	out, exit := validateOne(t, r)
	if !out.OK || exit != 0 {
		t.Fatalf("the records that declare a version where `unit` is legal must still pass: %+v",
			out.Reports[0].Problems)
	}
	if len(out.Reports[0].Unjudged) != 0 {
		t.Fatalf("both versions ship in this binary; neither record goes unjudged: %+v",
			out.Reports[0].Unjudged)
	}
	if out.Reports[0].Entries != 3 {
		t.Errorf("every entry is counted, got %d", out.Reports[0].Entries)
	}
}

// Dropping DisallowUnknownFields on the read path did not lose typo
// protection; it moved it to where typos originate. The CLI is what a person
// mistypes, and it refuses before anything reaches the log.
func TestAMistypedFlagIsRefusedBeforeAnythingIsWritten(t *testing.T) {
	r := throwawayRepo(t)
	add(t, r, "RS256 over HS256")
	before := lines(t, r, r.log)

	res := run(t, r, "add", "--kind", "decision", "--titel", "Typo", "--about", "auth")
	if res.code == 0 {
		t.Fatal("a mistyped flag must be refused")
	}
	if !strings.Contains(res.stderr, "titel") {
		t.Errorf("the refusal should name the flag it did not recognise, got %q", res.stderr)
	}
	if after := lines(t, r, r.log); len(after) != len(before) {
		t.Fatalf("a refused add must write nothing, log went from %d to %d lines", len(before), len(after))
	}
}

// -------------------------------------------------------------- the sweep ---

type validateOutput struct {
	OK         bool     `json:"ok"`
	Unjudged   int      `json:"unjudged"`
	InitFailed []string `json:"init_failed"`
	Reports    []struct {
		Path     string `json:"path"`
		Entries  int    `json:"entries"`
		Edges    int    `json:"edges"`
		Problems []struct {
			Line    int    `json:"line"`
			Field   string `json:"field"`
			Message string `json:"message"`
		} `json:"problems"`
		Unjudged []struct {
			Line    int    `json:"line"`
			Version int    `json:"version"`
			ID      string `json:"id"`
			Message string `json:"message"`
		} `json:"unjudged"`
		OK bool `json:"ok"`
	} `json:"reports"`
}

func validate(t *testing.T, r repo, args ...string) (validateOutput, int) {
	t.Helper()
	res := run(t, r, append([]string{"validate"}, args...)...)
	var out validateOutput
	if err := json.Unmarshal([]byte(res.stdout), &out); err != nil {
		t.Fatalf("validate printed something that is not JSON: %v\n%s%s", err, res.stdout, res.stderr)
	}
	return out, res.code
}

// validateOne is the single-log case, which is every test that is not about
// --all.
func validateOne(t *testing.T, r repo) (validateOutput, int) {
	t.Helper()
	out, code := validate(t, r)
	if len(out.Reports) != 1 {
		t.Fatalf("expected one report, got %d", len(out.Reports))
	}
	return out, code
}

// The whole mechanism, end to end: a record this binary knows how to judge, a
// record from some later schema, and a line that is simply broken. Exactly one
// of the three is a problem.
func TestValidateJudgesV1DeclinesTheUnknownAndComplainsOnlyAboutTheBroken(t *testing.T) {
	r := throwawayRepo(t)
	add(t, r, "RS256 over HS256")

	// Written by some later binary: unknown version, and every field wrong by
	// today's rules. If the sweep ever fell back to the current schema this
	// line would produce a fistful of problems.
	appendLine(t, r, r.log, `{"version":99,"id":"01KZ34725V37YX845RTVPC7N0E","kind":"unheard-of",`+
		`"slug":"Not A Slug","scope":"galaxy","at":"whenever","about":[],"title":""}`)
	appendLine(t, r, r.log, `{"kind":"decision",`)

	out, code := validateOne(t, r)
	rep := out.Reports[0]

	if len(rep.Problems) != 1 {
		t.Fatalf("expected one problem, the broken line, got %+v", rep.Problems)
	}
	if rep.Problems[0].Line != 3 {
		t.Errorf("the problem should point at line 3, got %+v", rep.Problems[0])
	}
	if len(rep.Unjudged) != 1 || rep.Unjudged[0].Line != 2 || rep.Unjudged[0].Version != 99 {
		t.Fatalf("expected line 2 listed as unjudged at version 99, got %+v", rep.Unjudged)
	}
	if rep.Unjudged[0].ID != "01KZ34725V37YX845RTVPC7N0E" {
		t.Errorf("an unjudged record must be findable by id, got %+v", rep.Unjudged[0])
	}
	if rep.Entries != 2 {
		t.Errorf("both entries are counted whether or not they could be judged, got %d", rep.Entries)
	}
	if out.Unjudged != 1 {
		t.Errorf("the unjudged count belongs next to ok, got %d", out.Unjudged)
	}
	if code != 1 || out.OK {
		t.Errorf("the broken line is a failure: ok=%v exit=%d", out.OK, code)
	}
}

// An unrecognised version on its own is not a failure. An older binary meeting
// a record written by a newer one is a normal state of an evolving schema, and
// "not checked" is reported where nobody can read it as "checked and fine".
func TestAnUnknownVersionAloneDoesNotFailTheSweep(t *testing.T) {
	r := throwawayRepo(t)
	add(t, r, "RS256 over HS256")
	appendLine(t, r, r.log, `{"version":99,"id":"01KZ34725V37YX845RTVPC7N0E","kind":"unheard-of",`+
		`"slug":"Not A Slug","scope":"galaxy","at":"whenever","about":[],"title":""}`)

	out, code := validateOne(t, r)
	if !out.OK || code != 0 {
		t.Fatalf("an unrecognised version must not fail the sweep: ok=%v exit=%d problems=%+v",
			out.OK, code, out.Reports[0].Problems)
	}
	if len(out.Reports[0].Unjudged) != 1 {
		t.Fatalf("it must still be reported, and apart from problems: %+v", out.Reports[0])
	}
	if !strings.Contains(out.Reports[0].Unjudged[0].Message, "not checked") {
		t.Errorf("the message must say the fields went unchecked, got %q", out.Reports[0].Unjudged[0].Message)
	}
	if out.Unjudged != 1 {
		t.Errorf("expected the count beside ok, got %d", out.Unjudged)
	}
}

// The records that predate the stamp are the reason this exists. They are
// judged as version 1, they pass, and reading them does not add a stamp — the
// bytes on disk are the same afterwards as before.
//
// The two hand-written lines are the real thing: `scope: plan`, a value the
// current version refuses and version 1 still allows, and `origin.plan`, which
// is the only handle a bounded query has on them now.
func TestUnstampedRecordsAreJudgedAsV1AndLeftAlone(t *testing.T) {
	r := throwawayRepo(t)
	current := add(t, r, "RS256 over HS256")

	// No version key, plus a key this binary has never heard of: exactly what
	// a log written before and after this change looks like from here.
	appendLine(t, r, r.log, `{"id":"01KZ2WHVK0GPM4XWVFFKYXGMHQ","kind":"decision","slug":"unstamped",`+
		`"scope":"plan","at":"2026-08-01T10:00:00Z","about":["auth"],`+
		`"title":"Written before the stamp","confidence":"high","origin":{"plan":"demo"}}`)
	appendLine(t, r, r.log, `{"kind":"edge","type":"relates_to","from":"01KZ2WHVK0GPM4XWVFFKYXGMHQ",`+
		`"to":"01KZ34725V37YX845RTVPC7N0E","at":"2026-08-01T10:00:00Z"}`)
	appendLine(t, r, r.log, `{"version":1,"id":"01KZ34725V37YX845RTVPC7N0E","kind":"adr","slug":"stamped",`+
		`"scope":"plan","at":"2026-08-01T10:05:00Z","about":["auth"],"title":"Written after it",`+
		`"origin":{"plan":"demo"}}`)

	before := logText(t, r, r.log)

	out, code := validateOne(t, r)
	if !out.OK || code != 0 {
		t.Fatalf("unstamped records are version 1 and valid: %+v", out.Reports[0].Problems)
	}
	if len(out.Reports[0].Unjudged) != 0 {
		t.Fatalf("an absent version is version 1, not an unknown one: %+v", out.Reports[0].Unjudged)
	}
	// Printed, the unstamped record says the version it is judged at. Version
	// 0 is on no line and names no schema that ever existed; a reader who met
	// it would go looking for rules that do not exist.
	res := run(t, r, "query", "--about", "auth")
	if res.code != 0 {
		t.Fatalf("query: exit %d\n%s%s", res.code, res.stdout, res.stderr)
	}
	var q struct {
		Records []struct {
			ID      string `json:"id"`
			Version int    `json:"version"`
		} `json:"records"`
	}
	if err := json.Unmarshal([]byte(res.stdout), &q); err != nil {
		t.Fatalf("query printed something that is not JSON: %v\n%s", err, res.stdout)
	}
	if len(q.Records) != 3 {
		t.Fatalf("expected three records back, got %d", len(q.Records))
	}
	for _, rec := range q.Records {
		want := 1
		if rec.ID == current {
			// The one this binary wrote. Everything else on this log was written
			// against version 1 and still says so.
			want = memory.CurrentSchemaVersion
		}
		if rec.Version != want {
			t.Errorf("record %s printed version %d, want %d", rec.ID, rec.Version, want)
		}
	}

	after := logText(t, r, r.log)
	if after != before {
		t.Fatal("reading the log rewrote it; nothing already written may be touched")
	}
	// The bytes are the proof: printing a resolved 1 must not put one on disk,
	// and neither must a version bump reach back and restamp what is already
	// there.
	if !strings.Contains(after, `{"id":"01KZ2WHVK0GPM4XWVFFKYXGMHQ","kind":"decision"`) {
		t.Fatalf("the unstamped line no longer looks unstamped:\n%s", after)
	}
	if !strings.Contains(string(after), `"version":1,"id":"01KZ34725V37YX845RTVPC7N0E"`) {
		t.Fatalf("the v1 line no longer declares version 1:\n%s", after)
	}
	if strings.Count(string(after), `"scope":"plan"`) != 2 {
		t.Fatalf("both `scope: plan` records must survive the read untouched:\n%s", after)
	}
}

// --------------------------------------------------------- one log per repo ---

// There is one log, at one path, and the scope that used to put a log under
// plans/ is not a scope any more. The refusal has to say that, not "unknown":
// `plan` was legal until version 2 and the habits that pass it are real, so an
// error reading like a typo would send someone hunting for a misspelling.
func TestInitCreatesTheRepoLogAndRefusesThePlanScope(t *testing.T) {
	r := throwawayRepo(t)

	res := run(t, r, "init")
	if res.code != 0 {
		t.Fatalf("init: exit %d\n%s%s", res.code, res.stdout, res.stderr)
	}
	var out struct {
		Path    string `json:"path"`
		Scope   string `json:"scope"`
		Created bool   `json:"created"`
	}
	if err := json.Unmarshal([]byte(res.stdout), &out); err != nil {
		t.Fatal(err)
	}
	if out.Path != r.log {
		t.Errorf("init should create %s, got %s", r.log, out.Path)
	}
	if out.Scope != "repo" || !out.Created {
		t.Errorf("expected a freshly created repo-scope log, got %+v", out)
	}
	if _, err := os.Stat(r.store); err != nil {
		t.Fatalf("the store the log lives in is not on disk: %v", err)
	}
	if _, err := os.Stat(r.log); !os.IsNotExist(err) {
		t.Fatalf("the log is rows in the store, so no file may be left at its path: %v", err)
	}

	res = run(t, r, "init", "--scope", "plan")
	if res.code == 0 {
		t.Fatal("--scope plan must be refused")
	}
	code, msg := errorCode(t, res)
	if code != "bad_scope" {
		t.Errorf("expected a bad_scope refusal, got %q", code)
	}
	if !strings.Contains(msg, "removed") {
		t.Errorf("the refusal must say the scope was removed, got %q", msg)
	}
	if _, err := os.Stat(filepath.Join(r.root, "plans")); !os.IsNotExist(err) {
		t.Errorf("a refused init must not create plans/: %v", err)
	}
}

// `kinds` is where a caller learns what the write gate will accept, so a version
// that narrows a closed set has to narrow this list with it. Otherwise the tool
// goes on advertising a value it refuses.
func TestKindsAdvertisesTwoScopesAndVersion3sKinds(t *testing.T) {
	// `kinds` reads no log, so it is run bare: with no --store to point it
	// anywhere, it still has nothing to open.
	cmd := exec.Command(memoryBin, "kinds")
	stdout, err := cmd.Output()
	if err != nil {
		t.Fatalf("kinds: %v\n%s", err, stdout)
	}
	var out struct {
		EntryKinds     []string            `json:"entry_kinds"`
		EdgeTypes      []string            `json:"edge_types"`
		Scopes         []string            `json:"scopes"`
		Threshold      int                 `json:"threshold"`
		Justifications map[string][]string `json:"justifications"`
		AnchoredKinds  []string            `json:"anchored_kinds"`
		Verdicts       []string            `json:"verdicts"`
	}
	if err := json.Unmarshal(stdout, &out); err != nil {
		t.Fatal(err)
	}
	if strings.Join(out.Scopes, ",") != "repo,user" {
		t.Errorf(`expected scopes ["repo","user"], got %v`, out.Scopes)
	}
	if out.Threshold != memory.DefaultThreshold {
		t.Errorf("the threshold key is part of the shape other things read, got %d", out.Threshold)
	}
	// Version 3 took `unit` away, and this is the list a caller reads to find out
	// what the write gate accepts — so the narrowing has to show up here, or the
	// tool advertises a kind it refuses. The ten that are left, plus the kinds
	// widened into version 3 in place afterwards — `hypothesis`, `gotcha`, then
	// `preference`: a widening owes this list the same update a narrowing does, in
	// the other direction, or the tool hides a value it accepts.
	wantKinds := []string{"objective", "decision", "deviation", "assumption",
		"open-question", "wont-fix", "dead-end", "adr", "postmortem", "review-finding",
		"hypothesis", "gotcha", "preference"}
	for _, want := range wantKinds {
		if !slices.Contains(out.EntryKinds, want) {
			t.Errorf("kind %q went missing from the advertised set: %v", want, out.EntryKinds)
		}
	}
	if len(out.EntryKinds) != len(wantKinds) {
		t.Errorf("expected %d entry kinds, got %d: %v", len(wantKinds), len(out.EntryKinds), out.EntryKinds)
	}
	if slices.Contains(out.EntryKinds, "unit") {
		t.Errorf("`unit` is gone in version 3 and must not be advertised: %v", out.EntryKinds)
	}
	// The other direction of the same obligation: a version that *widens* a
	// closed set has to widen this list with it, or the tool goes on hiding a
	// value it accepts. Eleven types — version 1's six and the five states.
	want := []string{
		"implements", "derives_from", "supersedes", "answers", "caused_by", "relates_to",
		"approved_for_execution", "met", "cancelled", "separated", "blocked",
	}
	if len(out.EdgeTypes) != len(want) {
		t.Errorf("expected %d edge types, got %d: %v", len(want), len(out.EdgeTypes), out.EdgeTypes)
	}
	for _, w := range want {
		if !slices.Contains(out.EdgeTypes, w) {
			t.Errorf("edge type %q is not advertised: %v", w, out.EdgeTypes)
		}
	}

	// A rule the tool enforces but does not publish is a rule a caller can only
	// meet as a refusal, so the allowlist is advertised the same way the closed
	// sets are — and only for the types that carry one, since an absent key says
	// "no rule" where an empty list would say "nothing may justify this".
	wantJustifications := map[string][]string{
		"approved_for_execution": {"decision"},
		"met":                    {"decision", "review-finding"},
		"cancelled":              {"decision"},
		"separated":              {"decision"},
		"blocked":                {"decision", "open-question"},
	}
	if len(out.Justifications) != len(wantJustifications) {
		t.Errorf("expected the five states to be published, got %v", out.Justifications)
	}
	for state, kinds := range wantJustifications {
		got, published := out.Justifications[state]
		if !published {
			t.Errorf("state %q has a rule that is not advertised: %v", state, out.Justifications)
			continue
		}
		if strings.Join(got, ",") != strings.Join(kinds, ",") {
			t.Errorf("%s may be justified by %v, advertised as %v", state, kinds, got)
		}
	}
	for _, relation := range []string{"implements", "derives_from", "supersedes",
		"answers", "caused_by", "relates_to"} {
		if _, published := out.Justifications[relation]; published {
			t.Errorf("%q carries no rule and must not be listed as though it did", relation)
		}
	}

	// Same obligation, second rule: which kinds are refused without an anchor is
	// enforced by the write gate, so a caller who cannot read it here only ever
	// learns it by being refused.
	if strings.Join(out.AnchoredKinds, ",") != "gotcha" {
		t.Errorf("`gotcha` is the one kind that owes an anchor, advertised as %v", out.AnchoredKinds)
	}
	// And the vocabulary `anchors` answers in, which agents/explore.md fixes at
	// exactly these three. A fourth would be two vocabularies for one idea.
	if strings.Join(out.Verdicts, ",") != "still_holds,changed,could_not_check" {
		t.Errorf("the three verdicts are published as they are documented, got %v", out.Verdicts)
	}
}

// What `kinds` publishes has to be what `link` enforces. Read out of the CLI on
// one side and exercised against a real log on the other, because the failure
// this catches is a published list that quietly drifts from the rule — which
// looks like documentation and behaves like a lie.
func TestTheAdvertisedAllowlistIsTheOneLinkEnforces(t *testing.T) {
	cmd := exec.Command(memoryBin, "kinds")
	stdout, err := cmd.Output()
	if err != nil {
		t.Fatalf("kinds: %v\n%s", err, stdout)
	}
	var out struct {
		Justifications map[string][]string `json:"justifications"`
	}
	if err := json.Unmarshal(stdout, &out); err != nil {
		t.Fatal(err)
	}
	if len(out.Justifications) == 0 {
		t.Fatal("kinds published no justifications at all")
	}

	r := throwawayRepo(t)
	source := addKind(t, r, "objective", "The objective whose state this is")
	targets := map[string]string{}
	for _, k := range memory.EntryKinds() {
		targets[string(k)] = addKind(t, r, string(k), "A "+string(k))
	}

	for state, kinds := range out.Justifications {
		advertised := map[string]bool{}
		for _, k := range kinds {
			advertised[k] = true
		}
		for kind, id := range targets {
			res := runBare(t, r, "link", "--type", state, "--from", source, "--to", id)
			if advertised[kind] != (res.code == 0) {
				t.Errorf("kinds says %s may be justified by %v; link exited %d for a %s target",
					state, kinds, res.code, kind)
			}
		}
	}
}

// -------------------------------------------------------- the state edges ---

// addKind is `add` with the kind spelled out. The justification rule is a
// question about the *target's* kind, so a test of it needs a log holding one
// entry of each.
//
// Every one of them gets an anchor, not only the kinds that are refused without
// one. Any record may carry anchors, and a helper that branched on which kinds
// require them would be a second copy of the rule its own tests are meant to
// catch drifting.
func addKind(t *testing.T, r repo, kind, title string) string {
	t.Helper()
	res := runBare(t, r, "add", "--plan", "demo", "--kind", kind, "--title", title,
		"--about", "auth", "--anchor", anchorSpec(t, r))
	if res.code != 0 {
		t.Fatalf("add --kind %s: exit %d\n%s%s", kind, res.code, res.stdout, res.stderr)
	}
	return recordID(t, res)
}

// The whole rule, exercised against a real log holding one entry of every kind:
// each state edge is written when it points at a record that could justify it
// and refused when it does not, the refusal says both what was allowed and what
// was actually there, and nothing refused reaches disk.
//
// The case worth naming is `met` from one objective to another. It is the shape
// the rule exists to stop — a state asserted with nothing behind it, which on
// disk is indistinguishable from one that was earned.
func TestAStateEdgeIsWrittenOnlyWhenSomethingJustifiesIt(t *testing.T) {
	r := throwawayRepo(t)

	source := addKind(t, r, "objective", "The objective whose state this is")
	targets := map[string]string{}
	for _, k := range memory.EntryKinds() {
		targets[string(k)] = addKind(t, r, string(k), "A "+string(k))
	}

	allowed := map[string][]string{
		"approved_for_execution": {"decision"},
		"met":                    {"decision", "review-finding"},
		"cancelled":              {"decision"},
		"separated":              {"decision"},
		"blocked":                {"decision", "open-question"},
	}

	for typ, kinds := range allowed {
		ok := map[string]bool{}
		for _, k := range kinds {
			ok[k] = true
		}
		for kind, id := range targets {
			before := len(lines(t, r, r.log))
			res := runBare(t, r, "link", "--type", typ, "--from", source, "--to", id)

			if ok[kind] {
				if res.code != 0 {
					t.Errorf("%s → %s must be accepted: exit %d\n%s%s", typ, kind, res.code, res.stdout, res.stderr)
				}
				if after := len(lines(t, r, r.log)); after != before+1 {
					t.Errorf("%s → %s should have appended one line, went from %d to %d", typ, kind, before, after)
				}
				continue
			}

			if res.code == 0 {
				t.Errorf("%s → %s must be refused", typ, kind)
			}
			code, msg := errorCode(t, res)
			if code != "unjustified_state" {
				t.Errorf("%s → %s: expected an unjustified_state refusal, got %q", typ, kind, code)
			}
			if !strings.Contains(msg, kind) {
				t.Errorf("%s → %s: the refusal must name the target's kind, got %q", typ, kind, msg)
			}
			for _, k := range kinds {
				if !strings.Contains(msg, k) {
					t.Errorf("%s → %s: the refusal must name %q as allowed, got %q", typ, kind, k, msg)
				}
			}
			if after := len(lines(t, r, r.log)); after != before {
				t.Errorf("a refused link wrote to the log: %d lines became %d", before, after)
			}
		}
	}

	// The six types that were here before carry no rule at all, and adding one
	// to them was explicitly out of scope: `supersedes` alone already has edges
	// on disk that a rule invented afterwards has no business condemning.
	for kind, id := range targets {
		if res := runBare(t, r, "link", "--type", "relates_to", "--from", source, "--to", id); res.code != 0 {
			t.Errorf("relates_to → %s must be unaffected: exit %d\n%s%s", kind, res.code, res.stdout, res.stderr)
		}
	}
}

// A state edge is a record like any other: it lands on disk stamped with the
// version whose rule set accepted it, and it is reachable from both of its
// endpoints by a graph walk — which is the only way anything reads a state, this
// unit having added no command that reports one.
func TestAWrittenStateEdgeIsStampedAndReachable(t *testing.T) {
	r := throwawayRepo(t)
	objective := addKind(t, r, "objective", "The objective whose state this is")
	decision := addKind(t, r, "decision", "The decision that closed it")

	res := runBare(t, r, "link", "--type", "met", "--from", objective, "--to", decision)
	if res.code != 0 {
		t.Fatalf("link --type met: exit %d\n%s%s", res.code, res.stdout, res.stderr)
	}

	got := lines(t, r, r.log)
	if len(got) != 3 {
		t.Fatalf("expected two entries and one edge, got %d lines", len(got))
	}
	var edge struct {
		Version int    `json:"version"`
		Kind    string `json:"kind"`
		Type    string `json:"type"`
		From    string `json:"from"`
		To      string `json:"to"`
	}
	if err := json.Unmarshal([]byte(got[2]), &edge); err != nil {
		t.Fatalf("the appended line is not JSON: %v\n%s", err, got[2])
	}
	if edge.Version != memory.CurrentSchemaVersion {
		t.Errorf("a state edge is stamped %d, got %d: %s", memory.CurrentSchemaVersion, edge.Version, got[2])
	}
	if edge.Kind != "edge" || edge.Type != "met" || edge.From != objective || edge.To != decision {
		t.Fatalf("the appended line is not that edge: %s", got[2])
	}

	res = runBare(t, r, "graph", "--from", objective, "--depth", "1")
	if res.code != 0 {
		t.Fatalf("graph: exit %d\n%s%s", res.code, res.stdout, res.stderr)
	}
	var g struct {
		Nodes []struct {
			Entry struct {
				ID string `json:"id"`
			} `json:"entry"`
			Via   string `json:"via"`
			Depth int    `json:"depth"`
		} `json:"nodes"`
	}
	if err := json.Unmarshal([]byte(res.stdout), &g); err != nil {
		t.Fatal(err)
	}
	reached := false
	for _, n := range g.Nodes {
		if n.Entry.ID == decision && n.Depth == 1 && n.Via == "met" {
			reached = true
		}
	}
	if !reached {
		t.Fatalf("the justifying record must be one hop from the objective: %+v", g.Nodes)
	}
}

// The rule refuses records on the way in; it does not condemn records already
// written. An edge the write path would refuse today is a perfectly readable
// line tomorrow, and the sweep says so — otherwise every rule added later would
// retroactively break a log nobody touched.
func TestValidateToleratesAStateEdgeTheWritePathWouldRefuse(t *testing.T) {
	r := throwawayRepo(t)
	objective := addKind(t, r, "objective", "The objective whose state this is")
	other := addKind(t, r, "objective", "Some other objective")

	if res := runBare(t, r, "link", "--type", "met", "--from", objective, "--to", other); res.code == 0 {
		t.Fatal("the premise of this test is that link refuses this edge")
	}

	// Hand-written, because the write path is exactly what will not produce it:
	// this is what an edge from a binary that predates the rule looks like.
	appendLine(t, r, r.log, `{"version":2,"kind":"edge","type":"met","from":"`+objective+`",`+
		`"to":"`+other+`","at":"2026-08-03T10:00:00Z"}`)

	out, code := validateOne(t, r)
	if !out.OK || code != 0 {
		t.Fatalf("the sweep must stay tolerant: ok=%v exit=%d %+v", out.OK, code, out.Reports[0].Problems)
	}
	if len(out.Reports[0].Problems) != 0 {
		t.Fatalf("expected no problems, got %+v", out.Reports[0].Problems)
	}
	if len(out.Reports[0].Unjudged) != 0 {
		t.Fatalf("version 2 is this binary's own version, nothing should go unjudged: %+v",
			out.Reports[0].Unjudged)
	}
	if out.Reports[0].Edges != 1 {
		t.Errorf("the edge should still be counted, got %d", out.Reports[0].Edges)
	}
}

// `add` no longer refuses a record that names no plan — such a record belongs to
// the repository rather than to a tree in it — but every other thing it refused
// about a plan it still refuses, and the environment is still the second way to
// name one.
func TestAddWritesWithoutAPlanAndStillTakesItFromTheEnvironment(t *testing.T) {
	r := throwawayRepo(t)

	res := runBare(t, r, "add", "--kind", "preference", "--title", "Migrations go through Atlas",
		"--about", "persistence")
	if res.code != 0 {
		t.Fatalf("add with no plan must be written: exit %d\n%s%s", res.code, res.stdout, res.stderr)
	}
	// Absent, not empty: `origin.plan` is omitempty, so a record belonging to the
	// repository carries no plan key at all rather than one holding "".
	if line := lines(t, r, r.log)[0]; strings.Contains(line, `"plan"`) {
		t.Errorf("a plan-less record must carry no plan key: %s", line)
	}

	// AI_KIT_PLAN is the other way to say it, and it has to be enough on its own.
	cmd := exec.Command(memoryBin, "add", "--store", r.store, "--kind", "decision", "--title", "From the environment",
		"--about", "auth", "--dir", r.root)
	cmd.Env = append(os.Environ(), "HOME="+r.home, "AI_KIT_AUTHOR=test-suite", "AI_KIT_PLAN=from-env")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("AI_KIT_PLAN should be enough: %v\n%s", err, out)
	}
	if got := lines(t, r, r.log)[1]; !strings.Contains(got, `"plan":"from-env"`) {
		t.Errorf("the record should carry the plan from the environment: %s", got)
	}

	// And a slug that is not a slug is refused rather than stamped, since
	// origin.plan is what every bounded query matches on.
	res = runBare(t, r, "add", "--plan", "Not A Slug", "--kind", "decision",
		"--title", "Shouty", "--about", "auth")
	if res.code == 0 {
		t.Fatal("a non-slug plan must be refused")
	}
	if code, _ := errorCode(t, res); code != "bad_plan" {
		t.Errorf("expected a bad_plan refusal, got %q", code)
	}
}

// A preference belongs to the repository, not to any objective tree inside it,
// and this is the whole of what that means through the binary: it is written with
// no `--plan` and no `AI_KIT_PLAN`, the log it lands in still sweeps clean, and a
// read bounded to any tree in the same log returns it.
//
// The second half is what stops the widening from being a filter that stopped
// filtering: the record that *does* name a plan is still absent from a query
// naming a different one.
func TestAPreferenceIsWrittenWithoutAPlanAndEveryPlanBoundedReadSeesIt(t *testing.T) {
	r := throwawayRepo(t)

	res := runBare(t, r, "add", "--kind", "preference", "--title", "Migrations go through Atlas",
		"--about", "persistence,migrations", "--body", "Hand-written scripts drifted from the schema twice.")
	if res.code != 0 {
		t.Fatalf("a preference must be writable with no plan: exit %d\n%s%s", res.code, res.stdout, res.stderr)
	}
	preference := recordID(t, res)
	addTo(t, r, "tree-a", "Cancel is async")

	if out, code := validateOne(t, r); !out.OK || code != 0 {
		t.Fatalf("a log holding a plan-less record must still sweep clean: ok=%v exit=%d %+v",
			out.OK, code, out.Reports[0].Problems)
	}

	// Every direction the bound can be asked in: no plan at all, the slug that is
	// in the log, and one that is not. A record the repository owns is in scope for
	// all three.
	for _, args := range [][]string{
		{"query", "--kind", "preference"},
		{"query", "--plan", "tree-a", "--kind", "preference"},
		{"query", "--plan", "tree-b", "--kind", "preference"},
	} {
		out := readQuery(t, r, args...)
		if out.Count != 1 || out.Records[0].ID != preference {
			t.Errorf("%v must return the repository's own record, got %v", args, ids(out))
		}
	}

	if out := readQuery(t, r, "query", "--plan", "tree-b", "--kind", "decision"); out.Count != 0 {
		t.Errorf("a record that names a plan stays out of another plan's query, got %v", ids(out))
	}
	if out := readQuery(t, r, "query", "--plan", "tree-a"); out.Count != 2 {
		t.Errorf("a bounded read returns its own tree's records and the repository's, got %d", out.Count)
	}
}

// The kind is not anchored, and the way to say that through the binary is beside
// the kind that is: the same `add`, with no `--anchor`, written for one and
// refused for the other.
func TestAPreferenceIsWrittenWithoutAnAnchorWhereAGotchaIsRefused(t *testing.T) {
	r := throwawayRepo(t)

	if res := runBare(t, r, "add", "--kind", "preference", "--title", "Migrations go through Atlas",
		"--about", "persistence"); res.code != 0 {
		t.Fatalf("a preference owes no anchor: exit %d\n%s%s", res.code, res.stdout, res.stderr)
	}

	res := runBare(t, r, "add", "--kind", "gotcha", "--title", "The lock is taken before the write",
		"--about", "persistence")
	if res.code == 0 {
		t.Fatal("`gotcha` is still the one kind refused without an anchor")
	}
	if code, _ := errorCode(t, res); code != "invalid_record" {
		t.Errorf("expected invalid_record, got %q", code)
	}
	if got := lines(t, r, r.log); len(got) != 1 {
		t.Fatalf("the refused gotcha must have written nothing, log has %d lines", len(got))
	}
}

// Removing one refusal must not remove the others. Everything `add` refused about
// a record before the plan became optional, it refuses now, with the code it
// returned then and with nothing reaching the log.
func TestAddWithNoPlanStillRefusesEverythingElseItRefused(t *testing.T) {
	r := throwawayRepo(t)

	for name, args := range map[string][]string{
		"a kind outside the closed set": {"add", "--kind", "invented", "--title", "Whatever", "--about", "auth"},
		"no about at all":               {"add", "--kind", "preference", "--title", "Whatever"},
		"a blank title":                 {"add", "--kind", "preference", "--title", "   ", "--about", "auth"},
	} {
		res := runBare(t, r, args...)
		if res.code == 0 {
			t.Errorf("%s must still be refused", name)
			continue
		}
		if code, _ := errorCode(t, res); code != "invalid_record" {
			t.Errorf("%s: expected invalid_record, got %q", name, code)
		}
	}
	if got := logText(t, r, r.log); got != "" {
		t.Errorf("a refused add must write nothing, got %q", got)
	}
}

// The newest preference under a tag is the live answer, read off the result order
// with no edge drawn — which is what makes the convention usable by a writer who
// does not know which record they are replacing. `supersedes` still works for the
// writer who does, and `--current` is where the difference shows.
//
// It also covers the plan-less endpoints: neither of these records carries a
// plan, and the edge between them is written all the same. The draft this
// implements expected `link` to *require* a `--plan` for it — it does not, and
// never did: `cmdLink` reads neither the flag nor AI_KIT_PLAN. So the edge is
// written twice over here, with a plan and without, because the claim worth
// pinning is that neither spelling depends on the endpoints having one.
func TestTheNewestPreferenceUnderATagIsTheLiveAnswer(t *testing.T) {
	r := throwawayRepo(t)

	addPreference := func(title string) string {
		t.Helper()
		res := runBare(t, r, "add", "--kind", "preference", "--title", title, "--about", "persistence,orm")
		if res.code != 0 {
			t.Fatalf("add %q: exit %d\n%s%s", title, res.code, res.stdout, res.stderr)
		}
		return recordID(t, res)
	}
	older := addPreference("Persistence is GORM")
	newer := addPreference("Persistence is sqlc")

	out := readQuery(t, r, "query", "--kind", "preference", "--about", "persistence", "--current")
	if out.Count != 2 {
		t.Fatalf("both preferences are current until something says otherwise, got %d", out.Count)
	}
	if out.Records[0].ID != newer || out.Records[1].ID != older {
		t.Fatalf("the live answer is the first result, most recent first: %v", ids(out))
	}

	// With a --plan, although neither endpoint carries one.
	if res := runBare(t, r, "link", "--plan", "some-tree", "--type", "supersedes",
		"--from", newer, "--to", older); res.code != 0 {
		t.Fatalf("link between two plan-less records: exit %d\n%s%s", res.code, res.stdout, res.stderr)
	}

	out = readQuery(t, r, "query", "--kind", "preference", "--about", "persistence", "--current")
	if out.Count != 1 || out.Records[0].ID != newer {
		t.Fatalf("the superseded preference must drop out of --current, got %v", ids(out))
	}

	// And without one, which is what `link` has always accepted.
	third := addPreference("Persistence is Ent")
	if res := runBare(t, r, "link", "--type", "supersedes", "--from", third, "--to", newer); res.code != 0 {
		t.Fatalf("link needs no plan and never did: exit %d\n%s%s", res.code, res.stdout, res.stderr)
	}
	out = readQuery(t, r, "query", "--kind", "preference", "--about", "persistence", "--current")
	if out.Count != 1 || out.Records[0].ID != third {
		t.Fatalf("the newest unsuperseded preference is the live answer, got %v", ids(out))
	}

	if out, code := validateOne(t, r); !out.OK || code != 0 {
		t.Fatalf("edges between plan-less records must sweep clean: ok=%v exit=%d %+v",
			out.OK, code, out.Reports[0].Problems)
	}
}

// The bound that replaced one file per plan. One log holds several plans'
// records, and `--plan` is what keeps a query from returning all of them —
// including the part that matters most, that it still withholds over the
// threshold rather than handing back a fragment.
func TestPlanBoundsTheQueryWithinOneLog(t *testing.T) {
	r := throwawayRepo(t)

	for i := 0; i < 3; i++ {
		addTo(t, r, "quiet", fmt.Sprintf("Quiet decision %d", i))
	}
	for i := 0; i < 30; i++ {
		addTo(t, r, "busy", fmt.Sprintf("Busy decision %d", i))
	}

	var q struct {
		Plan      string `json:"plan"`
		Count     int    `json:"count"`
		Returned  int    `json:"returned"`
		Truncated bool   `json:"truncated"`
		Hint      string `json:"hint"`
		Records   []struct {
			Title  string `json:"title"`
			Origin struct {
				Plan string `json:"plan"`
			} `json:"origin"`
		} `json:"records"`
	}
	read := func(args ...string) {
		t.Helper()
		res := runBare(t, r, args...)
		if res.code != 0 {
			t.Fatalf("%v: exit %d\n%s%s", args, res.code, res.stdout, res.stderr)
		}
		q = struct {
			Plan      string `json:"plan"`
			Count     int    `json:"count"`
			Returned  int    `json:"returned"`
			Truncated bool   `json:"truncated"`
			Hint      string `json:"hint"`
			Records   []struct {
				Title  string `json:"title"`
				Origin struct {
					Plan string `json:"plan"`
				} `json:"origin"`
			} `json:"records"`
		}{}
		if err := json.Unmarshal([]byte(res.stdout), &q); err != nil {
			t.Fatalf("%v printed something that is not JSON: %v\n%s", args, err, res.stdout)
		}
	}

	read("query", "--plan", "quiet", "--about", "auth")
	if q.Plan != "quiet" {
		t.Errorf("the answer should say what it was bounded to, got %q", q.Plan)
	}
	if q.Count != 3 || q.Returned != 3 {
		t.Fatalf("the count must cover only the named plan, got count=%d returned=%d", q.Count, q.Returned)
	}
	for _, rec := range q.Records {
		if rec.Origin.Plan != "quiet" {
			t.Errorf("record %q came from plan %q", rec.Title, rec.Origin.Plan)
		}
	}

	// Over the threshold within one plan: the count is the true count for that
	// plan, and no records come back.
	read("query", "--plan", "busy", "--about", "auth")
	if q.Count != 30 {
		t.Fatalf("expected 30 matches in the busy plan, got %d", q.Count)
	}
	if q.Returned != 0 || !q.Truncated || q.Hint == "" {
		t.Fatalf("over the threshold the records are withheld: returned=%d truncated=%v hint=%q",
			q.Returned, q.Truncated, q.Hint)
	}

	// Unbounded, the same log answers for everything in it.
	read("query", "--about", "auth", "--threshold", "-1")
	if q.Count != 33 {
		t.Fatalf("with no plan named the whole log answers, got %d", q.Count)
	}

	// And `search` bounds the same way.
	read("search", "--plan", "quiet", "decision")
	if q.Count != 3 {
		t.Fatalf("search should bound by plan too, got %d", q.Count)
	}
}

// The property the rest of this objective stands on. Two records born under
// different plans are two records in one file, so an edge between them is an
// ordinary edge — where before, `link` could not even see the other endpoint.
func TestRecordsFromDifferentPlansCanBeLinked(t *testing.T) {
	r := throwawayRepo(t)
	a := addTo(t, r, "parent-plan", "The objective above")
	b := addTo(t, r, "child-plan", "The objective below")

	res := runBare(t, r, "link", "--type", "relates_to", "--from", b, "--to", a)
	if res.code != 0 {
		t.Fatalf("linking across plans must work: exit %d\n%s%s", res.code, res.stdout, res.stderr)
	}
	got := lines(t, r, r.log)
	if len(got) != 3 {
		t.Fatalf("expected the edge to be appended, got %d lines", len(got))
	}
	if !strings.Contains(got[2], `"from":"`+b+`"`) || !strings.Contains(got[2], `"to":"`+a+`"`) {
		t.Fatalf("the appended line is not that edge: %s", got[2])
	}

	res = runBare(t, r, "graph", "--from", b, "--depth", "1")
	if res.code != 0 {
		t.Fatalf("graph: exit %d\n%s%s", res.code, res.stdout, res.stderr)
	}
	var g struct {
		Nodes []struct {
			Entry struct {
				ID string `json:"id"`
			} `json:"entry"`
			Depth int `json:"depth"`
		} `json:"nodes"`
	}
	if err := json.Unmarshal([]byte(res.stdout), &g); err != nil {
		t.Fatal(err)
	}
	reached := false
	for _, n := range g.Nodes {
		if n.Entry.ID == a && n.Depth == 1 {
			reached = true
		}
	}
	if !reached {
		t.Fatalf("the other plan's record must be reachable from this one: %+v", g.Nodes)
	}
}

// promote crosses logs, and since the collapse there is only one crossing left.
// repo → repo is not a promotion, it is where a plan's records already are.
func TestPromoteToRepoIsGoneAndToUserStillCarriesTheGraph(t *testing.T) {
	r := throwawayRepo(t)
	decision := add(t, r, "RS256 over HS256")
	adr := add(t, r, "Asymmetric signing")
	if res := run(t, r, "link", "--type", "derives_from", "--from", adr, "--to", decision); res.code != 0 {
		t.Fatalf("link: exit %d\n%s%s", res.code, res.stdout, res.stderr)
	}

	res := run(t, r, "promote", "--ids", adr, "--to", "repo")
	if res.code == 0 {
		t.Fatal("promote --to repo must be refused")
	}
	code, msg := errorCode(t, res)
	if code != "target_removed" {
		t.Errorf("expected a target_removed refusal, got %q", code)
	}
	if !strings.Contains(msg, "removed") {
		t.Errorf("the refusal must say it was removed, got %q", msg)
	}

	res = run(t, r, "promote", "--ids", adr+","+decision, "--to", "user")
	if res.code != 0 {
		t.Fatalf("promote --to user: exit %d\n%s%s", res.code, res.stdout, res.stderr)
	}
	var out struct {
		To          string   `json:"to"`
		Promoted    []string `json:"promoted"`
		EdgesCopied int      `json:"edges_copied"`
	}
	if err := json.Unmarshal([]byte(res.stdout), &out); err != nil {
		t.Fatal(err)
	}
	if out.To != r.user {
		t.Errorf("expected the target to be the user log, got %s", out.To)
	}
	if len(out.Promoted) != 2 || out.EdgesCopied != 1 {
		t.Fatalf("both records and the edge between them should move: %+v", out)
	}
	promoted := lines(t, r, r.user)
	if len(promoted) != 3 {
		t.Fatalf("expected two entries and one edge at the user level, got %d lines", len(promoted))
	}
	for _, line := range promoted[:2] {
		if !strings.Contains(line, `"scope":"user"`) {
			t.Errorf("a promoted record lands at the target scope: %s", line)
		}
	}
	// Ids unchanged is the whole payoff: nothing that points at a record has to
	// be rewritten when it moves.
	if !strings.Contains(promoted[0], adr) && !strings.Contains(promoted[1], adr) {
		t.Errorf("the promoted record kept a different id:\n%s", strings.Join(promoted, "\n"))
	}
}

// A v1 record promoted to the user log keeps saying version 1, and version 1 is
// what judges it there. Promotion is a copy, not a re-authoring: restamping it
// would be the tool claiming the record satisfies rules nobody checked it
// against.
func TestPromotingAV1RecordKeepsItsVersion(t *testing.T) {
	r := throwawayRepo(t)
	add(t, r, "Something current") // so the log exists
	appendLine(t, r, r.log, `{"version":1,"id":"01KZ34725V37YX845RTVPC7N0E","kind":"adr","slug":"old",`+
		`"scope":"plan","at":"2026-08-01T10:05:00Z","about":["auth"],"title":"Written under version 1",`+
		`"origin":{"plan":"demo"}}`)

	res := run(t, r, "promote", "--ids", "01KZ34725V37YX845RTVPC7N0E", "--to", "user")
	if res.code != 0 {
		t.Fatalf("promote: exit %d\n%s%s", res.code, res.stdout, res.stderr)
	}
	line := lines(t, r, r.user)[0]
	if !strings.HasPrefix(line, `{"version":1,`) {
		t.Errorf("the promoted copy must still declare version 1: %s", line)
	}
	if !strings.Contains(line, `"scope":"user"`) {
		t.Errorf("it should land scoped user, which version 1 accepts: %s", line)
	}

	// And the user log it landed in still validates: v1's rules judge it.
	res = run(t, r, "validate", "--scope", "user")
	if res.code != 0 {
		t.Fatalf("the user log must validate: exit %d\n%s%s", res.code, res.stdout, res.stderr)
	}
}

// --all used to pass by validating nothing: it resolved the plan scope, a repo
// with several plan directories gave it nothing to pick, the loop swallowed the
// failure, and `ok: true` was a report over an empty set. With one log per repo
// there is nothing to guess, and the counts are real.
func TestValidateAllReportsRealCountsForEveryLog(t *testing.T) {
	r := throwawayRepo(t)
	a := add(t, r, "RS256 over HS256")
	b := add(t, r, "Asymmetric signing")
	if res := run(t, r, "link", "--type", "derives_from", "--from", b, "--to", a); res.code != 0 {
		t.Fatalf("link: exit %d\n%s%s", res.code, res.stdout, res.stderr)
	}
	// Two plan directories, which is what used to defeat --all.
	for _, p := range []string{"one", "two"} {
		if err := os.MkdirAll(filepath.Join(r.root, "plans", p), 0o755); err != nil {
			t.Fatal(err)
		}
	}

	out, code := validate(t, r, "--all")
	if !out.OK || code != 0 {
		t.Fatalf("--all should pass here: ok=%v exit=%d %+v", out.OK, code, out.Reports)
	}
	var found bool
	for _, rep := range out.Reports {
		if rep.Path != r.log {
			continue
		}
		found = true
		if rep.Entries != 2 || rep.Edges != 1 {
			t.Errorf("--all must count what is actually in the log, got %d entries and %d edges",
				rep.Entries, rep.Edges)
		}
	}
	if !found {
		t.Fatalf("--all never looked at %s: %+v", r.log, out.Reports)
	}
}

// ------------------------------------------------------------ first contact ---

// git runs one git command in root and returns its stdout.
//
// The real user's git configuration is deliberately out of reach: a global
// core.excludesFile or a commit.gpgsign would change the answers this file reads
// out of `git status`, and the question here is what the sentinel does, not what
// the machine running the suite happens to be configured to ignore.
func git(t *testing.T, root string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = root
	cmd.Env = append(os.Environ(),
		"GIT_CONFIG_GLOBAL="+os.DevNull, "GIT_CONFIG_SYSTEM="+os.DevNull,
		"GIT_AUTHOR_NAME=test-suite", "GIT_AUTHOR_EMAIL=test@example.invalid",
		"GIT_COMMITTER_NAME=test-suite", "GIT_COMMITTER_EMAIL=test@example.invalid")
	var stdout, stderr strings.Builder
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("git %v: %v\n%s%s", args, err, stdout.String(), stderr.String())
	}
	return stdout.String()
}

// gitBackedRepo is throwawayRepo with a real repository in it rather than an
// empty .git directory. Only a claim about git's own behaviour needs it — the
// tool never runs git to resolve a root, so everything else is cheaper without.
func gitBackedRepo(t *testing.T) repo {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skipf("git is not installed: %v", err)
	}
	r := repo{root: t.TempDir(), home: t.TempDir(), store: throwawayStore(t)}
	git(t, r.root, "init", "-q")
	r.log = filepath.Join(r.root, ".ai-kit", "memory.jsonl")
	r.user = filepath.Join(r.home, ".ai-kit", "memory.jsonl")
	return r
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

// A read is first contact as much as a write is. `query` used to answer
// `count: 0` against a path that did not exist and leave nothing behind, which
// meant the directory — and the .gitignore that keeps it out of git — only
// arrived with the first record somebody wrote.
func TestAReadAgainstAFreshRepoCreatesTheDirectoryAndItsSentinel(t *testing.T) {
	r := throwawayRepo(t)
	if _, err := os.Stat(filepath.Dir(r.log)); !os.IsNotExist(err) {
		t.Fatalf("the repo must start with no .ai-kit: %v", err)
	}

	res := run(t, r, "query", "--kind", "hypothesis")
	if res.code != 0 {
		t.Fatalf("query: exit %d\n%s%s", res.code, res.stdout, res.stderr)
	}
	if got := readFile(t, filepath.Join(r.root, ".ai-kit", ".gitignore")); got != "*\n" {
		t.Errorf("the sentinel must be exactly `*`, got %q", got)
	}
	if _, err := os.Stat(r.log); !os.IsNotExist(err) {
		t.Fatalf("the records live in the store, so nothing may be left at %s: %v", r.log, err)
	}
}

// The sentinel is checked against git itself, because "this makes the directory
// invisible" is a claim about git's behaviour and nothing else. It covers what
// the loop writes later — `plans/` under the log directory — and it does its job
// from inside, so the repository's own .gitignore is never opened.
func TestWhatTheLoopCreatesIsInvisibleToGitAndTouchesNoGitignore(t *testing.T) {
	r := gitBackedRepo(t)
	rootIgnore := filepath.Join(r.root, ".gitignore")
	const committed = "node_modules/\ndist/\n"
	if err := os.WriteFile(rootIgnore, []byte(committed), 0o644); err != nil {
		t.Fatal(err)
	}
	git(t, r.root, "add", ".gitignore")
	git(t, r.root, "commit", "-qm", "what this project ignores")
	if out := git(t, r.root, "status", "--porcelain"); out != "" {
		t.Fatalf("the repository should start clean, got %q", out)
	}

	if res := run(t, r, "query"); res.code != 0 {
		t.Fatalf("query: exit %d\n%s%s", res.code, res.stdout, res.stderr)
	}
	draft := filepath.Join(r.root, ".ai-kit", "plans", "x", "draft-objective.md")
	if err := os.MkdirAll(filepath.Dir(draft), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(draft, []byte("# Objective: x\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if got := readFile(t, filepath.Join(r.root, ".ai-kit", ".gitignore")); !strings.Contains(got, "*") {
		t.Errorf("the sentinel must contain `*`, got %q", got)
	}
	if out := git(t, r.root, "status", "--porcelain"); out != "" {
		t.Errorf("nothing the loop writes may reach git status, got %q", out)
	}
	if got := readFile(t, rootIgnore); got != committed {
		t.Errorf("the repository's own .gitignore must not be touched:\nwant %q\ngot  %q", committed, got)
	}
}

// Initialization runs on every command, so it runs over an existing directory far
// more often than over a new one. Nothing it finds there is its to rewrite: the
// log is append-only, and the sentinel may have been edited by whoever owns the
// repository.
func TestInitializationIsIdempotentAndClobbersNothing(t *testing.T) {
	r := throwawayRepo(t)
	if res := run(t, r, "query"); res.code != 0 {
		t.Fatalf("query: exit %d\n%s%s", res.code, res.stdout, res.stderr)
	}

	sentinel := filepath.Join(r.root, ".ai-kit", ".gitignore")
	appendToFile(t, sentinel, "!keep-me")
	id := add(t, r, "RS256 over HS256")

	if res := run(t, r, "query"); res.code != 0 {
		t.Fatalf("the second query: exit %d\n%s%s", res.code, res.stdout, res.stderr)
	}
	if got := readFile(t, sentinel); !strings.Contains(got, "!keep-me") {
		t.Errorf("an edited sentinel must survive, got %q", got)
	}
	if got := logText(t, r, r.log); !strings.Contains(got, id) {
		t.Errorf("the record %s is gone from the log:\n%s", id, got)
	}

	// User scope gets the sentinel too, with no conditional. ~/.ai-kit/ is
	// usually in no repository at all, where `*` is inert rather than wrong — and
	// a rule with an exception is a rule nobody can rely on.
	if res := run(t, r, "init", "--scope", "user"); res.code != 0 {
		t.Fatalf("init --scope user: exit %d\n%s%s", res.code, res.stdout, res.stderr)
	}
	if got := readFile(t, filepath.Join(r.home, ".ai-kit", ".gitignore")); !strings.Contains(got, "*") {
		t.Errorf("the user-scope sentinel must contain `*`, got %q", got)
	}
}

// unwritable makes a directory read-only for the rest of the test, and puts the
// bits back afterwards so t.TempDir can still delete what is inside it.
func unwritable(t *testing.T, dir string) {
	t.Helper()
	if os.Geteuid() == 0 {
		t.Skip("root writes through the permission bits this test is about")
	}
	if err := os.Chmod(dir, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o755) })
}

// Creating the location is best-effort on the read path. A repository checked out
// read-only answered `count: 0` before initialization moved onto that path, and it
// has to go on answering: a read that refuses because it could not create a
// directory it did not need would be this change breaking the command it exists to
// make work. A write, where the failure is fatal a line later anyway, still
// refuses — and says which of the two went wrong.
func TestAReadAgainstAnUnwritableRepoStillAnswersAndAWriteRefuses(t *testing.T) {
	r := throwawayRepo(t)
	unwritable(t, r.root)

	res := run(t, r, "query", "--kind", "decision")
	if res.code != 0 {
		t.Fatalf("a read must still answer: exit %d\n%s%s", res.code, res.stdout, res.stderr)
	}
	var out struct {
		Count int `json:"count"`
	}
	if err := json.Unmarshal([]byte(res.stdout), &out); err != nil {
		t.Fatal(err)
	}
	if out.Count != 0 {
		t.Errorf("expected count 0 from a repo with no log, got %d", out.Count)
	}
	if _, err := os.Stat(filepath.Dir(r.log)); !os.IsNotExist(err) {
		t.Errorf("nothing could have been created here: %v", err)
	}

	res = runBare(t, r, "add", "--plan", "demo", "--kind", "decision",
		"--title", "Nowhere to put this", "--about", "auth")
	if res.code == 0 {
		t.Fatal("a write into a repo whose log cannot be created must be refused")
	}
	if code, _ := errorCode(t, res); code != "init_failed" {
		t.Errorf("expected an init_failed refusal, got %q", code)
	}
}

// A location that could not be created is still validated, and the sweep that
// tried to create it still says it fell short.
//
// Two claims that have to stay apart. The log may be sitting right there, records
// and all — an .ai-kit/ nobody can write to reads fine — so `ok` keeps meaning what
// it means everywhere else, whether the records are well-formed. But a sweep that
// covered less than it was asked to has not passed, and a caller that reads only
// the exit code has to hear that: a JSON key nobody parses is not a report.
//
// Which sweep that is has moved. --all takes its set from the store and creates
// nothing, so nothing there can fall short; the one-log sweep still resolves a
// location on disk and still creates it, and that is where the second claim now
// lives.
func TestValidateKeepsALocationItCouldNotCreateAndSaysItFellShort(t *testing.T) {
	r := throwawayRepo(t)
	id := add(t, r, "RS256 over HS256")
	// A log directory an older binary left behind: it has the sentinel removed,
	// so initialization has something to write and cannot.
	if err := os.Remove(filepath.Join(r.root, ".ai-kit", ".gitignore")); err != nil {
		t.Fatal(err)
	}
	unwritable(t, filepath.Dir(r.log))

	out, code := validate(t, r, "--all")
	if !out.OK {
		t.Errorf("a log that reads must still be judged on its records alone: %+v", out.Reports)
	}
	if code != 0 {
		t.Errorf("--all creates nothing, so an unwritable location cannot make it fall short: exit %d", code)
	}
	if len(out.InitFailed) != 0 {
		t.Errorf("--all reports nothing it could not create, because it creates nothing: %v", out.InitFailed)
	}
	var found bool
	for _, rep := range out.Reports {
		if rep.Path == r.log {
			found = true
			if rep.Entries != 1 {
				t.Errorf("the record %s should have been counted, got %d entries", id, rep.Entries)
			}
		}
	}
	if !found {
		t.Fatalf("--all dropped the log whose location it could not create: %+v", out.Reports)
	}

	one, code := validateOne(t, r)
	if !one.OK {
		t.Errorf("a log that reads must still be judged on its records alone: %+v", one.Reports)
	}
	if code == 0 {
		t.Error("a sweep that could not create the location it was asked about must not exit 0")
	}
	if len(one.InitFailed) == 0 {
		t.Error("the failure to create must be reported rather than swallowed")
	}
}

// And the ordinary case keeps its exit 0: the sweep that created everything it
// resolved has nothing to report, so the key is absent rather than empty.
func TestValidateAllExitsZeroWhenItCreatedEverythingItResolved(t *testing.T) {
	r := throwawayRepo(t)
	add(t, r, "RS256 over HS256")

	out, code := validate(t, r, "--all")
	if !out.OK || code != 0 {
		t.Fatalf("a clean sweep must exit 0: ok=%v exit=%d %+v", out.OK, code, out.Reports)
	}
	if len(out.InitFailed) != 0 {
		t.Errorf("nothing failed to be created here, got %v", out.InitFailed)
	}
}

// promote resolves its destination outside the flag machinery, which made it the
// one command that could reach a location without creating it.
func TestPromoteCreatesItsDestination(t *testing.T) {
	r := throwawayRepo(t)
	id := add(t, r, "RS256 over HS256")

	if res := run(t, r, "promote", "--ids", id, "--to", "user"); res.code != 0 {
		t.Fatalf("promote: exit %d\n%s%s", res.code, res.stdout, res.stderr)
	}
	if got := readFile(t, filepath.Join(r.home, ".ai-kit", ".gitignore")); got != "*\n" {
		t.Errorf("the destination must be created like any other location, got %q", got)
	}
}

// ------------------------------------------------- promotion that reaches ---

type promoteOutput struct {
	From           string   `json:"from"`
	To             string   `json:"to"`
	Promoted       []string `json:"promoted"`
	AlreadyPresent []string `json:"already_present"`
	Unknown        []string `json:"unknown"`
	EdgesCopied    int      `json:"edges_copied"`
	Border         []string `json:"border"`
	BorderCount    int      `json:"border_count"`
	EdgesSkipped   int      `json:"edges_skipped"`
	DryRun         bool     `json:"dry_run"`
}

func promote(t *testing.T, r repo, args ...string) promoteOutput {
	t.Helper()
	res := run(t, r, append([]string{"promote"}, args...)...)
	if res.code != 0 {
		t.Fatalf("promote %v: exit %d\n%s%s", args, res.code, res.stdout, res.stderr)
	}
	var out promoteOutput
	if err := json.Unmarshal([]byte(res.stdout), &out); err != nil {
		t.Fatalf("promote printed something that is not JSON: %v\n%s", err, res.stdout)
	}
	return out
}

// addBare is `add` with the kind, a title and one tag, and nothing else — no
// anchor and no plan, which is how a `preference` is actually written: it
// belongs to the repository rather than to a tree in it, and an anchor promoted
// to the user scope would carry a path:line meaning nothing there.
func addBare(t *testing.T, r repo, kind, title string) string {
	t.Helper()
	res := runBare(t, r, "add", "--kind", kind, "--title", title, "--about", "auth")
	if res.code != 0 {
		t.Fatalf("add --kind %s %q: exit %d\n%s%s", kind, title, res.code, res.stdout, res.stderr)
	}
	return recordID(t, res)
}

// kindsIn is what a log holds, read back as JSON rather than matched as a
// substring: a kind named inside somebody's body would otherwise pass for a
// record of that kind.
func kindsIn(t *testing.T, r repo, path string) []string {
	t.Helper()
	var out []string
	for _, l := range lines(t, r, path) {
		var rec struct {
			Kind string `json:"kind"`
		}
		if err := json.Unmarshal([]byte(l), &rec); err != nil {
			t.Fatalf("a line in %s is not JSON: %v", path, err)
		}
		out = append(out, rec.Kind)
	}
	return out
}

func sweepValidates(t *testing.T, r repo) {
	t.Helper()
	out, code := validate(t, r, "--all")
	if !out.OK || code != 0 {
		t.Fatalf("both logs must still validate after a promotion: ok=%v exit=%d %+v",
			out.OK, code, out.Reports)
	}
}

// Selecting a class is what makes the command usable by somebody who does not
// already know every id they want. `--kind preference` takes the preferences and
// nothing else.
func TestPromoteByKindTakesTheClassWithoutBeingGivenAnID(t *testing.T) {
	r := throwawayRepo(t)
	addBare(t, r, "preference", "Migrations go through Atlas")
	addBare(t, r, "preference", "Errors wrap rather than replace")
	addBare(t, r, "decision", "Use RS256 over HS256")
	addBare(t, r, "adr", "Asymmetric signing")

	out := promote(t, r, "--kind", "preference", "--to", "user")
	if len(out.Promoted) != 2 {
		t.Fatalf("both preferences should have travelled: %+v", out)
	}
	got := kindsIn(t, r, r.user)
	if len(got) != 2 {
		t.Fatalf("the user log should hold exactly the two preferences, got %v", got)
	}
	for _, k := range got {
		if k != "preference" {
			t.Errorf("--kind preference carried a %s as well: %v", k, got)
		}
	}
	sweepValidates(t, r)
}

// The four kinds are a closed set, and a caller meets it as an answer rather
// than as an empty promotion. `decision` is the one that has to be refused by
// name: it is a real kind, and the loop writes one at the close of every
// objective, so sweeping the class would carry this repository's scaffolding
// upward.
func TestPromoteRefusesAKindItWillNotSweep(t *testing.T) {
	r := throwawayRepo(t)
	id := addBare(t, r, "adr", "Asymmetric signing")
	addBare(t, r, "decision", "Use RS256 over HS256")
	// A destination with something in it, so "nothing was written" is a claim
	// about bytes rather than about a file that never existed.
	promote(t, r, "--ids", id, "--to", "user")
	before := logText(t, r, r.user)

	for _, kind := range []string{"decision", "notakind"} {
		res := run(t, r, "promote", "--kind", kind, "--to", "user")
		if res.code != 1 {
			t.Fatalf("--kind %s must be refused with exit 1, got %d\n%s", kind, res.code, res.stdout)
		}
		var out struct {
			Code       string   `json:"code"`
			Error      string   `json:"error"`
			Kind       string   `json:"kind"`
			Promotable []string `json:"promotable"`
		}
		if err := json.Unmarshal([]byte(res.stdout), &out); err != nil {
			t.Fatalf("a refusal must still print JSON: %v\n%s%s", err, res.stdout, res.stderr)
		}
		if out.Code != "kind_not_promotable" {
			t.Errorf("--kind %s: got code %q", kind, out.Code)
		}
		if out.Kind != kind || !strings.Contains(out.Error, kind) {
			t.Errorf("the refusal must name the kind it rejected, got %q / %q", out.Kind, out.Error)
		}
		want := []string{"adr", "dead-end", "postmortem", "preference"}
		if strings.Join(out.Promotable, ",") != strings.Join(want, ",") {
			t.Errorf("the refusal must list the four kinds it does take, got %v", out.Promotable)
		}
		for _, k := range want {
			if !strings.Contains(out.Error, k) {
				t.Errorf("the message should name %s too, got %q", k, out.Error)
			}
		}
	}
	if after := logText(t, r, r.user); after != before {
		t.Errorf("a refused promotion must write nothing:\nbefore %q\nafter  %q", before, after)
	}
	sweepValidates(t, r)
}

// Naming a class and naming a record are two ways of asking for the same
// operation, so they add up. Having asked for neither is the only refusal.
func TestPromoteUnionsKindsAndIDsAndRefusesNeither(t *testing.T) {
	r := throwawayRepo(t)
	pref := addBare(t, r, "preference", "Migrations go through Atlas")
	decision := addBare(t, r, "decision", "Use RS256 over HS256")
	addBare(t, r, "adr", "Asymmetric signing")

	out := promote(t, r, "--kind", "preference", "--ids", decision, "--to", "user")
	if len(out.Promoted) != 2 {
		t.Fatalf("the union of both selections should travel: %+v", out)
	}
	landed := strings.Join(lines(t, r, r.user), "\n")
	for _, id := range []string{pref, decision} {
		if !strings.Contains(landed, id) {
			t.Errorf("%s never reached the user log:\n%s", id, landed)
		}
	}
	if got := kindsIn(t, r, r.user); len(got) != 2 {
		t.Errorf("the adr was in neither selection: %v", got)
	}

	res := run(t, r, "promote", "--to", "user")
	if res.code != 1 {
		t.Fatalf("promote with no selection must be refused, got exit %d\n%s", res.code, res.stdout)
	}
	code, msg := errorCode(t, res)
	if code != "missing_selection" {
		t.Errorf("got code %q", code)
	}
	if !strings.Contains(msg, "--ids") || !strings.Contains(msg, "--kind") {
		t.Errorf("the refusal must name both ways of asking, got %q", msg)
	}
	sweepValidates(t, r)
}

// An `adr` that arrives without the `decision` that fed it arrives as an
// assertion. Nobody names that decision, and it travels anyway.
func TestPromotingAnADRCarriesTheDecisionThatFedIt(t *testing.T) {
	r := throwawayRepo(t)
	decision := addBare(t, r, "decision", "Use RS256 over HS256")
	adr := addBare(t, r, "adr", "Asymmetric signing")
	mustLink(t, r, "derives_from", adr, decision)

	out := promote(t, r, "--ids", adr, "--to", "user")
	if len(out.Promoted) != 2 || out.EdgesCopied != 1 {
		t.Fatalf("the reasoning should have come along with the record: %+v", out)
	}
	landed := lines(t, r, r.user)
	if len(landed) != 3 {
		t.Fatalf("expected two entries and the edge between them, got %d lines:\n%s",
			len(landed), strings.Join(landed, "\n"))
	}
	joined := strings.Join(landed, "\n")
	for _, id := range []string{adr, decision} {
		if !strings.Contains(joined, id) {
			t.Errorf("%s is missing from the user log:\n%s", id, joined)
		}
	}
	if !strings.Contains(joined, `"kind":"edge"`) {
		t.Errorf("the edge between them has to be there or the decision arrives unattached:\n%s", joined)
	}
	sweepValidates(t, r)
}

// One hop, and the report says where it stopped. A promotion that leaves
// reasoning behind can no longer print success in silence.
func TestPromoteNamesWhatSatAtTheBorder(t *testing.T) {
	r := throwawayRepo(t)
	deviation := addBare(t, r, "deviation", "Rotated the signing key by hand")
	decision := addBare(t, r, "decision", "Use RS256 over HS256")
	adr := addBare(t, r, "adr", "Asymmetric signing")
	mustLink(t, r, "derives_from", adr, decision)
	mustLink(t, r, "derives_from", decision, deviation)

	out := promote(t, r, "--ids", adr, "--to", "user")
	if len(out.Border) != 1 || out.Border[0] != deviation {
		t.Fatalf("the record two hops out is what sat at the border: %+v", out)
	}
	if out.BorderCount != 1 {
		t.Errorf("the count has to agree with the list, got %d", out.BorderCount)
	}
	if out.EdgesSkipped != 1 {
		t.Errorf("the edge into it could not be copied and has to be counted, got %d", out.EdgesSkipped)
	}
	if out.EdgesCopied != 1 {
		t.Errorf("only the edge with both ends at the target travels, got %d", out.EdgesCopied)
	}
	if strings.Contains(strings.Join(lines(t, r, r.user), "\n"), deviation) {
		t.Error("the border is reported, not promoted: two hops is the closure this refused to be")
	}
	sweepValidates(t, r)
}

// The reach carries reasoning and refuses to carry scaffolding. An `adr` here
// derives from both the `decision` that fed it and the `objective` that produced
// it; only the first is knowledge that means anything in another repository.
//
// The exclusion is not allowed to be silent — that would be worse than not
// having it — so the objective left behind is reported as border and the edge to
// it is counted.
func TestOneHopWillNotCarryAnObjectiveAndSaysSo(t *testing.T) {
	r := throwawayRepo(t)
	objective := addBare(t, r, "objective", "Ship the token service")
	decision := addBare(t, r, "decision", "Use RS256 over HS256")
	adr := addBare(t, r, "adr", "Asymmetric signing")
	mustLink(t, r, "derives_from", adr, decision)
	mustLink(t, r, "derives_from", adr, objective)

	out := promote(t, r, "--ids", adr, "--to", "user")
	if len(out.Promoted) != 2 || out.EdgesCopied != 1 {
		t.Fatalf("the adr and the decision travel, with the edge between them: %+v", out)
	}
	landed := strings.Join(lines(t, r, r.user), "\n")
	if !strings.Contains(landed, decision) {
		t.Errorf("the reasoning must still come along:\n%s", landed)
	}
	if strings.Contains(landed, objective) {
		t.Errorf("the objective reached the user log:\n%s", landed)
	}
	if len(out.Border) != 1 || out.Border[0] != objective {
		t.Fatalf("an objective that was not reached for is still named: %+v", out)
	}
	if out.BorderCount != 1 || out.EdgesSkipped != 1 {
		t.Errorf("the edge into it stayed behind and has to be counted: %+v", out)
	}
	// Nothing dangles: the edge to the objective was never copied, so the user log
	// holds two entries and the one edge that joins them.
	sweepValidates(t, r)

	// The rule bounds the reach and not the selection. Naming the id promotes it,
	// and the edge that was border a moment ago comes along with it.
	second := promote(t, r, "--ids", objective, "--to", "user")
	if len(second.Promoted) != 1 || second.EdgesCopied != 1 {
		t.Fatalf("an objective named by id travels, and reconnects its edge: %+v", second)
	}
	sweepValidates(t, r)
}

// The same hole on the other side. `--kind` refuses `gotcha` as a class because
// an anchor's path:line is resolved against the repository it was written in;
// following an edge into one would have carried it up regardless, anchor and all.
func TestOneHopWillNotCarryAGotchaAndSaysSo(t *testing.T) {
	r := throwawayRepo(t)
	// addKind rather than addBare: version 3 refuses a gotcha with no anchor, and
	// the anchor is the whole reason this kind is not reached for.
	gotcha := addKind(t, r, "gotcha", "The lock is taken before the write")
	decision := addBare(t, r, "decision", "Use RS256 over HS256")
	adr := addBare(t, r, "adr", "Asymmetric signing")
	mustLink(t, r, "derives_from", adr, decision)
	mustLink(t, r, "derives_from", adr, gotcha)

	out := promote(t, r, "--ids", adr, "--to", "user")
	if len(out.Promoted) != 2 || out.EdgesCopied != 1 {
		t.Fatalf("the adr and the decision travel, with the edge between them: %+v", out)
	}
	landed := strings.Join(lines(t, r, r.user), "\n")
	if !strings.Contains(landed, decision) {
		t.Errorf("the reasoning must still come along:\n%s", landed)
	}
	if strings.Contains(landed, gotcha) {
		t.Errorf("the gotcha reached the user log, anchor and all:\n%s", landed)
	}
	if len(out.Border) != 1 || out.Border[0] != gotcha {
		t.Fatalf("a gotcha that was not reached for is still named: %+v", out)
	}
	if out.BorderCount != 1 || out.EdgesSkipped != 1 {
		t.Errorf("the edge into it stayed behind and has to be counted: %+v", out)
	}
	sweepValidates(t, r)

	// Reach, not selection: a caller who types the id has decided.
	second := promote(t, r, "--ids", gotcha, "--to", "user")
	if len(second.Promoted) != 1 || second.EdgesCopied != 1 {
		t.Fatalf("a gotcha named by id travels, and reconnects its edge: %+v", second)
	}
	sweepValidates(t, r)
}

// A preview has to be believed about what the real run will do, so it is the
// same report with one boolean added — and it leaves nothing behind, not even
// the destination it would have created.
func TestDryRunPrintsTheSameReportAndWritesNothing(t *testing.T) {
	r := throwawayRepo(t)
	pref := addBare(t, r, "preference", "Migrations go through Atlas")
	adr := addBare(t, r, "adr", "Asymmetric signing")

	preview := promote(t, r, "--kind", "preference", "--to", "user", "--dry-run")
	if !preview.DryRun || len(preview.Promoted) != 1 || preview.Promoted[0] != pref {
		t.Fatalf("the preview must name what a real run would move: %+v", preview)
	}
	if _, err := os.Stat(filepath.Join(r.home, ".ai-kit")); !os.IsNotExist(err) {
		t.Errorf("a preview that creates its destination is not a preview: %v", err)
	}

	// Now against a destination that exists, so "nothing was written" is a claim
	// about bytes.
	promote(t, r, "--ids", adr, "--to", "user")
	before := logText(t, r, r.user)
	preview = promote(t, r, "--kind", "preference", "--to", "user", "--dry-run")
	if after := logText(t, r, r.user); after != before {
		t.Fatalf("--dry-run wrote to the destination:\nbefore %q\nafter  %q", before, after)
	}

	real := promote(t, r, "--kind", "preference", "--to", "user")
	if real.DryRun {
		t.Error("a run without the flag must not claim to be one")
	}
	preview.DryRun = false
	if fmt.Sprintf("%+v", preview) != fmt.Sprintf("%+v", real) {
		t.Errorf("the preview and the run it previewed must report the same thing:\n%+v\n%+v", preview, real)
	}
	if !strings.Contains(logText(t, r, r.user), pref) {
		t.Error("dropping --dry-run must actually perform the promotion")
	}
	sweepValidates(t, r)
}

// Running it twice is something a person does by accident, and the second run
// has to be a no-op in the bytes as well as in the report. Nothing in the
// package can prove that: this is the binary, twice, over real files.
func TestPromotingTwiceAppendsNothingTheSecondTime(t *testing.T) {
	r := throwawayRepo(t)
	decision := addBare(t, r, "decision", "Use RS256 over HS256")
	adr := addBare(t, r, "adr", "Asymmetric signing")
	mustLink(t, r, "derives_from", adr, decision)
	addBare(t, r, "preference", "Migrations go through Atlas")

	first := promote(t, r, "--kind", "preference,adr", "--to", "user")
	if len(first.Promoted) != 3 || first.EdgesCopied != 1 {
		t.Fatalf("the preference, the adr and the decision it reached should all travel: %+v", first)
	}
	before := logText(t, r, r.user)

	second := promote(t, r, "--kind", "preference,adr", "--to", "user")
	if after := logText(t, r, r.user); after != before {
		t.Fatalf("the second run appended something:\nbefore %q\nafter  %q", before, after)
	}
	if len(second.Promoted) != 0 || second.EdgesCopied != 0 {
		t.Errorf("nothing was left to move: %+v", second)
	}
	if len(second.AlreadyPresent) != 3 {
		t.Errorf("everything the first run moved should read as already present: %+v", second)
	}
	sweepValidates(t, r)
}

// Two repositories called `api` are one label in a log that holds records from
// both. What tells them apart is the remote where there is one, and the absolute
// path where there is not.
func TestTwoReposWithOneNameStayApartInTheUserLog(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skipf("git is not installed: %v", err)
	}
	home := t.TempDir()
	base := t.TempDir()

	const remote = "https://example.invalid/somebody/api.git"
	store := throwawayStore(t)
	remoted := repo{root: filepath.Join(base, "with-a-remote", "api"), home: home, store: store}
	if err := os.MkdirAll(remoted.root, 0o755); err != nil {
		t.Fatal(err)
	}
	git(t, remoted.root, "init", "-q")
	git(t, remoted.root, "remote", "add", "origin", remote)

	// No real repository here, only the .git entry the tool looks for — which is
	// also the case `git remote get-url` has to fail on rather than answer.
	bare := repo{root: filepath.Join(base, "without-one", "api"), home: home, store: store}
	if err := os.MkdirAll(filepath.Join(bare.root, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}

	for _, r := range []*repo{&remoted, &bare} {
		r.log = filepath.Join(r.root, ".ai-kit", "memory.jsonl")
		r.user = filepath.Join(r.home, ".ai-kit", "memory.jsonl")
	}
	if filepath.Base(remoted.root) != filepath.Base(bare.root) {
		t.Fatal("the whole point is that the two directories share a name")
	}

	remotedID := addBare(t, remoted, "preference", "Migrations go through Atlas")
	bareID := addBare(t, bare, "preference", "Errors wrap rather than replace")
	promote(t, remoted, "--kind", "preference", "--to", "user")
	promote(t, bare, "--kind", "preference", "--to", "user")

	identities := map[string]string{}
	labels := map[string]string{}
	for _, l := range lines(t, remoted, remoted.user) {
		var rec struct {
			ID     string `json:"id"`
			Origin struct {
				Repo   string `json:"repo"`
				RepoID string `json:"repo_id"`
			} `json:"origin"`
		}
		if err := json.Unmarshal([]byte(l), &rec); err != nil {
			t.Fatal(err)
		}
		identities[rec.ID] = rec.Origin.RepoID
		labels[rec.ID] = rec.Origin.Repo
	}
	if labels[remotedID] != labels[bareID] || labels[remotedID] != "api" {
		t.Fatalf("both records should carry the same label `api`: %v", labels)
	}
	if identities[remotedID] == identities[bareID] {
		t.Fatalf("the two repositories are indistinguishable in the user log: %v", identities)
	}
	if identities[remotedID] != remote {
		t.Errorf("where a remote exists the identity is the remote, got %q", identities[remotedID])
	}
	if identities[bareID] != bare.root {
		t.Errorf("with no remote it falls back to the absolute path, got %q", identities[bareID])
	}
	// The one destination in this file that received records from two different
	// sources, so it is the one place a sweep could catch a log two promotions
	// left inconsistent. Swept from both sides, since --all resolves the repo log
	// from the repo it is run in and each of these has its own.
	sweepValidates(t, remoted)
	sweepValidates(t, bare)
}
