package memory

import (
	"strings"
	"testing"
)

func parse(t *testing.T, input string) ([]BatchLine, []Problem) {
	t.Helper()
	return ParseBatch(strings.NewReader(input))
}

// A blank line is skipped and still counted. The numbers a problem carries are
// the numbers an editor shows, or the caller is being sent to the wrong line.
func TestParseBatchNumbersLinesAsTheFileDoes(t *testing.T) {
	lines, ps := parse(t, `{"kind":"decision","title":"one","about":["auth"]}

{"kind":"decision","title":"two","about":["auth"]}

{"kind":"decision","title":"three","about":["auth"]}
`)
	if len(ps) != 0 {
		t.Fatalf("nothing here is wrong: %+v", ps)
	}
	got := []int{}
	for _, l := range lines {
		got = append(got, l.Num)
	}
	if len(got) != 3 || got[0] != 1 || got[1] != 3 || got[2] != 5 {
		t.Fatalf("expected lines 1, 3 and 5, got %v", got)
	}
}

// A key this format does not know is a refusal, where the log's read path
// deliberately ignores one. Both the typo and the field the tool stamps arrive as
// the same mistake, and the message names the field.
func TestParseBatchRefusesAKeyItDoesNotKnow(t *testing.T) {
	for _, key := range []string{"titel", "id", "at", "version", "scope", "origin"} {
		_, ps := parse(t, `{"kind":"decision","title":"t","about":["auth"],"`+key+`":"x"}`+"\n")
		if len(ps) != 1 {
			t.Fatalf("%q: one problem expected, got %+v", key, ps)
		}
		if !strings.Contains(ps[0].Message, key) {
			t.Errorf("%q: the message must name the field, got %q", key, ps[0].Message)
		}
		if ps[0].Line != 1 {
			t.Errorf("%q: the problem must name the line, got %d", key, ps[0].Line)
		}
	}
}

// One entry per line. A second JSON value on the line is a line that means two
// things, and the decoder would otherwise stop after the first and say nothing.
func TestParseBatchRefusesTwoValuesOnOneLine(t *testing.T) {
	_, ps := parse(t, `{"kind":"decision","title":"one","about":["auth"]} {"kind":"decision","title":"two","about":["auth"]}`+"\n")
	if len(ps) != 1 || !strings.Contains(ps[0].Message, "one entry per line") {
		t.Fatalf("expected one problem about the second value, got %+v", ps)
	}
}

// Either spelling of a list, because the caller is translating a command line
// where `--about` takes commas and `--anchor` repeats.
func TestParseBatchTakesAStringOrAnArray(t *testing.T) {
	lines, ps := parse(t, `{"kind":"decision","title":"t","about":"auth, pkg/auth/","anchor":"a.go:1"}
{"kind":"decision","title":"t","about":["auth","pkg/auth/"],"anchor":["a.go:1"]}
`)
	if len(ps) != 0 {
		t.Fatalf("both spellings are legal: %+v", ps)
	}
	for _, l := range lines {
		if strings.Join(l.Entry.About, "|") != "auth|pkg/auth/" {
			t.Errorf("line %d: about came out %v", l.Num, l.Entry.About)
		}
		if strings.Join(l.Entry.Anchor, "|") != "a.go:1" {
			t.Errorf("line %d: anchor came out %v", l.Num, l.Entry.Anchor)
		}
	}
}

// A value of the wrong shape is refused with the key on the problem, not only in
// the text. An UnmarshalJSON method cannot know which key it was reached through, so
// the refusal has to be the one error type the decoder fills a field in on — and
// without it `about` and `anchor` refuse identically, on a line whose Raw is
// truncated before either key is visible.
func TestParseBatchNamesTheKeyWhoseShapeIsWrong(t *testing.T) {
	for _, key := range []string{"about", "anchor"} {
		_, ps := parse(t, `{"kind":"decision","title":"t","`+key+`":5}`+"\n")
		if len(ps) != 1 {
			t.Fatalf("%s: one problem expected, got %+v", key, ps)
		}
		if ps[0].Field != key {
			t.Errorf("%s: the problem must carry the key as its field, got %q", key, ps[0].Field)
		}
		if !strings.Contains(ps[0].Message, "string or an array of strings") {
			t.Errorf("%s: the message must say what was wanted, got %q", key, ps[0].Message)
		}
		if !strings.Contains(ps[0].Message, "number") {
			t.Errorf("%s: the message must say what was there, got %q", key, ps[0].Message)
		}
	}

	// An array was one of the two shapes wanted, so a bad element is told what is
	// wrong with the array rather than that it is not one.
	_, ps := parse(t, `{"kind":"decision","title":"t","about":["auth",5]}`+"\n")
	if len(ps) != 1 || ps[0].Field != "about" ||
		!strings.Contains(ps[0].Message, "every element has to be a string") {
		t.Fatalf("expected one problem about the element, got %+v", ps)
	}
}

// A comma is a separator in a tag and not in a path: `--anchor` repeats precisely
// because a path is the one value here that could contain one.
func TestParseBatchDoesNotSplitAnAnchorOnAComma(t *testing.T) {
	lines, ps := parse(t, `{"kind":"gotcha","title":"t","about":["auth"],"anchor":"pkg/a,b.go:1-2"}`+"\n")
	if len(ps) != 0 {
		t.Fatalf("unexpected problems: %+v", ps)
	}
	if len(lines[0].Entry.Anchor) != 1 {
		t.Fatalf("a path holding a comma is one anchor, got %v", lines[0].Entry.Anchor)
	}
}

// An edge in a batch names exactly one endpoint. Neither means there is no edge
// to draw; both means there is nothing left for the entry on the line, which is
// the whole reason this shape can express an edge in either direction.
func TestParseBatchRefusesAnEdgeThatIsNotHalfNamed(t *testing.T) {
	for _, edge := range []string{
		`{"type":"relates_to"}`,
		`{"type":"relates_to","from":"01AAAAAAAAAAAAAAAAAAAAAAAA","to":"01BBBBBBBBBBBBBBBBBBBBBBBB"}`,
	} {
		_, ps := parse(t, `{"kind":"decision","title":"t","about":["auth"],"edges":[`+edge+`]}`+"\n")
		if len(ps) != 1 || ps[0].Field != "edges[0]" {
			t.Fatalf("%s: expected one problem on edges[0], got %+v", edge, ps)
		}
	}

	for _, edge := range []string{
		`{"type":"derives_from","to":"01AAAAAAAAAAAAAAAAAAAAAAAA"}`,
		`{"type":"met","from":"01AAAAAAAAAAAAAAAAAAAAAAAA"}`,
	} {
		_, ps := parse(t, `{"kind":"decision","title":"t","about":["auth"],"edges":[`+edge+`]}`+"\n")
		if len(ps) != 0 {
			t.Fatalf("%s: one endpoint is what this shape wants, got %+v", edge, ps)
		}
	}
}

// A line that is not JSON at all is one problem on one line, and the lines around
// it still parse — the same tolerance the log's own reader has, for the same
// reason: a caller fixing a batch wants every bad line at once.
func TestParseBatchReportsABadLineAndKeepsReading(t *testing.T) {
	lines, ps := parse(t, `{"kind":"decision","title":"one","about":["auth"]}
not json
{"kind":"decision","title":"three","about":["auth"]}
`)
	if len(lines) != 2 {
		t.Fatalf("the two good lines must still parse, got %d", len(lines))
	}
	if len(ps) != 1 || ps[0].Line != 2 {
		t.Fatalf("expected one problem on line 2, got %+v", ps)
	}
	if ps[0].Raw != "not json" {
		t.Errorf("the problem should carry the line it could not read, got %q", ps[0].Raw)
	}
}

// The fields a line may carry, all of them, arriving as what the CLI will stamp.
func TestParseBatchCarriesEveryFieldAddsFlagsCarry(t *testing.T) {
	lines, ps := parse(t, `{"kind":"gotcha","title":"The lock is taken first","body":"A deadlock in March.","slug":"lock-order","about":["auth","pkg/auth/"],"anchor":["pkg/auth/token.go:41-43"],"plan":"jwt-migration","edges":[{"type":"derives_from","to":"01AAAAAAAAAAAAAAAAAAAAAAAA"}]}`+"\n")
	if len(ps) != 0 {
		t.Fatalf("unexpected problems: %+v", ps)
	}
	e := lines[0].Entry
	switch {
	case e.Kind != KindGotcha:
		t.Errorf("kind came out %q", e.Kind)
	case e.Title != "The lock is taken first":
		t.Errorf("title came out %q", e.Title)
	case e.Body != "A deadlock in March.":
		t.Errorf("body came out %q", e.Body)
	case e.Slug != "lock-order":
		t.Errorf("slug came out %q", e.Slug)
	case e.Plan != "jwt-migration":
		t.Errorf("plan came out %q", e.Plan)
	case len(e.Edges) != 1 || e.Edges[0].Type != EdgeDerivesFrom:
		t.Errorf("edges came out %+v", e.Edges)
	}
}
