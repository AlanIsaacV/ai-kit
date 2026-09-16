package memory

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Verdict is what a check says about one anchor. It is exactly the vocabulary
// `agents/explore.md` states for a revalidated finding, and deliberately not one
// value wider: text that moved is `still_holds` reporting the line it is at now,
// because the cited content is still there. A fourth verdict `moved` would give
// the system two words for one idea and two places to keep them in step.
type Verdict string

const (
	VerdictStillHolds    Verdict = "still_holds"
	VerdictChanged       Verdict = "changed"
	VerdictCouldNotCheck Verdict = "could_not_check"
)

// hashAlgo is the digest an anchor written by this binary carries, named in the
// value itself; see Anchor.Hash.
const hashAlgo = "sha256"

// HashLines is the fingerprint an anchor stores: the cited lines joined by a
// newline and hashed exactly as they are.
//
// Nothing is normalized — not whitespace, not case, not indentation — and that
// is the whole design rather than an omission. A reformat that changes no
// behaviour therefore reports `changed`, and that false positive was chosen with
// its cost known: it costs a human one glance, where the tolerance that would
// have suppressed it also suppresses the real rewrites this exists to catch. A
// false `still_holds` is worse than having no record at all.
func HashLines(lines []string) string {
	sum := sha256.Sum256([]byte(strings.Join(lines, "\n")))
	return hashAlgo + ":" + hex.EncodeToString(sum[:])
}

// SplitLines cuts file content the way an editor numbers it: the first line is
// line 1, and a trailing newline ends the last line rather than starting an
// empty one.
//
// A CRLF file keeps its carriage returns inside the lines, so its anchors hash
// differently from the same text with unix endings. That follows from hashing
// the bytes as they are and is not worth an exception: a repo that changes its
// line endings has changed every line in the file.
func SplitLines(content []byte) []string {
	if len(content) == 0 {
		return nil
	}
	return strings.Split(strings.TrimSuffix(string(content), "\n"), "\n")
}

// ParseAnchorSpec reads the citation forms this system already writes in prose —
// `path:line` and `path:first-last` — so a caller passes the anchor exactly as a
// finding states it. The returned height counts both endpoints.
func ParseAnchorSpec(spec string) (path string, line, height int, err error) {
	spec = strings.TrimSpace(spec)
	// From the right, so a path that contains a colon does not lose its tail.
	i := strings.LastIndex(spec, ":")
	if i <= 0 || i == len(spec)-1 {
		return "", 0, 0, fmt.Errorf("anchor %q must be path:line or path:first-last", spec)
	}
	path, lines := spec[:i], spec[i+1:]

	first, last := lines, lines
	if j := strings.Index(lines, "-"); j >= 0 {
		first, last = lines[:j], lines[j+1:]
	}
	f, ferr := strconv.Atoi(first)
	l, lerr := strconv.Atoi(last)
	if ferr != nil || lerr != nil {
		return "", 0, 0, fmt.Errorf("anchor %q must be path:line or path:first-last", spec)
	}
	if f < 1 || l < f {
		return "", 0, 0, fmt.Errorf("anchor %q: lines are 1-based and the range must not run backwards", spec)
	}
	return path, f, l - f + 1, nil
}

// NewAnchor fingerprints the cited lines as they are in repoRoot right now.
//
// This is the write path, and it refuses rather than recording what it could not
// read: an anchor built from a path that does not exist or a range past the end
// of the file is a typo at the moment of writing, and the log is append-only, so
// here is the only place to catch it. Left to the check it would surface months
// later as a `could_not_check` indistinguishable from a file somebody deleted.
func NewAnchor(repoRoot, path string, line, height int) (Anchor, error) {
	rel, err := repoRelative(repoRoot, path)
	if err != nil {
		return Anchor{}, err
	}
	if line < 1 || height < 1 {
		return Anchor{}, fmt.Errorf("anchor %s: line and height are 1-based", rel)
	}
	content, err := os.ReadFile(filepath.Join(repoRoot, filepath.FromSlash(rel)))
	if err != nil {
		return Anchor{}, fmt.Errorf("anchor %s: %w", rel, err)
	}
	lines := SplitLines(content)
	if line+height-1 > len(lines) {
		return Anchor{}, fmt.Errorf("anchor %s:%d-%d: the file has %d lines",
			rel, line, line+height-1, len(lines))
	}
	return Anchor{
		Path:  rel,
		Line:  line,
		Lines: height,
		Hash:  HashLines(lines[line-1 : line-1+height]),
	}, nil
}

// AnchorCheck is one anchor's verdict and what the check saw to reach it.
type AnchorCheck struct {
	Anchor
	Verdict Verdict `json:"verdict"`
	// Matches is how many windows in the file hash to the stored digest, and it
	// is stated rather than resolved: text that appears twice is `still_holds`
	// with both lines named, because guessing which occurrence the record meant
	// is the one thing a mechanical check must not do.
	Matches int    `json:"matches"`
	FoundAt []int  `json:"found_at,omitempty"`
	Moved   bool   `json:"moved,omitempty"`
	Note    string `json:"note,omitempty"`
}

// RecordCheck is one record's anchors and the single verdict they add up to.
type RecordCheck struct {
	ID       string        `json:"id"`
	Kind     Kind          `json:"kind"`
	Slug     string        `json:"slug"`
	Title    string        `json:"title"`
	RepoRoot string        `json:"repo_root,omitempty"`
	Verdict  Verdict       `json:"verdict"`
	Anchors  []AnchorCheck `json:"anchors"`
}

// CheckAnchor says whether the cited text is still in the file, and where.
//
// Read-only, no network, no model, no git: it opens one file and hashes every
// window of the cited height in it. Movement therefore costs nothing to detect
// and needs no history, which matters because the repo this is pointed at may be
// a shallow clone or not a git repo at all.
func CheckAnchor(repoRoot string, a Anchor) AnchorCheck {
	out := AnchorCheck{Anchor: a}
	cannot := func(format string, args ...any) AnchorCheck {
		out.Verdict, out.Note = VerdictCouldNotCheck, fmt.Sprintf(format, args...)
		return out
	}

	if a.Line < 1 || a.Lines < 1 {
		return cannot("the anchor names line %d over %d lines, which is not a range", a.Line, a.Lines)
	}
	algo, _, ok := strings.Cut(a.Hash, ":")
	if !ok || algo != hashAlgo {
		return cannot("the anchor's hash is %q; this binary computes %s and cannot compare the two", a.Hash, hashAlgo)
	}
	rel, err := repoRelative(repoRoot, a.Path)
	if err != nil {
		return cannot("%s", err.Error())
	}
	content, err := os.ReadFile(filepath.Join(repoRoot, filepath.FromSlash(rel)))
	if err != nil {
		return cannot("%s", err.Error())
	}

	lines := SplitLines(content)
	for i := 0; i+a.Lines <= len(lines); i++ {
		if HashLines(lines[i:i+a.Lines]) == a.Hash {
			out.FoundAt = append(out.FoundAt, i+1)
		}
	}
	out.Matches = len(out.FoundAt)
	if out.Matches == 0 {
		out.Verdict = VerdictChanged
		return out
	}
	out.Verdict = VerdictStillHolds
	out.Moved = !containsInt(out.FoundAt, a.Line)
	return out
}

// CheckEntry checks every anchor on one record and rolls the verdicts into one.
//
// The record's verdict is its worst anchor's, ordered changed > could_not_check
// > still_holds. A record with one rewritten anchor is a record to look at
// whatever its others say, and one that is only partly checkable must not come
// back reading as checked.
func CheckEntry(repoRoot string, e *Entry) RecordCheck {
	out := RecordCheck{
		ID: e.ID, Kind: e.Kind, Slug: e.Slug, Title: e.Title,
		Verdict: VerdictStillHolds,
		Anchors: make([]AnchorCheck, 0, len(e.Anchors)),
	}
	for _, a := range e.Anchors {
		c := CheckAnchor(repoRoot, a)
		out.Anchors = append(out.Anchors, c)
		if severity(c.Verdict) > severity(out.Verdict) {
			out.Verdict = c.Verdict
		}
	}
	// A record with no anchors has nothing to have held, and saying `still_holds`
	// about it would be the laundering the third verdict exists to prevent.
	if len(e.Anchors) == 0 {
		out.Verdict = VerdictCouldNotCheck
	}
	return out
}

// CheckEntries checks the records that carry anchors and skips the rest, so a
// filter that admits unanchored records reports on what it could check rather
// than on how many records it read past.
func CheckEntries(repoRoot string, entries []*Entry) []RecordCheck {
	out := make([]RecordCheck, 0, len(entries))
	for _, e := range entries {
		if len(e.Anchors) == 0 {
			continue
		}
		out = append(out, CheckEntry(repoRoot, e))
	}
	return out
}

func CheckEntriesAt(locate func(*Entry) (root, reason string), entries []*Entry) []RecordCheck {
	out := make([]RecordCheck, 0, len(entries))
	for _, e := range entries {
		if len(e.Anchors) == 0 {
			continue
		}
		root, reason := locate(e)
		if root == "" {
			out = append(out, unlocatedEntry(e, reason))
			continue
		}
		c := CheckEntry(root, e)
		c.RepoRoot = root
		out = append(out, c)
	}
	return out
}

func unlocatedEntry(e *Entry, reason string) RecordCheck {
	out := RecordCheck{
		ID: e.ID, Kind: e.Kind, Slug: e.Slug, Title: e.Title,
		Verdict: VerdictCouldNotCheck,
		Anchors: make([]AnchorCheck, 0, len(e.Anchors)),
	}
	for _, a := range e.Anchors {
		out.Anchors = append(out.Anchors, AnchorCheck{Anchor: a, Verdict: VerdictCouldNotCheck, Note: reason})
	}
	return out
}

type CheckFilter struct {
	Verdict Verdict
	Path    string
}

func FilterChecks(checks []RecordCheck, f CheckFilter) []RecordCheck {
	path := strings.ToLower(strings.TrimSpace(f.Path))
	out := make([]RecordCheck, 0, len(checks))
	for _, c := range checks {
		if f.Verdict != "" && c.Verdict != f.Verdict {
			continue
		}
		if path != "" && !citesPath(c, path) {
			continue
		}
		out = append(out, c)
	}
	return out
}

func citesPath(c RecordCheck, pathLower string) bool {
	for _, a := range c.Anchors {
		if containsFold(a.Path, pathLower) {
			return true
		}
	}
	return false
}

// Verdicts is every verdict in a fixed order, so a report can carry all three
// counts whether or not any record produced them. A key that only appears when
// it is non-zero makes "nothing changed" and "the field is missing" look alike
// to whoever reads the JSON.
func Verdicts() []Verdict {
	return []Verdict{VerdictStillHolds, VerdictChanged, VerdictCouldNotCheck}
}

func severity(v Verdict) int {
	switch v {
	case VerdictChanged:
		return 2
	case VerdictCouldNotCheck:
		return 1
	default:
		return 0
	}
}

func containsInt(xs []int, want int) bool {
	for _, x := range xs {
		if x == want {
			return true
		}
	}
	return false
}

// repoRelative turns a path a caller typed into the repo-relative, slash-separated
// form an anchor stores, and refuses one that leaves the repo.
//
// A relative path is taken as repo-relative rather than as relative to the
// working directory: that is the form the anchor is stored in and the form every
// finding in this system is already written in, and resolving against the cwd
// would make the same flag mean different files from different directories.
func repoRelative(repoRoot, path string) (string, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return "", fmt.Errorf("an anchor needs a path")
	}
	if !filepath.IsAbs(path) {
		clean := filepath.ToSlash(filepath.Clean(path))
		if clean == ".." || strings.HasPrefix(clean, "../") {
			return "", fmt.Errorf("anchor path %q climbs out of the repository", path)
		}
		return clean, nil
	}
	if repoRoot == "" {
		return "", fmt.Errorf("anchor path %q is absolute and there is no repository to resolve it against", path)
	}
	if rel, ok := relativeTo(repoRoot, path); ok {
		return rel, nil
	}
	// Second try through the symlinks. A repo root and an absolute path can name
	// the same directory and share no prefix — macOS hands out temp directories
	// under /var, which is a link to /private/var — and a check that refused
	// those would refuse a correct path for the shape of the filesystem.
	realRoot, rootErr := filepath.EvalSymlinks(repoRoot)
	realPath, pathErr := filepath.EvalSymlinks(path)
	if rootErr == nil && pathErr == nil {
		if rel, ok := relativeTo(realRoot, realPath); ok {
			return rel, nil
		}
	}
	return "", fmt.Errorf("anchor path %q is outside the repository at %s", path, repoRoot)
}

func relativeTo(root, path string) (string, bool) {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return "", false
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", false
	}
	return filepath.ToSlash(rel), true
}
