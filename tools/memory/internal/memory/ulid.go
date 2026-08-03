package memory

import (
	"crypto/rand"
	"fmt"
	"io"
	"time"
)

// Crockford base32, the ULID alphabet. Excludes I, L, O and U so a human
// reading an id out of the log cannot confuse it with 1, 0 or V.
const ulidAlphabet = "0123456789ABCDEFGHJKMNPQRSTVWXYZ"

var ulidDecode = func() [256]int8 {
	var t [256]int8
	for i := range t {
		t[i] = -1
	}
	for i := 0; i < len(ulidAlphabet); i++ {
		t[ulidAlphabet[i]] = int8(i)
	}
	return t
}()

// NewULID returns a 26-character ULID: 48 bits of millisecond timestamp
// followed by 80 bits of randomness.
//
// The timestamp prefix makes ids sort chronologically, which is what lets the
// log be read in order without a separate index. The random suffix is what
// makes two people working in two repos unable to collide — the property that
// removes any need to rewrite references when a record is promoted between
// scopes.
func NewULID(t time.Time) (string, error) {
	return newULID(t, rand.Reader)
}

func newULID(t time.Time, entropy io.Reader) (string, error) {
	var b [16]byte
	ms := uint64(t.UTC().UnixMilli())
	b[0] = byte(ms >> 40)
	b[1] = byte(ms >> 32)
	b[2] = byte(ms >> 24)
	b[3] = byte(ms >> 16)
	b[4] = byte(ms >> 8)
	b[5] = byte(ms)
	if _, err := io.ReadFull(entropy, b[6:]); err != nil {
		return "", fmt.Errorf("read entropy: %w", err)
	}
	return encodeULID(b), nil
}

// encodeULID writes the 128 bits as 26 base32 characters. 26*5 = 130, so the
// value is left-padded with two zero bits; that is why a valid ULID never
// starts above '7'.
func encodeULID(b [16]byte) string {
	out := make([]byte, 0, 26)
	var acc uint32
	bits := uint(2) // the two-bit left pad
	for _, c := range b {
		acc = acc<<8 | uint32(c)
		bits += 8
		for bits >= 5 {
			bits -= 5
			out = append(out, ulidAlphabet[(acc>>bits)&31])
		}
	}
	return string(out)
}

// ValidULID reports whether s is a syntactically well-formed ULID.
func ValidULID(s string) bool {
	if len(s) != 26 {
		return false
	}
	// The first character carries only two significant bits.
	if ulidDecode[s[0]] < 0 || ulidDecode[s[0]] > 7 {
		return false
	}
	for i := 1; i < len(s); i++ {
		if ulidDecode[s[i]] < 0 {
			return false
		}
	}
	return true
}
