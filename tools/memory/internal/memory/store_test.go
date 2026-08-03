package memory

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func tempStore(t *testing.T) *Store {
	t.Helper()
	return Open(filepath.Join(t.TempDir(), "plans", "demo", "memory.jsonl"))
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
	before, err := os.ReadFile(s.Path)
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

	after, err := os.ReadFile(s.Path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(after), string(before)) {
		t.Fatal("the file no longer starts with its original bytes: something rewrote history")
	}
}

func TestAppendCreatesParentDirs(t *testing.T) {
	s := tempStore(t)
	if err := s.Append(newTestEntry(t, nil)); err != nil {
		t.Fatalf("append into a missing directory should just work: %v", err)
	}
	if _, err := os.Stat(s.Path); err != nil {
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
	before, _ := os.ReadFile(s.Path)

	created, err = s.Init()
	if err != nil {
		t.Fatal(err)
	}
	if created {
		t.Error("second init should report created=false")
	}
	after, _ := os.ReadFile(s.Path)
	if string(before) != string(after) {
		t.Fatal("init clobbered an existing log")
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
	raw, _ := os.ReadFile(s.Path)
	if !strings.Contains(string(raw), "<tag>") {
		t.Errorf("body was HTML-escaped on the way to disk: %s", raw)
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
		t.Fatalf("a long body must not break the scanner: %v", err)
	}
	if len(Entries(lines)) != 1 || len(Entries(lines)[0].Body) != len(big) {
		t.Fatal("long body did not survive the round trip")
	}
}
