package memory

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// tempStore is a log at the path a repo's log actually lives at, one directory
// deep, so the "append creates its parent" case is still exercised.
func tempStore(t *testing.T) *Store {
	t.Helper()
	dir := t.TempDir()
	return OpenIn(filepath.Join(dir, "memory.db"), filepath.Join(dir, ".ai-kit", "memory.jsonl"))
}

// The load-bearing invariant of this whole design: once a line is on disk,
// nothing in this package may change it. Corrections are new lines and
// invalidation is a supersedes edge — if an append ever rewrote history, the
// log would stop being trustworthy as a record of what was actually decided.
func TestAppendNeverRewritesExistingLines(t *testing.T) {
	s := tempStore(t)

	first := newTestEntry(t, func(e *Entry) { e.Title = "first" })
	if err := s.Append(first); err != nil {
		t.Fatal(err)
	}
	before, err := s.Read()
	if err != nil {
		t.Fatal(err)
	}

	for i := 0; i < 5; i++ {
		if err := s.Append(newTestEntry(t, nil)); err != nil {
			t.Fatal(err)
		}
	}
	// A supersedes edge is how a record is invalidated; it must not touch the
	// superseded line either.
	later := newTestEntry(t, nil)
	if err := s.Append(later, &Edge{
		Kind: KindEdge, Type: EdgeSupersedes, From: later.ID, To: first.ID,
		At: time.Now().UTC().Format(time.RFC3339),
	}); err != nil {
		t.Fatal(err)
	}

	after, err := s.Read()
	if err != nil {
		t.Fatal(err)
	}
	if len(after) <= len(before) {
		t.Fatalf("the log held %d lines and should have grown, got %d", len(before), len(after))
	}
	for i, want := range before {
		if after[i].Num != want.Num || after[i].Raw != want.Raw {
			t.Fatalf("line %d was %d:%s and is now %d:%s: something rewrote history",
				i+1, want.Num, want.Raw, after[i].Num, after[i].Raw)
		}
	}
}

func TestAppendCreatesParentDirs(t *testing.T) {
	s := tempStore(t)
	if err := s.Append(newTestEntry(t, nil)); err != nil {
		t.Fatalf("append into a missing directory should just work: %v", err)
	}
	dir := filepath.Dir(s.Path)
	if _, err := os.Stat(dir); err != nil {
		t.Fatal(err)
	}
	ignore, err := os.ReadFile(filepath.Join(dir, ".gitignore"))
	if err != nil {
		t.Fatalf("the directory has to arrive with the .gitignore that keeps it out of git: %v", err)
	}
	if string(ignore) != gitignoreSentinel {
		t.Errorf("the .gitignore should hold %q, got %q", gitignoreSentinel, ignore)
	}
	if _, err := os.Stat(s.Path); err == nil {
		t.Errorf("the rows are the log now, yet %s was written", s.Path)
	} else if !os.IsNotExist(err) {
		t.Fatal(err)
	}
}

func TestInitIsIdempotentAndNeverTruncates(t *testing.T) {
	s := tempStore(t)

	created, err := s.Init()
	if err != nil || !created {
		t.Fatalf("first init should create: created=%v err=%v", created, err)
	}
	if err := s.Append(newTestEntry(t, nil)); err != nil {
		t.Fatal(err)
	}
	before, err := s.Read()
	if err != nil {
		t.Fatal(err)
	}
	if len(before) != 1 {
		t.Fatalf("expected the appended record, got %d lines", len(before))
	}

	created, err = s.Init()
	if err != nil {
		t.Fatal(err)
	}
	if created {
		t.Error("second init should report created=false")
	}
	after, err := s.Read()
	if err != nil {
		t.Fatal(err)
	}
	if len(after) != len(before) {
		t.Fatalf("init took the log from %d lines to %d", len(before), len(after))
	}
	for i, want := range before {
		if after[i].Num != want.Num || after[i].Raw != want.Raw {
			t.Fatal("init clobbered an existing log")
		}
	}
}

func TestReadMissingFileIsEmptyNotAnError(t *testing.T) {
	lines, err := tempStore(t).Read()
	if err != nil {
		t.Fatalf("a log that does not exist yet is a normal state, got %v", err)
	}
	if len(lines) != 0 {
		t.Fatalf("expected no lines, got %d", len(lines))
	}
}

func TestRoundTrip(t *testing.T) {
	s := tempStore(t)
	e := newTestEntry(t, func(x *Entry) {
		x.Body = "Multi word body with \"quotes\" and a <tag>."
		x.About = []string{"auth", "pkg/auth/"}
		x.Origin = &Origin{Repo: "orders", Plan: "demo", Author: "alan"}
	})
	if err := s.Append(e); err != nil {
		t.Fatal(err)
	}

	lines, err := s.Read()
	if err != nil {
		t.Fatal(err)
	}
	got := Entries(lines)
	if len(got) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(got))
	}
	if got[0].Body != e.Body || got[0].Origin.Author != "alan" || len(got[0].About) != 2 {
		t.Fatalf("round trip lost data: %+v", got[0])
	}
	// Go's JSON encoder escapes <, > and & by default, which turns a body
	// mentioning a tag or a URL into unicode escapes and makes the log
	// unpleasant to read by hand. The store turns that off; this guards it.
	if !strings.Contains(lines[0].Raw, "<tag>") {
		t.Errorf("body was HTML-escaped on the way to the store: %s", lines[0].Raw)
	}
}

// The stamp has to be visible in the stored line, not just in the struct. A
// version nobody can see when they read the line back is a version nobody
// checks — which is also why it is written without omitempty and lands first on
// the line.
func TestWrittenLinesCarryTheSchemaVersionFirst(t *testing.T) {
	s := tempStore(t)
	e := newTestEntry(t, nil)
	if err := s.Append(e, newTestEdge(t, EdgeRelatesTo, e.ID, e.ID)); err != nil {
		t.Fatal(err)
	}

	lines, err := s.Read()
	if err != nil {
		t.Fatal(err)
	}
	if len(lines) != 2 {
		t.Fatalf("expected the entry and its edge, got %d lines", len(lines))
	}
	// Built from the constant rather than written out, so the assertion follows
	// the tool through a version bump instead of having to be remembered.
	want := fmt.Sprintf(`{"version":%d,`, CurrentSchemaVersion)
	for _, l := range lines {
		if !strings.HasPrefix(l.Raw, want) {
			t.Errorf("line %d should open with %s got %s", l.Num, want, l.Raw)
		}
	}
}

func TestReadHandlesLongBodies(t *testing.T) {
	s := tempStore(t)
	big := strings.Repeat("x", 200_000)
	if err := s.Append(newTestEntry(t, func(e *Entry) { e.Body = big })); err != nil {
		t.Fatal(err)
	}
	lines, err := s.Read()
	if err != nil {
		t.Fatalf("a long body must not break the read: %v", err)
	}
	if len(Entries(lines)) != 1 || len(Entries(lines)[0].Body) != len(big) {
		t.Fatal("long body did not survive the round trip")
	}
}
