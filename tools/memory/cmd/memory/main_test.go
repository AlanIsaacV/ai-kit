package main

import (
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
	root string // the repo root, whose .ai-kit/memory.jsonl is the only log
	home string // HOME, so nothing here can reach the real user-scope log
	log  string // <root>/.ai-kit/memory.jsonl
	user string // <home>/.ai-kit/memory.jsonl
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
	r := repo{root: t.TempDir(), home: t.TempDir()}
	if err := os.MkdirAll(filepath.Join(r.root, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	r.log = filepath.Join(r.root, ".ai-kit", "memory.jsonl")
	r.user = filepath.Join(r.home, ".ai-kit", "memory.jsonl")
	return r
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
	cmd := exec.Command(memoryBin, afterSubcommand(t, args, "--dir", r.root)...)
	cmd.Env = append(os.Environ(), "HOME="+r.home, "AI_KIT_AUTHOR=test-suite", "AI_KIT_PLAN=")
	// Belt as well as braces: --dir already points at the throwaway repo, and so
	// does the working directory, so a helper that ever failed to pass the flag
	// still could not reach the log of the repo these tests run inside.
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

func lines(t *testing.T, path string) []string {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return strings.Split(strings.TrimRight(string(raw), "\n"), "\n")
}

func appendLine(t *testing.T, path, line string) {
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

	got := lines(t, r.log)
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
	if err := json.Unmarshal([]byte(lines(t, r.log)[0]), &rec); err != nil {
		t.Fatal(err)
	}
	if rec.Scope != "repo" {
		t.Errorf("expected scope repo on disk, got %q", rec.Scope)
	}
	if rec.Origin.Plan != "demo" {
		t.Errorf("expected origin.plan to carry the plan, got %q", rec.Origin.Plan)
	}
	if _, err := os.Stat(r.user); !os.IsNotExist(err) {
		t.Errorf("nothing should have touched the user log: %v", err)
	}
}

// The version is stamped, not chosen. A caller that could pick one would
// eventually pick wrong, and a record that lies about its version is worse
// than one with no version at all.
func TestThereIsNoFlagForTheVersion(t *testing.T) {
	r := throwawayRepo(t)
	add(t, r, "RS256 over HS256")
	before := lines(t, r.log)

	res := run(t, r, "add", "--kind", "decision", "--title", "Forced", "--about", "auth", "--version", "3")
	if res.code == 0 {
		t.Fatal("--version should not be a flag this tool accepts")
	}
	if after := lines(t, r.log); len(after) != len(before) {
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
	before := lines(t, r.log)

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
	if after := lines(t, r.log); len(after) != len(before) {
		t.Fatalf("nothing refused may reach disk, log went from %d to %d lines", len(before), len(after))
	}

	// Via the environment, which is the other place leniency tends to hide.
	cmd := exec.Command(memoryBin, "add", "--kind", "decision", "--title", "Sneaky",
		"--about", "auth", "--plan", "demo", "--dir", r.root)
	cmd.Env = append(os.Environ(), "HOME="+r.home, "AI_KIT_AUTHOR=test-suite",
		"AI_KIT_SCOPE=plan", "MEMORY_SCOPE=plan", "AI_KIT_ALLOW_PLAN_SCOPE=1")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("the add itself should still succeed at the repo scope: %v\n%s", err, out)
	}
	for _, line := range lines(t, r.log) {
		if strings.Contains(line, `"scope":"plan"`) {
			t.Fatalf("an environment variable talked the gate into the plan scope: %s", line)
		}
	}
}

// Dropping DisallowUnknownFields on the read path did not lose typo
// protection; it moved it to where typos originate. The CLI is what a person
// mistypes, and it refuses before anything reaches the log.
func TestAMistypedFlagIsRefusedBeforeAnythingIsWritten(t *testing.T) {
	r := throwawayRepo(t)
	add(t, r, "RS256 over HS256")
	before := lines(t, r.log)

	res := run(t, r, "add", "--kind", "decision", "--titel", "Typo", "--about", "auth")
	if res.code == 0 {
		t.Fatal("a mistyped flag must be refused")
	}
	if !strings.Contains(res.stderr, "titel") {
		t.Errorf("the refusal should name the flag it did not recognise, got %q", res.stderr)
	}
	if after := lines(t, r.log); len(after) != len(before) {
		t.Fatalf("a refused add must write nothing, log went from %d to %d lines", len(before), len(after))
	}
}

// -------------------------------------------------------------- the sweep ---

type validateOutput struct {
	OK       bool `json:"ok"`
	Unjudged int  `json:"unjudged"`
	Reports  []struct {
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
	appendLine(t, r.log, `{"version":99,"id":"01KZ34725V37YX845RTVPC7N0E","kind":"unheard-of",`+
		`"slug":"Not A Slug","scope":"galaxy","at":"whenever","about":[],"title":""}`)
	appendLine(t, r.log, `{"kind":"decision",`)

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
	appendLine(t, r.log, `{"version":99,"id":"01KZ34725V37YX845RTVPC7N0E","kind":"unheard-of",`+
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
	appendLine(t, r.log, `{"id":"01KZ2WHVK0GPM4XWVFFKYXGMHQ","kind":"decision","slug":"unstamped",`+
		`"scope":"plan","at":"2026-08-01T10:00:00Z","about":["auth"],`+
		`"title":"Written before the stamp","confidence":"high","origin":{"plan":"demo"}}`)
	appendLine(t, r.log, `{"kind":"edge","type":"relates_to","from":"01KZ2WHVK0GPM4XWVFFKYXGMHQ",`+
		`"to":"01KZ34725V37YX845RTVPC7N0E","at":"2026-08-01T10:00:00Z"}`)
	appendLine(t, r.log, `{"version":1,"id":"01KZ34725V37YX845RTVPC7N0E","kind":"adr","slug":"stamped",`+
		`"scope":"plan","at":"2026-08-01T10:05:00Z","about":["auth"],"title":"Written after it",`+
		`"origin":{"plan":"demo"}}`)

	before, err := os.ReadFile(r.log)
	if err != nil {
		t.Fatal(err)
	}

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

	after, err := os.ReadFile(r.log)
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != string(before) {
		t.Fatal("reading the log rewrote it; nothing already written may be touched")
	}
	// The bytes are the proof: printing a resolved 1 must not put one on disk,
	// and neither must a version bump reach back and restamp what is already
	// there.
	if !strings.Contains(string(after), `{"id":"01KZ2WHVK0GPM4XWVFFKYXGMHQ","kind":"decision"`) {
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
	if _, err := os.Stat(r.log); err != nil {
		t.Fatalf("the log is not on disk: %v", err)
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
func TestKindsAdvertisesTwoScopesAndKeepsVersion1sKinds(t *testing.T) {
	// `kinds` reads no log and takes no flags, so it is run bare.
	cmd := exec.Command(memoryBin, "kinds")
	stdout, err := cmd.Output()
	if err != nil {
		t.Fatalf("kinds: %v\n%s", err, stdout)
	}
	var out struct {
		EntryKinds []string `json:"entry_kinds"`
		EdgeTypes  []string `json:"edge_types"`
		Scopes     []string `json:"scopes"`
	}
	if err := json.Unmarshal(stdout, &out); err != nil {
		t.Fatal(err)
	}
	if strings.Join(out.Scopes, ",") != "repo,user" {
		t.Errorf(`expected scopes ["repo","user"], got %v`, out.Scopes)
	}
	// Version 2 changed the scopes and nothing else. `unit` in particular stays:
	// taking it out belongs to another objective.
	for _, want := range []string{"objective", "unit", "decision", "deviation", "assumption",
		"open-question", "wont-fix", "dead-end", "adr", "postmortem", "review-finding"} {
		if !slices.Contains(out.EntryKinds, want) {
			t.Errorf("kind %q went missing from the advertised set: %v", want, out.EntryKinds)
		}
	}
	if !slices.Contains(out.EdgeTypes, "implements") {
		t.Errorf("edge types should be version 1's: %v", out.EdgeTypes)
	}
}

// A record that cannot name its plan is unreachable by every bounded query, and
// the log is append-only, so there is no fixing it afterwards. `add` stops.
func TestAddRefusesWithoutAPlanAndTakesItFromTheEnvironment(t *testing.T) {
	r := throwawayRepo(t)

	res := runBare(t, r, "add", "--kind", "decision", "--title", "Homeless", "--about", "auth")
	if res.code == 0 {
		t.Fatal("add with no plan must be refused")
	}
	if code, _ := errorCode(t, res); code != "no_plan" {
		t.Errorf("expected a no_plan refusal, got %q", code)
	}
	if _, err := os.Stat(r.log); !os.IsNotExist(err) {
		t.Errorf("a refused add must not create the log: %v", err)
	}

	// AI_KIT_PLAN is the other way to say it, and it has to be enough on its own.
	cmd := exec.Command(memoryBin, "add", "--kind", "decision", "--title", "From the environment",
		"--about", "auth", "--dir", r.root)
	cmd.Env = append(os.Environ(), "HOME="+r.home, "AI_KIT_AUTHOR=test-suite", "AI_KIT_PLAN=from-env")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("AI_KIT_PLAN should be enough: %v\n%s", err, out)
	}
	if !strings.Contains(lines(t, r.log)[0], `"plan":"from-env"`) {
		t.Errorf("the record should carry the plan from the environment: %s", lines(t, r.log)[0])
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
	got := lines(t, r.log)
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
	promoted := lines(t, r.user)
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
	appendLine(t, r.log, `{"version":1,"id":"01KZ34725V37YX845RTVPC7N0E","kind":"adr","slug":"old",`+
		`"scope":"plan","at":"2026-08-01T10:05:00Z","about":["auth"],"title":"Written under version 1",`+
		`"origin":{"plan":"demo"}}`)

	res := run(t, r, "promote", "--ids", "01KZ34725V37YX845RTVPC7N0E", "--to", "user")
	if res.code != 0 {
		t.Fatalf("promote: exit %d\n%s%s", res.code, res.stdout, res.stderr)
	}
	line := lines(t, r.user)[0]
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
