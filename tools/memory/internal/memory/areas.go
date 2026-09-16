package memory

import (
	"sort"
	"strings"
)

// Area is one distinct `--about` tag a log carries, with the little a caller
// needs to decide whether to ask the expensive question.
//
// It exists because `--about` is the only carrier of a preference's identity and
// it is freeform: a reader who guesses the tag wrong gets `count: 0`, which is
// byte-identical to the answer meaning nobody ever wrote one. Enumerating the
// tags is what makes those two distinguishable.
type Area struct {
	Name string `json:"area"`
	// Records is every record carrying the tag, settled ones included. Counting
	// only the live one would hide that an area has been answered three times,
	// which is exactly the history a reader about to answer it a fourth time
	// wants to see.
	Records int `json:"records"`
	// Live is nil when every record carrying the tag is settled — an area that
	// was answered and whose answer has since been retired, which is a different
	// thing from an area nobody ever wrote about.
	Live *AreaLive `json:"live"`
}

// AreaLive is the record that currently answers for an area: the most recent one
// carrying the tag that survives `--current`.
//
// No body. This command is the cheap read that tells a caller how to narrow the
// expensive one, and a row that carried the answer would be the expensive one.
type AreaLive struct {
	ID    string `json:"id"`
	Kind  Kind   `json:"kind"`
	Slug  string `json:"slug"`
	Title string `json:"title"`
	At    string `json:"at"`
	// Key says the area is this record's *first* `--about` tag. That is the
	// convention a `preference` is keyed by, and it is reported rather than
	// enforced: a tag in second position is enumerated like any other, so a
	// preference tagged `migrations,db` stays findable under `db` while only
	// `migrations` claims to be what it is about.
	Key bool `json:"key"`
}

// Areas enumerates the distinct tags the filtered records carry.
//
// The filtering is Query's, called twice rather than reimplemented once: the
// plan-less rule ("a record with no plan is the repository's rather than
// nobody's") is stated there, and a second reading of it would be a second thing
// to keep in step. What `--current` means is no longer one of those things.
// Query is QueryIn(lines, lines), so what settles a record here is this log's
// edges, where query, search and anchors now read the store's. The first pass
// counts, the second decides what is live — the two differ only in CurrentOnly,
// and they are separate passes because the count is deliberately over the
// settled records too.
//
// The caller's Limit, Threshold and CurrentOnly are overridden. A partial
// enumeration is not an enumeration, and the count-before-dump threshold guards
// against spending a caller's context on record bodies, which this returns none
// of.
func Areas(lines []Line, f Filter) []Area {
	f.Limit, f.Threshold = 0, -1

	all, live := f, f
	all.CurrentOnly = false
	live.CurrentOnly = true

	counts := map[string]int{}
	for _, e := range Query(lines, all).Records {
		for _, tag := range tagsOf(e) {
			counts[tag]++
		}
	}

	// Query returns most recent first, so the first record met carrying a tag is
	// the live answer for it.
	answers := map[string]*Entry{}
	for _, e := range Query(lines, live).Records {
		for _, tag := range tagsOf(e) {
			if _, taken := answers[tag]; !taken {
				answers[tag] = e
			}
		}
	}

	out := make([]Area, 0, len(counts))
	for tag, n := range counts {
		a := Area{Name: tag, Records: n}
		if e := answers[tag]; e != nil {
			a.Live = &AreaLive{
				ID: e.ID, Kind: e.Kind, Slug: e.Slug, Title: e.Title, At: e.At,
				Key: keyOf(e) == tag,
			}
		}
		out = append(out, a)
	}
	// By name, so two runs over one log agree. By count would reorder the whole
	// list on every write, and this is a read whose output a caller compares.
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// tagsOf is one record's tags as areas: normalized, and deduplicated so a record
// tagged twice with the same thing is still one record under it.
func tagsOf(e *Entry) []string {
	out := make([]string, 0, len(e.About))
	seen := map[string]bool{}
	for _, a := range e.About {
		t := normalizeTag(a)
		if t == "" || seen[t] {
			continue
		}
		seen[t] = true
		out = append(out, t)
	}
	return out
}

// keyOf is the tag a record claims to be about — its first — or "" when it has
// none that survives normalization.
func keyOf(e *Entry) string {
	for _, a := range e.About {
		if t := normalizeTag(a); t != "" {
			return t
		}
	}
	return ""
}

// normalizeTag lower-cases a tag, and does nothing else to it.
//
// Case is folded because matchesAbout folds it: `--about Persistence` and
// `--about persistence` are one tag to every query, so listing them as two areas
// with one record each would contradict the count `query --about persistence`
// returns. The folded spelling is what is reported, since one of the two had to
// be and the reader is going to type it back into a flag that does not care.
//
// **Nothing else is normalized, and surrounding whitespace least of all.**
// matchesAbout compares the stored tag as it is, so two tags differing only in
// padding match separately and are honestly two areas; trimming here would merge
// them into a row whose count no single query returns.
//
// It does not follow that the reported name is queryable. Against a hand-edited
// log carrying "  padded-tag  ", neither `query --about '  padded-tag  '` nor
// `query --about padded-tag` returns anything: splitList trims the wanted side
// while the stored side keeps its padding, so the two never meet. Dropping the
// trim here moved that mismatch rather than fixing it — what it bought is a row
// that says what is on disk instead of a tidied spelling that is no more
// queryable. Only a hand-edited log reaches this state, since splitList trims
// what `add` writes as well. Open question `01KZHJ6EG06DVHXX2ZCMW3EWX8` carries
// which side should eventually trim; do not resolve it here, because both sides
// belong to `query`'s behaviour and this command may not change that.
func normalizeTag(tag string) string {
	return strings.ToLower(tag)
}
