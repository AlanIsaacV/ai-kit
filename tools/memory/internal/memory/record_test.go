package memory

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

// A record written before the stamp existed has no `version` key on disk. It is
// version 1, and everything that prints it should say 1 — `"version": 0` names
// no schema that ever existed, and a reader who met it would go looking for
// version 0's rules.
//
// The other half of this is in validate_test.go: reading such a line leaves
// Version at zero, because zero is what the line says. Only the rendering
// resolves.
func TestAnUnstampedRecordRendersAsVersionOne(t *testing.T) {
	e := newTestEntry(t, func(e *Entry) { e.Version = 0 })

	for name, marshal := range map[string]func(any) ([]byte, error){
		"json.Marshal": json.Marshal,
		"MarshalLine":  MarshalLine,
	} {
		b, err := marshal(e)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		var got map[string]any
		if err := json.Unmarshal(b, &got); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if got["version"] != float64(SchemaVersion1) {
			t.Errorf("%s rendered version %v, want %d", name, got["version"], SchemaVersion1)
		}
		if !bytes.HasPrefix(b, []byte(`{"version":1,`)) {
			t.Errorf("%s moved the version off the front of the line: %s", name, b)
		}
		if got["title"] != e.Title || got["id"] != e.ID {
			t.Errorf("%s changed more than the version: %s", name, b)
		}
	}

	if e.Version != 0 {
		t.Error("rendering must not mutate the record: what the line said is still worth knowing")
	}
}

func TestAnUnstampedEdgeRendersAsVersionOne(t *testing.T) {
	a, _ := NewULID(time.Now())
	b, _ := NewULID(time.Now())
	edge := &Edge{Kind: KindEdge, Type: EdgeSupersedes, From: a, To: b,
		At: time.Now().UTC().Format(time.RFC3339)}

	line, err := MarshalLine(edge)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.HasPrefix(line, []byte(`{"version":1,`)) {
		t.Errorf("expected the edge to render as version 1, got %s", line)
	}
	if edge.Version != 0 {
		t.Error("rendering must not mutate the edge")
	}
}

// A stamped record renders the stamp it carries, including a version this
// binary has no rules for. Resolving is for the absent key, not for versions
// this binary happens not to like.
func TestARenderedVersionIsTheOneTheRecordDeclares(t *testing.T) {
	line, err := MarshalLine(newTestEntry(t, func(e *Entry) { e.Version = 99 }))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.HasPrefix(line, []byte(`{"version":99,`)) {
		t.Errorf("expected version 99 to survive rendering, got %s", line)
	}
}

// The marshaller must not reintroduce HTML escaping. Go escapes <, > and & by
// default, which would turn a body mentioning a tag or a URL into unicode
// escapes — and a json.Marshal call inside a MarshalJSON method escapes before
// the outer encoder ever sees the bytes, where its own setting can no longer
// undo it.
func TestRenderingLeavesAngleBracketsAlone(t *testing.T) {
	e := newTestEntry(t, func(e *Entry) { e.Body = "mentions a <tag> & an ampersand" })

	line, err := MarshalLine(e)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(line), "<tag> & an") {
		t.Errorf("the body was escaped on the way out: %s", line)
	}

	// The CLI's own encoder, which is what `query` prints through.
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(e); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "<tag> & an") {
		t.Errorf("the body was escaped on the way to stdout: %s", buf.String())
	}
}
