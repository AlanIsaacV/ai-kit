package memory

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// testHash is a well-formed digest for the fixtures that only need the *shape*
// of one. Nothing here hashes to it; the tests that care about the content hash
// a real file.
var testHash = "sha256:" + strings.Repeat("0", 64)

func testAnchors() []Anchor {
	return []Anchor{{Path: "pkg/auth/token.go", Line: 4, Lines: 2, Hash: testHash}}
}

// asGotcha is the fixture mutation for the one kind that owes an anchor.
func asGotcha(e *Entry) {
	e.Kind = KindGotcha
	e.Anchors = testAnchors()
}

// tokenSource writes the fixture the checks below read, and returns the root to
// point them at. A real file, because what is being tested is what happens when
// the code moves under a record — which is not observable against a slice of
// strings a test made up.
func tokenSource(t *testing.T, lines ...string) string {
	t.Helper()
	root := t.TempDir()
	if len(lines) == 0 {
		lines = []string{
			"package auth",
			"",
			"func rotate(k *Key) {",
			"\tk.mu.Lock()",
			"\tdefer k.mu.Unlock()",
			"\trotateKey(k)",
			"}",
		}
	}
	writeSource(t, root, "pkg/auth/token.go", lines...)
	return root
}

func writeSource(t *testing.T, root, rel string, lines ...string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func mustAnchor(t *testing.T, root, spec string) Anchor {
	t.Helper()
	path, line, height, err := ParseAnchorSpec(spec)
	if err != nil {
		t.Fatal(err)
	}
	a, err := NewAnchor(root, path, line, height)
	if err != nil {
		t.Fatal(err)
	}
	return a
}

func TestParseAnchorSpecReadsBothCitationForms(t *testing.T) {
	cases := map[string]struct {
		path   string
		line   int
		height int
	}{
		"pkg/auth/token.go:4":        {"pkg/auth/token.go", 4, 1},
		"pkg/auth/token.go:4-5":      {"pkg/auth/token.go", 4, 2},
		"pkg/auth/token.go:12-12":    {"pkg/auth/token.go", 12, 1},
		"  pkg/auth/token.go:1-100 ": {"pkg/auth/token.go", 1, 100},
	}
	for spec, want := range cases {
		path, line, height, err := ParseAnchorSpec(spec)
		if err != nil {
			t.Errorf("%q: %v", spec, err)
			continue
		}
		if path != want.path || line != want.line || height != want.height {
			t.Errorf("%q: got %s line %d over %d, want %s line %d over %d",
				spec, path, line, height, want.path, want.line, want.height)
		}
	}
	for _, spec := range []string{"pkg/auth/token.go", "pkg/auth/token.go:", ":4",
		"pkg/auth/token.go:x", "pkg/auth/token.go:5-4", "pkg/auth/token.go:0"} {
		if _, _, _, err := ParseAnchorSpec(spec); err == nil {
			t.Errorf("%q must be refused", spec)
		}
	}
}

// The no-normalization rule, stated as a test because it is the design decision
// most likely to be softened by somebody who reads a false `changed` as a bug.
// Whitespace, case and indentation all move the hash. A reformat that changes no
// behaviour therefore reports `changed`, and that is the trade taken on purpose:
// the tolerance that would suppress it also suppresses the rewrites this exists
// to catch, and a false `still_holds` is worse than having no record at all.
func TestHashLinesFoldsNothing(t *testing.T) {
	base := HashLines([]string{"\tk.mu.Lock()"})
	for name, variant := range map[string][]string{
		"reindented": {"    k.mu.Lock()"},
		"trimmed":    {"k.mu.Lock()"},
		"recased":    {"\tk.MU.Lock()"},
		"trailing":   {"\tk.mu.Lock() "},
	} {
		if HashLines(variant) == base {
			t.Errorf("%s must not hash the same: normalization is what this check does not do", name)
		}
	}
	if HashLines([]string{"\tk.mu.Lock()", "\tdefer k.mu.Unlock()"}) == base {
		t.Error("a taller citation is a different citation")
	}
}

func TestSplitLinesNumbersTheWayAnEditorDoes(t *testing.T) {
	cases := map[string]int{
		"":            0,
		"\n":          1,
		"a\n":         1,
		"a":           1,
		"a\nb\n":      2,
		"a\nb":        2,
		"a\n\nb\n":    3,
		"a\nb\n\n":    3,
		"\r\na\r\n":   2,
		"package a\n": 1,
	}
	for content, want := range cases {
		if got := len(SplitLines([]byte(content))); got != want {
			t.Errorf("%q: got %d lines, want %d", content, got, want)
		}
	}
	if got := SplitLines([]byte("a\r\nb\r\n")); got[0] != "a\r" {
		t.Errorf("a carriage return is part of the line, not stripped: %q", got[0])
	}
}

// The write path refuses what it could not read. An anchor built from a path
// that is not there, or a range past the end of the file, is a typo at the moment
// of writing — and the log is append-only, so this is the only place it can be
// caught.
func TestNewAnchorRefusesWhatItCannotRead(t *testing.T) {
	root := tokenSource(t)

	a := mustAnchor(t, root, "pkg/auth/token.go:4-5")
	if a.Path != "pkg/auth/token.go" || a.Line != 4 || a.Lines != 2 {
		t.Fatalf("the anchor records what was cited: %+v", a)
	}
	if want := HashLines([]string{"\tk.mu.Lock()", "\tdefer k.mu.Unlock()"}); a.Hash != want {
		t.Errorf("the hash covers the cited lines exactly: %s", a.Hash)
	}
	if ps := ValidateEntry(newTestEntry(t, func(e *Entry) { e.Kind = KindGotcha; e.Anchors = []Anchor{a} })); len(ps) != 0 {
		t.Errorf("what NewAnchor builds must pass the write gate: %v", ps)
	}

	for name, spec := range map[string]struct {
		path         string
		line, height int
	}{
		"no such file": {"pkg/auth/nope.go", 1, 1},
		"past the end": {"pkg/auth/token.go", 7, 3},
		"starts past":  {"pkg/auth/token.go", 99, 1},
		"line zero":    {"pkg/auth/token.go", 0, 1},
		"no height":    {"pkg/auth/token.go", 1, 0},
		"out of repo":  {"../elsewhere.go", 1, 1},
	} {
		if _, err := NewAnchor(root, spec.path, spec.line, spec.height); err == nil {
			t.Errorf("%s must be refused", name)
		}
	}
}

// A record's verdict is its worst anchor's, and the order is what says which
// news travels: a record with one rewritten anchor is one to look at whatever
// its others say, and one that is only partly checkable must not read as checked.
func TestARecordTakesItsWorstAnchorsVerdict(t *testing.T) {
	root := tokenSource(t)
	held := mustAnchor(t, root, "pkg/auth/token.go:4")
	unreadable := Anchor{Path: "pkg/auth/gone.go", Line: 1, Lines: 1, Hash: testHash}
	rewritten := Anchor{Path: "pkg/auth/token.go", Line: 1, Lines: 1, Hash: testHash}

	cases := map[string]struct {
		anchors []Anchor
		want    Verdict
	}{
		"all held":              {[]Anchor{held, held}, VerdictStillHolds},
		"one unreadable":        {[]Anchor{held, unreadable}, VerdictCouldNotCheck},
		"one rewritten":         {[]Anchor{held, rewritten}, VerdictChanged},
		"rewritten beats blind": {[]Anchor{unreadable, rewritten}, VerdictChanged},
		"none at all":           {nil, VerdictCouldNotCheck},
	}
	for name, tc := range cases {
		e := newTestEntry(t, func(e *Entry) { e.Anchors = tc.anchors })
		if got := CheckEntry(root, e).Verdict; got != tc.want {
			t.Errorf("%s: got %s, want %s", name, got, tc.want)
		}
	}
}

// A digest this binary cannot compute is a claim it cannot judge, and saying
// `changed` about one would be reporting a difference nobody measured.
func TestAnUnknownHashAlgorithmCannotBeChecked(t *testing.T) {
	root := tokenSource(t)
	a := mustAnchor(t, root, "pkg/auth/token.go:4")
	a.Hash = "blake3:" + strings.Repeat("ab", 32)

	got := CheckAnchor(root, a)
	if got.Verdict != VerdictCouldNotCheck {
		t.Fatalf("expected could_not_check, got %s", got.Verdict)
	}
	if !strings.Contains(got.Note, "blake3") {
		t.Errorf("the note must name what it met, got %q", got.Note)
	}
}

// Checking is read-only over the repository it is pointed at. Nothing here
// writes, and the file the check just read is byte for byte what it was.
func TestCheckingWritesNothing(t *testing.T) {
	root := tokenSource(t)
	path := filepath.Join(root, "pkg", "auth", "token.go")
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	a := mustAnchor(t, root, "pkg/auth/token.go:4-5")

	for _, anchor := range []Anchor{a, {Path: "pkg/auth/token.go", Line: 1, Lines: 1, Hash: testHash}} {
		CheckAnchor(root, anchor)
	}

	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Error("the check must not touch the file it read")
	}
}
