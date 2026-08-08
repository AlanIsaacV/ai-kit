package memory

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// Location resolves to one memory log on disk.
type Location struct {
	Scope Scope
	Repo  string // absolute repo root, empty for ScopeUser
	Path  string
}

// ErrPlanScopeRemoved is returned for `--scope plan`.
//
// It is a named refusal rather than a generic "unknown scope" because the value
// was legal until schema version 2, and the scripts, habits and older binaries
// that still pass it are all real. "Unknown" would read as a typo when the truth
// is that the scope was removed and its records now live in the repo log.
var ErrPlanScopeRemoved = errors.New(`the "plan" scope was removed in schema version 2: ` +
	`there is one log per repo at <repo>/.ai-kit/memory.jsonl, and --plan is now a filter ` +
	`over origin.plan rather than a choice of file`)

// Resolve locates the log for a scope.
//
//	repo → <repo>/.ai-kit/memory.jsonl   everything this repo's work has produced
//	user → ~/.ai-kit/memory.jsonl        distilled across repos, promoted deliberately
//
// Both keep themselves out of git, and nothing outside the .ai-kit/ directory is
// touched to arrange it: whichever command resolves a location first creates the
// directory with a .gitignore containing `*` inside it (Store.ensureDir), which
// covers the log and everything the loop later writes alongside it. These are
// files the assistant writes for itself, and they have no business adding noise
// to a pull request — but the repository's own .gitignore is not this tool's to
// edit either.
//
// There used to be a third — `plan → <repo>/plans/<slug>/memory.jsonl` — and
// removing it is what schema version 2 is. One file per plan was not a tidier
// arrangement of the same graph, it was a smaller graph: an edge needs both of
// its endpoints in the file it is written to, so nothing in one plan's log could
// ever point at anything in another's. The plan a record belongs to moved out of
// the filesystem and into `origin.plan`, where it bounds a query instead of
// walling off a file.
func Resolve(scope Scope, startDir string) (Location, error) {
	loc := Location{Scope: scope}

	if scope == ScopeUser {
		home, err := os.UserHomeDir()
		if err != nil {
			return loc, fmt.Errorf("resolve home directory: %w", err)
		}
		loc.Path = filepath.Join(home, ".ai-kit", "memory.jsonl")
		return loc, nil
	}

	root, err := RepoRoot(startDir)
	if err != nil {
		return loc, err
	}
	loc.Repo = root

	switch scope {
	case ScopeRepo:
		loc.Path = filepath.Join(root, ".ai-kit", "memory.jsonl")
	case ScopePlan:
		return loc, ErrPlanScopeRemoved
	default:
		return loc, fmt.Errorf("unknown scope %q; valid: %s", scope, joinScopes(Scopes()))
	}
	return loc, nil
}

// PlanSlug is the plan a command names: the flag when it carries something,
// otherwise AI_KIT_PLAN.
//
// An empty result means no plan was named at all, and both paths now read that
// the same way: a query reaches across every objective in the log, and a write
// records something the repository owns rather than something one objective
// produced. There used to be a RequirePlan beside this that turned the write case
// into a refusal, and removing it is what made a `preference` writable — nothing
// but the CLI ever required a plan, since no validator in any version looks at
// origin.plan at all.
func PlanSlug(plan string) (string, error) {
	if plan == "" {
		plan = os.Getenv("AI_KIT_PLAN")
	}
	if plan == "" {
		return "", nil
	}
	if !slugRE.MatchString(plan) {
		return "", fmt.Errorf("plan %q must be kebab-case: lowercase letters, digits and hyphens", plan)
	}
	return plan, nil
}

// RepoRoot walks up from dir looking for a .git entry.
func RepoRoot(dir string) (string, error) {
	if dir == "" {
		wd, err := os.Getwd()
		if err != nil {
			return "", err
		}
		dir = wd
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(abs, ".git")); err == nil {
			return abs, nil
		}
		parent := filepath.Dir(abs)
		if parent == abs {
			return "", fmt.Errorf("not inside a git repository (searched upward from %s)", dir)
		}
		abs = parent
	}
}
