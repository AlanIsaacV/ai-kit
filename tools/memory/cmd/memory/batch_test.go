package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// These drive the built binary like every other test in this package, because
// what is being checked is what lands on disk and what the process prints — and
// for a batch the interesting part is precisely what does *not* land.

// batchOutput is what a successful `add --batch` prints. The bodies are not in
// it: twenty-five of them printed back would cost the caller as much context as
// writing them did.
type batchOutput struct {
	Path    string `json:"path"`
	Count   int    `json:"count"`
	Edges   int    `json:"edges"`
	Records []struct {
		ID    string `json:"id"`
		Kind  string `json:"kind"`
		Slug  string `json:"slug"`
		Title string `json:"title"`
	} `json:"records"`
}

type batchRefusal struct {
	Code     string `json:"code"`
	Problems []struct {
		Line    int    `json:"line"`
		Field   string `json:"field"`
		Message string `json:"message"`
	} `json:"problems"`
}

func writeBatch(t *testing.T, dir, name string, lines ...string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func fileBytes(t *testing.T, path string) []byte {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func batchResult(t *testing.T, res result) batchOutput {
	t.Helper()
	if res.code != 0 {
		t.Fatalf("exit %d: %s%s", res.code, res.stdout, res.stderr)
	}
	var out batchOutput
	if err := json.Unmarshal([]byte(res.stdout), &out); err != nil {
		t.Fatalf("a batch must print JSON: %v\n%s", err, res.stdout)
	}
	return out
}

func batchRefused(t *testing.T, res result) batchRefusal {
	t.Helper()
	if res.code != 1 {
		t.Fatalf("a refused batch exits 1, got %d: %s%s", res.code, res.stdout, res.stderr)
	}
	var out batchRefusal
	if err := json.Unmarshal([]byte(res.stdout), &out); err != nil {
		t.Fatalf("a refusal must still print JSON: %v\n%s", err, res.stdout)
	}
	return out
}

// entryLine is one line of a batch, built here rather than written out as a
// string literal so that a test about one field is not a test about JSON quoting.
func entryLine(t *testing.T, fields map[string]any) string {
	t.Helper()
	b, err := json.Marshal(fields)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// ------------------------------------------------------------- one write ---

// Twenty-five entries in one file, then the same file with one line broken. The
// second half is the half that matters: the log's bytes have to be what they
// were, because an append-only log cannot take a partial batch back.
func TestABatchIsOneWriteOrNoWriteAtAll(t *testing.T) {
	r := throwawayRepo(t)
	add(t, r, "Something already in the log")
	before := logText(t, r, r.log)

	good := make([]string, 0, 25)
	for i := 1; i <= 25; i++ {
		good = append(good, entryLine(t, map[string]any{
			"kind":  "decision",
			"title": fmt.Sprintf("Decision number %d", i),
			"body":  "Why this one rather than the other.",
			"about": []string{"auth", "pkg/auth/"},
		}))
	}
	out := batchResult(t, run(t, r, "add", "--batch", writeBatch(t, r.root, "good.jsonl", good...)))
	if out.Count != 25 || len(out.Records) != 25 {
		t.Fatalf("25 entries in, 25 out: count %d, records %d", out.Count, len(out.Records))
	}
	if got := len(lines(t, r, r.log)) - len(strings.Split(strings.TrimRight(before, "\n"), "\n")); got != 25 {
		t.Fatalf("the log should have grown by exactly 25 lines, grew by %d", got)
	}
	// Every id is distinct and every one of them is on disk.
	onDisk := strings.Join(lines(t, r, r.log), "\n")
	seen := map[string]bool{}
	for _, rec := range out.Records {
		if seen[rec.ID] {
			t.Fatalf("id %s reported twice", rec.ID)
		}
		seen[rec.ID] = true
		if !strings.Contains(onDisk, rec.ID) {
			t.Fatalf("id %s was reported but is not in the log", rec.ID)
		}
	}

	// The same file with line 13 carrying a kind the closed set does not hold.
	beforeRefusal := logText(t, r, r.log)
	bad := make([]string, len(good))
	copy(bad, good)
	bad[12] = entryLine(t, map[string]any{
		"kind": "notakind", "title": "Decision number 13", "about": []string{"auth"},
	})
	ref := batchRefused(t, run(t, r, "add", "--batch", writeBatch(t, r.root, "bad.jsonl", bad...)))
	if ref.Code != "invalid_batch" {
		t.Errorf("a refused batch is refused as a batch, got code %q", ref.Code)
	}
	if len(ref.Problems) != 1 || ref.Problems[0].Line != 13 || ref.Problems[0].Field != "kind" {
		t.Fatalf("the problem must name line 13 and the kind field, got %+v", ref.Problems)
	}
	if got := logText(t, r, r.log); got != beforeRefusal {
		t.Fatalf("a refused batch must leave the log byte-identical\nbefore %d bytes\nafter  %d bytes",
			len(beforeRefusal), len(got))
	}
}

// A batch on stdin writes what a batch from a path writes. `-` is for the caller
// that already has the bytes; a path is what removes the shell from the flow.
func TestTheBatchReadsStdin(t *testing.T) {
	r := throwawayRepo(t)
	line := entryLine(t, map[string]any{
		"kind": "assumption", "title": "The caller has the bytes already", "about": []string{"auth"},
	})
	out := batchResult(t, runStdin(t, r, line+"\n", "add", "--plan", "demo", "--batch", "-"))
	if out.Count != 1 {
		t.Fatalf("one entry in, %d out", out.Count)
	}
	if got := entryByID(t, r, out.Records[0].ID); got.Kind != "assumption" || got.Origin.Plan != "demo" {
		t.Fatalf("stdin's entry landed as %+v", got)
	}
}

// The fields no caller may choose are stamped, exactly as the flagged path
// stamps them: a line that could name its own version or id could write a record
// that lies about itself, into a log that cannot take one back.
func TestABatchLineNeverStampsWhatTheToolStamps(t *testing.T) {
	r := throwawayRepo(t)

	for _, field := range []string{"id", "at", "version", "scope", "origin"} {
		line := entryLine(t, map[string]any{
			"kind": "decision", "title": "Stamped by the tool", "about": []string{"auth"},
			field: "whatever",
		})
		ref := batchRefused(t, runStdin(t, r, line+"\n", "add", "--plan", "demo", "--batch", "-"))
		if len(ref.Problems) != 1 || !strings.Contains(ref.Problems[0].Message, field) {
			t.Fatalf("%q must be refused by name, got %+v", field, ref.Problems)
		}
	}

	// And a typo is refused for the same reason and by the same rule, which is
	// where the flagged path's typo protection went.
	line := entryLine(t, map[string]any{
		"kind": "decision", "titel": "Typo", "about": []string{"auth"},
	})
	ref := batchRefused(t, runStdin(t, r, line+"\n", "add", "--plan", "demo", "--batch", "-"))
	if len(ref.Problems) != 1 || !strings.Contains(ref.Problems[0].Message, "titel") {
		t.Fatalf("a mistyped key must be named, got %+v", ref.Problems)
	}

	// What is stamped is stamped, and reads back as the current schema.
	good := entryLine(t, map[string]any{
		"kind": "decision", "title": "Stamped correctly", "about": []string{"auth"},
	})
	out := batchResult(t, runStdin(t, r, good+"\n", "add", "--plan", "demo", "--batch", "-"))
	e := entryByID(t, r, out.Records[0].ID)
	if e.Version != memoryCurrentVersion || e.Scope != "repo" || e.At == "" {
		t.Fatalf("version, scope and at are the tool's to write: %+v", e)
	}
}

// --plan stamps the batch; a line that names one wins for itself alone. Absent on
// the line is not the same claim as empty — it means "whatever the batch was".
func TestTheBatchPlanStampsEveryLineAndALinesOwnPlanWins(t *testing.T) {
	r := throwawayRepo(t)
	path := writeBatch(t, r.root, "plans.jsonl",
		entryLine(t, map[string]any{"kind": "decision", "title": "Takes the batch's plan", "about": []string{"auth"}}),
		entryLine(t, map[string]any{"kind": "decision", "title": "Names its own", "about": []string{"auth"}, "plan": "other-tree"}),
		entryLine(t, map[string]any{"kind": "decision", "title": "A plan that is not a slug", "about": []string{"auth"}, "plan": "Not A Slug"}),
	)
	ref := batchRefused(t, run(t, r, "add", "--batch", path))
	if len(ref.Problems) != 1 || ref.Problems[0].Line != 3 || ref.Problems[0].Field != "plan" {
		t.Fatalf("a plan that is not kebab-case refuses the batch on its own line, got %+v", ref.Problems)
	}

	path = writeBatch(t, r.root, "plans-ok.jsonl",
		entryLine(t, map[string]any{"kind": "decision", "title": "Takes the batch's plan", "about": []string{"auth"}}),
		entryLine(t, map[string]any{"kind": "decision", "title": "Names its own", "about": []string{"auth"}, "plan": "other-tree"}),
	)
	out := batchResult(t, run(t, r, "add", "--batch", path))
	if got := entryByID(t, r, out.Records[0].ID).Origin.Plan; got != "demo" {
		t.Errorf("the batch's --plan stamps a line that names none, got %q", got)
	}
	if got := entryByID(t, r, out.Records[1].ID).Origin.Plan; got != "other-tree" {
		t.Errorf("a line's own plan wins, got %q", got)
	}
}

// The two spellings of a list are one thing, because the caller translating a
// command line into a batch line should not have to remember which the format
// prefers. Commas expand for `about` and not for a path.
func TestABatchLineTakesEitherSpellingOfAList(t *testing.T) {
	r := throwawayRepo(t)
	path := writeBatch(t, r.root, "lists.jsonl",
		entryLine(t, map[string]any{"kind": "decision", "title": "An array", "about": []string{"auth", "pkg/auth/"}}),
		entryLine(t, map[string]any{"kind": "decision", "title": "One comma-separated string", "about": "auth,pkg/auth/"}),
	)
	out := batchResult(t, run(t, r, "add", "--batch", path))
	first := entryByID(t, r, out.Records[0].ID)
	second := entryByID(t, r, out.Records[1].ID)
	if strings.Join(first.About, "|") != strings.Join(second.About, "|") {
		t.Fatalf("both spellings must produce the same tags: %v vs %v", first.About, second.About)
	}
	if len(first.About) != 2 {
		t.Fatalf("two tags, got %v", first.About)
	}
}

// An anchored kind is refused in a batch exactly as it is on the command line,
// and an anchor a line does carry is resolved against the working tree.
func TestAGotchaInABatchStillNeedsAnAnchorThatResolves(t *testing.T) {
	r := throwawayRepo(t)
	if err := os.WriteFile(filepath.Join(r.root, "token.go"), []byte("package auth\n\nfunc Sign() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	ref := batchRefused(t, runStdin(t, r, entryLine(t, map[string]any{
		"kind": "gotcha", "title": "No anchor", "about": []string{"auth"},
	})+"\n", "add", "--plan", "demo", "--batch", "-"))
	if len(ref.Problems) != 1 || ref.Problems[0].Field != "anchors" {
		t.Fatalf("an unanchored gotcha is refused on the anchors field, got %+v", ref.Problems)
	}

	ref = batchRefused(t, runStdin(t, r, entryLine(t, map[string]any{
		"kind": "gotcha", "title": "An anchor naming nothing", "about": []string{"auth"},
		"anchor": []string{"token.go:99"},
	})+"\n", "add", "--plan", "demo", "--batch", "-"))
	if len(ref.Problems) != 1 || ref.Problems[0].Field != "anchor" {
		t.Fatalf("an anchor past the end of the file is refused, got %+v", ref.Problems)
	}

	out := batchResult(t, runStdin(t, r, entryLine(t, map[string]any{
		"kind": "gotcha", "title": "An anchor that resolves", "about": []string{"auth"},
		"anchor": "token.go:1-3",
	})+"\n", "add", "--plan", "demo", "--batch", "-"))
	if got := entryByID(t, r, out.Records[0].ID).Anchors; len(got) != 1 || got[0].Lines != 3 {
		t.Fatalf("the anchor should have been resolved against the working tree, got %+v", got)
	}
}

// ----------------------------------------------------------------- edges ---

// One endpoint named by an id already in the log, the other end being this
// line's own entry, on whichever side the line left out. Both directions, because
// the two callers this exists for need one each: `plan` points a new child *at*
// its parent, and `record` points an existing objective *at* the closing record.
func TestABatchLineDeclaresEdgesOnEitherSide(t *testing.T) {
	r := throwawayRepo(t)
	parent := addKind(t, r, "objective", "The parent objective")

	out := batchResult(t, runStdin(t, r, entryLine(t, map[string]any{
		"kind": "objective", "title": "The child objective", "about": []string{"auth"},
		"edges": []map[string]any{{"type": "derives_from", "to": parent}},
	})+"\n", "add", "--plan", "demo", "--batch", "-"))
	if out.Edges != 1 {
		t.Fatalf("the edge is written with the entry, got %d edges", out.Edges)
	}
	child := out.Records[0].ID
	if from, to := edgeEnds(t, r, "derives_from"); from != child || to != parent {
		t.Fatalf("`to` names the existing endpoint and the line's entry is the source: got %s -> %s", from, to)
	}

	// The other direction: the new record is the *target*, and the existing
	// objective is the source.
	out = batchResult(t, runStdin(t, r, entryLine(t, map[string]any{
		"kind": "review-finding", "title": "What verification established", "about": []string{"auth"},
		"edges": []map[string]any{{"type": "met", "from": child}},
	})+"\n", "add", "--plan", "demo", "--batch", "-"))
	closing := out.Records[0].ID
	if from, to := edgeEnds(t, r, "met"); from != child || to != closing {
		t.Fatalf("`from` names the existing endpoint and the line's entry is the target: got %s -> %s", from, to)
	}
	if got := objectiveState(t, r, child); got != "met" {
		t.Fatalf("the state edge a batch wrote is the state the tree reports, got %q", got)
	}
}

// The three log-wide checks `link` applies, applied here. A second write path for
// edges that skipped them would be a hole in all three — and each refusal has to
// leave the log where it was, since the entries the batch would have written are
// as unrecoverable as the edge.
func TestABatchEdgeMeetsTheSameThreeChecksLinkApplies(t *testing.T) {
	r := throwawayRepo(t)
	parentA := addKind(t, r, "objective", "Parent A")
	parentB := addKind(t, r, "objective", "Parent B")
	assumption := addKind(t, r, "assumption", "Something assumed")
	before := logText(t, r, r.log)

	cases := []struct {
		name string
		code string
		line string
	}{
		{
			name: "an id absent from the log",
			code: "dangling_edge",
			line: entryLine(t, map[string]any{
				"kind": "decision", "title": "Points at nothing", "about": []string{"auth"},
				"edges": []map[string]any{{"type": "relates_to", "to": "01ZZZZZZZZZZZZZZZZZZZZZZZZ"}},
			}),
		},
		{
			name: "a second parent for one objective",
			code: "not_a_tree",
			line: entryLine(t, map[string]any{
				"kind": "objective", "title": "A child with two parents", "about": []string{"auth"},
				"edges": []map[string]any{
					{"type": "derives_from", "to": parentA},
					{"type": "derives_from", "to": parentB},
				},
			}),
		},
		{
			name: "a state edge nothing justifies",
			code: "unjustified_state",
			line: entryLine(t, map[string]any{
				"kind": "assumption", "title": "Cannot justify a met", "about": []string{"auth"},
				"edges": []map[string]any{{"type": "met", "from": parentA}},
			}),
		},
		{
			name: "a state edge pointing at a kind that cannot justify it",
			code: "unjustified_state",
			line: entryLine(t, map[string]any{
				"kind": "decision", "title": "Met by an assumption", "about": []string{"auth"},
				"edges": []map[string]any{{"type": "met", "to": assumption}},
			}),
		},
	}

	for _, c := range cases {
		res := runStdin(t, r, c.line+"\n", "add", "--plan", "demo", "--batch", "-")
		if res.code != 1 {
			t.Fatalf("%s: exit 1 expected, got %d\n%s", c.name, res.code, res.stdout)
		}
		code, msg := errorCode(t, res)
		if code != c.code {
			t.Errorf("%s: code %q expected, got %q (%s)", c.name, c.code, code, msg)
		}
		if !strings.Contains(msg, "line 1") {
			t.Errorf("%s: the refusal must name the line to fix, got %q", c.name, msg)
		}
		if got := logText(t, r, r.log); got != before {
			t.Fatalf("%s: the log must be byte-identical after a refusal", c.name)
		}
	}
}

// A refusal may only quote an id a reader can look up. The tree check states its
// refusal in ids, and one of the two is the id this run stamped for an entry it is
// now not going to write — so a message carrying it sends the reader looking through
// the log for a record that does not exist and never will.
func TestARefusalNamesTheLineRatherThanAnIDThatWasNeverWritten(t *testing.T) {
	r := throwawayRepo(t)
	parentA := addKind(t, r, "objective", "Parent A")
	parentB := addKind(t, r, "objective", "Parent B")

	res := runStdin(t, r, entryLine(t, map[string]any{
		"kind": "objective", "title": "A child with two parents", "about": []string{"auth"},
		"edges": []map[string]any{
			{"type": "derives_from", "to": parentA},
			{"type": "derives_from", "to": parentB},
		},
	})+"\n", "add", "--plan", "demo", "--batch", "-")

	code, msg := errorCode(t, res)
	if res.code != 1 || code != "not_a_tree" {
		t.Fatalf("expected not_a_tree at exit 1, got exit %d code %q", res.code, code)
	}
	if !strings.Contains(msg, "the record on line 1") {
		t.Errorf("the refusal should name the line the entry came from, got %q", msg)
	}
	// Every id left in the message has to be one that is actually on disk.
	onDisk := map[string]bool{}
	for _, raw := range lines(t, r, r.log) {
		var e struct {
			ID string `json:"id"`
		}
		if err := json.Unmarshal([]byte(raw), &e); err == nil && e.ID != "" {
			onDisk[e.ID] = true
		}
	}
	for _, id := range ulidRE.FindAllString(msg, -1) {
		if !onDisk[id] {
			t.Errorf("the refusal quotes %s, which is in no line of the log: %q", id, msg)
		}
	}
}

// ulidRE finds an id in a message. Deliberately not anchored to the whole string:
// what it is for is spotting one quoted inside prose.
var ulidRE = regexp.MustCompile(`\b[0-9A-HJKMNP-TV-Z]{26}\b`)

// An edge in a batch names exactly one endpoint, because the other one is the
// entry on the line and has no id until the tool stamps it. Both and neither are
// refused, and neither refusal is about the log.
func TestAnEdgeInABatchNamesExactlyOneEndpoint(t *testing.T) {
	r := throwawayRepo(t)
	other := add(t, r, "Something to point at")

	for _, edge := range []map[string]any{
		{"type": "relates_to"},
		{"type": "relates_to", "from": other, "to": other},
	} {
		line := entryLine(t, map[string]any{
			"kind": "decision", "title": "One endpoint only", "about": []string{"auth"},
			"edges": []map[string]any{edge},
		})
		ref := batchRefused(t, runStdin(t, r, line+"\n", "add", "--plan", "demo", "--batch", "-"))
		if len(ref.Problems) == 0 || ref.Problems[0].Field != "edges[0]" {
			t.Fatalf("edge %v must be refused on its own field, got %+v", edge, ref.Problems)
		}
	}
}

// A key whose value is the wrong shape is refused by name. Two keys take the
// string-or-array form, and before this was pinned they refused identically: no
// field, one message, and `raw` truncated at 120 characters — so on a line carrying
// a multi-paragraph body the caller was told to fix something the refusal could not
// name.
func TestAValueOfTheWrongShapeIsRefusedByItsKey(t *testing.T) {
	r := throwawayRepo(t)
	longBody := strings.Repeat("A paragraph that pushes the offending key past where raw is truncated. ", 6)

	for _, key := range []string{"about", "anchor"} {
		fields := map[string]any{"kind": "decision", "title": "t", "body": longBody, "about": []string{"auth"}}
		fields[key] = 5
		ref := batchRefused(t, runStdin(t, r, entryLine(t, fields)+"\n", "add", "--plan", "demo", "--batch", "-"))
		if len(ref.Problems) != 1 {
			t.Fatalf("%s: one problem expected, got %+v", key, ref.Problems)
		}
		p := ref.Problems[0]
		if p.Field != key {
			t.Errorf("%s: the problem must carry the key as its field, got %q", key, p.Field)
		}
		if !strings.Contains(p.Message, "string or an array of strings") || !strings.Contains(p.Message, "number") {
			t.Errorf("%s: the message must say what was wanted and what was there, got %q", key, p.Message)
		}
	}

	// An array is the shape that was asked for, so a bad element gets its own
	// sentence rather than being told it is not an array.
	ref := batchRefused(t, runStdin(t, r, entryLine(t, map[string]any{
		"kind": "decision", "title": "t", "about": []any{"auth", 5},
	})+"\n", "add", "--plan", "demo", "--batch", "-"))
	if len(ref.Problems) != 1 || ref.Problems[0].Field != "about" ||
		!strings.Contains(ref.Problems[0].Message, "every element has to be a string") {
		t.Fatalf("an array with a non-string element should say so, got %+v", ref.Problems)
	}
}

// ------------------------------------------------------------ the refusals ---

// --batch and the flags that describe one record are refused together rather
// than one of them silently winning.
func TestBatchAndTheRecordFlagsAreRefusedTogether(t *testing.T) {
	r := throwawayRepo(t)
	path := writeBatch(t, r.root, "one.jsonl",
		entryLine(t, map[string]any{"kind": "decision", "title": "In the file", "about": []string{"auth"}}))
	res := run(t, r, "add", "--batch", path, "--title", "On the command line")
	code, msg := errorCode(t, res)
	if res.code != 1 || code != "batch_and_record_flags" {
		t.Fatalf("expected a refusal naming both ways of saying it, got exit %d code %q", res.code, code)
	}
	if !strings.Contains(msg, "--title") {
		t.Errorf("the refusal should name the flag that was also passed, got %q", msg)
	}
	if got := strings.TrimSpace(logText(t, r, r.log)); got != "" {
		t.Fatalf("nothing may be written, log holds %q", got)
	}
}

// A batch that writes nothing is refused rather than reported as a run that did
// nothing: at exit 0 it is indistinguishable from a file whose lines went
// somewhere else.
func TestAnEmptyBatchIsRefused(t *testing.T) {
	r := throwawayRepo(t)
	res := run(t, r, "add", "--batch", writeBatch(t, r.root, "empty.jsonl", "", "   "))
	code, _ := errorCode(t, res)
	if res.code != 1 || code != "empty_batch" {
		t.Fatalf("expected empty_batch, got exit %d code %q\n%s", res.code, code, res.stdout)
	}
}

// A path that cannot be read is a refusal that names it, not a stack trace and
// not an empty batch.
func TestABatchPathThatCannotBeReadIsRefused(t *testing.T) {
	r := throwawayRepo(t)
	res := run(t, r, "add", "--batch", filepath.Join(r.root, "absent.jsonl"))
	code, msg := errorCode(t, res)
	if res.code != 1 || code != "batch_unreadable" {
		t.Fatalf("expected batch_unreadable, got exit %d code %q", res.code, code)
	}
	if !strings.Contains(msg, "absent.jsonl") {
		t.Errorf("the refusal should name the path, got %q", msg)
	}
}

// ------------------------------------------------------------- the guard ---

// The shape that started this: a shell handed one argument where the command line
// meant two, and Go's flag package reported an undefined flag named `kind
// decision` — which reads like a bug in the tool.
func TestAnArgumentInFlagPositionWithASpaceIsRefusedBeforeParsing(t *testing.T) {
	r := throwawayRepo(t)
	add(t, r, "Something already in the log")
	before := logText(t, r, r.log)

	res := run(t, r, "add", "--kind decision", "--title", "x", "--about", "y")
	code, msg := errorCode(t, res)
	if res.code != 1 || code != "unsplit_argument" {
		t.Fatalf("expected unsplit_argument at exit 1, got exit %d code %q\n%s%s",
			res.code, code, res.stdout, res.stderr)
	}
	if !strings.Contains(msg, `"--kind decision"`) {
		t.Errorf("the refusal must quote the argument, got %q", msg)
	}
	if !strings.Contains(msg, "unquoted variable") {
		t.Errorf("the refusal must ask about the unquoted variable, got %q", msg)
	}
	if got := logText(t, r, r.log); got != before {
		t.Fatal("nothing may be written")
	}

	// Every subcommand, because the guard is the one thing all of them call. The
	// arguments go through untouched: `kinds` binds no --dir, and a flag it would
	// refuse anyway would leave this proving nothing.
	for _, sub := range []string{
		"init", "add", "link", "query", "search", "areas", "anchors",
		"graph", "state", "promote", "validate", "kinds",
	} {
		res := execMemory(t, r, "", []string{sub, "--kind decision"})
		code, _ := errorCode(t, res)
		if res.code != 1 || code != "unsplit_argument" {
			t.Errorf("%s: expected unsplit_argument at exit 1, got exit %d code %q", sub, res.code, code)
		}
	}
}

// The guard never looks at a flag's value, which is what stops it from refusing a
// body this repository's own log contains.
func TestTheGuardDoesNotFireOnAFlagsValue(t *testing.T) {
	r := throwawayRepo(t)
	const body = "--about persistence is a legitimate body"

	res := run(t, r, "add", "--kind", "decision", "--title", "A body that looks like a flag",
		"--about", "auth", "--body", body)
	if res.code != 0 {
		t.Fatalf("a body may say anything: exit %d\n%s%s", res.code, res.stdout, res.stderr)
	}
	if got := entryByID(t, r, recordID(t, res)).Body; got != body {
		t.Fatalf("the body must be written intact, got %q", got)
	}

	// A search text that does not begin with `-` is not in flag position by the time
	// it is read, and may hold a space like any other text.
	res = run(t, r, "search", "legitimate body")
	if res.code != 0 {
		t.Fatalf("search text may hold a space: exit %d\n%s%s", res.code, res.stdout, res.stderr)
	}
}

// The one place the guard reaches something that is not a flag: `search`'s text is
// the first argument, so it *is* in flag position, and a text beginning with `-` and
// holding a space is refused. That is the honest limit of "a value may say
// anything", and it is narrow — such a text was never usable, because the flag
// package reads it as a flag and refuses it as undefined.
func TestASearchTextThatLooksLikeAFlagIsRefused(t *testing.T) {
	r := throwawayRepo(t)
	add(t, r, "Something to find")

	res := run(t, r, "search", "--about auth")
	code, msg := errorCode(t, res)
	if res.code != 1 || code != "unsplit_argument" {
		t.Fatalf("expected unsplit_argument at exit 1, got exit %d code %q\n%s%s",
			res.code, code, res.stdout, res.stderr)
	}
	if !strings.Contains(msg, `"--about auth"`) {
		t.Errorf("the refusal must quote the argument, got %q", msg)
	}

	// And the shape it was already refusing before the guard existed, so the two
	// stay distinguishable: no space, so the guard passes it through, and the flag
	// package refuses it as an undefined flag with its own exit code.
	res = run(t, r, "search", "-about")
	if res.code != 2 || !strings.Contains(res.stderr, "not defined") {
		t.Errorf("a text beginning with - was never usable: exit %d\n%s%s", res.code, res.stdout, res.stderr)
	}
}

// The origin every record carries is read out of git twice — `remote get-url origin`
// and `config user.name` — and firstNonEmpty evaluates every argument, so a set
// AI_KIT_AUTHOR does not skip the call. Building it per line made a batch spawn two
// processes per record: 20 for ten lines, 400 for two hundred, and a 200-line batch
// spent 2.3s almost entirely waiting on them.
//
// The claim pinned here is that one run's git work does not grow with the number of
// lines in it. Measured through the built binary with a counting wrapper ahead of
// the real git on PATH, which is what makes this a measurement rather than a reading
// of the code.
func TestABatchDoesNotShellOutOncePerLine(t *testing.T) {
	r := throwawayRepo(t)

	binDir := t.TempDir()
	counter := filepath.Join(binDir, "calls.txt")
	script := "#!/bin/sh\necho \"$@\" >> " + counter + "\nexit 1\n"
	if err := os.WriteFile(filepath.Join(binDir, "git"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	pathEnv := "PATH=" + binDir + string(os.PathListSeparator) + os.Getenv("PATH")

	spawns := func(lines int) int {
		if err := os.WriteFile(counter, nil, 0o644); err != nil {
			t.Fatal(err)
		}
		batch := make([]string, 0, lines)
		for i := 0; i < lines; i++ {
			batch = append(batch, entryLine(t, map[string]any{
				"kind": "decision", "title": fmt.Sprintf("Record number %d", i), "about": []string{"auth"},
			}))
		}
		res := execMemory(t, r, strings.Join(batch, "\n")+"\n",
			[]string{"add", "--dir", r.root, "--plan", "demo", "--batch", "-"}, pathEnv)
		if res.code != 0 {
			t.Fatalf("%d-line batch: exit %d\n%s%s", lines, res.code, res.stdout, res.stderr)
		}
		raw, err := os.ReadFile(counter)
		if err != nil {
			t.Fatal(err)
		}
		count := 0
		for _, call := range strings.Split(string(raw), "\n") {
			if strings.TrimSpace(call) != "" {
				count++
			}
		}
		return count
	}

	one, ten := spawns(1), spawns(10)
	if ten != one {
		t.Fatalf("a batch's git work must not grow with its length: 1 line spawned %d, 10 lines spawned %d "+
			"(per-line would be %d)", one, ten, one*10)
	}
}

// ---------------------------------------------------------------- helpers ---

// memoryCurrentVersion is the version the tool stamps, read from the package the
// binary is built out of rather than written as a number here.
const memoryCurrentVersion = 3

type diskEntry struct {
	ID      string   `json:"id"`
	Kind    string   `json:"kind"`
	Slug    string   `json:"slug"`
	Scope   string   `json:"scope"`
	At      string   `json:"at"`
	Version int      `json:"version"`
	About   []string `json:"about"`
	Title   string   `json:"title"`
	Body    string   `json:"body"`
	Anchors []struct {
		Path  string `json:"path"`
		Line  int    `json:"line"`
		Lines int    `json:"lines"`
		Hash  string `json:"hash"`
	} `json:"anchors"`
	Origin struct {
		Plan string `json:"plan"`
	} `json:"origin"`
}

// entryByID reads one entry back out of the log's bytes. Out of the file rather
// than out of a query, because what a batch has to be believed about is what it
// appended.
func entryByID(t *testing.T, r repo, id string) diskEntry {
	t.Helper()
	for _, raw := range lines(t, r, r.log) {
		var e diskEntry
		if err := json.Unmarshal([]byte(raw), &e); err != nil {
			continue
		}
		if e.ID == id {
			return e
		}
	}
	t.Fatalf("no entry with id %s in the log", id)
	return diskEntry{}
}

// edgeEnds returns the endpoints of the last edge of a type in the log.
func edgeEnds(t *testing.T, r repo, typ string) (from, to string) {
	t.Helper()
	var e struct {
		Kind string `json:"kind"`
		Type string `json:"type"`
		From string `json:"from"`
		To   string `json:"to"`
	}
	for _, raw := range lines(t, r, r.log) {
		var got = e
		if err := json.Unmarshal([]byte(raw), &got); err != nil {
			continue
		}
		if got.Kind == "edge" && got.Type == typ {
			from, to = got.From, got.To
		}
	}
	if from == "" {
		t.Fatalf("no %s edge in the log", typ)
	}
	return from, to
}

func objectiveState(t *testing.T, r repo, id string) string {
	t.Helper()
	res := run(t, r, "state", "--id", id)
	if res.code != 0 {
		t.Fatalf("state --id %s: exit %d\n%s%s", id, res.code, res.stdout, res.stderr)
	}
	var out struct {
		Objectives []struct {
			State string `json:"state"`
		} `json:"objectives"`
	}
	if err := json.Unmarshal([]byte(res.stdout), &out); err != nil {
		t.Fatal(err)
	}
	if len(out.Objectives) != 1 {
		t.Fatalf("one objective expected, got %d", len(out.Objectives))
	}
	return out.Objectives[0].State
}

func TestABatchEdgeReachesAnEndpointInAnotherLogAndRefusesWhatLinkRefuses(t *testing.T) {
	_, alpha, beta := twoDepths(t)
	parent := addObjective(t, alpha, "The objective the whole plan hangs from")

	out := batchResult(t, runStdin(t, beta, entryLine(t, map[string]any{
		"kind": "objective", "title": "The objective in the service next door", "about": []string{"auth"},
		"edges": []map[string]any{{"type": "derives_from", "to": parent}},
	})+"\n", "add", "--plan", "demo", "--batch", "-"))
	if out.Count != 1 || out.Edges != 1 {
		t.Fatalf("both endpoints are in the store, one log apart: %d entries and %d edges", out.Count, out.Edges)
	}
	kid := out.Records[0].ID
	if from, to := edgeEnds(t, beta, "derives_from"); from != kid || to != parent {
		t.Fatalf("the edge belongs to the log the batch was written into: got %s -> %s", from, to)
	}
	if other := logText(t, alpha, alpha.log); strings.Contains(other, `"kind":"edge"`) {
		t.Errorf("the other repository's log was written to:\n%s", other)
	}

	const ghost = "01ARZ3NDEKTSV4RRFFQ69G5FAV"
	res := runStdin(t, beta, entryLine(t, map[string]any{
		"kind": "decision", "title": "Points at nothing", "about": []string{"auth"},
		"edges": []map[string]any{{"type": "relates_to", "to": ghost}},
	})+"\n", "add", "--plan", "demo", "--batch", "-")
	if res.code != 1 {
		t.Fatalf("an id no log of the store holds is still dangling, got exit %d\n%s", res.code, res.stdout)
	}
	code, msg := errorCode(t, res)
	if code != "dangling_edge" {
		t.Fatalf("the refusal keeps the code it has always had, got %q: %s", code, msg)
	}
	names := "no entry in any log of " + beta.store + " for to=" + ghost
	if !strings.Contains(msg, names) {
		t.Errorf("the refusal names the store rather than one log's path, want %q in %q", names, msg)
	}
	_, linked := errorCode(t, link(t, beta, "relates_to", parent, ghost))
	if !strings.Contains(linked, names) {
		t.Errorf("`link` refuses the same edge naming the same thing, want %q in %q", names, linked)
	}
}
