package memory

import (
	"bytes"
	"strings"
	"testing"
	"time"
)

func TestNewULIDShape(t *testing.T) {
	id, err := NewULID(time.Now())
	if err != nil {
		t.Fatalf("NewULID: %v", err)
	}
	if len(id) != 26 {
		t.Fatalf("want 26 chars, got %d (%q)", len(id), id)
	}
	if !ValidULID(id) {
		t.Fatalf("%q failed its own validator", id)
	}
	for _, c := range id {
		if !strings.ContainsRune(ulidAlphabet, c) {
			t.Fatalf("%q contains %q, outside the Crockford alphabet", id, c)
		}
	}
}

// Sorting a ULID lexically must sort it chronologically — the whole retrieval
// path relies on this instead of maintaining an index.
func TestULIDSortsByTime(t *testing.T) {
	base := time.Date(2026, 8, 1, 12, 0, 0, 0, time.UTC)
	entropy := bytes.NewReader(bytes.Repeat([]byte{0xFF}, 10))
	early, err := newULID(base, entropy)
	if err != nil {
		t.Fatal(err)
	}
	// Later timestamp, minimal entropy: it must still sort after.
	entropy = bytes.NewReader(make([]byte, 10))
	late, err := newULID(base.Add(time.Second), entropy)
	if err != nil {
		t.Fatal(err)
	}
	if !(early < late) {
		t.Fatalf("expected %q < %q", early, late)
	}
}

func TestULIDDistinct(t *testing.T) {
	seen := map[string]bool{}
	now := time.Now()
	for i := 0; i < 1000; i++ {
		id, err := NewULID(now) // same millisecond on purpose
		if err != nil {
			t.Fatal(err)
		}
		if seen[id] {
			t.Fatalf("duplicate id %q within one millisecond", id)
		}
		seen[id] = true
	}
}

func TestValidULIDRejects(t *testing.T) {
	cases := map[string]string{
		"too short":       "01JXKQ7M4T8YZP",
		"too long":        "01JXKQ7M4T8YZP01JXKQ7M4T8YZP",
		"letter I":        "0IJXKQ7M4T8YZPABCDEFGHJKMN",
		"letter U":        "0UJXKQ7M4T8YZPABCDEFGHJKMN",
		"overflow prefix": "ZZJXKQ7M4T8YZPABCDEFGHJKMN",
		"empty":           "",
	}
	for name, in := range cases {
		if ValidULID(in) {
			t.Errorf("%s: %q should be rejected", name, in)
		}
	}
}
