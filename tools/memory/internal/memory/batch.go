package memory

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"reflect"
	"strings"
)

// A batch is what `add --batch` reads: one JSON object per line, each one an
// entry to append, carrying the edges that entry takes part in.
//
// It lives in its own file because it is an input format rather than part of the
// schema. Nothing here decides what a record may say — that is record.go and the
// version rule sets — and nothing here writes: a batch is parsed into the same
// Entry and Edge values a flagged write builds, and goes through the same gate.
//
// The shape a line may take is deliberately narrower than a record's shape on
// disk. A line carries the fields `add`'s flags carry and never the fields the
// tool stamps — id, at, version, scope, origin — because a caller who could
// stamp a version or an id could write a record that lies about itself, and the
// log cannot take one back.

// StringList is a JSON field that accepts either one string or an array of
// them, so a line can spell a single-valued case the way it reads best.
//
// It exists because the fields it covers are the ones whose command-line
// spelling is not a single token: `--about` takes a comma-separated list and
// `--anchor` repeats. A caller translating a command line into a batch line
// should not have to remember which of the two shapes this format prefers.
type StringList []string

// stringListType is what a refusal is recognised by on the way back out. Held in a
// variable because reflect.TypeOf on a zero value is the only way to name a type
// that a *json.UnmarshalTypeError can be compared against.
var stringListType = reflect.TypeOf(StringList(nil))

// UnmarshalJSON accepts one string or an array of them, and refuses anything else
// with an error the decoder can attach a field name to.
//
// **It has to be a *json.UnmarshalTypeError and not a plain error.** An
// UnmarshalJSON method is handed the value and nothing else — it cannot know which
// key it was reached through — while the decoder knows exactly, and fills Struct
// and Field in on an error of that one type as it unwinds. A plain error comes back
// with no field at all, so `{"about":5}` and `{"anchor":7}` refuse identically; and
// since Raw is truncated, on a line carrying a multi-paragraph body neither refusal
// shows the caller the key. The whole design of a batch rests on the refusal naming
// the line and the key to fix.
func (l *StringList) UnmarshalJSON(b []byte) error {
	var one string
	if err := json.Unmarshal(b, &one); err == nil {
		*l = StringList{one}
		return nil
	}
	var many []string
	if err := json.Unmarshal(b, &many); err != nil {
		return &json.UnmarshalTypeError{Value: jsonValueKind(b), Type: stringListType}
	}
	*l = many
	return nil
}

// jsonValueKind names the shape a line actually carried, in json's own vocabulary,
// so a refusal can say what was there as well as what was wanted. It reads the first
// byte rather than decoding again: whatever this is, it has already failed to be
// either of the two things wanted, and the only question left is what to call it.
func jsonValueKind(b []byte) string {
	b = bytes.TrimSpace(b)
	if len(b) == 0 {
		return "nothing"
	}
	switch b[0] {
	case '{':
		return "object"
	case '[':
		return "array"
	case '"':
		return "string"
	case 't', 'f':
		return "bool"
	case 'n':
		return "null"
	default:
		return "number"
	}
}

// BatchEdge is one edge a batch line declares.
//
// Exactly one of From and To carries an id, and that id must already be in the
// log. The other side is the line's own entry, which has no id until the tool
// stamps one — so which of the two keys is present is how the line says which
// end of the edge its entry is on.
//
// That is also why there is no way to draw an edge between two entries of the
// same batch: neither of them has an id a caller could write down.
type BatchEdge struct {
	Type EdgeType `json:"type"`
	From string   `json:"from,omitempty"`
	To   string   `json:"to,omitempty"`
}

// BatchEntry is one line of a batch: the fields `add`'s flags carry, and the
// edges this entry takes part in.
//
// Plan overrides the batch-wide `--plan` for this line alone. Absent means the
// batch's stamp applies, which is not the same claim as an empty plan — a record
// that belongs to the repository rather than to a tree is written by a batch that
// names no plan at all.
type BatchEntry struct {
	Kind   Kind        `json:"kind"`
	Title  string      `json:"title"`
	Body   string      `json:"body,omitempty"`
	Slug   string      `json:"slug,omitempty"`
	About  StringList  `json:"about,omitempty"`
	Anchor StringList  `json:"anchor,omitempty"`
	Plan   string      `json:"plan,omitempty"`
	Edges  []BatchEdge `json:"edges,omitempty"`
}

// BatchLine is one parsed line, with the line of the input it came from. The
// number is what every problem is reported against: a batch is refused whole, so
// the only useful thing a refusal can say is which line to go and fix.
type BatchLine struct {
	Num   int
	Entry BatchEntry
}

// ParseBatch reads a JSONL batch and returns every line it could parse together
// with every problem it found. Both, always: a caller that gets one bad line back
// still has to be told about the other four.
//
// A blank line is skipped and still counted, so the numbers in a problem are the
// numbers an editor shows.
func ParseBatch(r io.Reader) ([]BatchLine, []Problem) {
	out := []BatchLine{}
	ps := []Problem{}

	sc := bufio.NewScanner(r)
	// Bodies can be long, the same way they can be long in the log: the default
	// 64 KiB token limit is not enough. See Store.Read.
	sc.Buffer(make([]byte, 0, 64*1024), 8*1024*1024)

	num := 0
	for sc.Scan() {
		num++
		raw := strings.TrimSpace(sc.Text())
		if raw == "" {
			continue
		}

		var e BatchEntry
		dec := json.NewDecoder(strings.NewReader(raw))
		// A key this format does not know is a refusal here, where the read path
		// deliberately ignores one. The two answer different questions: a line on
		// disk carrying an unknown key was written by a newer binary and calling it
		// malformed would break forward compatibility, while a key in a batch is
		// something a caller just typed — and `titel` silently dropped is exactly
		// the typo the flag parser catches on the flagged path. It is also what
		// refuses `id`, `at`, `version`, `scope` and `origin`, by naming the field
		// rather than by a rule of their own.
		dec.DisallowUnknownFields()
		if err := dec.Decode(&e); err != nil {
			ps = append(ps, batchDecodeProblem(num, raw, err))
			continue
		}
		// One JSON object per line, so anything after the first is a line that means
		// two things. Decode stops at the end of the first value and would leave the
		// rest unread and unreported.
		if _, err := dec.Token(); err != io.EOF {
			ps = append(ps, Problem{Line: num,
				Message: "more than one JSON value on this line; a batch is one entry per line",
				Raw:     truncate(raw, 120)})
			continue
		}

		// `about` is where the command line's comma-separated spelling is honoured,
		// and `anchor` is where it is not: a path is the one value here that could
		// plausibly contain a comma, which is why --anchor repeats rather than
		// splitting. See anchorSpecs in the CLI.
		e.About = splitTags(e.About)

		for i, ed := range e.Edges {
			field := fmt.Sprintf("edges[%d]", i)
			switch {
			case ed.From == "" && ed.To == "":
				ps = append(ps, Problem{Line: num, Field: field,
					Message: "names no endpoint: an edge in a batch carries exactly one of \"from\" or \"to\", " +
						"an id already in the log, and the entry on this line is the other end"})
			case ed.From != "" && ed.To != "":
				ps = append(ps, Problem{Line: num, Field: field,
					Message: "names both endpoints, so nothing is left for the entry on this line: " +
						"carry exactly one of \"from\" or \"to\", and the side you leave out is this entry's"})
			}
		}

		out = append(out, BatchLine{Num: num, Entry: e})
	}
	if err := sc.Err(); err != nil {
		ps = append(ps, Problem{Line: num + 1, Message: err.Error()})
	}
	return out, ps
}

// batchDecodeProblem turns a json error into the problem a caller can act on: the
// line, the key, and what to do about it.
//
// The key is the part that has to survive. A batch is refused whole and the caller's
// next move is to fix one line and run again, so a refusal that names no key sends
// them back to re-read a file the tool has already read — and Raw is truncated at
// 120 characters, which on a line carrying a multi-paragraph body ends long before
// the key it is meant to show.
//
// Two shapes of error arrive here and each carries the key somewhere different: the
// unknown-field error has it in its text, where json's own wording also names a Go
// type nobody reading a JSONL file has in front of them; a type error has it in a
// field, which is why StringList refuses with one of those.
func batchDecodeProblem(num int, raw string, err error) Problem {
	p := Problem{Line: num, Raw: truncate(raw, 120)}

	msg := err.Error()
	if i := strings.Index(msg, "unknown field "); i >= 0 {
		p.Message = fmt.Sprintf("unknown field %s; a batch line carries kind, title, body, slug, about, anchor, plan and edges, "+
			"and never the fields the tool stamps (id, at, version, scope, origin)", msg[i+len("unknown field "):])
		return p
	}

	var typeErr *json.UnmarshalTypeError
	if errors.As(err, &typeErr) {
		p.Field = typeErr.Field
		if typeErr.Type == stringListType {
			if key := wrongShapedListKey(raw); key != "" {
				p.Field = key
			}
			if typeErr.Value == "array" {
				p.Message = "is an array with something other than a string in it; every element has to be a string"
			} else {
				p.Message = fmt.Sprintf("must be a string or an array of strings, and this line carries %s", typeErr.Value)
			}
			return p
		}
	}

	p.Message = msg
	return p
}

func wrongShapedListKey(raw string) string {
	dec := json.NewDecoder(strings.NewReader(raw))
	if _, err := dec.Token(); err != nil {
		return ""
	}
	for dec.More() {
		tok, err := dec.Token()
		if err != nil {
			return ""
		}
		key, _ := tok.(string)
		var value json.RawMessage
		if err := dec.Decode(&value); err != nil {
			return ""
		}
		if key != "about" && key != "anchor" {
			continue
		}
		var l StringList
		if err := json.Unmarshal(value, &l); err != nil {
			return key
		}
	}
	return ""
}

// splitTags expands the comma-separated form of `about` so that ["auth,pkg/auth/"]
// and ["auth", "pkg/auth/"] are one thing, and drops what is left blank — the
// same treatment splitList gives the flag.
func splitTags(in StringList) StringList {
	out := make(StringList, 0, len(in))
	for _, s := range in {
		for _, part := range strings.Split(s, ",") {
			if part = strings.TrimSpace(part); part != "" {
				out = append(out, part)
			}
		}
	}
	return out
}
