package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/alanv/ai-kit/tools/memory/internal/memory"
)

// These tests drive the built binary against files that really exist, for the
// reason the file header at the top of main_test.go gives: what is being checked
// is the bytes on disk and what the process says when it reads them again. An
// anchor check over an in-memory fixture would prove the hashing and none of the
// thing the hashing is for.

const anchorPath = "pkg/auth/token.go"

// srcFile writes a file into the throwaway repo, creating its directory.
func srcFile(t *testing.T, r repo, rel string, lines ...string) {
	t.Helper()
	path := filepath.Join(r.root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func srcLines(t *testing.T, r repo, rel string) []string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(r.root, filepath.FromSlash(rel)))
	if err != nil {
		t.Fatal(err)
	}
	return memory.SplitLines(raw)
}

// tokenSource is the file every test here anchors into: a lock order held for a
// reason the code cannot state, which is the shape a `gotcha` is written about.
func tokenSource(t *testing.T, r repo) {
	t.Helper()
	srcFile(t, r, anchorPath,
		"package auth",
		"",
		"func rotate(k *Key) {",
		"\tk.mu.Lock()",
		"\tdefer k.mu.Unlock()",
		"\trotateKey(k)",
		"}",
	)
}

// anchorSpec is a citation into a file that is really there. addKind passes it on
// every record it writes, so the fixture has to exist before any add runs.
func anchorSpec(t *testing.T, r repo) string {
	t.Helper()
	if _, err := os.Stat(filepath.Join(r.root, filepath.FromSlash(anchorPath))); err != nil {
		tokenSource(t, r)
	}
	return anchorPath + ":4-5"
}

type anchorsOutput struct {
	RepoRoot string         `json:"repo_root"`
	Matched  int            `json:"matched"`
	Checked  int            `json:"checked"`
	Returned int            `json:"returned"`
	NotFound []string       `json:"not_found"`
	Counts   map[string]int `json:"counts"`
	Records  []struct {
		ID      string `json:"id"`
		Kind    string `json:"kind"`
		Verdict string `json:"verdict"`
		Anchors []struct {
			Path    string `json:"path"`
			Line    int    `json:"line"`
			Lines   int    `json:"lines"`
			Hash    string `json:"hash"`
			Verdict string `json:"verdict"`
			Matches int    `json:"matches"`
			FoundAt []int  `json:"found_at"`
			Moved   bool   `json:"moved"`
			Note    string `json:"note"`
		} `json:"anchors"`
	} `json:"records"`
}

func checkAnchors(t *testing.T, r repo, args ...string) anchorsOutput {
	t.Helper()
	res := run(t, r, append([]string{"anchors"}, args...)...)
	if res.code != 0 {
		t.Fatalf("anchors: exit %d\n%s%s", res.code, res.stdout, res.stderr)
	}
	var out anchorsOutput
	if err := json.Unmarshal([]byte(res.stdout), &out); err != nil {
		t.Fatalf("anchors printed something that is not JSON: %v\n%s", err, res.stdout)
	}
	return out
}

// addGotcha writes the record the whole mechanism exists for, anchored at the
// citations the caller chooses.
func addGotcha(t *testing.T, r repo, title string, specs ...string) string {
	t.Helper()
	args := []string{"add", "--kind", "gotcha", "--title", title,
		"--body", "The rate limit upstream is 5/s and this is what stays under it.",
		"--about", "auth"}
	for _, spec := range specs {
		args = append(args, "--anchor", spec)
	}
	res := run(t, r, args...)
	if res.code != 0 {
		t.Fatalf("add --kind gotcha: exit %d\n%s%s", res.code, res.stdout, res.stderr)
	}
	return recordID(t, res)
}

func recordIDs(out anchorsOutput) []string {
	ids := make([]string, 0, len(out.Records))
	for _, r := range out.Records {
		ids = append(ids, r.ID)
	}
	return ids
}

func onlyRecord(t *testing.T, out anchorsOutput) anchorsOutput {
	t.Helper()
	if len(out.Records) != 1 {
		t.Fatalf("expected one anchored record, got %d: %+v", len(out.Records), out.Records)
	}
	return out
}

// An anchor is a field, and a field is only worth having if it comes back the
// way it went in. Read back through `query` rather than off the struct that
// wrote it: what a round trip has to survive is the JSON on disk.
func TestAnchorsSurviveTheRoundTrip(t *testing.T) {
	r := throwawayRepo(t)
	tokenSource(t, r)
	srcFile(t, r, "pkg/auth/key.go", "package auth", "", "type Key struct{}")

	id := addGotcha(t, r, "The lock is taken before the write, not after", anchorPath+":4-5")
	res := run(t, r, "add", "--kind", "decision", "--title", "Two anchors on one record",
		"--about", "auth", "--anchor", anchorPath+":1", "--anchor", "pkg/auth/key.go:3")
	if res.code != 0 {
		t.Fatalf("add with two anchors: exit %d\n%s%s", res.code, res.stdout, res.stderr)
	}

	var out struct {
		Records []struct {
			ID      string          `json:"id"`
			Anchors []memory.Anchor `json:"anchors"`
		} `json:"records"`
	}
	q := run(t, r, "query", "--kind", "gotcha")
	if err := json.Unmarshal([]byte(q.stdout), &out); err != nil {
		t.Fatal(err)
	}
	if len(out.Records) != 1 || out.Records[0].ID != id {
		t.Fatalf("expected the gotcha back, got %+v", out.Records)
	}

	got := out.Records[0].Anchors
	want := memory.Anchor{
		Path:  anchorPath,
		Line:  4,
		Lines: 2,
		Hash:  memory.HashLines(srcLines(t, r, anchorPath)[3:5]),
	}
	if len(got) != 1 || got[0] != want {
		t.Fatalf("the anchor did not survive the round trip: %+v, want %+v", got, want)
	}

	// And the whole log still passes the sweep, because an optional field on
	// records that predate it is what makes this a widening rather than a bump.
	if rep, code := validateOne(t, r); !rep.OK || code != 0 {
		t.Fatalf("anchored records must sweep clean: %+v", rep.Reports[0].Problems)
	}
}

// `gotcha` is the kind whose whole content is a claim about code that nothing
// regenerates, so one nobody can ever check is the case the requirement exists to
// refuse. The refusal has to name the field: a caller who is told only that the
// record was rejected has to guess.
func TestAGotchaWithoutAnAnchorIsRefusedAndWithOneIsAccepted(t *testing.T) {
	r := throwawayRepo(t)
	tokenSource(t, r)
	add(t, r, "RS256 over HS256")
	before := len(lines(t, r.log))

	res := run(t, r, "add", "--kind", "gotcha",
		"--title", "The retry count is somebody else's rate limit", "--about", "auth")
	if res.code == 0 {
		t.Fatal("a gotcha with no anchor must be refused")
	}
	var refusal struct {
		Code     string           `json:"code"`
		Problems []memory.Problem `json:"problems"`
	}
	if err := json.Unmarshal([]byte(res.stdout), &refusal); err != nil {
		t.Fatalf("a refusal must still print JSON: %v\n%s", err, res.stdout)
	}
	if refusal.Code != "invalid_record" {
		t.Errorf("expected an invalid_record refusal, got %q", refusal.Code)
	}
	if len(refusal.Problems) != 1 || refusal.Problems[0].Field != "anchors" {
		t.Fatalf("the refusal must name the missing field: %+v", refusal.Problems)
	}
	if !strings.Contains(refusal.Problems[0].Message, "--anchor") {
		t.Errorf("and say how to satisfy it, got %q", refusal.Problems[0].Message)
	}
	if after := len(lines(t, r.log)); after != before {
		t.Fatalf("a refused add must write nothing, log went from %d to %d lines", before, after)
	}

	addGotcha(t, r, "The lock is taken before the write, not after", anchorPath+":4-5")
	if after := len(lines(t, r.log)); after != before+1 {
		t.Fatalf("the same record with an anchor must be accepted, log went from %d to %d lines", before, after)
	}

	// The other half of the rule: every other kind is written without one, so the
	// requirement lands on `gotcha` and nowhere else.
	if res := run(t, r, "add", "--kind", "decision", "--title", "No anchor here", "--about", "auth"); res.code != 0 {
		t.Fatalf("only `gotcha` requires an anchor: exit %d\n%s", res.code, res.stdout)
	}
}

// A citation that names nothing is a typo at the moment it is written, and the
// log is append-only — so it is refused there rather than surfacing months later
// as a `could_not_check` indistinguishable from a file somebody deleted.
func TestAnAnchorThatCannotBeResolvedIsRefusedAtWriteTime(t *testing.T) {
	r := throwawayRepo(t)
	tokenSource(t, r)
	add(t, r, "RS256 over HS256")
	before := len(lines(t, r.log))

	for name, spec := range map[string]string{
		"no such file":     "pkg/auth/nope.go:3",
		"past the end":     anchorPath + ":40-42",
		"no line at all":   anchorPath,
		"backwards range":  anchorPath + ":5-4",
		"line zero":        anchorPath + ":0",
		"outside the repo": "../elsewhere/token.go:1",
	} {
		res := run(t, r, "add", "--kind", "decision", "--title", "Anchored at nothing",
			"--about", "auth", "--anchor", spec)
		if res.code == 0 {
			t.Errorf("%s (%q) must be refused", name, spec)
			continue
		}
		if code, _ := errorCode(t, res); code != "bad_anchor" {
			t.Errorf("%s (%q): expected a bad_anchor refusal, got %q", name, spec, code)
		}
	}
	if after := len(lines(t, r.log)); after != before {
		t.Fatalf("a refused add must write nothing, log went from %d to %d lines", before, after)
	}
}

// The verdict the whole design turns on. Code that merely moved is `still_holds`
// — the cited content is still there — and the check names the line it is at now
// without touching the line the record stored, because the log never rewrites.
func TestMovedTextStillHoldsAndTheRecordIsNotRewritten(t *testing.T) {
	r := throwawayRepo(t)
	tokenSource(t, r)
	id := addGotcha(t, r, "The lock is taken before the write, not after", anchorPath+":4-5")
	written := lines(t, r.log)

	before := checkAnchors(t, r, "--kind", "gotcha")
	onlyRecord(t, before)
	if before.Records[0].Verdict != "still_holds" || before.Records[0].Anchors[0].Moved {
		t.Fatalf("untouched code holds and has not moved: %+v", before.Records[0])
	}

	// Three lines above the citation, which is what a new import or a doc comment
	// does to every anchor in a file.
	src := srcLines(t, r, anchorPath)
	srcFile(t, r, anchorPath, append([]string{"// Package auth issues and rotates keys.", "//", "// Rotation is single-writer."}, src...)...)

	after := onlyRecord(t, checkAnchors(t, r, "--kind", "gotcha"))
	rec := after.Records[0]
	if rec.ID != id || rec.Verdict != "still_holds" {
		t.Fatalf("text that moved still holds: %+v", rec)
	}
	a := rec.Anchors[0]
	if !a.Moved {
		t.Error("the check must say it moved rather than leaving that to be noticed")
	}
	if a.Line != 4 {
		t.Errorf("the stored line is what the record wrote, got %d", a.Line)
	}
	if len(a.FoundAt) != 1 || a.FoundAt[0] != 7 {
		t.Errorf("and the reported line is where it is now, got %v", a.FoundAt)
	}
	if after.Counts["still_holds"] != 1 || after.Counts["changed"] != 0 {
		t.Errorf("the counts carry every verdict, got %v", after.Counts)
	}

	if now := lines(t, r.log); strings.Join(now, "\n") != strings.Join(written, "\n") {
		t.Fatal("the check must not rewrite the record it checked")
	}
}

// The two verdicts a caller has to be able to tell apart: text that was rewritten
// where it stood, and a file that is not there to be read. Collapsed into one,
// a deleted file is indistinguishable from an edited one, and the next move
// differs between them.
func TestRewrittenTextIsChangedAndAnUnreadableFileCannotBeChecked(t *testing.T) {
	r := throwawayRepo(t)
	tokenSource(t, r)
	addGotcha(t, r, "The lock is taken before the write, not after", anchorPath+":4-5")

	src := srcLines(t, r, anchorPath)
	src[3] = "\tk.mu.RLock()"
	srcFile(t, r, anchorPath, src...)

	out := onlyRecord(t, checkAnchors(t, r, "--kind", "gotcha"))
	if out.Records[0].Verdict != "changed" || out.Records[0].Anchors[0].Matches != 0 {
		t.Fatalf("rewritten code is changed: %+v", out.Records[0])
	}
	if out.Counts["changed"] != 1 {
		t.Errorf("expected one changed, got %v", out.Counts)
	}

	if err := os.Rename(filepath.Join(r.root, filepath.FromSlash(anchorPath)),
		filepath.Join(r.root, "pkg", "auth", "moved.go")); err != nil {
		t.Fatal(err)
	}
	gone := onlyRecord(t, checkAnchors(t, r, "--kind", "gotcha"))
	if gone.Records[0].Verdict != "could_not_check" {
		t.Fatalf("a file that cannot be read is could_not_check, not changed: %+v", gone.Records[0])
	}
	if !strings.Contains(gone.Records[0].Anchors[0].Note, anchorPath) {
		t.Errorf("the note must say what could not be read, got %q", gone.Records[0].Anchors[0].Note)
	}
}

// Text that appears twice is reported twice. Picking one of them would be the
// check guessing which occurrence the record meant, which is the one thing a
// mechanical check must not do.
func TestTextAppearingMoreThanOnceStillHoldsAndSaysHowOften(t *testing.T) {
	r := throwawayRepo(t)
	tokenSource(t, r)
	addGotcha(t, r, "The lock is taken before the write, not after", anchorPath+":4-5")

	src := srcLines(t, r, anchorPath)
	srcFile(t, r, anchorPath, append(append([]string{}, src...),
		"", "func reissue(k *Key) {", src[3], src[4], "\treissueKey(k)", "}")...)

	out := onlyRecord(t, checkAnchors(t, r, "--kind", "gotcha"))
	a := out.Records[0].Anchors[0]
	if out.Records[0].Verdict != "still_holds" {
		t.Fatalf("the cited text is still there, twice over: %+v", out.Records[0])
	}
	if a.Matches != 2 || len(a.FoundAt) != 2 {
		t.Fatalf("both occurrences must be named rather than one chosen: %+v", a)
	}
	if a.FoundAt[0] != 4 || a.FoundAt[1] != 10 {
		t.Errorf("expected matches at lines 4 and 10, got %v", a.FoundAt)
	}
	if a.Moved {
		t.Error("the stored line is among the matches, so nothing moved")
	}
}

// The filter is the whole interface: the same one every read command takes, so
// "is what we wrote about this area still true" is asked the way every other
// question here is asked. And the count-before-dump threshold is off, because a
// check that withheld its verdicts above 25 records could not be run over a log.
func TestAnchorsTakesTheSameFilterAndWithholdsNothing(t *testing.T) {
	r := throwawayRepo(t)
	tokenSource(t, r)
	for i := 0; i < memory.DefaultThreshold+5; i++ {
		addGotcha(t, r, "Constraint number "+string(rune('a'+i)), anchorPath+":4-5")
	}
	res := runBare(t, r, "add", "--plan", "other", "--kind", "decision", "--title", "Elsewhere",
		"--about", "billing", "--anchor", anchorPath+":1")
	if res.code != 0 {
		t.Fatalf("add: %s%s", res.stdout, res.stderr)
	}

	start := time.Now()
	all := checkAnchors(t, r, "--kind", "gotcha")
	elapsed := time.Since(start)
	if all.Checked != memory.DefaultThreshold+5 {
		t.Fatalf("every match is checked, not the first %d: got %d", memory.DefaultThreshold, all.Checked)
	}
	if all.Counts["still_holds"] != all.Checked {
		t.Errorf("all of them hold: %v", all.Counts)
	}
	// A whole log, well past the threshold, is a process rather than a wait. The
	// budget is the objective's: under a second, wall clock.
	if elapsed > time.Second {
		t.Errorf("checking %d records took %s", all.Checked, elapsed)
	}

	byArea := checkAnchors(t, r, "--about", "billing")
	if byArea.Checked != 0 {
		t.Errorf("--about bounds the check the way it bounds a query, got %d", byArea.Checked)
	}
	if byPlan := checkAnchors(t, r, "--kind", "decision"); byPlan.Checked != 0 {
		t.Errorf("--plan bounds it too: the decision is another plan's, got %d", byPlan.Checked)
	}
}

// A --repo-root that is not there is one refusal, not a verdict on every record.
// `could_not_check` is read as "this claim could not be verified", which is a
// sentence about the code; a typo in the flag is a sentence about the command,
// and the two must not arrive looking the same.
func TestARepoRootThatIsNotADirectoryIsRefusedRatherThanUncheckable(t *testing.T) {
	r := throwawayRepo(t)
	tokenSource(t, r)
	addGotcha(t, r, "The lock is taken before the write, not after", anchorPath+":4-5")

	notADir := filepath.Join(r.root, filepath.FromSlash(anchorPath))
	for name, root := range map[string]string{
		"missing":     filepath.Join(r.root, "no-such-checkout"),
		"not a dir":   notADir,
		"empty value": "",
	} {
		res := run(t, r, "anchors", "--kind", "gotcha", "--repo-root", root)
		if name == "empty value" {
			// An empty --repo-root falls back to the repo the log lives in, which
			// is a directory: the refusal is for a value that names nothing, not
			// for the absence of one.
			if res.code != 0 {
				t.Errorf("an empty --repo-root falls back rather than refusing: %s", res.stdout)
			}
			continue
		}
		if res.code == 0 {
			t.Errorf("%s (%q) must be refused", name, root)
			continue
		}
		code, msg := errorCode(t, res)
		if code != "no_repo_root" {
			t.Errorf("%s: expected a no_repo_root refusal, got %q", name, code)
		}
		if !strings.Contains(msg, "repo-root") {
			t.Errorf("%s: the refusal must name the flag, got %q", name, msg)
		}
	}
}

// The user log lives outside any repository, so an anchor there has no root to
// be resolved against — and the anchor requirement does not bend for that: a
// `gotcha` is refused without one in every scope. --repo-root is what makes the
// kind reachable, by supplying the root rather than inferring it.
func TestAGotchaIsWritableAtUserScopeOnlyWithARepoRoot(t *testing.T) {
	r := throwawayRepo(t)
	tokenSource(t, r)

	bare := runBare(t, r, "add", "--plan", "demo", "--scope", "user", "--kind", "gotcha",
		"--title", "Every repo of ours pins this timeout", "--about", "auth")
	if bare.code == 0 {
		t.Fatal("the anchor requirement is not scope-dependent: a user-scope gotcha with no anchor must be refused")
	}
	var refusal struct {
		Code     string           `json:"code"`
		Problems []memory.Problem `json:"problems"`
	}
	if err := json.Unmarshal([]byte(bare.stdout), &refusal); err != nil {
		t.Fatalf("a refusal must still print JSON: %v\n%s", err, bare.stdout)
	}
	if refusal.Code != "invalid_record" || len(refusal.Problems) != 1 || refusal.Problems[0].Field != "anchors" {
		t.Fatalf("the same refusal as at repo scope, naming the same field: %+v", refusal)
	}

	// Without a root there is nothing to hash the cited lines out of, and the
	// refusal says which flag supplies one.
	noRoot := runBare(t, r, "add", "--plan", "demo", "--scope", "user", "--kind", "gotcha",
		"--title", "Every repo of ours pins this timeout", "--about", "auth",
		"--anchor", anchorPath+":4-5")
	if noRoot.code == 0 {
		t.Fatal("an anchor at user scope has no repo to resolve against and must be refused")
	}
	if code, msg := errorCode(t, noRoot); code != "no_repo_root" || !strings.Contains(msg, "repo-root") {
		t.Fatalf("the refusal must point at --repo-root, got %q: %s", code, msg)
	}

	res := runBare(t, r, "add", "--plan", "demo", "--scope", "user", "--kind", "gotcha",
		"--title", "Every repo of ours pins this timeout", "--about", "auth",
		"--anchor", anchorPath+":4-5", "--repo-root", r.root)
	if res.code != 0 {
		t.Fatalf("--repo-root is what makes the kind reachable at user scope: exit %d\n%s%s",
			res.code, res.stdout, res.stderr)
	}

	// And the record landed in the user log, anchored, with the repo it came from
	// on it — which is the only thing that says what its path is relative to.
	var out struct {
		Record struct {
			Anchors []memory.Anchor `json:"anchors"`
			Origin  struct {
				Repo string `json:"repo"`
			} `json:"origin"`
		} `json:"record"`
	}
	if err := json.Unmarshal([]byte(res.stdout), &out); err != nil {
		t.Fatal(err)
	}
	if len(out.Record.Anchors) != 1 || out.Record.Anchors[0].Path != anchorPath {
		t.Fatalf("the anchor must survive the write: %+v", out.Record.Anchors)
	}
	// origin.repo is the only thing on the record that says which repository the
	// anchor's path is relative to, so it has to name the one --repo-root pointed
	// at. Non-empty is not enough: at user scope loc.Repo is empty, and the
	// obvious default — filepath.Base of it — answers ".", which is non-empty and
	// names nothing.
	if want := filepath.Base(r.root); out.Record.Origin.Repo != want {
		t.Errorf("origin.repo must name the repository the lines were read out of: got %q, want %q",
			out.Record.Origin.Repo, want)
	}

	// And an explicit --repo alongside --repo-root still wins, because the
	// default is a default.
	named := runBare(t, r, "add", "--plan", "demo", "--scope", "user", "--kind", "gotcha",
		"--title", "Named repo wins", "--about", "auth",
		"--anchor", anchorPath+":4-5", "--repo-root", r.root, "--repo", "checkout-under-another-name")
	if named.code != 0 {
		t.Fatalf("add with --repo: exit %d\n%s%s", named.code, named.stdout, named.stderr)
	}
	if err := json.Unmarshal([]byte(named.stdout), &out); err != nil {
		t.Fatal(err)
	}
	if out.Record.Origin.Repo != "checkout-under-another-name" {
		t.Errorf("--repo given explicitly is not a default and must not be overridden, got %q",
			out.Record.Origin.Repo)
	}
	if _, err := os.Stat(r.user); err != nil {
		t.Fatalf("the record went to the user log: %v", err)
	}
}

// A record with no anchors is not a record whose claims hold, so it is skipped
// rather than counted as still holding. The report says how many records the
// filter matched next to how many carried anchors, so the gap between the two is
// visible instead of implied.
func TestUnanchoredRecordsAreSkippedRatherThanPassed(t *testing.T) {
	r := throwawayRepo(t)
	tokenSource(t, r)
	add(t, r, "RS256 over HS256")
	id := addGotcha(t, r, "The lock is taken before the write, not after", anchorPath+":4-5")

	out := checkAnchors(t, r)
	if out.Matched != 2 {
		t.Fatalf("both records matched the filter, got %d", out.Matched)
	}
	if out.Checked != 1 || len(out.Records) != 1 || out.Records[0].ID != id {
		t.Fatalf("only the anchored one can be checked: %+v", out.Records)
	}
	for _, v := range []string{"still_holds", "changed", "could_not_check"} {
		if _, present := out.Counts[v]; !present {
			t.Errorf("every verdict is counted whether or not it happened; %q is missing from %v", v, out.Counts)
		}
	}
}

// --repo-root is what says which working tree the anchors are read against. The
// log stays where it is; only the code being checked moves.
func TestRepoRootChoosesTheWorkingTreeTheAnchorsAreReadAgainst(t *testing.T) {
	r := throwawayRepo(t)
	tokenSource(t, r)
	addGotcha(t, r, "The lock is taken before the write, not after", anchorPath+":4-5")

	// A second checkout of the same file, with the cited lines rewritten in it and
	// not in the first.
	other := throwawayRepo(t)
	src := srcLines(t, r, anchorPath)
	src[3] = "\tk.mu.RLock()"
	srcFile(t, other, anchorPath, src...)

	here := onlyRecord(t, checkAnchors(t, r, "--kind", "gotcha"))
	if here.Records[0].Verdict != "still_holds" {
		t.Fatalf("the log's own repo still has the cited lines: %+v", here.Records[0])
	}
	if here.RepoRoot != r.root {
		t.Errorf("the default root is the repo the log lives in, got %q", here.RepoRoot)
	}

	there := onlyRecord(t, checkAnchors(t, r, "--kind", "gotcha", "--repo-root", other.root))
	if there.Records[0].Verdict != "changed" {
		t.Fatalf("the named root is what gets read: %+v", there.Records[0])
	}
}

func TestAVerdictFilterTakesTheRecordsVerdictAndNotEachAnchors(t *testing.T) {
	r := throwawayRepo(t)
	tokenSource(t, r)
	srcFile(t, r, "pkg/auth/key.go", "package auth", "", "type Key struct{}")

	mixed := addGotcha(t, r, "The lock is taken before the write, not after",
		anchorPath+":4-5", "pkg/auth/key.go:3")
	intact := addGotcha(t, r, "A key is a value everywhere it is passed", "pkg/auth/key.go:3")

	src := srcLines(t, r, anchorPath)
	src[3] = "\tk.mu.RLock()"
	srcFile(t, r, anchorPath, src...)

	changed := checkAnchors(t, r, "--kind", "gotcha", "--verdict", "changed")
	if ids := recordIDs(changed); len(ids) != 1 || ids[0] != mixed {
		t.Fatalf("a record with one rewritten anchor is a changed record: got %v, want [%s]", ids, mixed)
	}
	seen := map[string]int{}
	for _, a := range changed.Records[0].Anchors {
		seen[a.Verdict]++
	}
	if seen["changed"] != 1 || seen["still_holds"] != 1 {
		t.Fatalf("the record comes back whole, on its roll-up, not trimmed to the anchors that changed: %v", seen)
	}

	holds := checkAnchors(t, r, "--kind", "gotcha", "--verdict", "still_holds")
	if ids := recordIDs(holds); len(ids) != 1 || ids[0] != intact {
		t.Fatalf("a record whose worst anchor changed does not still hold: got %v, want [%s]", ids, intact)
	}
}

func TestARecordWhoseAnchorsBothMatchAPathIsReturnedOnce(t *testing.T) {
	r := throwawayRepo(t)
	tokenSource(t, r)
	srcFile(t, r, "pkg/auth/key.go", "package auth", "", "type Key struct{}")

	twice := addGotcha(t, r, "The lock is taken before the write, not after",
		anchorPath+":4-5", anchorPath+":1")
	addGotcha(t, r, "A key is a value everywhere it is passed", "pkg/auth/key.go:3")

	out := checkAnchors(t, r, "--kind", "gotcha", "--path", "token.go")
	ids := recordIDs(out)
	if len(ids) != 1 || ids[0] != twice {
		t.Fatalf("two matching anchors are one record: got %v, want [%s]", ids, twice)
	}
	if out.Returned != 1 {
		t.Errorf("returned counts records the way records does, got %d", out.Returned)
	}
	if n := len(out.Records[0].Anchors); n != 2 {
		t.Errorf("and the record keeps every anchor it was written with, got %d", n)
	}
}

func TestAnIdTheLogDoesNotCarryIsReportedNotFound(t *testing.T) {
	r := throwawayRepo(t)
	tokenSource(t, r)
	id := addGotcha(t, r, "The lock is taken before the write, not after", anchorPath+":4-5")
	const missing = "01JXKQ7M4T8YZPABCDEFGHJKMN"

	out := checkAnchors(t, r, "--id", id+","+missing)
	if ids := recordIDs(out); len(ids) != 1 || ids[0] != id {
		t.Fatalf("--id checks the records it names and nothing else: got %v, want [%s]", ids, id)
	}
	if strings.Join(out.NotFound, ",") != missing {
		t.Fatalf("the id the log does not carry has to come back named, got %v", out.NotFound)
	}
}
